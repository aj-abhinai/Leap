package user

import (
	"crm/internal/respond"
	"crm/internal/settings"
	"crm/internal/util"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Create inserts a live user flagged to change the password at first login.
// The name is trimmed and bounded; the email is normalized; an email
// collision is ErrEmailTaken. RoleID is written as given; role guards are the
// caller's concern.
func Create(q Queryer, in NewUser) (*User, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	if len([]rune(name)) > 100 {
		return nil, ErrNameTooLong
	}
	email := util.NormalizeEmail(in.Email)
	if email == "" {
		return nil, ErrEmailRequired
	}
	var roleArg any
	if in.RoleID != "" {
		roleArg = in.RoleID
	}
	u, err := scanUser(q.QueryRow(
		`INSERT INTO users (name, email, password_hash, role_id, must_change_password)
		VALUES ($1, $2, $3, $4, true)
		RETURNING `+userColumns,
		name, email, in.PasswordHash, roleArg,
	))
	if err != nil {
		if respond.IsDuplicate(err) {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

// Update edits the provided identity fields and returns the stored row. Nil
// fields are unchanged; an all-nil patch is a read. Name is trimmed and
// bounded, the email is normalized and must not collide, and the phone is
// canonicalized with the org default country code and bounded; a non-blank
// phone with no digits is refused, an empty phone clears the field. Live rows
// only; a missing or soft-deleted user is ErrNotFound.
func Update(q Queryer, id string, p Patch) (*User, error) {
	if p.Name == nil && p.Email == nil && p.Phone == nil {
		return Get(q, id)
	}
	var name *string
	if p.Name != nil {
		trimmed := strings.TrimSpace(*p.Name)
		if trimmed == "" {
			return nil, ErrNameRequired
		}
		if len([]rune(trimmed)) > 100 {
			return nil, ErrNameTooLong
		}
		name = &trimmed
	}
	var email *string
	if p.Email != nil {
		normalized := util.NormalizeEmail(*p.Email)
		if normalized == "" {
			return nil, ErrEmailRequired
		}
		email = &normalized
	}
	var phone *string
	if p.Phone != nil {
		defaultCC, err := settings.DefaultCountryCode(q)
		if err != nil {
			return nil, err
		}
		canonical := util.CanonicalPhone(*p.Phone, defaultCC)
		// A non-blank input with no digits cannot canonicalize; storing the
		// empty canonical would silently blank the phone. An explicit empty
		// string still clears the field.
		if canonical == "" && strings.TrimSpace(*p.Phone) != "" {
			return nil, ErrPhoneInvalid
		}
		if len([]rune(canonical)) > 20 {
			return nil, ErrPhoneTooLong
		}
		phone = &canonical
	}
	u, err := scanUser(q.QueryRow(
		`UPDATE users SET
			name = COALESCE($2, name),
			email = COALESCE($3, email),
			phone = COALESCE($4, phone),
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+userColumns,
		id, name, email, phone,
	))
	if err != nil {
		if respond.IsDuplicate(err) {
			return nil, ErrEmailTaken
		}
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update user: %w", err)
	}
	return u, nil
}

// SetPassword replaces the password hash and clears the forced-change flag.
// Live rows only; a missing or soft-deleted user is ErrNotFound.
func SetPassword(q Queryer, id, passwordHash string) error {
	return writePassword(q, id, passwordHash, false)
}

// ResetPassword replaces the password hash and sets the forced-change flag,
// forcing the owner to pick a new password at the next login. Live rows only;
// a missing or soft-deleted user is ErrNotFound.
func ResetPassword(q Queryer, id, passwordHash string) error {
	return writePassword(q, id, passwordHash, true)
}

func writePassword(q Queryer, id, passwordHash string, mustChange bool) error {
	res, err := q.Exec(
		`UPDATE users
		SET password_hash = $2, must_change_password = $3, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`,
		id, passwordHash, mustChange,
	)
	if err != nil {
		return fmt.Errorf("write password: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("write password: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// Deactivate soft-deletes a live user. A missing or already-deactivated user
// is ErrNotFound.
func Deactivate(q Queryer, id string) error {
	res, err := q.Exec(
		`UPDATE users SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)
	if err != nil {
		return fmt.Errorf("deactivate user: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("deactivate user: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// Reactivate restores a soft-deleted user. A missing or live user is
// ErrNotFound.
func Reactivate(q Queryer, id string) error {
	res, err := q.Exec(
		`UPDATE users SET deleted_at = NULL, updated_at = now() WHERE id = $1 AND deleted_at IS NOT NULL`,
		id,
	)
	if err != nil {
		return fmt.Errorf("reactivate user: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("reactivate user: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// SetRole writes the user's single role; an empty roleID clears it. A missing
// or soft-deleted user is ErrNotFound.
func SetRole(q Queryer, id, roleID string) error {
	var roleArg any
	if roleID != "" {
		roleArg = roleID
	}
	res, err := q.Exec(
		`UPDATE users SET role_id = $2, updated_at = now() WHERE id = $1 AND deleted_at IS NULL`,
		id, roleArg,
	)
	if err != nil {
		return fmt.Errorf("set user role: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("set user role: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// RecordLogin stamps the last successful sign-in time. It is bookkeeping; the
// caller treats a failure as best-effort.
func RecordLogin(q Queryer, id string) error {
	if _, err := q.Exec(`UPDATE users SET last_login_at = now() WHERE id = $1`, id); err != nil {
		return fmt.Errorf("record last login: %w", err)
	}
	return nil
}
