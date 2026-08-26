package pipeline

import (
	"crm/internal/testdb"
	"database/sql"
	"errors"
	"testing"
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

	err := svc.deletePipeline(pipelineID)
	if !errors.Is(err, ErrInUse) {
		t.Errorf("delete pipeline with leads = %v, want ErrInUse", err)
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

	err := svc.deleteStage(stageID)
	if !errors.Is(err, ErrInUse) {
		t.Errorf("delete stage with leads = %v, want ErrInUse", err)
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
