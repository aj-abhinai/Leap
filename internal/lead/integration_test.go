package lead

import (
	"context"
	"crm/internal/testdb"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
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

	leads, total, err := svc.list(ListFilters{ContactID: contactID}, 1, 20)
	if err != nil {
		t.Fatalf("list leads by contact: %v", err)
	}
	if total != 2 || len(leads) != 2 {
		t.Errorf("total = %d, want 2 leads for one contact", total)
	}
}

func TestListCapsPerPageHandlerIntegration(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))

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

func TestCreateActivityDescriptionOptionalHandlerIntegration(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))

	pipelineID, stageID := seedPipelineAndStage(t, db)
	svc := NewService(db)
	lead, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/leads/"+lead.ID+"/activities",
		strings.NewReader(`{"type":"note"}`),
	)
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("id", lead.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
	rr := httptest.NewRecorder()
	h.CreateActivity(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 when description missing; body = %s", rr.Code, rr.Body.String())
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

func TestPatchMissingLeadReturnsNotFoundHandlerIntegration(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/leads/00000000-0000-0000-0000-000000000000",
		strings.NewReader(`{"stage_id":"00000000-0000-0000-0000-000000000000"}`),
	)
	rr := httptest.NewRecorder()
	h.Update(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (not 500)", rr.Code)
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

func TestLeadListFiltersIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	// Pipeline with an open stage, a won closing stage, and a lost closing stage.
	var pipelineID string
	if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ('Filter Pipeline') RETURNING id`).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	var openStage, wonStage, lostStage string
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", outcome) VALUES ($1, 'Open', 0, 'open') RETURNING id`, pipelineID).Scan(&openStage); err != nil {
		t.Fatalf("seed open stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", is_closing, outcome) VALUES ($1, 'Won', 1, true, 'won') RETURNING id`, pipelineID).Scan(&wonStage); err != nil {
		t.Fatalf("seed won stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", is_closing, outcome) VALUES ($1, 'Lost', 2, true, 'lost') RETURNING id`, pipelineID).Scan(&lostStage); err != nil {
		t.Fatalf("seed lost stage: %v", err)
	}

	assignee := seedTestUser(t, db, "assignee@example.com")

	// Alice in the open stage, assigned to assignee.
	alice, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice Example", Phone: "1111111111"},
		PipelineID: pipelineID,
		StageID:    openStage,
		AssignedTo: &assignee,
	}, "")
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	// Bob moved to won.
	bob, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Bob Builder", Phone: "2222222222"},
		PipelineID: pipelineID,
		StageID:    openStage,
	}, "")
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}
	if _, err := svc.update(bob.ID, UpdateRequest{StageID: &wonStage}, ""); err != nil {
		t.Fatalf("move bob to won: %v", err)
	}
	// Carol moved to lost.
	carol, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Carol King", Phone: "3333333333"},
		PipelineID: pipelineID,
		StageID:    openStage,
	}, "")
	if err != nil {
		t.Fatalf("create carol: %v", err)
	}
	if _, err := svc.update(carol.ID, UpdateRequest{StageID: &lostStage}, ""); err != nil {
		t.Fatalf("move carol to lost: %v", err)
	}
	_ = alice

	// Search matches the contact name.
	searched, total, err := svc.list(ListFilters{Search: "builder"}, 1, 50)
	if err != nil {
		t.Fatalf("list search: %v", err)
	}
	if total != 1 || len(searched) != 1 || searched[0].ID != bob.ID {
		t.Errorf("search 'builder' = %+v (total %d), want only Bob", searched, total)
	}

	// Outcome filters by the stage's declared outcome.
	won, total, err := svc.list(ListFilters{Outcome: "won"}, 1, 50)
	if err != nil {
		t.Fatalf("list outcome won: %v", err)
	}
	if total != 1 || len(won) != 1 || won[0].ID != bob.ID {
		t.Errorf("outcome won = %+v (total %d), want only Bob", won, total)
	}
	open, total, err := svc.list(ListFilters{Outcome: "open"}, 1, 50)
	if err != nil {
		t.Fatalf("list outcome open: %v", err)
	}
	if total != 1 || open[0].ID != alice.ID {
		t.Errorf("outcome open = %+v (total %d), want only Alice", open, total)
	}

	// Assigned_to filters by user id and 'none' for unassigned.
	byAssignee, total, err := svc.list(ListFilters{AssignedTo: assignee}, 1, 50)
	if err != nil {
		t.Fatalf("list assigned_to: %v", err)
	}
	if total != 1 || byAssignee[0].ID != alice.ID {
		t.Errorf("assigned_to = %+v (total %d), want only Alice", byAssignee, total)
	}
	unassigned, total, err := svc.list(ListFilters{AssignedTo: "none"}, 1, 50)
	if err != nil {
		t.Fatalf("list unassigned: %v", err)
	}
	if total != 2 || len(unassigned) != 2 {
		t.Errorf("unassigned total = %d (len %d), want 2 (Bob and Carol)", total, len(unassigned))
	}
}

func TestStageMoveSetsOutcomeAndHistoryIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	// Pipeline with a non-closing "New" stage and a closing "Closed Lost" stage.
	var pipelineID string
	if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ('Journey Pipeline') RETURNING id`).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	var openStage, closedStage string
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", is_closing) VALUES ($1, 'Open', 0, false) RETURNING id`, pipelineID).Scan(&openStage); err != nil {
		t.Fatalf("seed open stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", is_closing, outcome) VALUES ($1, 'Closed Lost', 1, true, 'lost') RETURNING id`, pipelineID).Scan(&closedStage); err != nil {
		t.Fatalf("seed closed stage: %v", err)
	}

	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    openStage,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}

	lostReason := "Not intrested"
	updated, err := svc.update(created.ID, UpdateRequest{StageID: &closedStage, LostReason: &lostReason}, "")
	if err != nil {
		t.Fatalf("move to closed: %v", err)
	}
	if updated.Outcome != "lost" {
		t.Errorf("outcome = %q, want lost", updated.Outcome)
	}
	if updated.LostReason != "Not intrested" {
		t.Errorf("lost_reason = %q, want Not intrested", updated.LostReason)
	}

	history, err := svc.listHistory(created.ID)
	if err != nil {
		t.Fatalf("list history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history len = %d, want 1", len(history))
	}
	if history[0].FromStageName != "Open" || history[0].ToStageName != "Closed Lost" {
		t.Errorf("history move = %q -> %q, want Open -> Closed Lost", history[0].FromStageName, history[0].ToStageName)
	}
}

func TestStageMoveOutOfClosingClearsOutcomeIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var pipelineID string
	if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ('Journey Pipeline') RETURNING id`).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	var openStage, closedStage string
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", is_closing) VALUES ($1, 'Open', 0, false) RETURNING id`, pipelineID).Scan(&openStage); err != nil {
		t.Fatalf("seed open stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", is_closing, outcome) VALUES ($1, 'Converted', 1, true, 'won') RETURNING id`, pipelineID).Scan(&closedStage); err != nil {
		t.Fatalf("seed closed stage: %v", err)
	}

	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    openStage,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}

	won, err := svc.update(created.ID, UpdateRequest{StageID: &closedStage}, "")
	if err != nil {
		t.Fatalf("move to Converted: %v", err)
	}
	if won.Outcome != "won" {
		t.Errorf("outcome = %q, want won", won.Outcome)
	}

	// Move back out — outcome should clear.
	back, err := svc.update(created.ID, UpdateRequest{StageID: &openStage}, "")
	if err != nil {
		t.Fatalf("move back: %v", err)
	}
	if back.Outcome != "" || back.LostReason != "" {
		t.Errorf("outcome/lost_reason = %q/%q, want cleared", back.Outcome, back.LostReason)
	}
}

func TestActivityEditFieldsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	lead, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}

	remindAt := time.Now().Add(time.Hour)
	activity, err := svc.createActivity(lead.ID, stageID, "", CreateActivityRequest{
		Type:        "Call 1",
		Description: "Follow up",
		RemindAt:    &remindAt,
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	newType := "Call 2"
	newDesc := "Follow up again"
	newRemind := time.Now().Add(3 * time.Hour).Truncate(time.Microsecond)
	updated, err := svc.updateActivity(lead.ID, activity.ID, "", UpdateActivityRequest{
		Type:        &newType,
		Description: &newDesc,
		RemindAt:    optTime(&newRemind),
	})
	if err != nil {
		t.Fatalf("update activity: %v", err)
	}
	if updated.Type != "Call 2" || updated.Description != "Follow up again" {
		t.Errorf("type/desc = %q/%q, want Call 2/Follow up again", updated.Type, updated.Description)
	}
	if updated.RemindAt == nil || !updated.RemindAt.Equal(newRemind) {
		t.Errorf("remind_at = %v, want %v", updated.RemindAt, newRemind)
	}
	// Re-setting remind_at re-opens the reminder.
	if updated.IsReminded {
		t.Errorf("is_reminded = true, want false after remind_at edit")
	}
}

func TestActivityEditEmptyTypeRejectedIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	lead, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	activity, err := svc.createActivity(lead.ID, stageID, "", CreateActivityRequest{Type: "note", Description: "hello"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	emptyType := ""
	_, err = svc.updateActivity(lead.ID, activity.ID, "", UpdateActivityRequest{Type: &emptyType})
	if err == nil {
		t.Fatal("expected error for empty type, got nil")
	}
}

func TestActivityEditBlankDescriptionAllowedIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	lead, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	activity, err := svc.createActivity(lead.ID, stageID, "", CreateActivityRequest{Type: "note", Description: "hello"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	blankDesc := "   "
	updated, err := svc.updateActivity(lead.ID, activity.ID, "", UpdateActivityRequest{Description: &blankDesc})
	if err != nil {
		t.Fatalf("blank description should be allowed: %v", err)
	}
	if updated.Description != "" {
		t.Errorf("description = %q, want trimmed empty", updated.Description)
	}
}

func TestActivityPatchNullClearsScheduleHandlerIntegration(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))

	pipelineID, stageID := seedPipelineAndStage(t, db)
	svc := NewService(db)
	lead, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	sched := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	act, err := svc.createActivity(lead.ID, stageID, "", CreateActivityRequest{
		Type:        "Call 1",
		ScheduledAt: &sched,
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	// The edit form clears date/time inputs by sending null; a null must
	// clear the stored value rather than being merged away by COALESCE.
	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/leads/"+lead.ID+"/activities/"+act.ID,
		strings.NewReader(`{"scheduled_at":null,"remind_at":null}`),
	)
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("id", lead.ID)
	ctx.URLParams.Add("activity_id", act.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
	rr := httptest.NewRecorder()
	h.UpdateActivity(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}

	var resp Activity
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ScheduledAt != nil || resp.RemindAt != nil {
		t.Errorf("scheduled_at/remind_at = %v/%v, want nil/nil after null patch", resp.ScheduledAt, resp.RemindAt)
	}
}

func TestSnoozeReminderReopensIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	lead, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}

	remindAt := time.Now().Add(-time.Hour) // overdue
	activity, err := svc.createActivity(lead.ID, stageID, "", CreateActivityRequest{
		Type:        "note",
		Description: "hello",
		RemindAt:    &remindAt,
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}
	// Mark reminded so it drops out of pending; snoozing should re-open it.
	if _, err := svc.dismissReminder(lead.ID, activity.ID); err != nil {
		t.Fatalf("dismiss: %v", err)
	}

	future := time.Now().Add(24 * time.Hour).Truncate(time.Microsecond)
	changed, err := svc.snoozeReminder(lead.ID, activity.ID, future)
	if err != nil {
		t.Fatalf("snooze: %v", err)
	}
	if !changed {
		t.Fatal("snooze reported no change")
	}

	var remindAt2 *time.Time
	var isReminded bool
	err = db.QueryRow(
		`SELECT remind_at, is_reminded FROM lead_activities WHERE id = $1`,
		activity.ID,
	).Scan(&remindAt2, &isReminded)
	if err != nil {
		t.Fatalf("reload activity: %v", err)
	}
	if remindAt2 == nil || !remindAt2.Equal(future) {
		t.Errorf("remind_at = %v, want %v", remindAt2, future)
	}
	if isReminded {
		t.Errorf("is_reminded = true, want false after snooze")
	}
}

func TestSnoozeMissingReminderIsCleanNoopIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	changed, err := svc.snoozeReminder("00000000-0000-0000-0000-000000000000", "00000000-0000-0000-0000-000000000000", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("snooze missing: %v", err)
	}
	if changed {
		t.Error("snooze missing id reported a change, want false")
	}
}

func TestSnoozeReminderLessOpenTaskIsCleanNoopIntegration(t *testing.T) {
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
	// An open task with neither a reminder nor a schedule carries no due time
	// and must not be silently converted into a future reminder by snoozing —
	// matching dismissReminder, which treats the same row as a clean no-op.
	var activityID string
	if err := db.QueryRow(
		`INSERT INTO lead_activities (lead_id, stage_id, type)
		VALUES ($1, $2, 'call') RETURNING id`,
		created.ID, stageID,
	).Scan(&activityID); err != nil {
		t.Fatalf("seed activity: %v", err)
	}

	changed, err := svc.snoozeReminder(created.ID, activityID, time.Now().Add(2*time.Hour).UTC().Truncate(time.Second))
	if err != nil {
		t.Fatalf("snooze reminder-less: %v", err)
	}
	if changed {
		t.Error("snooze reminder-less task reported a change, want false")
	}

	var remindAt *time.Time
	if err := db.QueryRow(`SELECT remind_at FROM lead_activities WHERE id = $1`, activityID).Scan(&remindAt); err != nil {
		t.Fatalf("reload activity: %v", err)
	}
	if remindAt != nil {
		t.Errorf("remind_at = %v, want nil (task had no reminder)", remindAt)
	}
}

func TestLeadAssignedToValidatesActiveUserIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	userID := seedTestUser(t, db, "agent@example.com")
	deletedUserID := seedTestUser(t, db, "gone@example.com")
	if _, err := db.Exec(`UPDATE users SET deleted_at = now() WHERE id = $1`, deletedUserID); err != nil {
		t.Fatalf("soft-delete user: %v", err)
	}

	base := CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}

	// Create with an active assignee succeeds.
	live := base
	live.AssignedTo = &userID
	created, err := svc.create(live, "")
	if err != nil {
		t.Fatalf("create with active assignee: %v", err)
	}

	// Create with a deleted assignee is rejected.
	deleted := base
	deleted.AssignedTo = &deletedUserID
	if _, err := svc.create(deleted, ""); !errors.Is(err, ErrInvalidAssignee) {
		t.Errorf("create with deleted assignee = %v, want ErrInvalidAssignee", err)
	}

	// Create with a missing UUID surfaces as a clean validation error.
	missing := base
	missingUUID := "00000000-0000-0000-0000-000000000000"
	missing.AssignedTo = &missingUUID
	if _, err := svc.create(missing, ""); !errors.Is(err, ErrInvalidAssignee) {
		t.Errorf("create with missing assignee = %v, want ErrInvalidAssignee", err)
	}

	// Create with a malformed (non-UUID) assignee is rejected up front instead
	// of surfacing as a Postgres cast error.
	malformed := base
	notAUserID := "not-a-uuid"
	malformed.AssignedTo = &notAUserID
	if _, err := svc.create(malformed, ""); !errors.Is(err, ErrInvalidAssignee) {
		t.Errorf("create with malformed assignee = %v, want ErrInvalidAssignee", err)
	}
	if _, err := svc.update(created.ID, UpdateRequest{AssignedTo: &notAUserID}, ""); !errors.Is(err, ErrInvalidAssignee) {
		t.Errorf("update to malformed assignee = %v, want ErrInvalidAssignee", err)
	}

	// Update reassigns to an active user and rejects a deleted one.
	reassign := userID
	if updated, err := svc.update(created.ID, UpdateRequest{AssignedTo: &reassign}, ""); err != nil {
		t.Fatalf("update to active assignee: %v", err)
	} else if updated.AssignedTo == nil || *updated.AssignedTo != userID {
		t.Errorf("assigned_to after update = %v, want %q", updated.AssignedTo, userID)
	}
	if _, err := svc.update(created.ID, UpdateRequest{AssignedTo: &deletedUserID}, ""); !errors.Is(err, ErrInvalidAssignee) {
		t.Errorf("update to deleted assignee = %v, want ErrInvalidAssignee", err)
	}
}

func TestPatchLeadStageMoveSucceedsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	// Lead in a pipeline with two stages; move between them.
	pipelineID, stageA := seedPipelineAndStage(t, db)
	var stageB string
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name) VALUES ($1, 'Contacted') RETURNING id`, pipelineID).Scan(&stageB); err != nil {
		t.Fatalf("seed second stage: %v", err)
	}
	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageA,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}

	updated, err := svc.update(created.ID, UpdateRequest{StageID: &stageB}, "")
	if err != nil {
		t.Fatalf("move lead: %v", err)
	}
	if updated.StageID != stageB {
		t.Errorf("stage_id = %q, want %q", updated.StageID, stageB)
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

func TestCreateLeadRejectsClosingStageIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var pipelineID string
	if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ('Closing Pipeline') RETURNING id`).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	var closingStage string
	if err := db.QueryRow(
		`INSERT INTO lead_stages (pipeline_id, name, "order", is_closing, outcome) VALUES ($1, 'Closed Lost', 0, true, 'lost') RETURNING id`,
		pipelineID,
	).Scan(&closingStage); err != nil {
		t.Fatalf("seed closing stage: %v", err)
	}

	_, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    closingStage,
	}, "")
	if !errors.Is(err, ErrClosingStageAtCreate) {
		t.Errorf("create into closing stage = %v, want ErrClosingStageAtCreate", err)
	}

	var leadCount, contactCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM leads`).Scan(&leadCount); err != nil {
		t.Fatalf("count leads: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM contacts`).Scan(&contactCount); err != nil {
		t.Fatalf("count contacts: %v", err)
	}
	if leadCount != 0 || contactCount != 0 {
		t.Errorf("expected no leads/contacts after rejected create, got %d/%d", leadCount, contactCount)
	}
}

func TestCreateLeadAuditsContactCreationIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	userID := seedTestUser(t, db, "audit@example.com")
	pipelineID, stageID := seedPipelineAndStage(t, db)

	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Audited Person", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, userID)
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}

	var auditCount int
	var gotUserID, gotUserName sql.NullString
	err = db.QueryRow(
		`SELECT COUNT(*), MIN(user_id::text), MIN(user_name) FROM audit_logs
		WHERE resource_type = 'contact' AND resource_id = $1 AND action = 'create'`,
		created.ContactID,
	).Scan(&auditCount, &gotUserID, &gotUserName)
	if err != nil {
		t.Fatalf("query contact audit: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("audit rows = %d, want 1 for contact created from lead entry", auditCount)
	}
	if !gotUserID.Valid || gotUserID.String != userID {
		t.Errorf("user_id = %+v, want %q", gotUserID, userID)
	}
	if !gotUserName.Valid || gotUserName.String != "Test User" {
		t.Errorf("user_name = %+v, want Test User", gotUserName)
	}
}

func TestCreateLeadAuditWithoutActorIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)

	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "No Actor", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}

	var gotUserID, gotUserName sql.NullString
	err = db.QueryRow(
		`SELECT user_id, user_name FROM audit_logs
		WHERE resource_type = 'contact' AND resource_id = $1 AND action = 'create'`,
		created.ContactID,
	).Scan(&gotUserID, &gotUserName)
	if err != nil {
		t.Fatalf("query contact audit: %v", err)
	}
	if gotUserID.Valid || gotUserName.Valid {
		t.Errorf("expected NULL actor for system-created contact, got %+v/%+v", gotUserID, gotUserName)
	}
}

func TestCreateLeadRejectsDeletedContactIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var contactID string
	if err := db.QueryRow(
		`INSERT INTO contacts (name) VALUES ('Soft Deleted') RETURNING id`,
	).Scan(&contactID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	if _, err := db.Exec(`UPDATE contacts SET deleted_at = now() WHERE id = $1`, contactID); err != nil {
		t.Fatalf("soft-delete contact: %v", err)
	}

	pipelineID, stageID := seedPipelineAndStage(t, db)

	_, err := svc.create(CreateRequest{
		ContactID:  &contactID,
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if !errors.Is(err, ErrContactNotActive) {
		t.Errorf("create with deleted contact = %v, want ErrContactNotActive", err)
	}

	// A malformed contact_id is rejected up front instead of surfacing as a
	// Postgres cast error.
	notAContactID := "not-a-uuid"
	if _, err := svc.create(CreateRequest{
		ContactID:  &notAContactID,
		PipelineID: pipelineID,
		StageID:    stageID,
	}, ""); !errors.Is(err, ErrInvalidContactID) {
		t.Errorf("create with malformed contact_id = %v, want ErrInvalidContactID", err)
	}

	var leadCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM leads`).Scan(&leadCount); err != nil {
		t.Fatalf("count leads: %v", err)
	}
	if leadCount != 0 {
		t.Errorf("expected no lead created, got %d", leadCount)
	}
}

func TestUpdateLeadRejectsDeletedContactIntegration(t *testing.T) {
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

	var deletedID string
	if err := db.QueryRow(
		`INSERT INTO contacts (name) VALUES ('Soft Deleted') RETURNING id`,
	).Scan(&deletedID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	if _, err := db.Exec(`UPDATE contacts SET deleted_at = now() WHERE id = $1`, deletedID); err != nil {
		t.Fatalf("soft-delete contact: %v", err)
	}

	_, err = svc.update(created.ID, UpdateRequest{ContactID: &deletedID}, "")
	if !errors.Is(err, ErrContactNotActive) {
		t.Errorf("update to deleted contact = %v, want ErrContactNotActive", err)
	}

	// A malformed contact_id on update is rejected up front.
	notAContactID := "not-a-uuid"
	if _, err := svc.update(created.ID, UpdateRequest{ContactID: &notAContactID}, ""); !errors.Is(err, ErrInvalidContactID) {
		t.Errorf("update to malformed contact_id = %v, want ErrInvalidContactID", err)
	}
}

func TestResolveOrCreateSkipsDeletedContactsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	// A deleted contact whose phone/email would otherwise match must not be
	// resolved: the lead entry creates a fresh contact instead. The phone and
	// email live in the child tables, where resolution looks.
	var deletedID string
	if err := db.QueryRow(
		`INSERT INTO contacts (name) VALUES ('Deleted Match') RETURNING id`,
	).Scan(&deletedID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO contact_phones (contact_id, value, is_primary) VALUES ($1, '9876543210', true)`,
		deletedID,
	); err != nil {
		t.Fatalf("seed contact phone: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO contact_emails (contact_id, value, is_primary) VALUES ($1, 'match@example.com', true)`,
		deletedID,
	); err != nil {
		t.Fatalf("seed contact email: %v", err)
	}
	if _, err := db.Exec(`UPDATE contacts SET deleted_at = now() WHERE id = $1`, deletedID); err != nil {
		t.Fatalf("soft-delete contact: %v", err)
	}

	pipelineID, stageID := seedPipelineAndStage(t, db)
	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Fresh", Phone: "9876543210"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	if created.ContactID == deletedID {
		t.Error("lead resolved to the soft-deleted contact")
	}
}
