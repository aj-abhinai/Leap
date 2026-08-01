package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSetRefreshCookieAttributes(t *testing.T) {
	rec := httptest.NewRecorder()
	cookieConfig{secure: true}.setRefreshCookie(rec, "tok-123", 7*24*time.Hour)

	res := rec.Result()
	defer res.Body.Close()
	cookies := res.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != RefreshCookieName {
		t.Errorf("name = %q, want %q", c.Name, RefreshCookieName)
	}
	if c.Value != "tok-123" {
		t.Errorf("value = %q, want %q", c.Value, "tok-123")
	}
	if !c.HttpOnly {
		t.Error("refresh cookie must be HttpOnly")
	}
	if !c.Secure {
		t.Error("refresh cookie must be Secure when secure_cookies is enabled")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", c.SameSite)
	}
	if c.Path != "/api/auth" {
		t.Errorf("path = %q, want %q", c.Path, "/api/auth")
	}
	if c.MaxAge != int((7 * 24 * time.Hour).Seconds()) {
		t.Errorf("MaxAge = %d, want %d", c.MaxAge, int((7*24*time.Hour).Seconds()))
	}
}

func TestClearRefreshCookieExpiresImmediately(t *testing.T) {
	rec := httptest.NewRecorder()
	cookieConfig{}.clearRefreshCookie(rec)

	res := rec.Result()
	defer res.Body.Close()
	cookies := res.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if c.MaxAge != -1 {
		t.Errorf("MaxAge = %d, want -1", c.MaxAge)
	}
	if !c.HttpOnly {
		t.Error("cleared refresh cookie must stay HttpOnly")
	}
}

func TestSetCSRFCookieIsReadableByJavaScript(t *testing.T) {
	rec := httptest.NewRecorder()
	cookieConfig{}.setCSRFCookie(rec)

	res := rec.Result()
	defer res.Body.Close()
	cookies := res.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if c.HttpOnly {
		t.Error("CSRF cookie must be readable by JavaScript")
	}
	if c.Value == "" {
		t.Error("expected a random CSRF token value")
	}
	if c.Path != "/" {
		t.Errorf("path = %q, want /", c.Path)
	}
}
