package auth

import (
	"crm/internal/config"
	"crm/internal/util"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	db  *sql.DB
	cfg config.Auth
	// dummyHash is compared against the supplied password when no account
	// matches, at the same bcrypt cost the service hashes real passwords
	// with, so unknown and known accounts take comparable time to reject.
	dummyHash string
}

func NewService(db *sql.DB, cfg config.Auth) *Service {
	return &Service{db: db, cfg: cfg, dummyHash: dummyHashForCost(cfg.BcryptCost)}
}

func (s *Service) login(email, password string) (*User, *TokenResponse, bool, error) {
	var u User
	err := s.db.QueryRow(
		`SELECT id, name, email, password_hash, must_change_password, last_login_at FROM users WHERE email = $1 AND deleted_at IS NULL`,
		util.NormalizeEmail(email),
	).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.MustChangePassword, &u.LastLoginAt)
	if err != nil {
		// Equalize response timing: an unknown or deleted account must take
		// about as long as a known one, so latency cannot enumerate accounts.
		_ = comparePassword([]byte(s.dummyHash), []byte(password))
		return nil, nil, false, ErrInvalidCredentials
	}
	if err := comparePassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, nil, false, ErrInvalidCredentials
	}
	resp, err := s.generateTokenPair(u.ID)
	if err != nil {
		return nil, nil, false, err
	}
	s.recordLogin(u.ID)
	return &u, resp, u.MustChangePassword, nil
}

// MustChangePassword reports whether the user is flagged to set a new
// password before using the application. A soft-deleted user reports false,
// preserving the documented behavior that existing access tokens stay valid
// until their short TTL expires.
func (s *Service) MustChangePassword(userID string) (bool, error) {
	var must bool
	err := s.db.QueryRow(
		`SELECT must_change_password FROM users WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&must)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check must_change_password: %w", err)
	}
	return must, nil
}

// recordLogin updates the user's last successful sign-in timestamp. It is a
// best-effort bookkeeping write: a failure must not fail an already-successful
// login or orphan the refresh token just issued.
func (s *Service) recordLogin(userID string) {
	if _, err := s.db.Exec(`UPDATE users SET last_login_at = now() WHERE id = $1`, userID); err != nil {
		slog.Error("record last login", "error", err, "user_id", userID)
	}
}

func (s *Service) refresh(refreshToken string) (*TokenResponse, error) {
	hash := hashToken(refreshToken)

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("refresh: %w", err)
	}
	defer tx.Rollback()

	var userID string
	var revoked bool
	var expiresAt time.Time
	var userDeleted bool
	err = tx.QueryRow(
		`SELECT rt.user_id, rt.revoked, rt.expires_at, (u.deleted_at IS NOT NULL)
		FROM refresh_tokens rt
		JOIN users u ON u.id = rt.user_id
		WHERE rt.token_hash = $1
		FOR UPDATE OF rt`,
		hash,
	).Scan(&userID, &revoked, &expiresAt, &userDeleted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidToken
	}
	if err != nil {
		return nil, fmt.Errorf("refresh: %w", err)
	}
	if revoked || time.Now().After(expiresAt) || userDeleted {
		if _, err := tx.Exec(`UPDATE refresh_tokens SET revoked = true WHERE token_hash = $1`, hash); err != nil {
			return nil, fmt.Errorf("refresh: revoke stale token: %w", err)
		}
		return nil, ErrTokenRevoked
	}
	_, err = tx.Exec(`UPDATE refresh_tokens SET revoked = true WHERE token_hash = $1`, hash)
	if err != nil {
		return nil, fmt.Errorf("refresh: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("refresh: %w", err)
	}
	return s.generateTokenPair(userID)
}

// logout revokes the refresh token and returns the owning user's id, name,
// and email so the caller can record a logout audit entry.
func (s *Service) logout(refreshToken string) (string, string, string, error) {
	hash := hashToken(refreshToken)
	var userID, name, email string
	err := s.db.QueryRow(
		`SELECT u.id, u.name, u.email FROM refresh_tokens rt
		JOIN users u ON u.id = rt.user_id
		WHERE rt.token_hash = $1 AND NOT rt.revoked`,
		hash,
	).Scan(&userID, &name, &email)
	if errors.Is(err, sql.ErrNoRows) {
		// Unknown or already-revoked token: nothing to revoke, no actor to log.
		return "", "", "", nil
	}
	if err != nil {
		return "", "", "", fmt.Errorf("logout: lookup user: %w", err)
	}
	if _, err := s.db.Exec(`UPDATE refresh_tokens SET revoked = true WHERE token_hash = $1`, hash); err != nil {
		return "", "", "", fmt.Errorf("logout: revoke token: %w", err)
	}
	return userID, name, email, nil
}

func (s *Service) generateTokenPair(userID string) (*TokenResponse, error) {
	now := time.Now()
	accessExpiry := now.Add(s.cfg.AccessTokenTTL)
	refreshExpiry := now.Add(s.cfg.RefreshTokenTTL)

	accessToken, err := s.createJWT(userID, accessExpiry)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := generateRandomToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}
	refreshHash := hashToken(refreshToken)
	_, err = s.db.Exec(
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, refreshHash, refreshExpiry,
	)
	if err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessExpiry.Unix(),
	}, nil
}

func (s *Service) createJWT(userID string, expiresAt time.Time) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": expiresAt.Unix(),
		"iat": time.Now().Unix(),
		"iss": s.cfg.JWTIssuer,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

func (s *Service) ValidateJWT(tokenStr string) (string, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return []byte(s.cfg.JWTSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return "", ErrInvalidToken
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", ErrInvalidToken
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", ErrInvalidToken
	}
	if iss, ok := claims["iss"].(string); ok && iss != s.cfg.JWTIssuer {
		return "", ErrInvalidToken
	}
	return sub, nil
}

func (s *Service) getUser(userID string) (*User, error) {
	var u User
	err := s.db.QueryRow(
		`SELECT id, name, email, COALESCE(phone, ''), COALESCE(avatar_url, ''), must_change_password, last_login_at, created_at, updated_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Phone, &u.AvatarURL, &u.MustChangePassword, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Service) updateProfile(userID string, req UpdateProfileRequest) (*User, error) {
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrNameRequired
		}
		if len([]rune(name)) > 100 {
			return nil, ErrNameTooLong
		}
		req.Name = &name
	}
	if req.Phone != nil {
		// Count runes, matching the name check: a phone with multibyte
		// formatting characters must not be rejected below the 20-character
		// limit just because its byte length is larger.
		if len([]rune(*req.Phone)) > 20 {
			return nil, ErrPhoneTooLong
		}
	}
	var u User
	err := s.db.QueryRow(`
		UPDATE users SET
			name = COALESCE($2, name),
			phone = COALESCE($3, phone),
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, name, email, COALESCE(phone, ''), COALESCE(avatar_url, ''), must_change_password, last_login_at, created_at, updated_at`,
		userID, req.Name, req.Phone,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Phone, &u.AvatarURL, &u.MustChangePassword, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}
	return &u, nil
}

// changePassword verifies the current password and replaces it. Every
// existing session for the user is revoked, then the caller receives a fresh
// token pair so the device that changed the password stays signed in. If the
// fresh pair cannot be issued after the revocation committed, the caller is
// logged out too (fail closed), matching the refresh rotation contract.
func (s *Service) changePassword(userID, currentPassword, newPassword string) (*TokenResponse, error) {
	if err := ValidatePassword(newPassword); err != nil {
		return nil, err
	}
	var currentHash string
	err := s.db.QueryRow(
		`SELECT password_hash FROM users WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&currentHash)
	if err != nil {
		return nil, fmt.Errorf("change password: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(currentPassword)); err != nil {
		return nil, ErrIncorrectPassword
	}
	newHash, err := HashPassword(newPassword, s.cfg.BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("change password: hash: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("change password: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`UPDATE users SET password_hash = $2, must_change_password = false, updated_at = now() WHERE id = $1`,
		userID, newHash,
	); err != nil {
		return nil, fmt.Errorf("change password: update: %w", err)
	}
	if _, err := tx.Exec(
		`UPDATE refresh_tokens SET revoked = true WHERE user_id = $1 AND NOT revoked`,
		userID,
	); err != nil {
		return nil, fmt.Errorf("change password: revoke sessions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("change password: %w", err)
	}
	return s.generateTokenPair(userID)
}

func HashPassword(password string, cost int) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	return string(bytes), err
}

// comparePassword is the bcrypt comparison used by login. It is a variable
// so tests can prove both the known and unknown login paths run a
// comparison without relying on brittle wall-clock thresholds.
var comparePassword = bcrypt.CompareHashAndPassword

// dummyPasswordSecret is the plaintext hashed for the login timing equalizer.
const dummyPasswordSecret = "crm-login-timing-equalizer"

var (
	dummyHashMu    sync.Mutex
	dummyHashCache = map[int]string{}
)

// dummyHashForCost returns a bcrypt hash of dummyPasswordSecret at the given
// cost, memoized so repeated NewService calls (e.g. in tests) do not keep
// regenerating it. An invalid configured cost falls back to bcrypt's default
// rather than failing startup, mirroring GenerateFromPassword's own
// leniency for costs below the minimum.
func dummyHashForCost(cost int) string {
	dummyHashMu.Lock()
	defer dummyHashMu.Unlock()
	if h, ok := dummyHashCache[cost]; ok {
		return h
	}
	h, err := bcrypt.GenerateFromPassword([]byte(dummyPasswordSecret), cost)
	if err != nil {
		if h, err = bcrypt.GenerateFromPassword([]byte(dummyPasswordSecret), bcrypt.DefaultCost); err != nil {
			panic("auth: generate dummy password hash: " + err.Error())
		}
	}
	dummyHashCache[cost] = string(h)
	return string(h)
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
