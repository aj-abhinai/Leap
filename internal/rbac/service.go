package rbac

import (
	"crm/internal/auth"
	"crm/internal/respond"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrSelfDelete is returned when an actor targets their own account.
	ErrSelfDelete = errors.New("a user cannot delete their own account")

	// ErrSuperadminUserProtected is returned when an operation would delete
	// the seeded superadmin user or the user holding the superadmin role.
	ErrSuperadminUserProtected = errors.New("the superadmin user cannot be deleted")

	// ErrSuperadminRoleProtected is returned when an operation would delete,
	// rename, or strip the wildcard permission from the superadmin role.
	ErrSuperadminRoleProtected = errors.New("the superadmin role is protected")

	// ErrLastManagerProtected is returned when an operation would leave no
	// user able to manage roles and users.
	ErrLastManagerProtected = errors.New("cannot remove the last user who can manage roles and users")

	// ErrNotFound marks mutations targeting a role or user that does not
	// exist.
	ErrNotFound = errors.New("role or user not found")

	// ErrDuplicate marks name collisions on uniquely named resources.
	ErrDuplicate = errors.New("name already in use")
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
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
			return nil, err
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

func (s *Service) createRole(req CreateRoleRequest) (*Role, error) {
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
	return &r, nil
}

func (s *Service) updateRole(id string, req UpdateRoleRequest) (*Role, error) {
	if req.Name != "" {
		var currentName string
		if err := s.db.QueryRow(`SELECT name FROM roles WHERE id = $1`, id).Scan(&currentName); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrNotFound
			}
			return nil, fmt.Errorf("update role: %w", err)
		}
		if currentName == "superadmin" && req.Name != "superadmin" {
			return nil, ErrSuperadminRoleProtected
		}
	}
	var r Role
	err := s.db.QueryRow(
		`UPDATE roles SET
			name = CASE WHEN NULLIF($2, '') IS NOT NULL THEN $2 ELSE name END,
			description = CASE WHEN $3 IS NOT NULL THEN NULLIF($3, '') ELSE description END,
			updated_at = now()
		WHERE id = $1
		RETURNING id, name, COALESCE(description, ''), created_at, updated_at`,
		id,
		req.Name,
		req.Description,
	).Scan(&r.ID, &r.Name, &r.Description, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update role: %w", err)
	}
	return &r, nil
}

func (s *Service) deleteRole(id string) error {
	var name string
	err := s.db.QueryRow(`SELECT name FROM roles WHERE id = $1`, id).Scan(&name)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	if name == "superadmin" {
		return ErrSuperadminRoleProtected
	}
	if _, err := s.db.Exec(`DELETE FROM roles WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	return nil
}

func (s *Service) assignPermission(roleID, permissionID string) error {
	_, err := s.db.Exec(
		`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		roleID, permissionID,
	)
	if err != nil {
		if respond.IsForeignKeyViolation(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s *Service) removePermission(roleID, permissionID string) error {
	var roleName string
	if err := s.db.QueryRow(`SELECT name FROM roles WHERE id = $1`, roleID).Scan(&roleName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	var permName string
	if err := s.db.QueryRow(`SELECT name FROM permissions WHERE id = $1`, permissionID).Scan(&permName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if roleName == "superadmin" && permName == "*" {
		return ErrSuperadminRoleProtected
	}
	_, err := s.db.Exec(`DELETE FROM role_permissions WHERE role_id = $1 AND permission_id = $2`, roleID, permissionID)
	return err
}

func (s *Service) GetUserPermissions(userID string) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT p.name FROM permissions p
		JOIN role_permissions rp ON p.id = rp.permission_id
		JOIN user_roles ur ON rp.role_id = ur.role_id
		WHERE ur.user_id = $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get user permissions: %w", err)
	}
	defer rows.Close()
	perms := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
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
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, nil
}

func (s *Service) assignUserRole(userID, roleID string) error {
	_, err := s.db.Exec(`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, roleID)
	if err != nil {
		if respond.IsForeignKeyViolation(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s *Service) removeUserRole(userID, roleID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("remove user role: %w", err)
	}
	defer tx.Rollback()

	if err := lockRBACMutations(tx); err != nil {
		return fmt.Errorf("remove user role: %w", err)
	}

	var exists bool
	if err := tx.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM roles WHERE id = $1)`,
		roleID,
	).Scan(&exists); err != nil {
		return fmt.Errorf("remove user role: %w", err)
	}
	if !exists {
		return ErrNotFound
	}
	if err := tx.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL)`,
		userID,
	).Scan(&exists); err != nil {
		return fmt.Errorf("remove user role: %w", err)
	}
	if !exists {
		return ErrNotFound
	}

	if _, err := tx.Exec(`DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2`, userID, roleID); err != nil {
		return fmt.Errorf("remove user role: %w", err)
	}

	canStillManage, err := userCanManageRBAC(tx, userID)
	if err != nil {
		return fmt.Errorf("remove user role: %w", err)
	}
	if !canStillManage {
		others, err := countOtherRBACManagers(tx, userID)
		if err != nil {
			return fmt.Errorf("remove user role: %w", err)
		}
		if others == 0 {
			return ErrLastManagerProtected
		}
	}
	return tx.Commit()
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
		`SELECT id, name, email, COALESCE(avatar_url, ''), created_at FROM users
		WHERE deleted_at IS NULL
		ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	users := []UserInfo{}
	userIDs := []string{}
	for rows.Next() {
		var u UserInfo
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.AvatarURL, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
		userIDs = append(userIDs, u.ID)
	}
	if len(userIDs) == 0 {
		return users, nil
	}
	roleMap, err := s.getUserRolesBatch(userIDs)
	if err != nil {
		return nil, err
	}
	for i := range users {
		users[i].Roles = roleMap[users[i].ID]
	}
	return users, nil
}

func (s *Service) getUserRolesBatch(userIDs []string) (map[string][]Role, error) {
	rows, err := s.db.Query(`
		SELECT ur.user_id, r.id, r.name, COALESCE(r.description, ''), r.created_at, r.updated_at
		FROM roles r
		JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = ANY($1)
		ORDER BY r.name
	`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("get user roles batch: %w", err)
	}
	defer rows.Close()
	roleMap := map[string][]Role{}
	for rows.Next() {
		var userID string
		var r Role
		if err := rows.Scan(&userID, &r.ID, &r.Name, &r.Description, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		roleMap[userID] = append(roleMap[userID], r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return roleMap, nil
}

func (s *Service) createUser(name, email, password string) (*UserInfo, error) {
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
	return &u, nil
}

func (s *Service) deleteUser(id, actorID string) error {
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
		return ErrSuperadminUserProtected
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

	if _, err := tx.Exec(`DELETE FROM user_roles WHERE user_id = $1`, id); err != nil {
		return fmt.Errorf("delete user roles: %w", err)
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
	return tx.Commit()
}

// lockRBACMutations serializes mutations that can remove the last RBAC
// manager so concurrent deletions cannot both pass the count check.
func lockRBACMutations(tx *sql.Tx) error {
	const rbacMutationLockKey = 748391621
	_, err := tx.Exec(`SELECT pg_advisory_xact_lock($1)`, rbacMutationLockKey)
	return err
}

// userHasRole reports whether the user holds a role with the given name.
func userHasRole(tx *sql.Tx, userID, roleName string) (bool, error) {
	var exists bool
	err := tx.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			WHERE ur.user_id = $1 AND r.name = $2
		)`,
		userID, roleName,
	).Scan(&exists)
	return exists, err
}

// userCanManageRBAC reports whether the user could manage roles and users,
// either through rbac:manage or the wildcard.
func userCanManageRBAC(tx *sql.Tx, userID string) (bool, error) {
	var can bool
	err := tx.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			JOIN role_permissions rp ON rp.role_id = r.id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE ur.user_id = $1 AND p.name IN ('rbac:manage', '*')
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
			SELECT 1 FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			JOIN role_permissions rp ON rp.role_id = r.id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE ur.user_id = u.id AND p.name IN ('rbac:manage', '*')
		  )`,
		excludeID,
	).Scan(&count)
	return count, err
}

type UserInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Roles     []Role    `json:"roles,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
