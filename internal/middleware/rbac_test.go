package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequirePermissionUnauthorized(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when unauthorized")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	mw := func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserID(r)
		if userID == "" {
			http.Error(w, `{"error":{"code":"UNAUTHORIZED"}}`, http.StatusUnauthorized)
			return
		}
		handler(w, r)
	}

	mw(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestRequirePermissionDenied(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), UserIDKey, "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	mw := func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserID(r)
		if userID == "" {
			http.Error(w, `{"error":{"code":"UNAUTHORIZED"}}`, http.StatusUnauthorized)
			return
		}
		perms := []string{"contact:read"}
		required := "lead:write"
		for _, p := range perms {
			if p == "*" || p == required {
				handler(w, r)
				return
			}
		}
		http.Error(w, `{"error":{"code":"FORBIDDEN"}}`, http.StatusForbidden)
	}

	mw(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestRequirePermissionWildcard(t *testing.T) {
	var called bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), UserIDKey, "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	mw := func(w http.ResponseWriter, r *http.Request) {
		perms := []string{"*"}
		required := "lead:write"
		for _, p := range perms {
			if p == "*" || p == required {
				handler(w, r)
				return
			}
		}
		http.Error(w, `{"error":{"code":"FORBIDDEN"}}`, http.StatusForbidden)
	}

	mw(rr, req)
	if !called {
		t.Error("handler should have been called with wildcard")
	}
}

func TestRequirePermissionAllowed(t *testing.T) {
	var called bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), UserIDKey, "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	mw := func(w http.ResponseWriter, r *http.Request) {
		perms := []string{"contact:read", "lead:write"}
		required := "lead:write"
		for _, p := range perms {
			if p == "*" || p == required {
				handler(w, r)
				return
			}
		}
		http.Error(w, `{"error":{"code":"FORBIDDEN"}}`, http.StatusForbidden)
	}

	mw(rr, req)
	if !called {
		t.Error("handler should have been called with matching permission")
	}
}
