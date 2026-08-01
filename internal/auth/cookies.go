package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"
)

const (
	RefreshCookieName = "crm_refresh"
	CSRFCookieName    = "crm_csrf"
)

type cookieConfig struct {
	secure bool
}

// setRefreshCookie stores the refresh token in an HttpOnly cookie scoped to
// /api/auth so JavaScript never reads the long-lived token. SameSite=Lax keeps
// the cookie out of cross-site requests while allowing same-site navigation.
func (c cookieConfig) setRefreshCookie(w http.ResponseWriter, token string, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    token,
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   c.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(maxAge.Seconds()),
	})
}

func (c cookieConfig) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    "",
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   c.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// setCSRFCookie issues the double-submit token as a readable cookie so the
// client can echo it back in X-CSRF-Token on cookie-authenticated requests.
func (c cookieConfig) setCSRFCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    randomHex(32),
		Path:     "/",
		HttpOnly: false,
		Secure:   c.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	})
}

func (c cookieConfig) clearCSRFCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   c.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func randomHex(size int) string {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
