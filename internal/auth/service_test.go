package auth

import (
	"crm/internal/config"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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
	t1, err := generateRandomToken()
	if err != nil {
		t.Fatalf("generateRandomToken failed: %v", err)
	}
	t2, err := generateRandomToken()
	if err != nil {
		t.Fatalf("generateRandomToken failed: %v", err)
	}
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

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "lowercases", input: "Alice@Example.COM", want: "alice@example.com"},
		{name: "trims whitespace", input: "  alice@example.com  ", want: "alice@example.com"},
		{name: "already normalized", input: "alice@example.com", want: "alice@example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeEmail(tt.input); got != tt.want {
				t.Errorf("normalizeEmail(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateJWTRejectsTamperedToken(t *testing.T) {
	svc := &Service{cfg: authTestConfig()}
	token, err := svc.createJWT("user-1", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("createJWT: %v", err)
	}
	tampered := token[:len(token)-2] + "xx"
	if _, err := svc.ValidateJWT(tampered); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for tampered token, got %v", err)
	}
}

func TestValidateJWTRejectsWrongSecret(t *testing.T) {
	good := &Service{cfg: authTestConfig()}
	token, err := good.createJWT("user-1", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("createJWT: %v", err)
	}
	bad := &Service{cfg: config.Auth{
		JWTSecret:       "another-secret-key-0123456789abcdef",
		JWTIssuer:       "crm-test",
		AccessTokenTTL:  time.Minute,
		RefreshTokenTTL: time.Hour,
	}}
	if _, err := bad.ValidateJWT(token); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for wrong secret, got %v", err)
	}
}

func TestValidateJWTRejectsWrongIssuer(t *testing.T) {
	cfg := authTestConfig()
	svc := &Service{cfg: cfg}
	token, err := svc.createJWT("user-1", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("createJWT: %v", err)
	}
	svc.cfg.JWTIssuer = "different-issuer"
	if _, err := svc.ValidateJWT(token); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for issuer mismatch, got %v", err)
	}
}

func TestValidateJWTRejectsMissingSubject(t *testing.T) {
	cfg := authTestConfig()
	svc := &Service{cfg: cfg}
	now := time.Now()
	claims := jwt.MapClaims{
		"exp": now.Add(time.Minute).Unix(),
		"iat": now.Unix(),
		"iss": cfg.JWTIssuer,
	}
	tokenStr, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, err := svc.ValidateJWT(tokenStr); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for missing sub, got %v", err)
	}
}

func TestValidateJWTRejectsExpiredToken(t *testing.T) {
	svc := &Service{cfg: authTestConfig()}
	token, err := svc.createJWT("user-1", time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatalf("createJWT: %v", err)
	}
	if _, err := svc.ValidateJWT(token); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for expired token, got %v", err)
	}
}
