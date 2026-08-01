package rbac

import (
	"crm/internal/auth"
	"crm/internal/util"
	"database/sql"
	"fmt"
	"time"
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
	for rows.Next() {
		var r Role
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}
	return roles, nil
}

func (s *Service) createRole(req CreateRoleRequest) (*Role, error) {
	var r Role
	err := s.db.QueryRow(
		`INSERT INTO roles (name, description) VALUES ($1, $2)
		RETURNING id, name, COALESCE(description, ''), created_at, updated_at`,
		req.Name, req.Description,
	).Scan(&r.ID, &r.Name, &r.Description, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create role: %w", err)
	}
	return &r, nil
}

func (s *Service) updateRole(id string, req UpdateRoleRequest) (*Role, error) {
	var r Role
	err := s.db.QueryRow(
		`UPDATE roles SET name = COALESCE($2, name), description = COALESCE($3, description), updated_at = now()
		WHERE id = $1
		RETURNING id, name, COALESCE(description, ''), created_at, updated_at`,
		id,
		util.StrPtr(&req.Name),
		util.StrPtr(&req.Description),
	).Scan(&r.ID, &r.Name, &r.Description, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update role: %w", err)
	}
	return &r, nil
}

func (s *Service) deleteRole(id string) error {
	_, err := s.db.Exec(`DELETE FROM roles WHERE id = $1`, id)
	return err
}

func (s *Service) assignPermission(roleID, permissionID string) error {
	_, err := s.db.Exec(
		`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		roleID, permissionID,
	)
	return err
}

func (s *Service) removePermission(roleID, permissionID string) error {
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
	return err
}

func (s *Service) removeUserRole(userID, roleID string) error {
	_, err := s.db.Exec(`DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2`, userID, roleID)
	return err
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
	var u UserInfo
	err = s.db.QueryRow(
		`INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3)
		RETURNING id, name, email, COALESCE(avatar_url, ''), created_at`,
		name, email, hash,
	).Scan(&u.ID, &u.Name, &u.Email, &u.AvatarURL, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

func (s *Service) deleteUser(id string) error {
	_, err := s.db.Exec(`DELETE FROM user_roles WHERE user_id = $1`, id)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE users SET deleted_at = now() WHERE id = $1`, id)
	return err
}

type UserInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Roles     []Role    `json:"roles,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
