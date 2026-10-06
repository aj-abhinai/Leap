// Package user owns the users table: the row model, the field rules, the
// password policy, and the account lifecycle. It never reads the roles or
// permissions tables; callers merge role data.
package user

import (
	"database/sql"
	"time"
)

// Queryer is the database surface every user operation needs. Both *sql.DB
// and *sql.Tx satisfy it, so an operation runs standalone or inside a
// caller-owned transaction.
type Queryer interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// User is one row of the users table. PasswordHash and RoleID are carried for
// the credential and role-merge paths; neither is serialized.
type User struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	Email              string     `json:"email"`
	Phone              string     `json:"phone,omitempty"`
	PasswordHash       string     `json:"-"`
	AvatarURL          *string    `json:"avatar_url,omitempty"`
	RoleID             *string    `json:"-"`
	MustChangePassword bool       `json:"must_change_password"`
	LastLoginAt        *time.Time `json:"last_login_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	DeletedAt          *time.Time `json:"deleted_at,omitempty"`
}

// NewUser is the input to Create. PasswordHash comes from HashPassword; the
// stored account always starts flagged to change the password. An empty
// RoleID creates the user with no role.
type NewUser struct {
	Name         string
	Email        string
	PasswordHash string
	RoleID       string
}

// Patch carries the profile fields an update may change. Nil fields are left
// unchanged.
type Patch struct {
	Name  *string
	Email *string
	Phone *string
}
