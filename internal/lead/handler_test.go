package lead

import (
	"crm/internal/testdb"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListAllActivitiesRejectsInvalidFromTo(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))

	for _, q := range []string{"from=not-a-date", "to=not-a-date", "from=2026-13-45T00:00:00Z"} {
		req := httptest.NewRequest(http.MethodGet, "/api/activities?"+q, nil)
		rr := httptest.NewRecorder()
		h.ListAllActivities(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", q, rr.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/activities?from=2026-08-01T00:00:00Z&to=2026-08-31T00:00:00Z", nil)
	rr := httptest.NewRecorder()
	h.ListAllActivities(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("valid from/to: status = %d, want 200", rr.Code)
	}
}