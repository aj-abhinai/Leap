package auth

import "testing"

func TestAuthErrorError(t *testing.T) {
	err := &AuthError{Code: "INVALID_CREDENTIALS", Message: "Invalid email or password"}
	if err.Error() != "Invalid email or password" {
		t.Errorf("Error() = %q, want %q", err.Error(), "Invalid email or password")
	}
}
