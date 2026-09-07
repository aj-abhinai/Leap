package tag

import (
	"crm/internal/testdb"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// The delete handler maps the in-use refusal to a 409 with the ERR_IN_USE
// code and the count in the message, and lets free deletes through.
func TestDeleteHandlerRefusesInUseStatus(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	handler := NewHandler(svc)

	status, err := svc.create(CreateRequest{Name: "New", Type: "status"})
	if err != nil {
		t.Fatalf("create status: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO contacts (name, status_id) VALUES ('Alice', $1)`,
		status.ID,
	); err != nil {
		t.Fatalf("seed contact: %v", err)
	}

	r := chi.NewRouter()
	r.Delete("/api/tags/{id}", handler.Delete)
	req := httptest.NewRequest(http.MethodDelete, "/api/tags/"+status.ID, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("delete status = %d, want 409", rr.Code)
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "ERR_IN_USE" {
		t.Errorf("error code = %q, want ERR_IN_USE", body.Error.Code)
	}
	if body.Error.Message != "1 contacts still carry the status \"New\"; move them first" {
		t.Errorf("message = %q, want the count in it", body.Error.Message)
	}
}

func TestDeleteHandlerAllowsFreeTag(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	handler := NewHandler(svc)

	plain, err := svc.create(CreateRequest{Name: "Student", Type: "tag"})
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}

	r := chi.NewRouter()
	r.Delete("/api/tags/{id}", handler.Delete)
	req := httptest.NewRequest(http.MethodDelete, "/api/tags/"+plain.ID, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("delete tag = %d, want 200", rr.Code)
	}
}