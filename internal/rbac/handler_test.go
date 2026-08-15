package rbac

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRespondErrorMapsSuperadminAssignmentRestrictionToForbidden(t *testing.T) {
	h := NewHandler(NewService(nil))
	rr := httptest.NewRecorder()

	h.respondError(rr, ErrSuperadminAssignmentRestricted)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "FORBIDDEN" {
		t.Errorf("error code = %q, want FORBIDDEN", body.Error.Code)
	}
}

func TestCreateRoleRejectsBlankName(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/roles",
		strings.NewReader(`{"name":"  \t  "}`),
	)
	rr := httptest.NewRecorder()

	h.CreateRole(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestUpdateRoleRejectsBlankName(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/roles/role-id",
		strings.NewReader(`{"name":"  \t  "}`),
	)
	rr := httptest.NewRecorder()

	h.UpdateRole(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateUserRejectsWeakPassword(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/users",
		strings.NewReader(`{"name":"Rep","email":"rep@example.com","password":"short"}`),
	)
	rr := httptest.NewRecorder()

	h.CreateUser(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "BAD_REQUEST" {
		t.Errorf("error code = %q, want BAD_REQUEST", body.Error.Code)
	}
}

func TestSetRolePermissionsRejectsMissingPermissionIDs(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest(
		http.MethodPut,
		"/api/roles/role-id/permissions",
		strings.NewReader(`{}`),
	)
	rr := httptest.NewRecorder()

	h.SetRolePermissions(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestSetRolePermissionsRejectsInvalidJSON(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest(
		http.MethodPut,
		"/api/roles/role-id/permissions",
		strings.NewReader(`{bad-json`),
	)
	rr := httptest.NewRecorder()

	h.SetRolePermissions(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}
