package auth

import (
	"crm/internal/activity"
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

	_, resp, mustChange, err := svc.login("alice@example.com", "correct-horse")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if mustChange {
		t.Error("expected must_change_password=false for a normal login")
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

	_, _, _, err := svc.login("alice@example.com", "wrong-password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginUnknownUserIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db, authTestConfig())

	_, _, _, err := svc.login("nobody@example.com", "whatever-password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestRefreshRotationIntegration(t *testing.T) {
	db := testdb.New(t)
	seedUser(t, db, "alice@example.com", "correct-horse")
	svc := NewService(db, authTestConfig())

	_, first, _, err := svc.login("alice@example.com", "correct-horse")
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

	_, first, _, err := svc.login("alice@example.com", "correct-horse")
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

	_, resp, _, err := svc.login("alice@example.com", "correct-horse")
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

	_, resp, _, err := svc.login("alice@example.com", "correct-horse")
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

func TestLoginMustChangePasswordFlagIntegration(t *testing.T) {
	db := testdb.New(t)
	id := seedUserWithFlag(t, db, "bob@example.com", "correct-horse", true)
	svc := NewService(db, authTestConfig())

	_, resp, mustChange, err := svc.login("bob@example.com", "correct-horse")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if !mustChange {
		t.Error("expected must_change_password=true for a flagged user")
	}
	if resp.AccessToken == "" {
		t.Error("expected non-empty access token")
	}

	// Verify the /me endpoint returns the flag.
	u, err := svc.getUser(id)
	if err != nil {
		t.Fatalf("getUser: %v", err)
	}
	if !u.MustChangePassword {
		t.Error("expected MustChangePassword=true from getUser")
	}
}

func TestChangePasswordSuccessIntegration(t *testing.T) {
	db := testdb.New(t)
	id := seedUserWithFlag(t, db, "carol@example.com", "original-pw", true)
	svc := NewService(db, authTestConfig())

	// The fresh pair issued to the changing device keeps it signed in.
	resp, err := svc.changePassword(id, "original-pw", "New-Passw0rd!")
	if err != nil {
		t.Fatalf("changePassword: %v", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Error("expected a fresh token pair after password change")
	}

	// Log in with the new password, flag should now be false.
	_, _, mustChange, err := svc.login("carol@example.com", "New-Passw0rd!")
	if err != nil {
		t.Fatalf("login with new password: %v", err)
	}
	if mustChange {
		t.Error("expected must_change_password=false after successful password change")
	}
}

func TestChangePasswordWrongCurrentIntegration(t *testing.T) {
	db := testdb.New(t)
	id := seedUserWithFlag(t, db, "dave@example.com", "real-pw", true)
	svc := NewService(db, authTestConfig())

	_, err := svc.changePassword(id, "wrong-current", "New-Passw0rd!")
	if !errors.Is(err, ErrIncorrectPassword) {
		t.Errorf("expected ErrIncorrectPassword, got %v", err)
	}
}

func TestChangePasswordTooShortIntegration(t *testing.T) {
	db := testdb.New(t)
	id := seedUser(t, db, "eve@example.com", "correct-horse")
	svc := NewService(db, authTestConfig())

	_, err := svc.changePassword(id, "correct-horse", "short")
	if !errors.Is(err, ErrPasswordTooShort) {
		t.Errorf("expected ErrPasswordTooShort, got %v", err)
	}
}

func TestChangePasswordRevokesSessionsIntegration(t *testing.T) {
	db := testdb.New(t)
	id := seedUserWithFlag(t, db, "frank@example.com", "frank-pw", false)
	svc := NewService(db, authTestConfig())

	// Create two refresh-token sessions.
	_, resp1, _, err := svc.login("frank@example.com", "frank-pw")
	if err != nil {
		t.Fatalf("login 1: %v", err)
	}
	_, resp2, _, err := svc.login("frank@example.com", "frank-pw")
	if err != nil {
		t.Fatalf("login 2: %v", err)
	}

	// The fresh pair issued to the changing device keeps it signed in;
	// the two earlier sessions are revoked.
	resp3, err := svc.changePassword(id, "frank-pw", "New-Frank-Pw1!")
	if err != nil {
		t.Fatalf("changePassword: %v", err)
	}
	if resp3.AccessToken == "" || resp3.RefreshToken == "" {
		t.Error("expected a fresh token pair after password change")
	}
	if _, err := svc.refresh(resp3.RefreshToken); err != nil {
		t.Errorf("fresh session after change: expected refresh to succeed, got %v", err)
	}

	// Both earlier sessions should be revoked.
	if _, err := svc.refresh(resp1.RefreshToken); !errors.Is(err, ErrTokenRevoked) {
		t.Errorf("session 1: expected ErrTokenRevoked, got %v", err)
	}
	if _, err := svc.refresh(resp2.RefreshToken); !errors.Is(err, ErrTokenRevoked) {
		t.Errorf("session 2: expected ErrTokenRevoked, got %v", err)
	}
}

func TestLoginUnknownUserRunsPasswordComparison(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db, authTestConfig())

	orig := comparePassword
	calls := 0
	comparePassword = func(hash, pw []byte) error {
		calls++
		return orig(hash, pw)
	}
	t.Cleanup(func() { comparePassword = orig })

	_, _, _, err := svc.login("nobody@example.com", "whatever-password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 password comparison for an unknown account, got %d", calls)
	}
}

func TestLoginKnownUserRunsPasswordComparison(t *testing.T) {
	db := testdb.New(t)
	seedUser(t, db, "alice@example.com", "correct-horse")
	svc := NewService(db, authTestConfig())

	orig := comparePassword
	calls := 0
	comparePassword = func(hash, pw []byte) error {
		calls++
		return orig(hash, pw)
	}
	t.Cleanup(func() { comparePassword = orig })

	_, _, _, err := svc.login("alice@example.com", "wrong-password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 password comparison for a known account, got %d", calls)
	}
}

func TestMustChangePasswordIntegration(t *testing.T) {
	db := testdb.New(t)
	flagID := seedUserWithFlag(t, db, "grace@example.com", "correct-horse", true)
	clearID := seedUserWithFlag(t, db, "henry@example.com", "correct-horse", false)
	svc := NewService(db, authTestConfig())

	must, err := svc.MustChangePassword(flagID)
	if err != nil {
		t.Fatalf("MustChangePassword(flagged): %v", err)
	}
	if !must {
		t.Error("expected true for a flagged user")
	}

	must, err = svc.MustChangePassword(clearID)
	if err != nil {
		t.Fatalf("MustChangePassword(clear): %v", err)
	}
	if must {
		t.Error("expected false for a user who already changed their password")
	}

	// A soft-deleted user is not flagged, preserving the documented behavior
	// that existing access tokens stay valid until their short TTL expires.
	if _, err := db.Exec(`UPDATE users SET deleted_at = now() WHERE id = $1`, flagID); err != nil {
		t.Fatalf("soft-delete user: %v", err)
	}
	must, err = svc.MustChangePassword(flagID)
	if err != nil {
		t.Fatalf("MustChangePassword(deleted): %v", err)
	}
	if must {
		t.Error("expected false for a soft-deleted user")
	}
}

func authTestConfig() config.Auth {
	return config.Auth{
		JWTSecret:       "test-secret-key-0123456789abcdef0123456789",
		JWTIssuer:       "crm-test",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		SecureCookies:   false,
	}
}

func TestLoginRecordsLastLoginIntegration(t *testing.T) {
	db := testdb.New(t)
	userID := seedUser(t, db, "alice@example.com", "correct-horse")
	svc := NewService(db, authTestConfig())

	// Before login, last_login_at is NULL.
	u, err := svc.getUser(userID)
	if err != nil {
		t.Fatalf("getUser: %v", err)
	}
	if u.LastLoginAt != nil {
		t.Fatalf("expected last_login_at to be NULL before login, got %v", u.LastLoginAt)
	}

	_, _, _, err = svc.login("alice@example.com", "correct-horse")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	// After login, last_login_at is set.
	u, err = svc.getUser(userID)
	if err != nil {
		t.Fatalf("getUser: %v", err)
	}
	if u.LastLoginAt == nil {
		t.Fatal("expected last_login_at to be set after login")
	}
}

func TestLoginLogoutAuditIntegration(t *testing.T) {
	db := testdb.New(t)
	userID := seedUser(t, db, "alice@example.com", "correct-horse")
	svc := NewService(db, authTestConfig())
	act := activity.NewService(db)

	_, resp, _, err := svc.login("alice@example.com", "correct-horse")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	act.LogLogin(userID, "Test User", "alice@example.com")

	var loginCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM audit_logs WHERE action = 'login' AND user_id = $1`, userID,
	).Scan(&loginCount); err != nil {
		t.Fatalf("count login audit: %v", err)
	}
	if loginCount != 1 {
		t.Errorf("expected 1 login audit entry, got %d", loginCount)
	}

	act.LogLogout(userID, "Test User", "alice@example.com")
	var logoutCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM audit_logs WHERE action = 'logout' AND user_id = $1`, userID,
	).Scan(&logoutCount); err != nil {
		t.Fatalf("count logout audit: %v", err)
	}
	if logoutCount != 1 {
		t.Errorf("expected 1 logout audit entry, got %d", logoutCount)
	}

	// Logout via the service should revoke the token and still identify the actor.
	gotID, gotName, gotEmail, err := svc.logout(resp.RefreshToken)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	if gotID != userID || gotName != "Test User" || gotEmail != "alice@example.com" {
		t.Errorf("expected logout to return user %q, got id=%q name=%q email=%q", userID, gotID, gotName, gotEmail)
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

func seedUserWithFlag(t *testing.T, db *sql.DB, email, password string, mustChange bool) string {
	t.Helper()
	hash, err := HashPassword(password, bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	var id string
	err = db.QueryRow(
		`INSERT INTO users (name, email, password_hash, must_change_password) VALUES ($1, $2, $3, $4) RETURNING id`,
		"Test User", email, hash, mustChange,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}
