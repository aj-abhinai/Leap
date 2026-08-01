package auth

import (
	"crm/internal/config"
	"crm/internal/testdb"
	"database/sql"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestLoginIntegration(t *testing.T) {
	db := testdb.New(t)
	seedUser(t, db, "alice@example.com", "correct-horse")
	svc := NewService(db, authTestConfig())

	resp, err := svc.login("alice@example.com", "correct-horse")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if resp.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}
	if resp.ExpiresAt <= time.Now().Unix() {
		t.Error("expected access token expiry in the future")
	}
}

func TestLoginWrongPasswordIntegration(t *testing.T) {
	db := testdb.New(t)
	seedUser(t, db, "alice@example.com", "correct-horse")
	svc := NewService(db, authTestConfig())

	_, err := svc.login("alice@example.com", "wrong-password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginUnknownUserIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db, authTestConfig())

	_, err := svc.login("nobody@example.com", "whatever-password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestRefreshRotationIntegration(t *testing.T) {
	db := testdb.New(t)
	seedUser(t, db, "alice@example.com", "correct-horse")
	svc := NewService(db, authTestConfig())

	first, err := svc.login("alice@example.com", "correct-horse")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	second, err := svc.refresh(first.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if second.AccessToken == "" || second.RefreshToken == "" {
		t.Error("expected a fresh token pair from refresh")
	}
	if second.RefreshToken == first.RefreshToken {
		t.Error("refresh token should rotate on every refresh")
	}
}

func TestRefreshReuseOldTokenRejectedIntegration(t *testing.T) {
	db := testdb.New(t)
	seedUser(t, db, "alice@example.com", "correct-horse")
	svc := NewService(db, authTestConfig())

	first, err := svc.login("alice@example.com", "correct-horse")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := svc.refresh(first.RefreshToken); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	_, err = svc.refresh(first.RefreshToken)
	if !errors.Is(err, ErrTokenRevoked) {
		t.Errorf("expected ErrTokenRevoked for reused token, got %v", err)
	}
}

func TestRefreshUnknownTokenIntegration(t *testing.T) {
	db := testdb.New(t)
	seedUser(t, db, "alice@example.com", "correct-horse")
	svc := NewService(db, authTestConfig())

	_, err := svc.refresh("never-issued-token")
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestRefreshRejectedAfterUserDeactivationIntegration(t *testing.T) {
	db := testdb.New(t)
	userID := seedUser(t, db, "alice@example.com", "correct-horse")
	svc := NewService(db, authTestConfig())

	resp, err := svc.login("alice@example.com", "correct-horse")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := db.Exec(`UPDATE users SET deleted_at = now() WHERE id = $1`, userID); err != nil {
		t.Fatalf("deactivate user: %v", err)
	}

	_, err = svc.refresh(resp.RefreshToken)
	if !errors.Is(err, ErrTokenRevoked) {
		t.Errorf("expected ErrTokenRevoked after deactivation, got %v", err)
	}
}

func TestAccessTokenValidationIntegration(t *testing.T) {
	db := testdb.New(t)
	userID := seedUser(t, db, "alice@example.com", "correct-horse")
	svc := NewService(db, authTestConfig())

	resp, err := svc.login("alice@example.com", "correct-horse")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	got, err := svc.ValidateJWT(resp.AccessToken)
	if err != nil {
		t.Fatalf("validate access token: %v", err)
	}
	if got != userID {
		t.Errorf("expected user ID %q, got %q", userID, got)
	}

	tampered := resp.AccessToken[:len(resp.AccessToken)-2] + "xx"
	if _, err := svc.ValidateJWT(tampered); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for tampered token, got %v", err)
	}
}

func authTestConfig() config.Auth {
	return config.Auth{
		JWTSecret:       "test-secret-key-0123456789abcdef0123456789",
		JWTIssuer:       "crm-test",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}
}

func seedUser(t *testing.T, db *sql.DB, email, password string) string {
	t.Helper()
	hash, err := HashPassword(password, bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	var id string
	err = db.QueryRow(
		`INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		"Test User", email, hash,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}
