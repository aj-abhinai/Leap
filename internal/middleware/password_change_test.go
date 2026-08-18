package middleware

import (
	"context"
	"crm/internal/ctxutil"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePasswordChangeChecker struct {
	must bool
	err  error
}

func (f fakePasswordChangeChecker) MustChangePassword(string) (bool, error) {
	return f.must, f.err
}

func TestRequirePasswordChangedUnauthenticated(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when unauthenticated")
	}
	mw := RequirePasswordChanged(fakePasswordChangeChecker{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	mw(http.HandlerFunc(handler)).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestRequirePasswordChangedLookupFailure(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when lookup fails")
	}
	mw := RequirePasswordChanged(fakePasswordChangeChecker{err: errors.New("database down")})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), ctxutil.UserIDKey, "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	mw(http.HandlerFunc(handler)).ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestRequirePasswordChangedBlockedWhileFlagged(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for a flagged account")
	}
	mw := RequirePasswordChanged(fakePasswordChangeChecker{must: true})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), ctxutil.UserIDKey, "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	mw(http.HandlerFunc(handler)).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestRequirePasswordChangedAllowedAfterChange(t *testing.T) {
	var called bool
	handler := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}
	mw := RequirePasswordChanged(fakePasswordChangeChecker{must: false})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), ctxutil.UserIDKey, "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	mw(http.HandlerFunc(handler)).ServeHTTP(rr, req)
	if !called {
		t.Fatal("handler should have been called once the flag is cleared")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}