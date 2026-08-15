package rbac

import (
	"crm/internal/audit"
	"crm/internal/auth"
	"crm/internal/respond"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// logActivity records a best-effort audit entry for an RBAC mutation.
func (s *Service) logActivity(resourceID, resourceType, action, changes, userID string) {
	audit.Log(s.db, resourceID, resourceType, action, changes, userID)
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
	rows, err := s.db.Query(`SELECT id, name, COALESCE(description, ''), created_at, updated_at FROM roles ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()
	roles := []Role{}
	roleIDs := []string{}
	for rows.Next() {
		var r Role
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.CreatedAt, &r.UpdatedAt); err != nil {
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
		RETURNING id, name, COALESCE(description, ''), created_at, updated_at`,
		req.Name, req.Description,
	).Scan(&r.ID, &r.Name, &r.Description, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if respond.IsDuplicate(err) {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("create role: %w", err)
	}
	changes, _ := json.Marshal(map[string]string{"name": r.Name, "description": r.Description})
	s.logActivity(r.ID, "role", "create", string(changes), actorID)
	return &r, nil
}

func (s *Service) updateRole(id string, req UpdateRoleRequest, actorID string) (*Role, error) {
	if !validUUID(id) {
		return nil, ErrNotFound
	}
	var currentName, currentDesc string
	if err := s.db.QueryRow(
		`SELECT name, COALESCE(description, '') FROM roles WHERE id = $1`,
		id,
	).Scan(&currentName, &currentDesc); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update role: %w", err)
	}
	if req.Name != nil && currentName == "superadmin" && *req.Name != "superadmin" {
		return nil, ErrSuperadminRoleProtected
	}
	var r Role
	err := s.db.QueryRow(
		`UPDATE roles SET
			name = COALESCE(NULLIF($2, ''), name),
			description = COALESCE($3, description),
			updated_at = now()
		WHERE id = $1
		RETURNING id, name, COALESCE(description, ''), created_at, updated_at`,
		id,
		req.Name,
		req.Description,
	).Scan(&r.ID, &r.Name, &r.Description, &r.CreatedAt, &r.UpdatedAt)
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
			s.logActivity(r.ID, "role", "update", string(b), actorID)
		}
	}
	return &r, nil
}

func (s *Service) deleteRole(id, actorID string) error {
	if !validUUID(id) {
		return nil
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
	err = tx.QueryRow(`SELECT name FROM roles WHERE id = $1`, id).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	if name == "superadmin" {
		return ErrSuperadminRoleProtected
	}

	var assigned int
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND role_id = $1`,
		id,
	).Scan(&assigned); err != nil {
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
	s.logActivity(id, "role", "delete", string(changes), actorID)
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
	_, err = tx.Exec(
		`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		roleID, permissionID,
	)
	if err != nil {
		if respond.IsForeignKeyViolation(err) {
			return ErrNotFound
		}
		return fmt.Errorf("assign permission: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("assign permission: %w", err)
	}
	changes, _ := json.Marshal(map[string]any{"permissions": map[string]any{"added": []string{permName}}})
	s.logActivity(roleID, "role", "update", string(changes), actorID)
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
	if _, err := tx.Exec(
		`DELETE FROM role_permissions WHERE role_id = $1 AND permission_id = $2`,
		roleID, permissionID,
	); err != nil {
		return fmt.Errorf("remove permission: %w", err)
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
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("remove permission: %w", err)
	}
	changes, _ := json.Marshal(map[string]any{"permissions": map[string]any{"removed": []string{permName}}})
	s.logActivity(roleID, "role", "update", string(changes), actorID)
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
	var createdAt, updatedAt time.Time
	if err := tx.QueryRow(
		`SELECT name, COALESCE(description, ''), created_at, updated_at FROM roles WHERE id = $1`,
		roleID,
	).Scan(&roleName, &description, &createdAt, &updatedAt); err != nil {
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
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("set role permissions: %w", err)
	}

	if len(added) > 0 || len(removed) > 0 {
		changes, _ := json.Marshal(map[string]any{
			"permissions": map[string]any{"added": added, "removed": removed},
		})
		s.logActivity(roleID, "role", "update", string(changes), actorID)
	}

	perms, err := s.getRolePermissions(roleID)
	if err != nil {
		return nil, fmt.Errorf("set role permissions: reload: %w", err)
	}
	return &Role{
		ID:          roleID,
		Name:        roleName,
		Description: description,
		Permissions: perms,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func (s *Service) GetUserPermissions(userID string) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT p.name FROM permissions p
		JOIN role_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = (SELECT role_id FROM users WHERE id = $1)
	`, userID)
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
	rows, err := s.db.Query(`
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

	var exists bool
	if roleID != "" {
		if err := tx.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM roles WHERE id = $1)`,
			roleID,
		).Scan(&exists); err != nil {
			return fmt.Errorf("set user role: %w", err)
		}
		if !exists {
			return ErrNotFound
		}
	}
	if err := tx.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL)`,
		userID,
	).Scan(&exists); err != nil {
		return fmt.Errorf("set user role: %w", err)
	}
	if !exists {
		return ErrNotFound
	}

	cur, err := userRoleStatus(tx, userID)
	if err != nil {
		return fmt.Errorf("set user role: %w", err)
	}
	next, err := roleStatusForID(tx, roleID)
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

	// Clearing passes NULL, not an empty string, so the role_id column is
	// actually cleared instead of raising a 22P02 uuid error.
	var roleArg any
	if roleID != "" {
		roleArg = roleID
	}
	if _, err := tx.Exec(
		`UPDATE users SET role_id = $2, updated_at = now() WHERE id = $1`,
		userID, roleArg,
	); err != nil {
		return fmt.Errorf("set user role: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("set user role: %w", err)
	}
	changes, _ := json.Marshal(map[string]any{
		"role": map[string]any{"old": nullName(cur.name), "new": nullName(next.name)},
	})
	s.logActivity(userID, "user", "update", string(changes), actorID)
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
	rows, err := s.db.Query(
		`SELECT u.id, u.name, u.email, COALESCE(u.avatar_url, ''), u.created_at,
			r.id, r.name, COALESCE(r.description, ''), r.created_at, r.updated_at
		FROM users u
		LEFT JOIN roles r ON r.id = u.role_id
		WHERE u.deleted_at IS NULL
		ORDER BY u.name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	users := []UserInfo{}
	for rows.Next() {
		var u UserInfo
		var roleID, roleName, roleDesc sql.NullString
		var roleCreatedAt, roleUpdatedAt sql.NullTime
		if err := rows.Scan(
			&u.ID, &u.Name, &u.Email, &u.AvatarURL, &u.CreatedAt,
			&roleID, &roleName, &roleDesc, &roleCreatedAt, &roleUpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("list users: scan: %w", err)
		}
		if roleID.Valid {
			u.Role = &Role{
				ID:          roleID.String,
				Name:        roleName.String,
				Description: roleDesc.String,
				CreatedAt:   roleCreatedAt.Time,
				UpdatedAt:   roleUpdatedAt.Time,
			}
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list users: iterate: %w", err)
	}
	totalSuperadmins, err := s.countSuperadminsAll()
	if err != nil {
		return nil, fmt.Errorf("list users: count superadmins: %w", err)
	}
	for i := range users {
		users[i].Protected = totalSuperadmins == 1 && users[i].Role != nil && users[i].Role.Name == "superadmin"
	}
	return users, nil
}

func (s *Service) createUser(name, email, password string, actorID string) (*UserInfo, error) {
	hash, err := auth.HashPassword(password, 12)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	email = strings.ToLower(strings.TrimSpace(email))
	var u UserInfo
	err = s.db.QueryRow(
		`INSERT INTO users (name, email, password_hash, must_change_password) VALUES ($1, $2, $3, true)
		RETURNING id, name, email, COALESCE(avatar_url, ''), created_at`,
		name, email, hash,
	).Scan(&u.ID, &u.Name, &u.Email, &u.AvatarURL, &u.CreatedAt)
	if err != nil {
		if respond.IsDuplicate(err) {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	changes, _ := json.Marshal(map[string]string{"name": name, "email": email})
	s.logActivity(u.ID, "user", "create", string(changes), actorID)
	return &u, nil
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

	protected, err := userHasRole(tx, id, "superadmin")
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if protected {
		others, err := countSuperadmins(tx, id)
		if err != nil {
			return fmt.Errorf("delete user: %w", err)
		}
		if others == 0 {
			return ErrLastSuperadminProtected
		}
	}

	canManage, err := userCanManageRBAC(tx, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if canManage {
		others, err := countOtherRBACManagers(tx, id)
		if err != nil {
			return fmt.Errorf("delete user: %w", err)
		}
		if others == 0 {
			return ErrLastManagerProtected
		}
	}

	if _, err := tx.Exec(`UPDATE users SET deleted_at = now() WHERE id = $1`, id); err != nil {
		return fmt.Errorf("soft-delete user: %w", err)
	}
	if _, err := tx.Exec(
		`UPDATE refresh_tokens SET revoked = true WHERE user_id = $1 AND NOT revoked`,
		id,
	); err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	s.logActivity(id, "user", "delete", `{"action":"deleted"}`, actorID)
	return nil
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

// userHasRole reports whether the user holds a role with the given name.
func userHasRole(tx *sql.Tx, userID, roleName string) (bool, error) {
	var exists bool
	err := tx.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM roles r
			WHERE r.id = (SELECT role_id FROM users WHERE id = $1)
			  AND r.name = $2
		)`,
		userID, roleName,
	).Scan(&exists)
	return exists, err
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

// userRoleStatus reports the status of the role the user currently holds.
func userRoleStatus(tx *sql.Tx, userID string) (roleStatus, error) {
	var st roleStatus
	var name sql.NullString
	err := tx.QueryRow(
		`SELECT r.name,
			COALESCE(r.name = 'superadmin', false),
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
		FROM users u
		LEFT JOIN roles r ON r.id = u.role_id
		WHERE u.id = $1`,
		userID,
	).Scan(&name, &st.isSuperadmin, &st.hasWildcard, &st.canManage)
	st.name = name.String
	return st, err
}

// userHoldsWildcard reports whether the user's role carries the wildcard
// permission. An empty userID (no authenticated actor) reports false.
func userHoldsWildcard(tx *sql.Tx, userID string) (bool, error) {
	if userID == "" {
		return false, nil
	}
	var holds bool
	err := tx.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM role_permissions rp
			JOIN permissions p ON p.id = rp.permission_id
			WHERE rp.role_id = (SELECT role_id FROM users WHERE id = $1)
			  AND p.name = '*'
		)`,
		userID,
	).Scan(&holds)
	return holds, err
}

// userCanManageRBAC reports whether the user could manage roles and users,
// either through settings:manage or the wildcard.
func userCanManageRBAC(tx *sql.Tx, userID string) (bool, error) {
	var can bool
	err := tx.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM role_permissions rp
			JOIN permissions p ON p.id = rp.permission_id
			WHERE rp.role_id = (SELECT role_id FROM users WHERE id = $1)
			  AND p.name IN ('settings:manage', '*')
		)`,
		userID,
	).Scan(&can)
	return can, err
}

// countOtherRBACManagers counts users other than the excluded one who could
// manage roles and users.
func countOtherRBACManagers(tx *sql.Tx, excludeID string) (int, error) {
	var count int
	err := tx.QueryRow(
		`SELECT COUNT(DISTINCT u.id) FROM users u
		WHERE u.deleted_at IS NULL AND u.id <> $1
		  AND EXISTS(
			SELECT 1 FROM role_permissions rp
			JOIN permissions p ON p.id = rp.permission_id
			WHERE rp.role_id = u.role_id AND p.name IN ('settings:manage', '*')
		  )`,
		excludeID,
	).Scan(&count)
	return count, err
}

// countRBACManagers counts every non-deleted user who could manage roles and
// users.
func countRBACManagers(tx *sql.Tx) (int, error) {
	var count int
	err := tx.QueryRow(
		`SELECT COUNT(DISTINCT u.id) FROM users u
		WHERE u.deleted_at IS NULL
		  AND EXISTS(
			SELECT 1 FROM role_permissions rp
			JOIN permissions p ON p.id = rp.permission_id
			WHERE rp.role_id = u.role_id AND p.name IN ('settings:manage', '*')
		  )`,
	).Scan(&count)
	return count, err
}

// countSuperadmins counts non-deleted users holding the superadmin role,
// excluding the given user.
func countSuperadmins(tx *sql.Tx, excludeID string) (int, error) {
	var count int
	err := tx.QueryRow(
		`SELECT COUNT(*) FROM users u
		WHERE u.deleted_at IS NULL AND u.id <> $1
		  AND EXISTS(
			SELECT 1 FROM roles r
			WHERE r.id = u.role_id AND r.name = 'superadmin'
		  )`,
		excludeID,
	).Scan(&count)
	return count, err
}

// countSuperadminsAll counts all non-deleted users holding the superadmin role.
func (s *Service) countSuperadminsAll() (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM users u
		WHERE u.deleted_at IS NULL
		  AND EXISTS(
			SELECT 1 FROM roles r
			WHERE r.id = u.role_id AND r.name = 'superadmin'
		  )`,
	).Scan(&count)
	return count, err
}

type UserInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Role      *Role     `json:"role,omitempty"`
	Protected bool      `json:"protected"`
	CreatedAt time.Time `json:"created_at"`
}
