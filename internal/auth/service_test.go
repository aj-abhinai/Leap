package auth

import (
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
	_, err := HashPassword("", bcrypt.MinCost)
	if err != nil {
		t.Error("hashing empty password should not fail")
	}
}

func TestGenerateRandomToken(t *testing.T) {
	t1 := generateRandomToken()
	t2 := generateRandomToken()
	if t1 == "" || t2 == "" {
		t.Error("tokens should not be empty")
	}
	if t1 == t2 {
		t.Error("tokens should be unique")
	}
	if len(t1) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(t1))
	}
}

func TestHashToken(t *testing.T) {
	token := "test-token"
	h := hashToken(token)
	if h == "" {
		t.Error("hash should not be empty")
	}
	h2 := hashToken(token)
	if h != h2 {
		t.Error("hash should be deterministic")
	}
}
