package activity

import (
	"crm/internal/testdb"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListCapsPerPageHandlerIntegration(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))

	req := httptest.NewRequest(http.MethodGet, "/api/activity?per_page=9999", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body struct {
		Meta struct {
			PerPage int `json:"per_page"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Meta.PerPage != 100 {
		t.Errorf("per_page = %d, want capped at 100", body.Meta.PerPage)
	}
}
