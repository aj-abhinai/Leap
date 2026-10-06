package user

import (
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const (
	// MinPasswordLength is the shortest allowed password.
	MinPasswordLength = 10
	// MaxPasswordLength is the longest allowed password in bytes; it matches
	// the bcrypt input limit so hashing can never fail on length.
	MaxPasswordLength = 72
)

// ValidatePassword enforces the shared password policy for user-created and
// self-changed passwords: 10-72 characters with at least one uppercase
// letter, one lowercase letter, one digit, and one special character. The
// 72-byte cap mirrors bcrypt's input limit, so an over-long password fails
// here with a clean error instead of a hashing failure.
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if len(password) > MaxPasswordLength {
		return ErrPasswordTooLong
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r) || unicode.IsSpace(r):
			hasSpecial = true
		}
	}
	switch {
	case !hasUpper:
		return ErrPasswordNeedsUpper
	case !hasLower:
		return ErrPasswordNeedsLower
	case !hasDigit:
		return ErrPasswordNeedsDigit
	case !hasSpecial:
		return ErrPasswordNeedsSpecial
	}
	return nil
}

// HashPassword returns the bcrypt hash of password at cost. It performs no
// policy check; callers that accept a user-supplied password call
// ValidatePassword first. The seeded superadmin password is deployment
// configuration and is hashed as given.
func HashPassword(password string, cost int) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	return string(bytes), err
}
