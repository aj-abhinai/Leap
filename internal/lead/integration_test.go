package lead

import (
	"crm/internal/testdb"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreateLeadStoresActorIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	userID := seedTestUser(t, db, "alice@example.com")
	pipelineID, stageID := seedPipelineAndStage(t, db)

	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice Example", Phone: "1234567890"},
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
		NewContact: &NewContact{Name: "Alice Example", Phone: "1234567890"},
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

func TestCreateLeadSnapshotsProgramPriceIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	programID := seedProgram(t, db, "Coaching", 25000)

	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice Example", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
		ProgramID:  &programID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	if created.Value == nil || *created.Value != 25000 {
		t.Errorf("value = %+v, want snapshot 25000", created.Value)
	}
	if created.ProgramID == nil || *created.ProgramID != programID {
		t.Errorf("program_id = %+v, want %q", created.ProgramID, programID)
	}
	if created.ProgramName != "Coaching" {
		t.Errorf("program_name = %q, want Coaching", created.ProgramName)
	}
}

func TestCatalogPriceChangeLeavesLeadValueIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	programID := seedProgram(t, db, "Coaching", 25000)

	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice Example", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
		ProgramID:  &programID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}

	if _, err := db.Exec(`UPDATE programs SET price = 30000 WHERE id = $1`, programID); err != nil {
		t.Fatalf("change catalog price: %v", err)
	}

	got, err := svc.get(created.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.Value == nil || *got.Value != 25000 {
		t.Errorf("value = %+v, want original snapshot 25000 after price change", got.Value)
	}
}

func TestProgramChangeResnapshotsValueIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	programA := seedProgram(t, db, "Coaching", 25000)
	programB := seedProgram(t, db, "Mentorship", 40000)

	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice Example", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
		ProgramID:  &programA,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}

	updated, err := svc.update(created.ID, UpdateRequest{ProgramID: &programB}, "")
	if err != nil {
		t.Fatalf("change program: %v", err)
	}
	if updated.Value == nil || *updated.Value != 40000 {
		t.Errorf("value = %+v, want resnapshot 40000", updated.Value)
	}
	if updated.ProgramName != "Mentorship" {
		t.Errorf("program_name = %q, want Mentorship", updated.ProgramName)
	}
}

func TestArchivedProgramRejectedForNewLeadIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	programID := seedProgram(t, db, "Coaching", 25000)
	if _, err := db.Exec(`UPDATE programs SET deleted_at = now() WHERE id = $1`, programID); err != nil {
		t.Fatalf("archive program: %v", err)
	}

	_, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice Example", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
		ProgramID:  &programID,
	}, "")
	if !errors.Is(err, ErrProgramNotActive) {
		t.Errorf("expected ErrProgramNotActive, got %v", err)
	}
}

func TestCustomValueOverrideRejectedIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	custom := 12345.0
	_, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice Example", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
		Value:      &custom,
	}, "")
	if !errors.Is(err, ErrCustomValueRejected) {
		t.Errorf("expected ErrCustomValueRejected, got %v", err)
	}
}

func TestLeadWithoutProgramHasNullValueIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice Example", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	if created.ProgramID != nil || created.Value != nil {
		t.Errorf("program_id/value = %+v/%+v, want both nil", created.ProgramID, created.Value)
	}
}

func TestOneContactTwoProgramsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	programA := seedProgram(t, db, "Coaching", 25000)
	programB := seedProgram(t, db, "Mentorship", 40000)

	var contactID string
	if err := db.QueryRow(
		`INSERT INTO contacts (name) VALUES ('Alice Example') RETURNING id`,
	).Scan(&contactID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}

	for _, tc := range []struct {
		programID string
		price     float64
	}{
		{programA, 25000},
		{programB, 40000},
	} {
		created, err := svc.create(CreateRequest{
			ContactID:  &contactID,
			PipelineID: pipelineID,
			StageID:    stageID,
			ProgramID:  &tc.programID,
		}, "")
		if err != nil {
			t.Fatalf("create lead for program %q: %v", tc.programID, err)
		}
		if created.Value == nil || *created.Value != tc.price {
			t.Errorf("value = %+v, want %v", created.Value, tc.price)
		}
	}

	leads, total, err := svc.list("", "", contactID, 1, 20)
	if err != nil {
		t.Fatalf("list leads by contact: %v", err)
	}
	if total != 2 || len(leads) != 2 {
		t.Errorf("total = %d, want 2 leads for one contact", total)
	}
}

func TestListCapsPerPageHandlerIntegration(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db), stubPerms{can: true})

	req := httptest.NewRequest(http.MethodGet, "/api/leads?per_page=9999", nil)
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
	if body.Meta.PerPage != 200 {
		t.Errorf("per_page = %d, want capped at 200", body.Meta.PerPage)
	}
}

func TestCreateLeadBlocksWhileProgramLockedIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	programID := seedProgram(t, db, "Coaching", 25000)

	lockTx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin lock tx: %v", err)
	}
	defer lockTx.Rollback()
	if _, err := lockTx.Exec(
		`SELECT price FROM programs WHERE id = $1 FOR UPDATE`,
		programID,
	); err != nil {
		t.Fatalf("lock program: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := svc.create(CreateRequest{
			NewContact: &NewContact{Name: "Alice Example", Phone: "1234567890"},
			PipelineID: pipelineID,
			StageID:    stageID,
			ProgramID:  &programID,
		}, "")
		done <- err
	}()

	select {
	case <-done:
		t.Fatal("create returned while the program row was locked; FOR SHARE lock is missing")
	case <-time.After(300 * time.Millisecond):
	}

	if err := lockTx.Commit(); err != nil {
		t.Fatalf("commit lock tx: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("create after lock release: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("create did not complete after the lock was released")
	}
}

// stubPerms satisfies the handler's PermissionChecker for handler-level tests.
type stubPerms struct {
	can bool
}

func (p stubPerms) UserCan(_ string, _ string) (bool, error) {
	return p.can, nil
}

func TestUpdateLeadMissingReturnsNotFoundIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	_, err := svc.update("00000000-0000-0000-0000-000000000000", UpdateRequest{}, "")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("update missing lead = %v, want ErrNotFound", err)
	}

	if err := svc.delete("00000000-0000-0000-0000-000000000000", ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete missing lead = %v, want ErrNotFound", err)
	}
}

func TestCreateLeadRejectsForeignStageIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineA, _ := seedPipelineAndStage(t, db)
	_, stageB := seedPipelineAndStage(t, db)

	_, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineA,
		StageID:    stageB,
	}, "")
	if !errors.Is(err, ErrStageNotInPipeline) {
		t.Errorf("create with foreign stage = %v, want ErrStageNotInPipeline", err)
	}
}

func TestDeleteActivityScopedToLeadIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	activity, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "note", Description: "hello"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	if err := svc.deleteActivity("00000000-0000-0000-0000-000000000000", activity.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete via wrong lead = %v, want ErrNotFound", err)
	}

	if err := svc.deleteActivity(created.ID, activity.ID); err != nil {
		t.Errorf("delete via owning lead = %v, want nil", err)
	}
}

func seedProgram(t *testing.T, db *sql.DB, name string, price float64) string {
	t.Helper()
	var id string
	if err := db.QueryRow(
		`INSERT INTO programs (name, price) VALUES ($1, $2) RETURNING id`,
		name, price,
	).Scan(&id); err != nil {
		t.Fatalf("seed program: %v", err)
	}
	return id
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
