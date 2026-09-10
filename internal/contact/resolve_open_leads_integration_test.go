package contact

import (
	"crm/internal/testdb"
	"database/sql"
	"testing"
)

func seedOpenPipelineStage(t *testing.T, db *sql.DB) (string, string) {
	t.Helper()
	var pipelineID, stageID string
	if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ('Test Pipeline') RETURNING id`).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name) VALUES ($1, 'New') RETURNING id`, pipelineID).Scan(&stageID); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	return pipelineID, stageID
}

func seedOpenProgram(t *testing.T, db *sql.DB, name string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(`INSERT INTO programs (name, price) VALUES ($1, 25000) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("seed program: %v", err)
	}
	return id
}

func seedLead(t *testing.T, db *sql.DB, contactID, pipelineID, stageID string, programID *string, nickname string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(
		`INSERT INTO leads (nickname, contact_id, pipeline_id, stage_id, program_id, value)
		VALUES ($1, $2, $3, $4, $5, 25000) RETURNING id`,
		nickname, contactID, pipelineID, stageID, programID,
	).Scan(&id); err != nil {
		t.Fatalf("seed lead: %v", err)
	}
	return id
}

func TestResolveByPhoneCarriesOpenLeadsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	created, err := svc.create(CreateRequest{Name: "Alice Example", Phone: "9876543210"})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}
	pipelineID, stageID := seedOpenPipelineStage(t, db)
	programID := seedOpenProgram(t, db, "Coaching")
	leadID := seedLead(t, db, created.ID, pipelineID, stageID, &programID, "Deal One")

	matches, err := svc.resolveByPhone("9876543210")
	if err != nil {
		t.Fatalf("resolve by phone: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(matches))
	}
	leads := matches[0].OpenLeads
	if len(leads) != 1 {
		t.Fatalf("open_leads = %d, want 1", len(leads))
	}
	if leads[0].ID != leadID {
		t.Errorf("open lead id = %q, want %q", leads[0].ID, leadID)
	}
	if leads[0].DisplayName != "Deal One" {
		t.Errorf("display_name = %q, want Deal One", leads[0].DisplayName)
	}
	if leads[0].StageName != "New" {
		t.Errorf("stage_name = %q, want New", leads[0].StageName)
	}
	if leads[0].ProgramName != "Coaching" {
		t.Errorf("program_name = %q, want Coaching", leads[0].ProgramName)
	}
	if leads[0].PipelineID != pipelineID {
		t.Errorf("pipeline_id = %q, want %q", leads[0].PipelineID, pipelineID)
	}
}

func TestResolveByPhoneOmitsClosedAndDeletedLeadsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	created, err := svc.create(CreateRequest{Name: "Alice Example", Phone: "9876543210"})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}
	pipelineID, stageID := seedOpenPipelineStage(t, db)
	programID := seedOpenProgram(t, db, "Coaching")

	closedLead := seedLead(t, db, created.ID, pipelineID, stageID, &programID, "Closed Deal")
	var closedStage string
	if err := db.QueryRow(
		`INSERT INTO lead_stages (pipeline_id, name, "order", outcome) VALUES ($1, 'Won', 1, 'won') RETURNING id`,
		pipelineID,
	).Scan(&closedStage); err != nil {
		t.Fatalf("seed won stage: %v", err)
	}
	if _, err := db.Exec(`UPDATE leads SET stage_id = $1 WHERE id = $2`, closedStage, closedLead); err != nil {
		t.Fatalf("close lead: %v", err)
	}

	deletedLead := seedLead(t, db, created.ID, pipelineID, stageID, &programID, "Deleted Deal")
	if _, err := db.Exec(`UPDATE leads SET deleted_at = now() WHERE id = $1`, deletedLead); err != nil {
		t.Fatalf("soft-delete lead: %v", err)
	}

	matches, err := svc.resolveByPhone("9876543210")
	if err != nil {
		t.Fatalf("resolve by phone: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(matches))
	}
	if len(matches[0].OpenLeads) != 0 {
		t.Errorf("open_leads = %+v, want none (closed and deleted leads do not hold the slot)", matches[0].OpenLeads)
	}
}

func TestResolveByPhoneListsOpenLeadsOldestFirstIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	created, err := svc.create(CreateRequest{Name: "Alice Example", Phone: "9876543210"})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}
	pipelineA, stageA := seedOpenPipelineStage(t, db)
	pipelineB, stageB := seedOpenPipelineStage(t, db)
	programID := seedOpenProgram(t, db, "Coaching")
	first := seedLead(t, db, created.ID, pipelineA, stageA, &programID, "First Deal")
	second := seedLead(t, db, created.ID, pipelineB, stageB, &programID, "Second Deal")

	matches, err := svc.resolveByPhone("9876543210")
	if err != nil {
		t.Fatalf("resolve by phone: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(matches))
	}
	leads := matches[0].OpenLeads
	if len(leads) != 2 {
		t.Fatalf("open_leads = %d, want 2", len(leads))
	}
	if leads[0].ID != first || leads[1].ID != second {
		t.Errorf("open_leads order = [%q, %q], want [%q, %q] (oldest first)",
			leads[0].ID, leads[1].ID, first, second)
	}
}

func TestResolveByPhoneEmptyOpenLeadsIsEmptyListIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	if _, err := svc.create(CreateRequest{Name: "Alice Example", Phone: "9876543210"}); err != nil {
		t.Fatalf("create contact: %v", err)
	}

	matches, err := svc.resolveByPhone("9876543210")
	if err != nil {
		t.Fatalf("resolve by phone: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(matches))
	}
	if matches[0].OpenLeads == nil || len(matches[0].OpenLeads) != 0 {
		t.Errorf("open_leads = %#v, want empty non-nil list", matches[0].OpenLeads)
	}
}
