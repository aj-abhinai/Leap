package rbac

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
