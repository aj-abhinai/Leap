package middleware

import (
	"context"
	"crm/internal/ctxutil"
	"crm/internal/rbac"
	"crm/internal/testdb"
	"database/sql"
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

func TestRequireAnyAllowedWhenOneMatches(t *testing.T) {
	var called bool
	handler := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}
	mw := RequireAny(
		fakePermissionChecker{perms: []string{"contact:read"}},
		[]string{"contact:read", "lead:read", "settings:manage"},
		handler,
	)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), ctxutil.UserIDKey, "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	mw(rr, req)
	if !called {
		t.Fatal("handler should have been called when one of the allowed permissions matches")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRequireAnyDeniedWhenNoneMatch(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when no permission matches")
	}
	mw := RequireAny(
		fakePermissionChecker{perms: []string{"contact:read"}},
		[]string{"lead:read", "settings:manage"},
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

func TestRequireAnyWildcardPasses(t *testing.T) {
	var called bool
	handler := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}
	mw := RequireAny(
		fakePermissionChecker{perms: []string{"*"}},
		[]string{"lead:read", "settings:manage"},
		handler,
	)
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

func TestRequireAnyUnauthenticated(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when unauthenticated")
	}
	mw := RequireAny(
		fakePermissionChecker{},
		[]string{"contact:read", "lead:read", "settings:manage"},
		handler,
	)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	mw(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// seedRBACUser inserts a user holding a role with the given permission names
// (creating the permission rows on demand) and returns the user id.
func seedRBACUser(t *testing.T, db *sql.DB, perms ...string) string {
	t.Helper()
	var roleID string
	if err := db.QueryRow(
		`INSERT INTO roles (name) VALUES ('test-role') RETURNING id`,
	).Scan(&roleID); err != nil {
		t.Fatalf("insert role: %v", err)
	}
	for _, perm := range perms {
		if _, err := db.Exec(
			`INSERT INTO permissions (name, description) VALUES ($1, '') ON CONFLICT (name) DO NOTHING`,
			perm,
		); err != nil {
			t.Fatalf("insert permission %s: %v", perm, err)
		}
		if _, err := db.Exec(
			`INSERT INTO role_permissions (role_id, permission_id)
			SELECT $1, id FROM permissions WHERE name = $2`,
			roleID, perm,
		); err != nil {
			t.Fatalf("assign permission %s: %v", perm, err)
		}
	}
	var userID string
	if err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash, role_id) VALUES ('User', 'u@example.com', 'hash', $1) RETURNING id`,
		roleID,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return userID
}

// The audit-log surface gates on settings:manage: a Sales persona (contact
// and lead permissions only) is refused, while a settings:manage holder
// passes — the same permission check the /api/activity route registers.
func TestAuditGateRejectsSalesPersonaIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := rbac.NewService(db)
	salesUser := seedRBACUser(t, db, "contact:read", "contact:write", "lead:read", "lead:write")

	var called bool
	handler := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}
	mw := RequirePermission(svc, "settings:manage", handler)
	req := httptest.NewRequest(http.MethodGet, "/api/activity", nil)
	ctx := context.WithValue(req.Context(), ctxutil.UserIDKey, salesUser)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	mw(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Sales persona audit gate = %d, want 403", rr.Code)
	}
	if called {
		t.Error("handler must not run for a Sales persona")
	}
}

func TestAuditGateAllowsSettingsManagerIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := rbac.NewService(db)
	managerUser := seedRBACUser(t, db, "settings:manage")

	var called bool
	handler := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}
	mw := RequirePermission(svc, "settings:manage", handler)
	req := httptest.NewRequest(http.MethodGet, "/api/activity", nil)
	ctx := context.WithValue(req.Context(), ctxutil.UserIDKey, managerUser)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	mw(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("manager audit gate = %d, want 200", rr.Code)
	}
	if !called {
		t.Error("handler should have run for a settings:manage holder")
	}
}

func TestRequireAnyLookupFailure(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when lookup fails")
	}
	mw := RequireAny(
		fakePermissionChecker{err: errors.New("database down")},
		[]string{"contact:read", "lead:read", "settings:manage"},
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
