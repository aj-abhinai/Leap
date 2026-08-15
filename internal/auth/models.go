package auth

import "time"

type User struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	Email              string     `json:"email"`
	Phone              string     `json:"phone,omitempty"`
	PasswordHash       string     `json:"-"`
	AvatarURL          *string    `json:"avatar_url,omitempty"`
	MustChangePassword bool       `json:"must_change_password"`
	LastLoginAt        *time.Time `json:"last_login_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	DeletedAt          *time.Time `json:"deleted_at,omitempty"`
}

type UpdateProfileRequest struct {
	Name  *string `json:"name,omitempty"`
	Phone *string `json:"phone,omitempty"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

var (
	ErrInvalidCredentials   = &AuthError{Code: "INVALID_CREDENTIALS", Message: "Invalid email or password"}
	ErrInvalidToken         = &AuthError{Code: "INVALID_TOKEN", Message: "Invalid or expired token"}
	ErrTokenRevoked         = &AuthError{Code: "TOKEN_REVOKED", Message: "Token has been revoked"}
	ErrIncorrectPassword    = &AuthError{Code: "INCORRECT_PASSWORD", Message: "Current password is incorrect"}
	ErrPasswordTooShort     = &AuthError{Code: "BAD_REQUEST", Message: "Password must be at least 10 characters"}
	ErrPasswordTooLong      = &AuthError{Code: "BAD_REQUEST", Message: "Password must be at most 72 characters"}
	ErrPasswordNeedsUpper   = &AuthError{Code: "BAD_REQUEST", Message: "Password must include an uppercase letter"}
	ErrPasswordNeedsLower   = &AuthError{Code: "BAD_REQUEST", Message: "Password must include a lowercase letter"}
	ErrPasswordNeedsDigit   = &AuthError{Code: "BAD_REQUEST", Message: "Password must include a digit"}
	ErrPasswordNeedsSpecial = &AuthError{Code: "BAD_REQUEST", Message: "Password must include a special character"}
)

type AuthError struct {
	Code    string
	Message string
}

func (e *AuthError) Error() string { return e.Message }
