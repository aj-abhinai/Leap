package pipeline

import (
	"context"
	"crm/internal/testdb"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestUpdatePipelineMissingReturnsNotFoundIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	name := "Renamed"
	_, err := svc.updatePipeline("00000000-0000-0000-0000-000000000000", UpdatePipelineRequest{Name: &name})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("update missing pipeline = %v, want ErrNotFound", err)
	}
}

func TestCreateStageOnMissingPipelineReturnsNotFoundIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	_, err := svc.createStage("00000000-0000-0000-0000-000000000000", CreateStageRequest{Name: "New"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("create stage on missing pipeline = %v, want ErrNotFound", err)
	}
}

func TestDeletePipelineWithLeadsReturnsInUseIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipeline(t, db)
	var contactID string
	if err := db.QueryRow(
		`INSERT INTO contacts (name) VALUES ('Alice') RETURNING id`,
	).Scan(&contactID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	var leadID string
	if err := db.QueryRow(
		`INSERT INTO leads (contact_id, pipeline_id, stage_id) VALUES ($1, $2, $3) RETURNING id`,
		contactID, pipelineID, stageID,
	).Scan(&leadID); err != nil {
		t.Fatalf("seed lead: %v", err)
	}

	_, err := svc.deletePipeline(pipelineID)
	if !errors.Is(err, ErrInUse) {
		t.Errorf("delete pipeline with leads = %v, want ErrInUse", err)
	}
}

func TestDeleteMissingPipelineReturnsNotFoundWithoutAuditIntegration(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))
	missingID := "00000000-0000-0000-0000-000000000000"

	req := httptest.NewRequest(http.MethodDelete, "/api/pipelines/"+missingID, nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", missingID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rr.Code, rr.Body.String())
	}
	var auditCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM audit_logs WHERE resource_type = 'pipeline' AND resource_id = $1 AND action = 'delete'`,
		missingID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if auditCount != 0 {
		t.Errorf("deletion audit rows = %d, want 0", auditCount)
	}
}

func TestDeleteStageWithLeadsReturnsInUseIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipeline(t, db)
	var contactID string
	if err := db.QueryRow(
		`INSERT INTO contacts (name) VALUES ('Alice') RETURNING id`,
	).Scan(&contactID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO leads (contact_id, pipeline_id, stage_id) VALUES ($1, $2, $3)`,
		contactID, pipelineID, stageID,
	); err != nil {
		t.Fatalf("seed lead: %v", err)
	}

	var inUse *StageInUseError
	if _, err := svc.deleteStage(stageID); !errors.As(err, &inUse) {
		t.Errorf("delete stage with leads = %v, want *StageInUseError", err)
	} else if inUse.LeadCount != 1 {
		t.Errorf("blocked lead count = %d, want 1", inUse.LeadCount)
	}
}

func TestDeleteMissingStageReturnsNotFoundWithoutAuditIntegration(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))
	missingID := "00000000-0000-0000-0000-000000000001"

	req := httptest.NewRequest(http.MethodDelete, "/api/pipelines/test/stages/"+missingID, nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("stage_id", missingID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()
	h.DeleteStage(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rr.Code, rr.Body.String())
	}
	var auditCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM audit_logs WHERE resource_type = 'pipeline' AND resource_id = $1 AND action = 'delete'`,
		missingID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if auditCount != 0 {
		t.Errorf("deletion audit rows = %d, want 0", auditCount)
	}
}

// The mutation itself returns the deleted row's name, so the audit snapshot
// cannot race a rename; a missing row returns ErrNotFound.
func TestDeletesReturnNameForAuditIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipeline, err := svc.createPipeline(CreatePipelineRequest{Name: "Sales"})
	if err != nil {
		t.Fatalf("create pipeline: %v", err)
	}
	stage, err := svc.createStage(pipeline.ID, CreateStageRequest{Name: "Qualified"})
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}
	// The pipeline must keep an open stage, so add a second one before deleting
	// the first.
	if _, err := svc.createStage(pipeline.ID, CreateStageRequest{Name: "Follow-up"}); err != nil {
		t.Fatalf("create second stage: %v", err)
	}
	stageName, err := svc.deleteStage(stage.ID)
	if err != nil {
		t.Fatalf("delete stage: %v", err)
	}
	if stageName != "Qualified" {
		t.Errorf("deleted stage name = %q, want Qualified", stageName)
	}
	if _, err := svc.deleteStage(stage.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second stage delete = %v, want ErrNotFound", err)
	}

	name, err := svc.deletePipeline(pipeline.ID)
	if err != nil {
		t.Fatalf("delete pipeline: %v", err)
	}
	if name != "Sales" {
		t.Errorf("deleted pipeline name = %q, want Sales", name)
	}
	if _, err := svc.deletePipeline(pipeline.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete = %v, want ErrNotFound", err)
	}
}

func TestUpdateStageRenamesAndReordersIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipeline(t, db)
	name := "Qualified"
	order := 3
	updated, err := svc.updateStage(stageID, UpdateStageRequest{Name: &name, Order: &order})
	if err != nil {
		t.Fatalf("update stage: %v", err)
	}
	if updated.Name != "Qualified" || updated.Order != 3 {
		t.Errorf("updated stage = %+v, want name Qualified order 3", updated)
	}

	stages, err := svc.listAllStages([]string{pipelineID})
	if err != nil {
		t.Fatalf("list stages: %v", err)
	}
	if got := stages[pipelineID]; len(got) != 1 || got[0].Name != "Qualified" || got[0].Order != 3 {
		t.Errorf("stages after update = %+v, want [Qualified order 3]", got)
	}
}

// An explicit empty outcome is malformed input, not a quiet reopen.
func TestUpdateStageRejectsEmptyOutcomeIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	_, stageID := seedPipeline(t, db)
	empty := ""
	if _, err := svc.updateStage(stageID, UpdateStageRequest{Outcome: &empty}); !errors.Is(err, ErrInvalidStageOutcome) {
		t.Errorf("empty outcome update = %v, want ErrInvalidStageOutcome", err)
	}

	// A missing stage is not-found even when the payload is invalid.
	if _, err := svc.updateStage("00000000-0000-0000-0000-000000000000", UpdateStageRequest{Outcome: &empty}); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing stage with empty outcome = %v, want ErrNotFound", err)
	}
}

func TestUpdateStageMissingReturnsNotFoundIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	name := "Renamed"
	_, err := svc.updateStage("00000000-0000-0000-0000-000000000000", UpdateStageRequest{Name: &name})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("update missing stage = %v, want ErrNotFound", err)
	}
}

func seedPipeline(t *testing.T, db *sql.DB) (string, string) {
	t.Helper()
	var pipelineID string
	if err := db.QueryRow(
		`INSERT INTO pipelines (name) VALUES ('Test Pipeline') RETURNING id`,
	).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	var stageID string
	if err := db.QueryRow(
		`INSERT INTO lead_stages (pipeline_id, name) VALUES ($1, 'New') RETURNING id`,
		pipelineID,
	).Scan(&stageID); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	return pipelineID, stageID
}

func seedLeadInStage(t *testing.T, db *sql.DB, pipelineID, stageID, name string) string {
	t.Helper()
	var contactID string
	if err := db.QueryRow(
		`INSERT INTO contacts (name) VALUES ($1) RETURNING id`, name,
	).Scan(&contactID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	var leadID string
	if err := db.QueryRow(
		`INSERT INTO leads (contact_id, pipeline_id, stage_id) VALUES ($1, $2, $3) RETURNING id`,
		contactID, pipelineID, stageID,
	).Scan(&leadID); err != nil {
		t.Fatalf("seed lead: %v", err)
	}
	return leadID
}

func TestUpdateStageOutcomeBlockedWhileLeadsPresentIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipeline(t, db)
	// A second open stage keeps the last-stage guard out of the way so the
	// lead guard is what the test observes.
	if _, err := svc.createStage(pipelineID, CreateStageRequest{Name: "Second"}); err != nil {
		t.Fatalf("create second stage: %v", err)
	}
	seedLeadInStage(t, db, pipelineID, stageID, "Alice")

	lost := "lost"
	var inUse *StageInUseError
	if _, err := svc.updateStage(stageID, UpdateStageRequest{Outcome: &lost}); !errors.As(err, &inUse) {
		t.Fatalf("outcome change with a live lead = %v, want *StageInUseError", err)
	}
	if inUse.LeadCount != 1 {
		t.Errorf("blocked lead count = %d, want 1", inUse.LeadCount)
	}

	// A soft-deleted lead no longer renders on any screen, so it does not
	// block an outcome change.
	if _, err := db.Exec(`UPDATE leads SET deleted_at = now() WHERE stage_id = $1`, stageID); err != nil {
		t.Fatalf("soft-delete lead: %v", err)
	}
	updated, err := svc.updateStage(stageID, UpdateStageRequest{Outcome: &lost})
	if err != nil {
		t.Fatalf("outcome change with only soft-deleted leads: %v", err)
	}
	if updated.Outcome != OutcomeLost {
		t.Errorf("outcome = %q, want lost", updated.Outcome)
	}
}

func TestDeleteStageBlockedBySoftDeletedLeadIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipeline(t, db)
	leadID := seedLeadInStage(t, db, pipelineID, stageID, "Alice")
	if _, err := db.Exec(`UPDATE leads SET deleted_at = now() WHERE id = $1`, leadID); err != nil {
		t.Fatalf("soft-delete lead: %v", err)
	}

	// The FK still holds the row, so the guard must count soft-deleted leads
	// too and say why instead of leaking a constraint error.
	var inUse *StageInUseError
	if _, err := svc.deleteStage(stageID); !errors.As(err, &inUse) {
		t.Fatalf("delete stage with a soft-deleted lead = %v, want *StageInUseError", err)
	}
	if inUse.LeadCount != 1 {
		t.Errorf("blocked lead count = %d, want 1", inUse.LeadCount)
	}
}

func TestLastStageGuardsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipeline(t, db)

	var lastStage *LastStageError
	if _, err := svc.deleteStage(stageID); !errors.As(err, &lastStage) || lastStage.Outcome != OutcomeOpen {
		t.Fatalf("delete the only open stage = %v, want *LastStageError(open)", err)
	}
	won := "won"
	if _, err := svc.updateStage(stageID, UpdateStageRequest{Outcome: &won}); !errors.As(err, &lastStage) || lastStage.Outcome != OutcomeOpen {
		t.Fatalf("close the only open stage = %v, want *LastStageError(open)", err)
	}

	// With a second open stage the change is allowed.
	if _, err := svc.createStage(pipelineID, CreateStageRequest{Name: "Second"}); err != nil {
		t.Fatalf("create second stage: %v", err)
	}
	if _, err := svc.updateStage(stageID, UpdateStageRequest{Outcome: &won}); err != nil {
		t.Fatalf("close stage with a sibling open stage: %v", err)
	}

	lostStage, err := svc.createStage(pipelineID, CreateStageRequest{Name: "Lost", Outcome: OutcomeLost})
	if err != nil {
		t.Fatalf("create lost stage: %v", err)
	}
	if _, err := svc.deleteStage(lostStage.ID); !errors.As(err, &lastStage) || lastStage.Outcome != OutcomeLost {
		t.Fatalf("delete the only lost stage = %v, want *LastStageError(lost)", err)
	}
	open := "open"
	if _, err := svc.updateStage(lostStage.ID, UpdateStageRequest{Outcome: &open}); !errors.As(err, &lastStage) || lastStage.Outcome != OutcomeLost {
		t.Fatalf("open the only lost stage = %v, want *LastStageError(lost)", err)
	}
}

func TestCreateStageAppendsAndReorderIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, firstID := seedPipeline(t, db)
	second, err := svc.createStage(pipelineID, CreateStageRequest{Name: "Second"})
	if err != nil {
		t.Fatalf("create second stage: %v", err)
	}
	if second.Order != 1 {
		t.Errorf("appended stage order = %d, want 1 (after the seeded order 0)", second.Order)
	}

	// Reversing the list swaps the stored order.
	if err := svc.reorderStages(pipelineID, []string{second.ID, firstID}); err != nil {
		t.Fatalf("reorder stages: %v", err)
	}
	stages, err := svc.listAllStages([]string{pipelineID})
	if err != nil {
		t.Fatalf("list stages: %v", err)
	}
	got := stages[pipelineID]
	if len(got) != 2 || got[0].ID != second.ID || got[0].Order != 0 || got[1].ID != firstID || got[1].Order != 1 {
		t.Errorf("stages after reorder = %+v, want Second(0) then the seeded stage(1)", got)
	}
}

func TestReorderStagesRejectsIncompleteListsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, firstID := seedPipeline(t, db)
	second, err := svc.createStage(pipelineID, CreateStageRequest{Name: "Second"})
	if err != nil {
		t.Fatalf("create second stage: %v", err)
	}

	if err := svc.reorderStages(pipelineID, []string{firstID}); !errors.Is(err, ErrInvalidStageOrder) {
		t.Errorf("partial reorder = %v, want ErrInvalidStageOrder", err)
	}
	if err := svc.reorderStages(pipelineID, []string{firstID, second.ID, "00000000-0000-0000-0000-000000000009"}); !errors.Is(err, ErrInvalidStageOrder) {
		t.Errorf("foreign-id reorder = %v, want ErrInvalidStageOrder", err)
	}
	if err := svc.reorderStages(pipelineID, []string{firstID, firstID}); !errors.Is(err, ErrInvalidStageOrder) {
		t.Errorf("duplicate-id reorder = %v, want ErrInvalidStageOrder", err)
	}
	if err := svc.reorderStages(pipelineID, []string{firstID, "not-a-uuid"}); !errors.Is(err, ErrInvalidStageOrder) {
		t.Errorf("malformed-id reorder = %v, want ErrInvalidStageOrder", err)
	}
	if err := svc.reorderStages("00000000-0000-0000-0000-000000000009", []string{firstID, second.ID}); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing-pipeline reorder = %v, want ErrNotFound", err)
	}
}

func TestUpdateStageInUseReturns409Integration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	h := NewHandler(svc)

	pipelineID, stageID := seedPipeline(t, db)
	if _, err := svc.createStage(pipelineID, CreateStageRequest{Name: "Second"}); err != nil {
		t.Fatalf("create second stage: %v", err)
	}
	seedLeadInStage(t, db, pipelineID, stageID, "Alice")

	req := httptest.NewRequest(http.MethodPatch, "/api/stages/"+stageID, strings.NewReader(`{"outcome":"lost"}`))
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("stage_id", stageID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	h.UpdateStage(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "STAGE_IN_USE") {
		t.Errorf("body = %s, want STAGE_IN_USE code", rr.Body.String())
	}
}
