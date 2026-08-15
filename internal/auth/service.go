package auth

import (
	"crm/internal/config"
	"crm/internal/util"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	db  *sql.DB
	cfg config.Auth
}

func NewService(db *sql.DB, cfg config.Auth) *Service {
	return &Service{db: db, cfg: cfg}
}

func (s *Service) login(email, password string) (*User, *TokenResponse, bool, error) {
	var u User
	err := s.db.QueryRow(
		`SELECT id, name, email, password_hash, must_change_password, last_login_at FROM users WHERE email = $1 AND deleted_at IS NULL`,
		util.NormalizeEmail(email),
	).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.MustChangePassword, &u.LastLoginAt)
	if err != nil {
		return nil, nil, false, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, nil, false, ErrInvalidCredentials
	}
	resp, err := s.generateTokenPair(u.ID)
	if err != nil {
		return nil, nil, false, err
	}
	s.recordLogin(u.ID)
	return &u, resp, u.MustChangePassword, nil
}

// recordLogin updates the user's last successful sign-in timestamp. It is a
// best-effort bookkeeping write: a failure must not fail an already-successful
// login or orphan the refresh token just issued.
func (s *Service) recordLogin(userID string) {
	if _, err := s.db.Exec(`UPDATE users SET last_login_at = now() WHERE id = $1`, userID); err != nil {
		slog.Error("record last login", "error", err, "user_id", userID)
	}
}

// normalizeEmail trims and lowercases so case and whitespace differences in
// stored or submitted addresses never lock a user out of their account.
func normalizeEmail(email string) string {
	return util.NormalizeEmail(email)
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

// logout revokes the refresh token and returns the owning user's id and name
// so the caller can record a logout audit entry.
func (s *Service) logout(refreshToken string) (string, string, error) {
	hash := hashToken(refreshToken)
	var userID, name string
	err := s.db.QueryRow(
		`SELECT u.id, u.name FROM refresh_tokens rt
		JOIN users u ON u.id = rt.user_id
		WHERE rt.token_hash = $1 AND NOT rt.revoked`,
		hash,
	).Scan(&userID, &name)
	if errors.Is(err, sql.ErrNoRows) {
		// Unknown or already-revoked token: nothing to revoke, no actor to log.
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("logout: lookup user: %w", err)
	}
	if _, err := s.db.Exec(`UPDATE refresh_tokens SET revoked = true WHERE token_hash = $1`, hash); err != nil {
		return "", "", fmt.Errorf("logout: revoke token: %w", err)
	}
	return userID, name, nil
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

// changePassword verifies the current password and replaces it. The caller's
// session survives (refreshing with a fresh token); all other sessions for
// the user are revoked.
func (s *Service) changePassword(userID, currentPassword, newPassword string) error {
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	var currentHash string
	err := s.db.QueryRow(
		`SELECT password_hash FROM users WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&currentHash)
	if err != nil {
		return fmt.Errorf("change password: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(currentPassword)); err != nil {
		return ErrIncorrectPassword
	}
	newHash, err := HashPassword(newPassword, s.cfg.BcryptCost)
	if err != nil {
		return fmt.Errorf("change password: hash: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("change password: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`UPDATE users SET password_hash = $2, must_change_password = false, updated_at = now() WHERE id = $1`,
		userID, newHash,
	); err != nil {
		return fmt.Errorf("change password: update: %w", err)
	}
	if _, err := tx.Exec(
		`UPDATE refresh_tokens SET revoked = true WHERE user_id = $1 AND NOT revoked`,
		userID,
	); err != nil {
		return fmt.Errorf("change password: revoke sessions: %w", err)
	}
	return tx.Commit()
}

func HashPassword(password string, cost int) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	return string(bytes), err
}

func generateRandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
