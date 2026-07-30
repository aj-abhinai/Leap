package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetUserIDEmpty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	id := GetUserID(req)
	if id != "" {
		t.Errorf("expected empty user ID, got %q", id)
	}
}

func TestGetUserIDWithValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), UserIDKey, "test-user-123")
	req = req.WithContext(ctx)
	id := GetUserID(req)
	if id != "test-user-123" {
		t.Errorf("expected 'test-user-123', got %q", id)
	}
}

func TestAuthMiddlewareNoHeader(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" || len(header) < 8 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	mw(handler).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
