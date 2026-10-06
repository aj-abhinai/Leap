package user

import "errors"

// PolicyError is a user-input rule violation. Code and Message are the API
// contract the calling package surfaces to its own clients.
type PolicyError struct {
	Code    string
	Message string
}

func (e *PolicyError) Error() string { return e.Message }

var (
	// ErrNotFound reports a missing or soft-deleted user.
	ErrNotFound = errors.New("user not found")
	// ErrEmailTaken reports an email collision on write.
	ErrEmailTaken = errors.New("email already in use")

	ErrNameRequired  = &PolicyError{Code: "BAD_REQUEST", Message: "Name cannot be empty"}
	ErrNameTooLong   = &PolicyError{Code: "BAD_REQUEST", Message: "Name must be at most 100 characters"}
	ErrEmailRequired = &PolicyError{Code: "BAD_REQUEST", Message: "Email cannot be empty"}
	ErrPhoneTooLong  = &PolicyError{Code: "BAD_REQUEST", Message: "Phone must be at most 20 characters"}

	// ErrPhoneInvalid reports a non-blank phone with no digits; an explicit
	// empty string still clears the phone. The lowercase message matches the
	// contact and lead entry points.
	ErrPhoneInvalid = &PolicyError{Code: "BAD_REQUEST", Message: "phone must contain at least one digit"}

	ErrPasswordTooShort     = &PolicyError{Code: "BAD_REQUEST", Message: "Password must be at least 10 characters"}
	ErrPasswordTooLong      = &PolicyError{Code: "BAD_REQUEST", Message: "Password must be at most 72 characters"}
	ErrPasswordNeedsUpper   = &PolicyError{Code: "BAD_REQUEST", Message: "Password must include an uppercase letter"}
	ErrPasswordNeedsLower   = &PolicyError{Code: "BAD_REQUEST", Message: "Password must include a lowercase letter"}
	ErrPasswordNeedsDigit   = &PolicyError{Code: "BAD_REQUEST", Message: "Password must include a digit"}
	ErrPasswordNeedsSpecial = &PolicyError{Code: "BAD_REQUEST", Message: "Password must include a special character"}
)
