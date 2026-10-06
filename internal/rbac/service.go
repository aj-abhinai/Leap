package rbac

import (
	"crm/internal/audit"
	"crm/internal/auth"
	"crm/internal/respond"
	"crm/internal/user"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

var (
	// ErrSelfDelete is returned when an actor targets their own account.
	ErrSelfDelete = errors.New("a user cannot delete their own account")

	// ErrLastSuperadminProtected is returned when an operation would leave no
	// superadmin in the system.
	ErrLastSuperadminProtected = errors.New("cannot remove the last superadmin")

	// ErrSelfRoleChange is returned when an actor tries to change their own role.
	ErrSelfRoleChange = errors.New("a user cannot change their own role")

	// ErrWildcardRestricted is returned when the wildcard permission is
	// assigned to a non-superadmin role.
	ErrWildcardRestricted = errors.New("the wildcard permission can only be assigned to the superadmin role")

	// ErrSuperadminAssignmentRestricted is returned when an actor who does not
	// hold the wildcard tries to assign a wildcard-carrying role to a user.
	ErrSuperadminAssignmentRestricted = errors.New("only a superadmin can assign the superadmin role")

	// ErrSuperadminRoleProtected is returned when an operation would delete,
	// rename, or strip the wildcard permission from the superadmin role.
	ErrSuperadminRoleProtected = errors.New("the superadmin role is protected")

	// ErrLastManagerProtected is returned when an operation would leave no
	// user able to manage roles and users.
	ErrLastManagerProtected = errors.New("cannot remove the last user who can manage roles and users")

	// ErrRoleInUse is returned when deleting a role that active users still
	// hold; deleting it would silently strip those users of their role.
	ErrRoleInUse = errors.New("role is assigned to one or more users")

	// ErrSystemRoleProtected is returned when deleting or renaming a seeded
	// role (is_system). System roles are permanent by name; their description
	// and permissions stay editable.
	ErrSystemRoleProtected = errors.New("system roles are permanent and cannot be renamed or deleted")

	// ErrSelfReactivate is returned when an actor targets their own account for
	// reactivation; a deactivated user must not undo their own deactivation.
	ErrSelfReactivate = errors.New("a user cannot reactivate their own account")

	// ErrDeactivatedActor is returned when an actor whose account is
	// deactivated tries a privileged RBAC mutation within their access-token
	// TTL; deactivation must be the end of an account's reach.
	ErrDeactivatedActor = errors.New("a deactivated user cannot perform this action")

	// ErrNotFound marks mutations targeting a role or user that does not
	// exist.
	ErrNotFound = errors.New("role or user not found")

	// ErrDuplicate marks name collisions on uniquely named resources.
	ErrDuplicate = errors.New("name already in use")

	// ErrInvalidPermission is returned when a bulk permission set references
	// a permission that does not exist in the catalog.
	ErrInvalidPermission = errors.New("one or more permissions do not exist")
)

type Service struct {
	db         *sql.DB
	bcryptCost int
}

// defaultBcryptCost mirrors the auth default for deployments that do not
// thread the configured cost through.
const defaultBcryptCost = 12

// Option configures a Service.
type Option func(*Service)

// WithBcryptCost sets the password hashing cost for admin-created and
// admin-reset passwords, mirroring auth.bcrypt_cost for the self-service
// paths. A non-positive cost keeps the default.
func WithBcryptCost(cost int) Option {
	return func(s *Service) {
		if cost > 0 {
			s.bcryptCost = cost
		}
	}
}

func NewService(db *sql.DB, opts ...Option) *Service {
	s := &Service{db: db, bcryptCost: defaultBcryptCost}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// auditChange records a best-effort audit entry with a human-readable
// description that names the changed entity, never a bare id.
func (s *Service) auditChange(description, resourceType, resourceID, action, changes, userID string) {
	audit.LogCustom(s.db, description, resourceType, resourceID, action, changes, userID)
}

// nullName maps an empty role name to JSON null so an absent role renders as
// null rather than "" in the audit changes.
func nullName(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *Service) listPermissions() ([]Permission, error) {
	rows, err := s.db.Query(`SELECT id, name, COALESCE(description, ''), created_at FROM permissions ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	defer rows.Close()
	perms := []Permission{}
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("list permissions: scan: %w", err)
		}
		perms = append(perms, p)
	}
	return perms, nil
}

func (s *Service) listRoles() ([]Role, error) {
	rows, err := s.db.Query(`SELECT id, name, COALESCE(description, ''), is_system, created_at, updated_at FROM roles ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()
	roles := []Role{}
	roleIDs := []string{}
	for rows.Next() {
		var r Role
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.IsSystem, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, r)
		roleIDs = append(roleIDs, r.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(roleIDs) == 0 {
		return roles, nil
	}
	permMap, err := s.getRolePermissionsBatch(roleIDs)
	if err != nil {
		return nil, err
	}
	for i := range roles {
		roles[i].Permissions = permMap[roles[i].ID]
	}
	return roles, nil
}

func (s *Service) getRolePermissionsBatch(roleIDs []string) (map[string][]Permission, error) {
	rows, err := s.db.Query(`
		SELECT rp.role_id, p.id, p.name, COALESCE(p.description, ''), p.created_at
		FROM role_permissions rp
		JOIN permissions p ON p.id = rp.permission_id
		WHERE rp.role_id = ANY($1)
		ORDER BY p.name
	`, roleIDs)
	if err != nil {
		return nil, fmt.Errorf("get role permissions batch: %w", err)
	}
	defer rows.Close()
	permMap := map[string][]Permission{}
	for rows.Next() {
		var roleID string
		var p Permission
		if err := rows.Scan(&roleID, &p.ID, &p.Name, &p.Description, &p.CreatedAt); err != nil {
			return nil, err
		}
		permMap[roleID] = append(permMap[roleID], p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return permMap, nil
}

func (s *Service) createRole(req CreateRoleRequest, actorID string) (*Role, error) {
	var r Role
	err := s.db.QueryRow(
		`INSERT INTO roles (name, description) VALUES ($1, $2)
		RETURNING id, name, COALESCE(description, ''), is_system, created_at, updated_at`,
		req.Name, req.Description,
	).Scan(&r.ID, &r.Name, &r.Description, &r.IsSystem, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if respond.IsDuplicate(err) {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("create role: %w", err)
	}
	changes, _ := json.Marshal(map[string]string{"name": r.Name, "description": r.Description})
	s.auditChange(fmt.Sprintf("Created role %q", r.Name), "role", r.ID, "create", string(changes), actorID)
	return &r, nil
}

func (s *Service) updateRole(id string, req UpdateRoleRequest, actorID string) (*Role, error) {
	if !validUUID(id) {
		return nil, ErrNotFound
	}
	var currentName, currentDesc string
	var isSystem bool
	if err := s.db.QueryRow(
		`SELECT name, COALESCE(description, ''), is_system FROM roles WHERE id = $1`,
		id,
	).Scan(&currentName, &currentDesc, &isSystem); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update role: %w", err)
	}
	if req.Name != nil && currentName == "superadmin" && *req.Name != "superadmin" {
		return nil, ErrSuperadminRoleProtected
	}
	// System roles are permanent by name: the boot seed keys its guarantee on
	// the name, so renaming one would let a fresh role with the original name
	// appear at the next boot. Description and permissions stay editable.
	if isSystem && req.Name != nil && *req.Name != currentName {
		return nil, ErrSystemRoleProtected
	}
	var r Role
	err := s.db.QueryRow(
		`UPDATE roles SET
			name = COALESCE(NULLIF($2, ''), name),
			description = COALESCE($3, description),
			updated_at = now()
		WHERE id = $1
		RETURNING id, name, COALESCE(description, ''), is_system, created_at, updated_at`,
		id,
		req.Name,
		req.Description,
	).Scan(&r.ID, &r.Name, &r.Description, &r.IsSystem, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update role: %w", err)
	}
	changes := map[string]any{}
	if currentName != r.Name {
		changes["name"] = map[string]string{"old": currentName, "new": r.Name}
	}
	if currentDesc != r.Description {
		changes["description"] = map[string]string{"old": currentDesc, "new": r.Description}
	}
	if len(changes) > 0 {
		if b, err := json.Marshal(changes); err == nil {
			s.auditChange(fmt.Sprintf("Updated role %q", r.Name), "role", r.ID, "update", string(b), actorID)
		}
	}
	return &r, nil
}

func (s *Service) deleteRole(id, actorID string) error {
	if !validUUID(id) {
		return ErrNotFound
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	defer tx.Rollback()

	if err := lockRBACMutations(tx); err != nil {
		return fmt.Errorf("delete role: %w", err)
	}

	var name string
	var isSystem bool
	err = tx.QueryRow(`SELECT name, is_system FROM roles WHERE id = $1`, id).Scan(&name, &isSystem)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	if name == "superadmin" {
		return ErrSuperadminRoleProtected
	}
	// Seeded roles are permanent: deleting one would let the boot seed
	// resurrect it (or, pre-flag, silently re-create it) on every restart.
	if isSystem {
		return ErrSystemRoleProtected
	}

	assigned, err := user.CountByRoles(tx, []string{id}, "")
	if err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	if assigned > 0 {
		return ErrRoleInUse
	}

	if _, err := tx.Exec(`DELETE FROM roles WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	changes, _ := json.Marshal(map[string]string{"name": name})
	s.auditChange(fmt.Sprintf("Deleted role %q", name), "role", id, "delete", string(changes), actorID)
	return nil
}

func (s *Service) assignPermission(roleID, permissionID, actorID string) error {
	if !validUUID(roleID) || !validUUID(permissionID) {
		return ErrNotFound
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("assign permission: %w", err)
	}
	defer tx.Rollback()

	if err := lockRBACMutations(tx); err != nil {
		return fmt.Errorf("assign permission: %w", err)
	}

	var roleName, permName string
	if err := tx.QueryRow(`SELECT name FROM roles WHERE id = $1`, roleID).Scan(&roleName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("assign permission: lookup role: %w", err)
	}
	if err := tx.QueryRow(`SELECT name FROM permissions WHERE id = $1`, permissionID).Scan(&permName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("assign permission: lookup permission: %w", err)
	}
	if roleName != "superadmin" && permName == "*" {
		return ErrWildcardRestricted
	}
	res, err := tx.Exec(
		`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		roleID, permissionID,
	)
	if err != nil {
		if respond.IsForeignKeyViolation(err) {
			return ErrNotFound
		}
		return fmt.Errorf("assign permission: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("assign permission: rows affected: %w", err)
	}
	if affected == 0 {
		// Already granted: a no-op, not an event.
		return nil
	}
	if _, err := tx.Exec(`UPDATE roles SET updated_at = now() WHERE id = $1`, roleID); err != nil {
		return fmt.Errorf("assign permission: touch role: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("assign permission: %w", err)
	}
	changes, _ := json.Marshal(map[string]any{"permissions": map[string]any{"added": []string{permName}}})
	s.auditChange(fmt.Sprintf("Granted %q to role %q", permName, roleName), "role", roleID, "update", string(changes), actorID)
	return nil
}

func (s *Service) removePermission(roleID, permissionID, actorID string) error {
	if !validUUID(roleID) || !validUUID(permissionID) {
		return ErrNotFound
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("remove permission: %w", err)
	}
	defer tx.Rollback()

	if err := lockRBACMutations(tx); err != nil {
		return fmt.Errorf("remove permission: %w", err)
	}

	var roleName, permName string
	if err := tx.QueryRow(`SELECT name FROM roles WHERE id = $1`, roleID).Scan(&roleName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("remove permission: lookup role: %w", err)
	}
	if err := tx.QueryRow(`SELECT name FROM permissions WHERE id = $1`, permissionID).Scan(&permName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("remove permission: lookup permission: %w", err)
	}
	if roleName == "superadmin" && permName == "*" {
		return ErrSuperadminRoleProtected
	}
	res, err := tx.Exec(
		`DELETE FROM role_permissions WHERE role_id = $1 AND permission_id = $2`,
		roleID, permissionID,
	)
	if err != nil {
		return fmt.Errorf("remove permission: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("remove permission: rows affected: %w", err)
	}
	if affected == 0 {
		// Not granted: a no-op, not an event.
		return nil
	}

	// Removing a permission that grants RBAC management must not orphan the
	// system: the transaction rolls back if no user could manage roles and
	// users afterwards.
	if permName == "settings:manage" {
		managers, err := countRBACManagers(tx)
		if err != nil {
			return fmt.Errorf("remove permission: %w", err)
		}
		if managers == 0 {
			return ErrLastManagerProtected
		}
	}
	if _, err := tx.Exec(`UPDATE roles SET updated_at = now() WHERE id = $1`, roleID); err != nil {
		return fmt.Errorf("remove permission: touch role: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("remove permission: %w", err)
	}
	changes, _ := json.Marshal(map[string]any{"permissions": map[string]any{"removed": []string{permName}}})
	s.auditChange(fmt.Sprintf("Removed %q from role %q", permName, roleName), "role", roleID, "update", string(changes), actorID)
	return nil
}

// setRolePermissions replaces the role's permission set atomically. It diffs
// the requested set against the current one, rejects unknown permission IDs,
// and enforces the existing protections (wildcard restrictions and the
// last-manager guard) under the RBAC mutation lock so concurrent changes
// cannot race the guards. An empty request clears the role's permissions.
func (s *Service) setRolePermissions(roleID string, permissionIDs []string, actorID string) (*Role, error) {
	if !validUUID(roleID) {
		return nil, ErrNotFound
	}
	requested := make([]string, 0, len(permissionIDs))
	seen := map[string]bool{}
	for _, pid := range permissionIDs {
		if !validUUID(pid) {
			return nil, ErrInvalidPermission
		}
		if !seen[pid] {
			seen[pid] = true
			requested = append(requested, pid)
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("set role permissions: %w", err)
	}
	defer tx.Rollback()

	if err := lockRBACMutations(tx); err != nil {
		return nil, fmt.Errorf("set role permissions: %w", err)
	}

	var roleName, description string
	var isSystem bool
	var createdAt, updatedAt time.Time
	if err := tx.QueryRow(
		`SELECT name, COALESCE(description, ''), is_system, created_at, updated_at FROM roles WHERE id = $1`,
		roleID,
	).Scan(&roleName, &description, &isSystem, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("set role permissions: lookup role: %w", err)
	}

	// Resolve the requested permission IDs to names, rejecting any that are
	// not in the catalog.
	permNameByID := map[string]string{}
	if len(requested) > 0 {
		rows, err := tx.Query(`SELECT id, name FROM permissions WHERE id = ANY($1)`, requested)
		if err != nil {
			return nil, fmt.Errorf("set role permissions: lookup permissions: %w", err)
		}
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				rows.Close()
				return nil, fmt.Errorf("set role permissions: scan permission: %w", err)
			}
			permNameByID[id] = name
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("set role permissions: iterate permissions: %w", err)
		}
		rows.Close()
	}
	for _, pid := range requested {
		if _, ok := permNameByID[pid]; !ok {
			return nil, ErrInvalidPermission
		}
	}

	// Current permission IDs and names for the role.
	currentNameByID := map[string]string{}
	{
		rows, err := tx.Query(`
			SELECT p.id, p.name FROM permissions p
			JOIN role_permissions rp ON p.id = rp.permission_id
			WHERE rp.role_id = $1`, roleID)
		if err != nil {
			return nil, fmt.Errorf("set role permissions: load current: %w", err)
		}
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				rows.Close()
				return nil, fmt.Errorf("set role permissions: scan current: %w", err)
			}
			currentNameByID[id] = name
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("set role permissions: iterate current: %w", err)
		}
		rows.Close()
	}

	// Diff requested against current.
	requestedSet := make(map[string]bool, len(requested))
	for _, pid := range requested {
		requestedSet[pid] = true
	}
	var addedIDs, removedIDs []string
	added := []string{}
	removed := []string{}
	for _, pid := range requested {
		if _, ok := currentNameByID[pid]; !ok {
			addedIDs = append(addedIDs, pid)
			added = append(added, permNameByID[pid])
		}
	}
	for id, name := range currentNameByID {
		if !requestedSet[id] {
			removedIDs = append(removedIDs, id)
			removed = append(removed, name)
		}
	}

	// Guards: the wildcard may only belong to the superadmin role, and the
	// superadmin role may never lose it.
	if roleName != "superadmin" {
		for _, n := range added {
			if n == "*" {
				return nil, ErrWildcardRestricted
			}
		}
	}
	if roleName == "superadmin" {
		for _, n := range removed {
			if n == "*" {
				return nil, ErrSuperadminRoleProtected
			}
		}
	}

	if len(removedIDs) > 0 {
		if _, err := tx.Exec(
			`DELETE FROM role_permissions WHERE role_id = $1 AND permission_id = ANY($2)`,
			roleID, removedIDs,
		); err != nil {
			return nil, fmt.Errorf("set role permissions: remove: %w", err)
		}
	}

	// Dropping settings:manage must not orphan the system; the transaction
	// rolls back if no user could manage roles and users afterwards.
	removesManage := false
	for _, n := range removed {
		if n == "settings:manage" {
			removesManage = true
			break
		}
	}
	if removesManage {
		managers, err := countRBACManagers(tx)
		if err != nil {
			return nil, fmt.Errorf("set role permissions: %w", err)
		}
		if managers == 0 {
			return nil, ErrLastManagerProtected
		}
	}

	if len(addedIDs) > 0 {
		for _, pid := range addedIDs {
			if _, err := tx.Exec(
				`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				roleID, pid,
			); err != nil {
				return nil, fmt.Errorf("set role permissions: add: %w", err)
			}
		}
	}
	if len(addedIDs) > 0 || len(removedIDs) > 0 {
		if _, err := tx.Exec(`UPDATE roles SET updated_at = now() WHERE id = $1`, roleID); err != nil {
			return nil, fmt.Errorf("set role permissions: touch role: %w", err)
		}
	}
	// Build the response inside the transaction: a read failure rolls the
	// write back instead of reporting an error after the change committed.
	perms, err := getRolePermissionsFrom(tx, roleID)
	if err != nil {
		return nil, fmt.Errorf("set role permissions: load response: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("set role permissions: %w", err)
	}

	if len(added) > 0 || len(removed) > 0 {
		changes, _ := json.Marshal(map[string]any{
			"permissions": map[string]any{"added": added, "removed": removed},
		})
		s.auditChange(fmt.Sprintf("Updated permissions for role %q", roleName), "role", roleID, "update", string(changes), actorID)
	}

	return &Role{
		ID:          roleID,
		Name:        roleName,
		Description: description,
		IsSystem:    isSystem,
		Permissions: perms,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func (s *Service) GetUserPermissions(userID string) ([]string, error) {
	// The row is read including soft-deleted users: an access token stays
	// valid until its short TTL expires.
	u, err := user.GetIncludingDeleted(s.db, userID)
	if errors.Is(err, user.ErrNotFound) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user permissions: %w", err)
	}
	if u.RoleID == nil {
		return []string{}, nil
	}
	rows, err := s.db.Query(`
		SELECT DISTINCT p.name FROM permissions p
		JOIN role_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = $1
	`, *u.RoleID)
	if err != nil {
		return nil, fmt.Errorf("get user permissions: %w", err)
	}
	defer rows.Close()
	perms := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("get user permissions: scan: %w", err)
		}
		if name == "*" {
			return []string{"*"}, nil
		}
		perms = append(perms, name)
	}
	return perms, nil
}

func (s *Service) getRolePermissions(roleID string) ([]Permission, error) {
	return getRolePermissionsFrom(s.db, roleID)
}

// rolePermissionQuerier is the read surface a role-permission load needs, so
// the set path can build its response inside the transaction instead of
// re-reading after commit. Both *sql.DB and *sql.Tx satisfy it.
type rolePermissionQuerier interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// getRolePermissionsFrom loads a role's permissions through q; a missing or
// malformed role is not-found.
func getRolePermissionsFrom(q rolePermissionQuerier, roleID string) ([]Permission, error) {
	if !validUUID(roleID) {
		return nil, ErrNotFound
	}
	var exists bool
	if err := q.QueryRow(`SELECT EXISTS(SELECT 1 FROM roles WHERE id = $1)`, roleID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("get role permissions: %w", err)
	}
	if !exists {
		return nil, ErrNotFound
	}
	rows, err := q.Query(`
		SELECT p.id, p.name, COALESCE(p.description, ''), p.created_at FROM permissions p
		JOIN role_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = $1 ORDER BY p.name
	`, roleID)
	if err != nil {
		return nil, fmt.Errorf("get role permissions: %w", err)
	}
	defer rows.Close()
	perms := []Permission{}
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("get role permissions: scan: %w", err)
		}
		perms = append(perms, p)
	}
	return perms, nil
}

// setUserRole sets the single role for a user. An empty roleID clears the
// role. Removing the last superadmin — or the last user able to manage roles
// and users — is blocked, whether the change clears the role or swaps it.
func (s *Service) setUserRole(userID, roleID, actorID string) error {
	if actorID != "" && actorID == userID {
		return ErrSelfRoleChange
	}
	if !validUUID(userID) || (roleID != "" && !validUUID(roleID)) {
		return ErrNotFound
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("set user role: %w", err)
	}
	defer tx.Rollback()

	if err := lockRBACMutations(tx); err != nil {
		return fmt.Errorf("set user role: %w", err)
	}

	target, err := user.Get(tx, userID)
	if errors.Is(err, user.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("set user role: %w", err)
	}

	cur, err := roleStatusForID(tx, userRoleID(target))
	if err != nil {
		return fmt.Errorf("set user role: %w", err)
	}
	next, err := roleStatusForID(tx, roleID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("set user role: %w", err)
	}

	// Only an actor who already holds the wildcard may assign a role that
	// carries it (the superadmin role); settings:manage alone must not mint
	// new superadmins.
	if next.hasWildcard {
		actorIsSuper, err := userHoldsWildcard(tx, actorID)
		if err != nil {
			return fmt.Errorf("set user role: %w", err)
		}
		if !actorIsSuper {
			return ErrSuperadminAssignmentRestricted
		}
	}

	// The last-of-kind protections apply to clears and swaps alike: losing
	// the final superadmin or the final RBAC manager locks the system out.
	if cur.isSuperadmin && !next.isSuperadmin {
		others, err := countSuperadmins(tx, userID)
		if err != nil {
			return fmt.Errorf("set user role: %w", err)
		}
		if others == 0 {
			return ErrLastSuperadminProtected
		}
	}
	if cur.canManage && !next.canManage {
		others, err := countOtherRBACManagers(tx, userID)
		if err != nil {
			return fmt.Errorf("set user role: %w", err)
		}
		if others == 0 {
			return ErrLastManagerProtected
		}
	}

	if nullName(cur.name) == nullName(next.name) {
		// Same role: a no-op, not an event.
		return nil
	}

	if err := user.SetRole(tx, userID, roleID); err != nil {
		return fmt.Errorf("set user role: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("set user role: %w", err)
	}
	changes, _ := json.Marshal(map[string]any{
		"role": map[string]any{"old": nullName(cur.name), "new": nullName(next.name)},
	})
	s.auditChange(fmt.Sprintf("Changed role for %q from %q to %q", target.Name, cur.name, next.name), "user", userID, "update", string(changes), actorID)
	return nil
}

// UserCan reports whether the user holds the permission, honoring the
// wildcard. Used by handlers that need conditional permission checks the
// route-level middleware cannot express.
func (s *Service) UserCan(userID, permission string) (bool, error) {
	perms, err := s.GetUserPermissions(userID)
	if err != nil {
		return false, err
	}
	for _, p := range perms {
		if p == "*" || p == permission {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) listUsers() ([]UserInfo, error) {
	users, err := user.List(s.db)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	roles, err := s.rolesByID()
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	totalSuperadmins, err := s.countSuperadminsAll()
	if err != nil {
		return nil, fmt.Errorf("list users: count superadmins: %w", err)
	}
	out := make([]UserInfo, 0, len(users))
	for _, u := range users {
		info := userInfo(u)
		if u.RoleID != nil {
			info.Role = roles[*u.RoleID]
		}
		info.Protected = totalSuperadmins == 1 && info.Role != nil && info.Role.Name == "superadmin"
		out = append(out, info)
	}
	return out, nil
}

// userInfo maps a user row to the Team listing shape; role data is merged by
// the caller.
func userInfo(u user.User) UserInfo {
	info := UserInfo{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		Phone:     u.Phone,
		Active:    u.DeletedAt == nil,
		CreatedAt: u.CreatedAt,
	}
	if u.AvatarURL != nil {
		info.AvatarURL = *u.AvatarURL
	}
	return info
}

// userAvatar returns the row's avatar URL, empty when unset.
func userAvatar(u *user.User) string {
	if u.AvatarURL == nil {
		return ""
	}
	return *u.AvatarURL
}

// rolesByID loads every role keyed by id. The user package never reads the
// roles table, so listings merge role data here.
func (s *Service) rolesByID() (map[string]*Role, error) {
	rows, err := s.db.Query(
		`SELECT id, name, COALESCE(description, ''), is_system, created_at, updated_at FROM roles`,
	)
	if err != nil {
		return nil, fmt.Errorf("load roles: %w", err)
	}
	defer rows.Close()
	roles := map[string]*Role{}
	for rows.Next() {
		var r Role
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.IsSystem, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("load roles: scan: %w", err)
		}
		role := r
		roles[r.ID] = &role
	}
	return roles, rows.Err()
}

// listAssigneeOptions returns the id+name of every live user for lead
// assignment pickers. It exposes no email/role/avatar so a lead:read caller
// can power the dropdown without admin user details.
func (s *Service) listAssigneeOptions() ([]AssigneeOption, error) {
	users, err := user.ListActive(s.db)
	if err != nil {
		return nil, fmt.Errorf("list assignee options: %w", err)
	}
	options := make([]AssigneeOption, 0, len(users))
	for _, u := range users {
		options = append(options, AssigneeOption{ID: u.ID, Name: u.Name})
	}
	return options, nil
}

// createUser creates a user with an optional role, forced to change the
// password at first login. Assigning a wildcard-carrying role (the superadmin
// role) requires the actor to already hold the wildcard — the same guard as
// setUserRole. The insert and role assignment commit in one transaction.
// createUser creates a user with an optional role, forced to change the
// password at first login. Assigning a wildcard-carrying role (the superadmin
// role) requires the actor to already hold the wildcard — the same guard as
// setUserRole. The insert and role assignment commit in one transaction.
func (s *Service) createUser(name, email, password, roleID, actorID string) (*UserInfo, error) {
	if roleID != "" && !validUUID(roleID) {
		return nil, ErrNotFound
	}
	if err := user.ValidatePassword(password); err != nil {
		return nil, err
	}
	hash, err := user.HashPassword(password, s.bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	defer tx.Rollback()

	if err := lockRBACMutations(tx); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	var roleName string
	if roleID != "" {
		next, err := roleStatusForID(tx, roleID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrNotFound
			}
			return nil, fmt.Errorf("create user: %w", err)
		}
		if next.hasWildcard {
			actorIsSuper, err := userHoldsWildcard(tx, actorID)
			if err != nil {
				return nil, fmt.Errorf("create user: %w", err)
			}
			if !actorIsSuper {
				return nil, ErrSuperadminAssignmentRestricted
			}
		}
		roleName = next.name
	}

	created, err := user.Create(tx, user.NewUser{
		Name:         name,
		Email:        email,
		PasswordHash: hash,
		RoleID:       roleID,
	})
	if err != nil {
		if errors.Is(err, user.ErrEmailTaken) {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	out := UserInfo{
		ID:        created.ID,
		Name:      created.Name,
		Email:     created.Email,
		AvatarURL: userAvatar(created),
		CreatedAt: created.CreatedAt,
	}
	if roleID != "" {
		// The response role mirrors listUsers' shape (is_system + timestamps),
		// not the minimal guard read. A role deleted concurrently after the
		// commit is reported as the id+name known to this call, so the user is
		// created either way (the 201 must not 500 after the commit).
		out.Role = &Role{ID: roleID, Name: roleName}
		err := s.db.QueryRow(
			`SELECT id, name, COALESCE(description, ''), is_system, created_at, updated_at FROM roles WHERE id = $1`,
			roleID,
		).Scan(&out.Role.ID, &out.Role.Name, &out.Role.Description, &out.Role.IsSystem, &out.Role.CreatedAt, &out.Role.UpdatedAt)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("create user: load role: %w", err)
		}
	}
	changes, _ := json.Marshal(map[string]any{"name": created.Name, "email": created.Email})
	if roleName != "" {
		changes, _ = json.Marshal(map[string]any{
			"name":  created.Name,
			"email": created.Email,
			"role":  map[string]string{"new": roleName},
		})
	}
	s.auditChange(fmt.Sprintf("Created user %q", created.Name), "user", created.ID, "create", string(changes), actorID)
	return &out, nil
}

func (s *Service) deleteUser(id, actorID string) error {
	if !validUUID(id) {
		return ErrNotFound
	}
	if actorID != "" && actorID == id {
		return ErrSelfDelete
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	defer tx.Rollback()

	if err := lockRBACMutations(tx); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	target, err := user.Get(tx, id)
	if errors.Is(err, user.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	st, err := roleStatusForID(tx, userRoleID(target))
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	if st.isSuperadmin {
		others, err := countSuperadmins(tx, id)
		if err != nil {
			return fmt.Errorf("delete user: %w", err)
		}
		if others == 0 {
			return ErrLastSuperadminProtected
		}
	}
	if st.canManage {
		others, err := countOtherRBACManagers(tx, id)
		if err != nil {
			return fmt.Errorf("delete user: %w", err)
		}
		if others == 0 {
			return ErrLastManagerProtected
		}
	}

	if err := user.Deactivate(tx, id); err != nil {
		return fmt.Errorf("soft-delete user: %w", err)
	}
	if err := auth.RevokeUserSessions(tx, id); err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	s.auditChange(fmt.Sprintf("Deactivated user %q", target.Name), "user", id, "delete", `{"action":"deleted"}`, actorID)
	return nil
}

// updateUser edits a live user's name, email, or phone. The field rules live
// in internal/user, so the admin path applies the same trimming, bounds, and
// canonicalization as the self-service path.
func (s *Service) updateUser(id string, req UpdateUserRequest, actorID string) (*UserInfo, error) {
	if !validUUID(id) {
		return nil, ErrNotFound
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	defer tx.Rollback()

	if err := lockRBACMutations(tx); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	before, err := user.Get(tx, id)
	if errors.Is(err, user.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	// A wildcard-carrying account is governed by the same rule as role
	// assignment: only a wildcard holder may touch it, so settings:manage
	// alone cannot seize or tamper with the owner account.
	target, err := roleStatusForID(tx, userRoleID(before))
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	if target.hasWildcard {
		actorIsSuper, err := userHoldsWildcard(tx, actorID)
		if err != nil {
			return nil, fmt.Errorf("update user: %w", err)
		}
		if !actorIsSuper {
			return nil, ErrSuperadminAssignmentRestricted
		}
	}

	after, err := user.Update(tx, id, user.Patch{Name: req.Name, Email: req.Email, Phone: req.Phone})
	if err != nil {
		if errors.Is(err, user.ErrEmailTaken) {
			return nil, ErrDuplicate
		}
		if errors.Is(err, user.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	changes := map[string]any{}
	if before.Name != after.Name {
		changes["name"] = map[string]string{"old": before.Name, "new": after.Name}
	}
	if before.Email != after.Email {
		changes["email"] = map[string]string{"old": before.Email, "new": after.Email}
	}
	if before.Phone != after.Phone {
		changes["phone"] = map[string]string{"old": before.Phone, "new": after.Phone}
	}
	if len(changes) > 0 {
		if b, err := json.Marshal(changes); err == nil {
			s.auditChange(fmt.Sprintf("Updated user %q", after.Name), "user", id, "update", string(b), actorID)
		}
	}
	out := userInfo(*after)
	return &out, nil
}

// resetPassword sets a temporary password for a user and forces a change at
// their next login, revoking every existing session like a password change
// does. The event is audit-logged as a security action.
func (s *Service) resetPassword(id, password, actorID string) error {
	if !validUUID(id) {
		return ErrNotFound
	}
	if err := user.ValidatePassword(password); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("reset password: %w", err)
	}
	defer tx.Rollback()

	if err := lockRBACMutations(tx); err != nil {
		return fmt.Errorf("reset password: %w", err)
	}

	target, err := user.Get(tx, id)
	if errors.Is(err, user.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("reset password: %w", err)
	}

	// Same rule as role assignment and user edits: only a wildcard holder may
	// reset a wildcard-carrying account.
	st, err := roleStatusForID(tx, userRoleID(target))
	if err != nil {
		return fmt.Errorf("reset password: %w", err)
	}
	if st.hasWildcard {
		actorIsSuper, err := userHoldsWildcard(tx, actorID)
		if err != nil {
			return fmt.Errorf("reset password: %w", err)
		}
		if !actorIsSuper {
			return ErrSuperadminAssignmentRestricted
		}
	}

	// The hash is computed after the guards so a refused reset costs no
	// bcrypt work.
	hash, err := user.HashPassword(password, s.bcryptCost)
	if err != nil {
		return fmt.Errorf("hash reset password: %w", err)
	}
	if err := user.ResetPassword(tx, id, hash); err != nil {
		return fmt.Errorf("reset password: %w", err)
	}
	if err := auth.RevokeUserSessions(tx, id); err != nil {
		return fmt.Errorf("reset password: revoke sessions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("reset password: %w", err)
	}
	s.auditChange(fmt.Sprintf("Reset password for user %q", target.Name), "user", id, "reset_password", `{"password":"reset"}`, actorID)
	return nil
}

// reactivateUser lifts a deactivated (soft-deleted) user's account. Their
// revoked sessions stay revoked; the next login issues a fresh pair. The
// user's data and role were never touched by the deactivation. A user cannot
// reactivate themselves (deactivation would be self-undoable within the
// access-token TTL), a deactivated actor cannot reactivate anyone (the undo
// tool must not serve the account it just disabled), and reactivating a
// wildcard-carrying account requires a wildcard holder, like every other
// touch of the owner account.
func (s *Service) reactivateUser(id, actorID string) (*UserInfo, error) {
	if !validUUID(id) {
		return nil, ErrNotFound
	}
	if actorID != "" && actorID == id {
		return nil, ErrSelfReactivate
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("reactivate user: %w", err)
	}
	defer tx.Rollback()

	if actorID != "" {
		if _, err := user.Get(tx, actorID); errors.Is(err, user.ErrNotFound) {
			return nil, ErrDeactivatedActor
		} else if err != nil {
			return nil, fmt.Errorf("reactivate user: %w", err)
		}
	}

	target, err := user.GetIncludingDeleted(tx, id)
	if errors.Is(err, user.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("reactivate user: %w", err)
	}
	if target.DeletedAt == nil {
		return nil, ErrNotFound
	}

	st, err := roleStatusForID(tx, userRoleID(target))
	if err != nil {
		return nil, fmt.Errorf("reactivate user: %w", err)
	}
	if st.hasWildcard {
		actorIsSuper, err := userHoldsWildcard(tx, actorID)
		if err != nil {
			return nil, fmt.Errorf("reactivate user: %w", err)
		}
		if !actorIsSuper {
			return nil, ErrSuperadminAssignmentRestricted
		}
	}

	if err := user.Reactivate(tx, id); err != nil {
		return nil, fmt.Errorf("reactivate user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("reactivate user: %w", err)
	}
	out := userInfo(*target)
	out.Active = true
	s.auditChange(fmt.Sprintf("Reactivated user %q", out.Name), "user", id, "reactivate", `{"action":"reactivated"}`, actorID)
	return &out, nil
}

// lockRBACMutations serializes mutations that can remove the last RBAC
// manager so concurrent deletions cannot both pass the count check.
func lockRBACMutations(tx *sql.Tx) error {
	const rbacMutationLockKey = 748391621
	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock($1)`, rbacMutationLockKey); err != nil {
		return fmt.Errorf("acquire rbac mutation lock: %w", err)
	}
	return nil
}

// validUUID reports whether s parses as a UUID, so malformed identifiers fail
// with a clean not-found error instead of a database 22P02 error.
func validUUID(s string) bool {
	var u pgtype.UUID
	return u.Scan(s) == nil
}

// roleStatus describes what a role confers for the purposes of the last-*
// protections.
type roleStatus struct {
	name         string // role name, or "" when none
	isSuperadmin bool   // the canonical superadmin role
	hasWildcard  bool   // carries '*', i.e. unconditional access
	canManage    bool   // could manage roles and users (settings:manage or *)
}

// roleStatusForID reports the status a role confers; an empty roleID confers
// none.
func roleStatusForID(tx *sql.Tx, roleID string) (roleStatus, error) {
	var st roleStatus
	if roleID == "" {
		return st, nil
	}
	err := tx.QueryRow(
		`SELECT r.name,
			r.name = 'superadmin',
			EXISTS(
				SELECT 1 FROM role_permissions rp
				JOIN permissions p ON p.id = rp.permission_id
				WHERE rp.role_id = r.id AND p.name = '*'
			),
			EXISTS(
				SELECT 1 FROM role_permissions rp
				JOIN permissions p ON p.id = rp.permission_id
				WHERE rp.role_id = r.id AND p.name IN ('settings:manage', '*')
			)
		FROM roles r WHERE r.id = $1`,
		roleID,
	).Scan(&st.name, &st.isSuperadmin, &st.hasWildcard, &st.canManage)
	return st, err
}

// userHoldsWildcard reports whether the user's role carries the wildcard
// permission. An empty userID (no authenticated actor) reports false.
func userHoldsWildcard(tx *sql.Tx, userID string) (bool, error) {
	if userID == "" {
		return false, nil
	}
	u, err := user.GetIncludingDeleted(tx, userID)
	if errors.Is(err, user.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	st, err := roleStatusForID(tx, userRoleID(u))
	if err != nil {
		return false, fmt.Errorf("load role status: %w", err)
	}
	return st.hasWildcard, nil
}

// userRoleID returns the user row's role id, or "" when none is set.
func userRoleID(u *user.User) string {
	if u.RoleID == nil {
		return ""
	}
	return *u.RoleID
}

// roleIDsByName returns the ids of the named roles.
func roleIDsByName(q rolePermissionQuerier, names ...string) ([]string, error) {
	rows, err := q.Query(`SELECT id FROM roles WHERE name = ANY($1)`, names)
	if err != nil {
		return nil, fmt.Errorf("load roles by name: %w", err)
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("load roles by name: scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// roleIDsWithPermission returns the ids of roles carrying any of the given
// permission names.
func roleIDsWithPermission(q rolePermissionQuerier, names ...string) ([]string, error) {
	rows, err := q.Query(
		`SELECT DISTINCT r.id FROM roles r
		JOIN role_permissions rp ON rp.role_id = r.id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE p.name = ANY($1)`,
		names,
	)
	if err != nil {
		return nil, fmt.Errorf("load roles by permission: %w", err)
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("load roles by permission: scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// countOtherRBACManagers counts users other than the excluded one who could
// manage roles and users.
func countOtherRBACManagers(tx *sql.Tx, excludeID string) (int, error) {
	roleIDs, err := roleIDsWithPermission(tx, "settings:manage", "*")
	if err != nil {
		return 0, err
	}
	return user.CountByRoles(tx, roleIDs, excludeID)
}

// countRBACManagers counts every non-deleted user who could manage roles and
// users.
func countRBACManagers(tx *sql.Tx) (int, error) {
	roleIDs, err := roleIDsWithPermission(tx, "settings:manage", "*")
	if err != nil {
		return 0, err
	}
	return user.CountByRoles(tx, roleIDs, "")
}

// countSuperadmins counts non-deleted users holding the superadmin role,
// excluding the given user.
func countSuperadmins(tx *sql.Tx, excludeID string) (int, error) {
	roleIDs, err := roleIDsByName(tx, "superadmin")
	if err != nil {
		return 0, err
	}
	return user.CountByRoles(tx, roleIDs, excludeID)
}

// countSuperadminsAll counts all non-deleted users holding the superadmin role.
func (s *Service) countSuperadminsAll() (int, error) {
	roleIDs, err := roleIDsByName(s.db, "superadmin")
	if err != nil {
		return 0, err
	}
	return user.CountByRoles(s.db, roleIDs, "")
}

type UserInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone,omitempty"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Role      *Role     `json:"role,omitempty"`
	Protected bool      `json:"protected"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}
