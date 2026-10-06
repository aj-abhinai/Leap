package user

import (
	"crm/internal/util"
	"database/sql"
	"errors"
	"fmt"
)

// userColumns is the projection every user read returns, in the order
// scanUser expects.
const userColumns = `id, name, email, COALESCE(phone, ''), COALESCE(avatar_url, ''), must_change_password, last_login_at, created_at, updated_at, deleted_at, role_id, password_hash`

// rowScanner is satisfied by *sql.Row and *sql.Rows, so one scan function
// serves single reads and lists.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanUser reads one row in the userColumns order.
func scanUser(row rowScanner) (*User, error) {
	var u User
	err := row.Scan(
		&u.ID,
		&u.Name,
		&u.Email,
		&u.Phone,
		&u.AvatarURL,
		&u.MustChangePassword,
		&u.LastLoginAt,
		&u.CreatedAt,
		&u.UpdatedAt,
		&u.DeletedAt,
		&u.RoleID,
		&u.PasswordHash,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Get returns the live user with id. A missing or soft-deleted user is
// ErrNotFound.
func Get(q Queryer, id string) (*User, error) {
	u, err := scanUser(q.QueryRow(
		`SELECT `+userColumns+`
		FROM users
		WHERE id = $1 AND deleted_at IS NULL`,
		id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

// GetByEmail returns the live user whose normalized email matches. A missing
// or soft-deleted user is ErrNotFound.
func GetByEmail(q Queryer, email string) (*User, error) {
	u, err := scanUser(q.QueryRow(
		`SELECT `+userColumns+`
		FROM users
		WHERE email = $1 AND deleted_at IS NULL`,
		util.NormalizeEmail(email),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

// GetIncludingDeleted returns the user with id even when soft-deleted. It
// serves the reactivation flow and the permission reads that must honor an
// access token until its expiry.
func GetIncludingDeleted(q Queryer, id string) (*User, error) {
	u, err := scanUser(q.QueryRow(
		`SELECT `+userColumns+`
		FROM users
		WHERE id = $1`,
		id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

// List returns every user, live accounts first, each group ordered by name.
// Soft-deleted users are included; role data is not.
func List(q Queryer) ([]User, error) {
	rows, err := q.Query(`SELECT ` + userColumns + ` FROM users ORDER BY deleted_at IS NOT NULL, name`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("list users: scan: %w", err)
		}
		users = append(users, *u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list users: iterate: %w", err)
	}
	return users, nil
}

// ListActive returns every live user ordered by name.
func ListActive(q Queryer) ([]User, error) {
	rows, err := q.Query(`SELECT ` + userColumns + ` FROM users WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list active users: %w", err)
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("list active users: scan: %w", err)
		}
		users = append(users, *u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list active users: iterate: %w", err)
	}
	return users, nil
}

// Count returns the number of user rows, soft-deleted rows included.
func Count(q Queryer) (int, error) {
	var n int
	if err := q.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return n, nil
}

// CountByRoles counts live users holding any of roleIDs, excluding
// excludeUserID when it is not empty. An empty role list counts zero.
func CountByRoles(q Queryer, roleIDs []string, excludeUserID string) (int, error) {
	if len(roleIDs) == 0 {
		return 0, nil
	}
	var n int
	var err error
	if excludeUserID == "" {
		err = q.QueryRow(
			`SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND role_id = ANY($1)`,
			roleIDs,
		).Scan(&n)
	} else {
		err = q.QueryRow(
			`SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND role_id = ANY($1) AND id <> $2`,
			roleIDs, excludeUserID,
		).Scan(&n)
	}
	if err != nil {
		return 0, fmt.Errorf("count users by role: %w", err)
	}
	return n, nil
}

// MustChangePassword reports whether the live user is flagged to change their
// password before using the application. A missing or soft-deleted user
// reports false.
func MustChangePassword(q Queryer, id string) (bool, error) {
	var must bool
	err := q.QueryRow(
		`SELECT must_change_password FROM users WHERE id = $1 AND deleted_at IS NULL`,
		id,
	).Scan(&must)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check must_change_password: %w", err)
	}
	return must, nil
}

// PasswordChangeChecker answers must_change_password reads for the middleware
// that gates routes until the forced change is done.
type PasswordChangeChecker struct {
	q Queryer
}

// NewPasswordChangeChecker builds the middleware adapter over the given
// database handle.
func NewPasswordChangeChecker(q Queryer) *PasswordChangeChecker {
	return &PasswordChangeChecker{q: q}
}

// MustChangePassword reports whether the user must set a new password before
// using the application.
func (c *PasswordChangeChecker) MustChangePassword(userID string) (bool, error) {
	return MustChangePassword(c.q, userID)
}
