package config

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	App  App
	DB   DB
	Auth Auth
	Log  Log
}

type App struct {
	Port int    `toml:"port"`
	Name string `toml:"name"`
}

type DB struct {
	Host    string `toml:"host"`
	Port    int    `toml:"port"`
	User    string `toml:"user"`
	Password string `toml:"password"`
	Name    string `toml:"name"`
	SSLMode string `toml:"sslmode"`
}

type Auth struct {
	JWTSecret       string        `toml:"jwt_secret"`
	JWTIssuer       string        `toml:"jwt_issuer"`
	AccessTokenTTL  time.Duration `toml:"access_token_ttl"`
	RefreshTokenTTL time.Duration `toml:"refresh_token_ttl"`
	BcryptCost      int           `toml:"bcrypt_cost"`
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
	return &cfg, nil
}

func (d DB) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode)
}
