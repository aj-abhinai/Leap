package lead

import (
	"crm/internal/testdb"
	"database/sql"
	"testing"
)

func TestCreateLeadStoresActorIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	userID := seedTestUser(t, db, "alice@example.com")
	pipelineID, stageID := seedPipelineAndStage(t, db)

	created, err := svc.create(CreateRequest{
		Name:       "Alice Example",
		PipelineID: pipelineID,
		StageID:    stageID,
	}, userID)
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}

	var gotUserID, gotUserName sql.NullString
	err = db.QueryRow(
		`SELECT user_id, user_name FROM audit_logs WHERE resource_type = 'lead' AND resource_id = $1 AND action = 'create'`,
		created.ID,
	).Scan(&gotUserID, &gotUserName)
	if err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if !gotUserID.Valid || gotUserID.String != userID {
		t.Errorf("user_id = %+v, want %q", gotUserID, userID)
	}
	if !gotUserName.Valid || gotUserName.String != "Test User" {
		t.Errorf("user_name = %+v, want %q", gotUserName, "Test User")
	}
}

func TestCreateLeadWithoutActorStoresNullActorIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)

	created, err := svc.create(CreateRequest{
		Name:       "Alice Example",
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}

	var gotUserID, gotUserName sql.NullString
	err = db.QueryRow(
		`SELECT user_id, user_name FROM audit_logs WHERE resource_type = 'lead' AND resource_id = $1 AND action = 'create'`,
		created.ID,
	).Scan(&gotUserID, &gotUserName)
	if err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if gotUserID.Valid {
		t.Errorf("user_id = %+v, want NULL for system action", gotUserID)
	}
	if gotUserName.Valid {
		t.Errorf("user_name = %+v, want NULL for system action", gotUserName)
	}
}

func seedTestUser(t *testing.T, db *sql.DB, email string) string {
	t.Helper()
	var id string
	err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash) VALUES ('Test User', $1, 'hash') RETURNING id`,
		email,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func seedPipelineAndStage(t *testing.T, db *sql.DB) (string, string) {
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
