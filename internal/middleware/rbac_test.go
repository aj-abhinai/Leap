package middleware

import (
	"context"
	"crm/internal/ctxutil"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePermissionChecker struct {
	perms []string
	err   error
}

func (f fakePermissionChecker) GetUserPermissions(string) ([]string, error) {
	return f.perms, f.err
}

func TestRequirePermissionUnauthenticated(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when unauthenticated")
	}
	mw := RequirePermission(fakePermissionChecker{}, "contact:read", handler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	mw(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestRequirePermissionLookupFailure(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when lookup fails")
	}
	mw := RequirePermission(
		fakePermissionChecker{err: errors.New("database down")},
		"contact:read",
		handler,
	)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), ctxutil.UserIDKey, "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	mw(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestRequirePermissionDenied(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without the required permission")
	}
	mw := RequirePermission(
		fakePermissionChecker{perms: []string{"contact:read"}},
		"lead:write",
		handler,
	)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), ctxutil.UserIDKey, "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	mw(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestRequirePermissionAllowed(t *testing.T) {
	var called bool
	handler := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}
	mw := RequirePermission(
		fakePermissionChecker{perms: []string{"contact:read", "lead:write"}},
		"lead:write",
		handler,
	)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), ctxutil.UserIDKey, "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	mw(rr, req)
	if !called {
		t.Fatal("handler should have been called with matching permission")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRequirePermissionWildcard(t *testing.T) {
	var called bool
	handler := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}
	mw := RequirePermission(fakePermissionChecker{perms: []string{"*"}}, "lead:write", handler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), ctxutil.UserIDKey, "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	mw(rr, req)
	if !called {
		t.Fatal("handler should have been called with wildcard permission")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}
