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
	var leadID string
	if err := db.QueryRow(
		`INSERT INTO leads (name, pipeline_id, stage_id) VALUES ('Alice', $1, $2) RETURNING id`,
		pipelineID, stageID,
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
	if _, err := db.Exec(
		`INSERT INTO leads (name, pipeline_id, stage_id) VALUES ('Alice', $1, $2)`,
		pipelineID, stageID,
	); err != nil {
		t.Fatalf("seed lead: %v", err)
	}

	err := svc.deleteStage(stageID)
	if !errors.Is(err, ErrInUse) {
		t.Errorf("delete stage with leads = %v, want ErrInUse", err)
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
