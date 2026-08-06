package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	App        App
	DB         DB
	Auth       Auth
	Superadmin Superadmin
	Log        Log
}

type App struct {
	Port        int    `toml:"port"`
	Name        string `toml:"name"`
	Environment string `toml:"environment"`
}

type DB struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	Name     string `toml:"name"`
	SSLMode  string `toml:"sslmode"`
}

type Auth struct {
	JWTSecret       string        `toml:"jwt_secret"`
	JWTIssuer       string        `toml:"jwt_issuer"`
	AccessTokenTTL  time.Duration `toml:"access_token_ttl"`
	RefreshTokenTTL time.Duration `toml:"refresh_token_ttl"`
	BcryptCost      int           `toml:"bcrypt_cost"`
	SecureCookies   bool          `toml:"secure_cookies"`
}

type Superadmin struct {
	Email    string `toml:"email"`
	Password string `toml:"password"`
}

type Log struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	setDefaults(&cfg)
	if err := applyEnvironment(&cfg); err != nil {
		return nil, err
	}
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.App.Environment == "" {
		cfg.App.Environment = "production"
	}
	if cfg.Superadmin.Email == "" {
		cfg.Superadmin.Email = "admin@admin.com"
	}
	if cfg.Superadmin.Password == "" {
		cfg.Superadmin.Password = "admin"
	}
}

func applyEnvironment(cfg *Config) error {
	if value := os.Getenv("APP_PORT"); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("APP_PORT must be an integer")
		}
		cfg.App.Port = port
	}
	if value := os.Getenv("DB_PORT"); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("DB_PORT must be an integer")
		}
		cfg.DB.Port = port
	}

	setString(&cfg.DB.Host, "DB_HOST")
	setString(&cfg.DB.User, "DB_USER")
	setString(&cfg.DB.Password, "DB_PASSWORD")
	setString(&cfg.DB.Name, "DB_NAME")
	setString(&cfg.DB.SSLMode, "DB_SSLMODE")
	setString(&cfg.App.Environment, "APP_ENV")
	setString(&cfg.Auth.JWTSecret, "JWT_SECRET")
	setString(&cfg.Auth.JWTIssuer, "JWT_ISSUER")
	setString(&cfg.Superadmin.Email, "SUPERADMIN_EMAIL")
	setString(&cfg.Superadmin.Password, "SUPERADMIN_PASSWORD")
	if value := os.Getenv("COOKIE_SECURE"); value != "" {
		secure, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("COOKIE_SECURE must be a boolean")
		}
		cfg.Auth.SecureCookies = secure
	}
	return nil
}

func setString(target *string, key string) {
	if value := os.Getenv(key); value != "" {
		*target = value
	}
}

func Validate(cfg Config) error {
	secret := strings.TrimSpace(cfg.Auth.JWTSecret)
	if secret == "" {
		return fmt.Errorf("auth.jwt_secret must be set")
	}
	if isPlaceholder(secret) {
		return fmt.Errorf("auth.jwt_secret must not be a placeholder")
	}
	if len(secret) < 32 {
		return fmt.Errorf("auth.jwt_secret must be at least 32 characters")
	}

	email := strings.TrimSpace(cfg.Superadmin.Email)
	development := strings.EqualFold(strings.TrimSpace(cfg.App.Environment), "development") ||
		strings.EqualFold(strings.TrimSpace(cfg.App.Environment), "dev")
	if email == "" || strings.EqualFold(email, "admin@crm.local") ||
		(!development && strings.EqualFold(email, "admin@admin.com")) || !strings.Contains(email, "@") {
		return fmt.Errorf("superadmin.email must be an explicit non-placeholder email")
	}

	password := strings.TrimSpace(cfg.Superadmin.Password)
	developmentPassword := development && strings.EqualFold(password, "admin")
	if password == "" || (isPlaceholder(password) && !developmentPassword) {
		return fmt.Errorf("superadmin.password must be an explicit non-placeholder value")
	}
	if len(password) < 12 && !developmentPassword {
		return fmt.Errorf("superadmin.password must be at least 12 characters")
	}
	return nil
}

func isPlaceholder(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "change-me-in-production", "change-me", "replace-me", "your-secret", "secret", "admin",
		"replace_with_a_random_secret_at_least_32_characters",
		"replace_with_a_local_admin_password":
		return true
	default:
		return false
	}
}

func WriteTemplate(path string) error {
	const template = `# Replace all placeholder credentials before starting the application.
# Secrets may also be supplied through environment variables.

[app]
port = 9000
name = "CRM"
environment = "production"

[db]
host = "localhost"
port = 5432
user = "crm"
password = "crm"
name = "crm"
sslmode = "disable"

[auth]
jwt_secret = "REPLACE_WITH_A_RANDOM_SECRET_AT_LEAST_32_CHARACTERS"
jwt_issuer = "crm"
access_token_ttl = "15m"
refresh_token_ttl = "168h"
bcrypt_cost = 12
secure_cookies = false

[superadmin]
email = "replace-admin@example.test"
password = "REPLACE_WITH_A_LOCAL_ADMIN_PASSWORD"

[log]
level = "info"
format = "json"
`

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create config %q: %w", path, err)
	}
	defer file.Close()
	if _, err := file.WriteString(template); err != nil {
		return fmt.Errorf("write config %q: %w", path, err)
	}
	return nil
}

func (d DB) DSN() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(d.User, d.Password),
		Host:   net.JoinHostPort(d.Host, strconv.Itoa(d.Port)),
		Path:   "/" + d.Name,
	}
	q := u.Query()
	q.Set("sslmode", d.SSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}

// SlogLevel resolves the configured log level, defaulting to info.
func (l Log) SlogLevel() (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(l.Level)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid log level (use debug, info, warn or error)")
	}
}
