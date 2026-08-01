package middleware

import (
	"crm/internal/auth"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFAllowsSafeMethods(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	CSRF(handler).ServeHTTP(rr, req)
	if !called {
		t.Error("GET should pass CSRF middleware")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestCSRFMissingCookieAndHeader(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without a CSRF token")
	})
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rr := httptest.NewRecorder()

	CSRF(handler).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestCSRFMismatch(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called on token mismatch")
	})
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "cookie-token"})
	req.Header.Set("X-CSRF-Token", "different-token")
	rr := httptest.NewRecorder()

	CSRF(handler).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestCSRFMatchPasses(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "cookie-token"})
	req.Header.Set("X-CSRF-Token", "cookie-token")
	rr := httptest.NewRecorder()

	CSRF(handler).ServeHTTP(rr, req)
	if !called {
		t.Error("handler should be called when cookie and header match")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestCSRFSkipsWithBearerHeader(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer abc")
	rr := httptest.NewRecorder()

	CSRF(handler).ServeHTTP(rr, req)
	if !called {
		t.Error("Bearer-authenticated requests should skip CSRF validation")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}
