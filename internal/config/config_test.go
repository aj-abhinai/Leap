package config

import (
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validTOML = `
[app]
port = 9000
name = "CRM"
environment = "development"

[db]
host = "localhost"
port = 5432
user = "crm"
password = "crm"
name = "crm"
sslmode = "disable"

[auth]
jwt_secret = "0123456789abcdef0123456789abcdef"
jwt_issuer = "crm"
access_token_ttl = "15m"
refresh_token_ttl = "168h"
bcrypt_cost = 12

[superadmin]
 email = "admin@admin.com"
 password = "admin"

[log]
level = "info"
format = "json"
`

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"APP_ENV", "APP_PORT", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE",
		"JWT_SECRET", "JWT_ISSUER", "SUPERADMIN_EMAIL", "SUPERADMIN_PASSWORD", "TRUSTED_PROXIES",
	} {
		t.Setenv(k, "")
	}
}

func TestLoadFromTOML(t *testing.T) {
	clearEnv(t)
	cfg, err := Load(writeTemp(t, validTOML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.App.Port != 9000 || cfg.DB.Host != "localhost" || cfg.Auth.JWTSecret != "0123456789abcdef0123456789abcdef" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestEnvOverrides(t *testing.T) {
	clearEnv(t)
	t.Setenv("APP_PORT", "8080")
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_USER", "other")
	t.Setenv("DB_PASSWORD", "other-pass")
	t.Setenv("DB_NAME", "other-db")
	t.Setenv("DB_SSLMODE", "require")
	t.Setenv("JWT_SECRET", "env-secret-0123456789abcdef0123456789")
	t.Setenv("JWT_ISSUER", "env-issuer")
	t.Setenv("SUPERADMIN_EMAIL", "env@crm.local")
	t.Setenv("SUPERADMIN_PASSWORD", "env-admin-password")

	cfg, err := Load(writeTemp(t, validTOML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.App.Port != 8080 {
		t.Errorf("APP_PORT = %d, want 8080", cfg.App.Port)
	}
	if cfg.DB.Host != "db.example.com" || cfg.DB.Port != 5433 || cfg.DB.User != "other" {
		t.Errorf("DB overrides not applied: %+v", cfg.DB)
	}
	if cfg.DB.SSLMode != "require" {
		t.Errorf("DB_SSLMODE = %q, want require", cfg.DB.SSLMode)
	}
	if cfg.Auth.JWTSecret != "env-secret-0123456789abcdef0123456789" {
		t.Errorf("JWT_SECRET not applied")
	}
	if cfg.Auth.JWTIssuer != "env-issuer" {
		t.Errorf("JWT_ISSUER not applied")
	}
	if cfg.Superadmin.Email != "env@crm.local" {
		t.Errorf("SUPERADMIN_EMAIL not applied")
	}
}

func TestEmptyEnvKeepsTOML(t *testing.T) {
	clearEnv(t)
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_PORT", "")
	cfg, err := Load(writeTemp(t, validTOML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DB.Host != "localhost" || cfg.DB.Port != 5432 {
		t.Errorf("empty env erased TOML values: %+v", cfg.DB)
	}
}

func TestInvalidEnvPortFails(t *testing.T) {
	clearEnv(t)
	t.Setenv("APP_PORT", "not-a-number")
	if _, err := Load(writeTemp(t, validTOML)); err == nil {
		t.Fatal("expected error for invalid APP_PORT")
	}
	t.Setenv("APP_PORT", "")
	t.Setenv("DB_PORT", "also-not-a-number")
	if _, err := Load(writeTemp(t, validTOML)); err == nil {
		t.Fatal("expected error for invalid DB_PORT")
	}
}

func TestValidate(t *testing.T) {
	cfg := &Config{}
	cfg.App.Environment = "development"
	cfg.Superadmin.Email = "admin@admin.com"
	cfg.Superadmin.Password = "admin"

	tests := []struct {
		name   string
		secret string
		wantOK bool
	}{
		{name: "empty", secret: "", wantOK: false},
		{name: "placeholder", secret: "change-me-in-production", wantOK: false},
		{name: "short", secret: "short", wantOK: false},
		{name: "valid", secret: "0123456789abcdef0123456789abcdef", wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg.Auth.JWTSecret = tt.secret
			err := Validate(*cfg)
			if (err == nil) != tt.wantOK {
				t.Fatalf("Validate(%q) err = %v, wantOK %v", tt.secret, err, tt.wantOK)
			}
		})
	}

	cfg.Auth.JWTSecret = "0123456789abcdef0123456789abcdef"
	testsSuperadmin := []struct {
		name     string
		email    string
		password string
		wantOK   bool
	}{
		{name: "legacy default email", email: "admin@crm.local", password: "admin", wantOK: false},
		{name: "placeholder password", email: "admin@admin.com", password: "change-me", wantOK: false},
		{name: "short password", email: "admin@admin.com", password: "short", wantOK: false},
		{name: "development credentials", email: "admin@admin.com", password: "admin", wantOK: true},
	}
	for _, tt := range testsSuperadmin {
		t.Run(tt.name, func(t *testing.T) {
			cfg.Superadmin.Email = tt.email
			cfg.Superadmin.Password = tt.password
			err := Validate(*cfg)
			if (err == nil) != tt.wantOK {
				t.Fatalf("Validate(email=%q, pass=%q) err = %v, wantOK %v", tt.email, tt.password, err, tt.wantOK)
			}
		})
	}

	cfg.App.Environment = "production"
	cfg.Superadmin.Email = "admin@admin.com"
	cfg.Superadmin.Password = "admin"
	if err := Validate(*cfg); err == nil {
		t.Fatal("production should reject development credentials")
	}
}

func TestValidateTrustedProxies(t *testing.T) {
	cfg := &Config{}
	cfg.App.Environment = "development"
	cfg.Auth.JWTSecret = "0123456789abcdef0123456789abcdef"
	cfg.Superadmin.Email = "admin@admin.com"
	cfg.Superadmin.Password = "admin"

	tests := []struct {
		name    string
		proxies []string
		wantOK  bool
	}{
		{name: "empty list", proxies: nil, wantOK: true},
		{name: "valid CIDR", proxies: []string{"10.0.0.0/8"}, wantOK: true},
		{name: "valid CIDRs", proxies: []string{"10.0.0.0/8", " 192.168.1.0/24 "}, wantOK: true},
		{name: "invalid CIDR", proxies: []string{"not-a-prefix"}, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg.App.TrustedProxies = tt.proxies
			err := Validate(*cfg)
			if (err == nil) != tt.wantOK {
				t.Fatalf("Validate(proxies=%v) err = %v, wantOK %v", tt.proxies, err, tt.wantOK)
			}
		})
	}
}

func TestLoadTrustedProxiesFromTOML(t *testing.T) {
	clearEnv(t)
	path := writeTemp(t, strings.Replace(validTOML, "environment = \"development\"",
		"environment = \"development\"\ntrusted_proxies = [\"10.0.0.0/8\", \"192.168.0.0/16\"]", 1))
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.App.TrustedProxies) != 2 || cfg.App.TrustedProxies[0] != "10.0.0.0/8" {
		t.Errorf("trusted proxies = %v, want [10.0.0.0/8 192.168.0.0/16]", cfg.App.TrustedProxies)
	}
}

func TestEnvTrustedProxies(t *testing.T) {
	clearEnv(t)
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8, 192.168.0.0/16")
	cfg, err := Load(writeTemp(t, validTOML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.App.TrustedProxies) != 2 || cfg.App.TrustedProxies[1] != "192.168.0.0/16" {
		t.Errorf("trusted proxies = %v, want [10.0.0.0/8 192.168.0.0/16]", cfg.App.TrustedProxies)
	}
}

func TestValidateRejectsDevSecretOutsideDevelopment(t *testing.T) {
	cfg := &Config{}
	cfg.App.Environment = "production"
	cfg.Auth.JWTSecret = "dev-only-jwt-secret-0123456789abcdef"
	cfg.Superadmin.Email = "admin@example.com"
	cfg.Superadmin.Password = "a-strong-password-12+"

	if err := Validate(*cfg); err == nil {
		t.Fatal("expected Validate to reject the shipped dev JWT secret in production")
	}
}

func TestValidateAllowsDevSecretInDevelopment(t *testing.T) {
	cfg := &Config{}
	cfg.App.Environment = "development"
	cfg.Auth.JWTSecret = "dev-only-jwt-secret-0123456789abcdef"
	cfg.Superadmin.Email = "admin@admin.com"
	cfg.Superadmin.Password = "admin"

	if err := Validate(*cfg); err != nil {
		t.Fatalf("development must accept the shipped dev JWT secret: %v", err)
	}
}

func TestLoadForcesSecureCookiesOutsideDevelopment(t *testing.T) {
	clearEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("SUPERADMIN_EMAIL", "admin@example.com")
	t.Setenv("SUPERADMIN_PASSWORD", "a-strong-password-12+")

	cfg, err := Load(writeTemp(t, validTOML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Auth.SecureCookies {
		t.Error("SecureCookies must be forced true outside development")
	}
}

func TestLoadKeepsSecureCookiesInDevelopment(t *testing.T) {
	clearEnv(t)
	cfg, err := Load(writeTemp(t, validTOML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Auth.SecureCookies {
		t.Error("SecureCookies must stay false in development by default")
	}

	clearEnv(t)
	t.Setenv("COOKIE_SECURE", "true")
	cfg, err = Load(writeTemp(t, validTOML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Auth.SecureCookies {
		t.Error("COOKIE_SECURE=true must be honored in development")
	}
}

func TestWriteTemplate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := WriteTemplate(path); err != nil {
		t.Fatalf("WriteTemplate: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "jwt_secret") || !strings.Contains(content, "REPLACE") {
		t.Errorf("template missing secrets documentation: %s", content)
	}
	if err := WriteTemplate(path); err == nil {
		t.Fatal("expected refusal to overwrite existing file")
	}
}

func TestTemplateDoesNotBootUntilEdited(t *testing.T) {
	clearEnv(t)
	path := writeTemp(t, "")
	_ = os.Remove(path)
	if err := WriteTemplate(path); err != nil {
		t.Fatalf("WriteTemplate: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("template config must not boot until placeholders are replaced")
	}
}

func TestDSNEscapesReservedCharacters(t *testing.T) {
	cfg := Config{}
	cfg.DB = DB{Host: "db.example.com", Port: 5432, User: "crm", Password: "p@ss:w/rd?", Name: "crm db", SSLMode: "disable"}

	dsn := cfg.DB.DSN()
	if strings.Contains(dsn, "p@ss") {
		t.Errorf("DSN leaks raw password characters: %s", dsn)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}
	password, _ := parsed.User.Password()
	if password != "p@ss:w/rd?" {
		t.Errorf("round-tripped password = %q, want %q", password, "p@ss:w/rd?")
	}
	if parsed.User.Username() != "crm" {
		t.Errorf("round-tripped user = %q, want crm", parsed.User.Username())
	}
}

func TestLogLevel(t *testing.T) {
	tests := []struct {
		level string
		want  slog.Level
		ok    bool
	}{
		{level: "", want: slog.LevelInfo, ok: true},
		{level: "info", want: slog.LevelInfo, ok: true},
		{level: "debug", want: slog.LevelDebug, ok: true},
		{level: "WARN", want: slog.LevelWarn, ok: true},
		{level: "error", want: slog.LevelError, ok: true},
		{level: "verbose", ok: false},
	}
	for _, tt := range tests {
		got, err := (Log{Level: tt.level}).SlogLevel()
		if (err == nil) != tt.ok {
			t.Errorf("Level(%q) err = %v, ok want %v", tt.level, err, tt.ok)
			continue
		}
		if tt.ok && got != tt.want {
			t.Errorf("Level(%q) = %v, want %v", tt.level, got, tt.want)
		}
	}
}
