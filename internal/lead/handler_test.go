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

func TestListRejectsInvalidFilters(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))

	for _, q := range []string{
		"stage_id=none",
		"stage_id=not-a-uuid",
		"contact_id=abc",
		"pipeline_id=nope",
		"assigned_to=someone",
		"outcome=maybe",
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/leads?"+q, nil)
		rr := httptest.NewRecorder()
		h.List(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", q, rr.Code)
		}
	}

	// "none" is the unassigned sentinel and is valid only for assigned_to.
	req := httptest.NewRequest(http.MethodGet, "/api/leads?assigned_to=none", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("assigned_to=none: status = %d, want 200", rr.Code)
	}
}

func TestBoardRejectsInvalidFilters(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))

	pipeline := "00000000-0000-0000-0000-000000000001"
	for _, q := range []string{
		"assigned_to=someone",
		"outcome=maybe",
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/leads/board?pipeline_id="+pipeline+"&"+q, nil)
		rr := httptest.NewRecorder()
		h.Board(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", q, rr.Code)
		}
	}
}

func TestStatsRejectsInvalidPipeline(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))

	for _, q := range []string{"", "pipeline_id=not-a-uuid"} {
		req := httptest.NewRequest(http.MethodGet, "/api/leads/stats?"+q, nil)
		rr := httptest.NewRecorder()
		h.Stats(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("%q: status = %d, want 400", q, rr.Code)
		}
	}
}
