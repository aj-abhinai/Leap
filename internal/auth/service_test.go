package auth

import (
	"crm/internal/config"
	"errors"
	"strings"
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

func TestDummyHashUsesConfiguredCost(t *testing.T) {
	cfg := authTestConfig()
	cfg.BcryptCost = bcrypt.MinCost
	s := NewService(nil, cfg)
	if !strings.HasPrefix(s.dummyHash, "$2a$04$") {
		t.Errorf("dummy hash = %q, want bcrypt cost prefix 04", s.dummyHash[:7])
	}

	// The dummy tracks the configured cost so the unknown-account path never
	// runs systematically slower or faster than real (config-cost) hashes.
	cfg.BcryptCost = 5
	s = NewService(nil, cfg)
	if !strings.HasPrefix(s.dummyHash, "$2a$05$") {
		t.Errorf("dummy hash = %q, want bcrypt cost prefix 05", s.dummyHash[:7])
	}
}

func TestDummyHashSurvivesInvalidCost(t *testing.T) {
	cfg := authTestConfig()
	cfg.BcryptCost = 99 // above bcrypt.MaxCost; must fall back, not panic
	s := NewService(nil, cfg)
	if !strings.HasPrefix(s.dummyHash, "$2a$") {
		t.Errorf("dummy hash = %q, want a valid bcrypt hash", s.dummyHash[:7])
	}
}

func TestHashPasswordEmpty(t *testing.T) {
	_, err := HashPassword("", bcrypt.MinCost)
	if err != nil {
		t.Error("hashing empty password should not fail")
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

func TestValidateJWTRejectsTamperedToken(t *testing.T) {
	svc := &Service{cfg: authTestConfig()}
	token, err := svc.createJWT("user-1", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("createJWT: %v", err)
	}
	// Flip a fully-significant base64url char inside the 43-char HS256
	// signature region. Tampering the trailing char alone is unreliable: its
	// final 2 bits are ignored on decode, so the altered token can still
	// produce the original signature bytes.
	sigStart := len(token) - 43
	repl := byte('a')
	if token[sigStart] == 'a' {
		repl = 'b'
	}
	tampered := token[:sigStart] + string(repl) + token[sigStart+1:]
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
