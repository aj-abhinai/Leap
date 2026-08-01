package middleware

import (
	"context"
	"crm/internal/auth"
	"crm/internal/config"
	"crm/internal/ctxutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGetUserIDEmpty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	id := ctxutil.GetUserID(req)
	if id != "" {
		t.Errorf("expected empty user ID, got %q", id)
	}
}

func TestGetUserIDWithValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), ctxutil.UserIDKey, "test-user-123")
	req = req.WithContext(ctx)
	id := ctxutil.GetUserID(req)
	if id != "test-user-123" {
		t.Errorf("expected 'test-user-123', got %q", id)
	}
}

func TestAuthMissingAuthorization(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})
	mw := Auth(auth.NewService(nil, authTestConfig()))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	mw(handler).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMalformedScheme(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})
	mw := Auth(auth.NewService(nil, authTestConfig()))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rr := httptest.NewRecorder()

	mw(handler).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthInvalidToken(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})
	mw := Auth(auth.NewService(nil, authTestConfig()))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	rr := httptest.NewRecorder()

	mw(handler).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthTokenSignedWithWrongSecret(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})
	mw := Auth(auth.NewService(nil, authTestConfig()))
	token := signedToken(t, authTestConfig(), "user-123", "different-secret-key-0123456789abcdef")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	mw(handler).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthValidToken(t *testing.T) {
	var called bool
	var gotUserID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotUserID = ctxutil.GetUserID(r)
		w.WriteHeader(http.StatusOK)
	})
	cfg := authTestConfig()
	mw := Auth(auth.NewService(nil, cfg))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+signedToken(t, cfg, "user-123", cfg.JWTSecret))
	rr := httptest.NewRecorder()

	mw(handler).ServeHTTP(rr, req)
	if !called {
		t.Fatal("handler should have been called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if gotUserID != "user-123" {
		t.Errorf("expected user ID 'user-123' in context, got %q", gotUserID)
	}
}

func authTestConfig() config.Auth {
	return config.Auth{
		JWTSecret:       "test-secret-key-0123456789abcdef0123456789",
		JWTIssuer:       "test-issuer",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}
}

func signedToken(t *testing.T, cfg config.Auth, userID, secret string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
		"iss": cfg.JWTIssuer,
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}
