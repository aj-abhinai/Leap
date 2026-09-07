package rbac

import "time"

type Role struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	IsSystem    bool         `json:"is_system"`
	Permissions []Permission `json:"permissions,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

type Permission struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type RolePermission struct {
	RoleID       string `json:"role_id"`
	PermissionID string `json:"permission_id"`
}

type CreateRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type UpdateRoleRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type SetUserRoleRequest struct {
	RoleID string `json:"role_id"`
}

// UpdateUserRequest carries the editable identity fields of a user; only the
// provided fields change.
type UpdateUserRequest struct {
	Name  *string `json:"name,omitempty"`
	Email *string `json:"email,omitempty"`
	Phone *string `json:"phone,omitempty"`
}

type SetRolePermissionsRequest struct {
	PermissionIDs []string `json:"permission_ids"`
}

// AssigneeOption is the minimal user shape a lead assignee dropdown needs:
// id + name, deliberately excluding email/role/avatar so a lead:read caller
// can power the picker without admin user details.
type AssigneeOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
