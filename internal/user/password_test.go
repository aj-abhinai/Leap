package user

import (
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	password := "test-password"
	hash, err := HashPassword(password, bcrypt.MinCost)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		t.Error("password verification failed")
	}
}

func TestHashPasswordEmpty(t *testing.T) {
	hash, err := HashPassword("", bcrypt.MinCost)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("")); err != nil {
		t.Error("password verification failed")
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{name: "valid", password: "Sup3r-Secret!"},
		{name: "too short", password: "Ab1!cdef", wantErr: ErrPasswordTooShort},
		{name: "exactly min length", password: "Ab1!cdefgh"},
		{name: "too long (73 bytes)", password: "Ab1!cdefgh" + "x0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234", wantErr: ErrPasswordTooLong},
		{name: "no uppercase", password: "sup3r-secret!", wantErr: ErrPasswordNeedsUpper},
		{name: "no lowercase", password: "SUP3R-SECRET!", wantErr: ErrPasswordNeedsLower},
		{name: "no digit", password: "Super-Secret!", wantErr: ErrPasswordNeedsDigit},
		{name: "no special", password: "Sup3rSecret", wantErr: ErrPasswordNeedsSpecial},
		{name: "empty", password: "", wantErr: ErrPasswordTooShort},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidatePassword(%q) err = %v, want %v", tt.password, err, tt.wantErr)
			}
		})
	}
}
