package lead

import (
	"context"
	"crm/internal/testdb"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
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

// The contact created through lead entry stores the phone in the same
// canonical form ('+' + digits) as every other entry point.
func TestLeadEntryContactStoresCanonicalPhoneIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	if _, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice Example", Phone: "98765 43210"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, ""); err != nil {
		t.Fatalf("create lead: %v", err)
	}

	var stored string
	if err := db.QueryRow(`SELECT value FROM contact_phones`).Scan(&stored); err != nil {
		t.Fatalf("load stored phone: %v", err)
	}
	if stored != "+919876543210" {
		t.Errorf("stored phone = %q, want +919876543210", stored)
	}
}

// A phone with no digits at all is not a phone: lead entry rejects it with
// the same sentinel as the contact module, and no contact row is created —
// whether or not an email is also present.
func TestLeadEntryRejectsDigitslessPhoneIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	_, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice Example", Phone: "+"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if !errors.Is(err, ErrInvalidPhone) {
		t.Fatalf("create lead = %v, want ErrInvalidPhone", err)
	}
	var contacts int
	if err := db.QueryRow(`SELECT COUNT(*) FROM contacts`).Scan(&contacts); err != nil {
		t.Fatalf("count contacts: %v", err)
	}
	if contacts != 0 {
		t.Errorf("contacts = %d, want 0 (no empty phone rows)", contacts)
	}

	// A valid email does not excuse the digitless phone: the create is
	// rejected, not silently stripped of the phone.
	_, err = svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Bob Example", Phone: "+", Email: "bob@example.com"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if !errors.Is(err, ErrInvalidPhone) {
		t.Fatalf("create lead with email = %v, want ErrInvalidPhone", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM contacts`).Scan(&contacts); err != nil {
		t.Fatalf("count contacts: %v", err)
	}
	if contacts != 0 {
		t.Errorf("contacts = %d, want 0 (rejected create must not store)", contacts)
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

func TestCreateActivityRejectsEmptyTypeHandlerIntegration(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))
	pipelineID, stageID := seedPipelineAndStage(t, db)
	lead, err := NewService(db).create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/leads/"+lead.ID+"/activities", strings.NewReader(`{"description":"Call attempt"}`))
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("id", lead.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
	rr := httptest.NewRecorder()
	h.CreateActivity(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rr.Code, rr.Body.String())
	}
}

func TestCreateActivityInvalidRangeMapsBadRequestIntegration(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))
	pipelineID, stageID := seedPipelineAndStage(t, db)
	lead, err := NewService(db).create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/leads/"+lead.ID+"/activities", strings.NewReader(`{"type":"Call","scheduled_at":"2026-09-08T15:00:00Z","scheduled_end_at":"2026-09-08T15:00:00Z"}`))
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("id", lead.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
	rr := httptest.NewRecorder()
	h.CreateActivity(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rr.Code, rr.Body.String())
	}
}

func TestStageMoveUsesStageHistoryNotAuditLogIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	pipelineID, firstStageID := seedPipelineAndStage(t, db)
	var secondStageID string
	if err := db.QueryRow(
		`INSERT INTO lead_stages (pipeline_id, name, "order", outcome)
		VALUES ($1, 'Contacted', 2, 'open') RETURNING id`,
		pipelineID,
	).Scan(&secondStageID); err != nil {
		t.Fatalf("create second stage: %v", err)
	}
	lead, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    firstStageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	if _, err := svc.update(lead.ID, UpdateRequest{StageID: &secondStageID}, ""); err != nil {
		t.Fatalf("move lead: %v", err)
	}

	var historyCount, auditCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM lead_stage_history WHERE lead_id = $1`, lead.ID).Scan(&historyCount); err != nil {
		t.Fatalf("count stage history: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE resource_id = $1 AND action = 'move_stage'`, lead.ID).Scan(&auditCount); err != nil {
		t.Fatalf("count move audit rows: %v", err)
	}
	if historyCount != 1 {
		t.Fatalf("stage history rows = %d, want 1", historyCount)
	}
	if auditCount != 0 {
		t.Fatalf("move audit rows = %d, want 0", auditCount)
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

func TestCreateLeadBlocksWhileContactLockedIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	pipelineID, stageID := seedPipelineAndStage(t, db)

	var contactID string
	if err := db.QueryRow(
		`INSERT INTO contacts (name) VALUES ('Locked Contact') RETURNING id`,
	).Scan(&contactID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}

	lockTx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin lock tx: %v", err)
	}
	defer lockTx.Rollback()
	if _, err := lockTx.Exec(
		`SELECT id FROM contacts WHERE id = $1 FOR UPDATE`,
		contactID,
	); err != nil {
		t.Fatalf("lock contact: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		id := contactID
		_, err := svc.create(CreateRequest{
			ContactID:  &id,
			PipelineID: pipelineID,
			StageID:    stageID,
		}, "")
		done <- err
	}()

	select {
	case <-done:
		t.Fatal("create returned while the contact row was locked; FOR SHARE lock is missing")
	case <-time.After(300 * time.Millisecond):
	}

	// The delete that held the lock commits: the create must refuse instead
	// of linking a lead to the hidden contact.
	if _, err := lockTx.Exec(`UPDATE contacts SET deleted_at = now() WHERE id = $1`, contactID); err != nil {
		t.Fatalf("soft-delete contact: %v", err)
	}
	if err := lockTx.Commit(); err != nil {
		t.Fatalf("commit lock tx: %v", err)
	}

	select {
	case err := <-done:
		if !errors.Is(err, ErrContactNotActive) {
			t.Fatalf("create after contact delete = %v, want ErrContactNotActive", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("create did not complete after the contact lock was released")
	}
}

func TestResolveContactBlocksWhileContactLockedIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	pipelineID, stageID := seedPipelineAndStage(t, db)

	const phone = "9876543210"
	var contactID string
	if err := db.QueryRow(
		`INSERT INTO contacts (name) VALUES ('Resolved Contact') RETURNING id`,
	).Scan(&contactID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO contact_phones (contact_id, value, is_primary) VALUES ($1, $2, true)`,
		contactID, phone,
	); err != nil {
		t.Fatalf("seed contact phone: %v", err)
	}

	lockTx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin lock tx: %v", err)
	}
	defer lockTx.Rollback()
	if _, err := lockTx.Exec(
		`SELECT id FROM contacts WHERE id = $1 FOR UPDATE`,
		contactID,
	); err != nil {
		t.Fatalf("lock contact: %v", err)
	}

	created := make(chan *Lead, 1)
	failed := make(chan error, 1)
	go func() {
		lead, err := svc.create(CreateRequest{
			NewContact: &NewContact{Name: "Fresh", Phone: phone},
			PipelineID: pipelineID,
			StageID:    stageID,
		}, "")
		if err != nil {
			failed <- err
			return
		}
		created <- lead
	}()

	select {
	case <-created:
		t.Fatal("create returned while the resolved contact was locked; FOR SHARE lock is missing")
	case err := <-failed:
		t.Fatalf("create failed while the contact was locked: %v", err)
	case <-time.After(300 * time.Millisecond):
	}

	// The delete that held the lock commits: resolving must fall through to a
	// fresh contact, never link the lead to the hidden one.
	if _, err := lockTx.Exec(`UPDATE contacts SET deleted_at = now() WHERE id = $1`, contactID); err != nil {
		t.Fatalf("soft-delete contact: %v", err)
	}
	if err := lockTx.Commit(); err != nil {
		t.Fatalf("commit lock tx: %v", err)
	}

	select {
	case err := <-failed:
		t.Fatalf("create after contact delete: %v", err)
	case lead := <-created:
		if lead.ContactID == contactID {
			t.Fatal("lead linked to the contact that was deleted while resolving")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("create did not complete after the contact lock was released")
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
	userID := seedTestUser(t, db, "delete-activity@example.com")

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

	if err := svc.deleteActivity("00000000-0000-0000-0000-000000000000", activity.ID, userID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete via wrong lead = %v, want ErrNotFound", err)
	}

	if err := svc.deleteActivity(created.ID, activity.ID, userID); err != nil {
		t.Errorf("delete via owning lead = %v, want nil", err)
	}

	// The delete writes an audit entry with the lead identity and actor.
	var auditAction, auditResourceID, auditUserID string
	err = db.QueryRow(
		`SELECT action, resource_id, user_id::text FROM audit_logs
		WHERE resource_type = 'lead' AND action = 'activity/delete'`,
	).Scan(&auditAction, &auditResourceID, &auditUserID)
	if err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if auditAction != "activity/delete" || auditResourceID != created.ID {
		t.Errorf("audit = %q/%q, want activity/delete for lead %q", auditAction, auditResourceID, created.ID)
	}
	if auditUserID != userID {
		t.Errorf("audit user_id = %q, want %q", auditUserID, userID)
	}
}

func TestDeleteLeadCancelsOpenActivitiesIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	userID := seedTestUser(t, db, "delete-lead@example.com")

	pipelineID, stageID := seedPipelineAndStage(t, db)
	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	open, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "call", Description: "open task"})
	if err != nil {
		t.Fatalf("create open activity: %v", err)
	}
	doneFlag := true
	done, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "call", Description: "done task", IsDone: &doneFlag})
	if err != nil {
		t.Fatalf("create done activity: %v", err)
	}

	if err := svc.delete(created.ID, userID); err != nil {
		t.Fatalf("delete lead: %v", err)
	}

	var openCancelled, doneCancelled bool
	if err := db.QueryRow(`SELECT is_cancelled FROM lead_activities WHERE id = $1`, open.ID).Scan(&openCancelled); err != nil {
		t.Fatalf("query open activity: %v", err)
	}
	if err := db.QueryRow(`SELECT is_cancelled FROM lead_activities WHERE id = $1`, done.ID).Scan(&doneCancelled); err != nil {
		t.Fatalf("query done activity: %v", err)
	}
	if !openCancelled {
		t.Error("open activity should be cancelled on lead delete")
	}
	if doneCancelled {
		t.Error("done activity should stay uncancelled on lead delete")
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
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", outcome) VALUES ($1, 'Won', 1, 'won') RETURNING id`, pipelineID).Scan(&wonStage); err != nil {
		t.Fatalf("seed won stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", outcome) VALUES ($1, 'Lost', 2, 'lost') RETURNING id`, pipelineID).Scan(&lostStage); err != nil {
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
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", outcome) VALUES ($1, 'Open', 0, 'open') RETURNING id`, pipelineID).Scan(&openStage); err != nil {
		t.Fatalf("seed open stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", outcome) VALUES ($1, 'Closed Lost', 1, 'lost') RETURNING id`, pipelineID).Scan(&closedStage); err != nil {
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

	var description string
	if err := db.QueryRow(
		`SELECT description FROM audit_logs
		WHERE resource_type = 'lead' AND resource_id = $1 AND action = 'update'`,
		created.ID,
	).Scan(&description); err != nil {
		t.Fatalf("query lost reason audit row: %v", err)
	}
	if !strings.Contains(description, `lost reason "" → "Not intrested"`) {
		t.Errorf("lost reason audit description = %q", description)
	}
}

func TestStageMoveOutOfClosingSpawnsCycleIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var pipelineID string
	if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ('Journey Pipeline') RETURNING id`).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	var openStage, closedStage string
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", outcome) VALUES ($1, 'Open', 0, 'open') RETURNING id`, pipelineID).Scan(&openStage); err != nil {
		t.Fatalf("seed open stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", outcome) VALUES ($1, 'Converted', 1, 'won') RETURNING id`, pipelineID).Scan(&closedStage); err != nil {
		t.Fatalf("seed closed stage: %v", err)
	}

	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    openStage,
		Nickname:   "Cycle One",
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

	// Drag the closed card back to the open stage: a NEW row is spawned for
	// the same contact; the old row stays terminal.
	spawned, err := svc.update(created.ID, UpdateRequest{StageID: &openStage}, "")
	if err != nil {
		t.Fatalf("spawn cycle: %v", err)
	}
	if spawned.ID == created.ID {
		t.Error("expected a new lead row, got the same id")
	}
	if spawned.StageID != openStage {
		t.Errorf("new cycle stage = %q, want open stage", spawned.StageID)
	}
	if spawned.ContactID != created.ContactID {
		t.Errorf("new cycle contact = %q, want %q", spawned.ContactID, created.ContactID)
	}
	if spawned.Nickname != "Cycle One" {
		t.Errorf("new cycle nickname = %q, want Cycle One (carried over)", spawned.Nickname)
	}
	if spawned.AssignedTo != nil {
		t.Errorf("new cycle assignee = %+v, want unassigned", spawned.AssignedTo)
	}
	if spawned.Outcome != "" || spawned.LostReason != "" {
		t.Errorf("new cycle outcome/lost_reason = %q/%q, want empty", spawned.Outcome, spawned.LostReason)
	}

	// The old row is unchanged: still won, still in the closed stage.
	oldRow, err := svc.get(created.ID)
	if err != nil {
		t.Fatalf("get old lead: %v", err)
	}
	if oldRow.StageID != closedStage || oldRow.Outcome != "won" {
		t.Errorf("old row = stage %q outcome %q, want terminal won", oldRow.StageID, oldRow.Outcome)
	}

	// The spawn writes an audit entry naming the closed lead by display name,
	// not its raw UUID.
	var desc string
	err = db.QueryRow(
		`SELECT description FROM audit_logs WHERE resource_type = 'lead' AND resource_id = $1 AND action = 'create'`,
		spawned.ID,
	).Scan(&desc)
	if err != nil {
		t.Fatalf("query spawn audit: %v", err)
	}
	if !strings.Contains(desc, "Cycle One") {
		t.Errorf("spawn audit description = %q, want it to name the closed lead by display name", desc)
	}
	if strings.Contains(desc, created.ID) {
		t.Errorf("spawn audit description = %q, want no raw lead UUID", desc)
	}
}

func TestStageMoveClosedToClosedRejectedIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var pipelineID string
	if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ('Closed Pipeline') RETURNING id`).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	var openStage, lostStage, wonStage string
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", outcome) VALUES ($1, 'Open', 0, 'open') RETURNING id`, pipelineID).Scan(&openStage); err != nil {
		t.Fatalf("seed open stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", outcome) VALUES ($1, 'Lost', 1, 'lost') RETURNING id`, pipelineID).Scan(&lostStage); err != nil {
		t.Fatalf("seed lost stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", outcome) VALUES ($1, 'Won', 2, 'won') RETURNING id`, pipelineID).Scan(&wonStage); err != nil {
		t.Fatalf("seed won stage: %v", err)
	}

	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Bob", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    openStage,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	// Close it as lost first (a lead cannot be created in a closing stage).
	if _, err := svc.update(created.ID, UpdateRequest{StageID: &lostStage}, ""); err != nil {
		t.Fatalf("move to lost: %v", err)
	}

	// A closed lead cannot be re-closed into another closing stage.
	if _, err := svc.update(created.ID, UpdateRequest{StageID: &wonStage}, ""); !errors.Is(err, ErrClosedToClosedMove) {
		t.Errorf("lost → won = %v, want ErrClosedToClosedMove", err)
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
	if _, err := svc.dismissReminder(lead.ID, activity.ID, ""); err != nil {
		t.Fatalf("dismiss: %v", err)
	}

	future := time.Now().Add(24 * time.Hour).Truncate(time.Microsecond)
	changed, err := svc.snoozeReminder(lead.ID, activity.ID, "", future)
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

	changed, err := svc.snoozeReminder("00000000-0000-0000-0000-000000000000", "00000000-0000-0000-0000-000000000000", "", time.Now().Add(time.Hour))
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

	changed, err := svc.snoozeReminder(created.ID, activityID, "", time.Now().Add(2*time.Hour).UTC().Truncate(time.Second))
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

// TestSpawnCycleRejectsSiblingFieldsIntegration asserts a reopen PATCH that
// carries fields other than stage_id is refused and spawns nothing.
func TestSpawnCycleRejectsSiblingFieldsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	closingStage := seedClosingStage(t, db, pipelineID)
	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	if _, err := svc.update(created.ID, UpdateRequest{StageID: &closingStage}, ""); err != nil {
		t.Fatalf("close lead: %v", err)
	}

	note := "dropped?"
	if _, err := svc.update(created.ID, UpdateRequest{StageID: &stageID, Notes: &note}, ""); !errors.Is(err, ErrSpawnOnlyStage) {
		t.Fatalf("reopen with sibling fields = %v, want ErrSpawnOnlyStage", err)
	}

	// The closed row stays terminal and no new cycle was spawned.
	got, err := svc.get(created.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.StageID != closingStage {
		t.Errorf("stage after refused reopen = %q, want the closed stage %q", got.StageID, closingStage)
	}
	var leads int
	if err := db.QueryRow(`SELECT COUNT(*) FROM leads WHERE contact_id = $1`, got.ContactID).Scan(&leads); err != nil {
		t.Fatalf("count leads: %v", err)
	}
	if leads != 1 {
		t.Errorf("lead rows for contact = %d, want 1 (no spawn)", leads)
	}
}

// TestPatchLeadReopenWithSiblingReturns400Integration asserts the handler maps
// ErrSpawnOnlyStage to 400 instead of dropping the sibling fields silently.
func TestPatchLeadReopenWithSiblingReturns400Integration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	h := NewHandler(svc)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	closingStage := seedClosingStage(t, db, pipelineID)
	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Bob", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	if _, err := svc.update(created.ID, UpdateRequest{StageID: &closingStage}, ""); err != nil {
		t.Fatalf("close lead: %v", err)
	}

	note := "x"
	body, _ := json.Marshal(UpdateRequest{StageID: &stageID, Notes: &note})
	req := httptest.NewRequest(http.MethodPatch, "/api/leads/"+created.ID, strings.NewReader(string(body)))
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("id", created.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
	rr := httptest.NewRecorder()

	h.Update(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

// TestUpdateActivityRejectsMalformedQuickReplyIntegration asserts a malformed
// quick_reply_id surfaces as 400, not a 404/500 from the uuid cast.
func TestUpdateActivityRejectsMalformedQuickReplyIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	h := NewHandler(svc)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/leads/"+created.ID+"/activities/"+act.ID,
		strings.NewReader(`{"quick_reply_id":"not-a-uuid"}`),
	)
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("id", created.ID)
	ctx.URLParams.Add("activity_id", act.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
	rr := httptest.NewRecorder()

	h.UpdateActivity(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestNestedReadsRejectDeletedLeadIntegration(t *testing.T) {
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
	if err := svc.delete(created.ID, ""); err != nil {
		t.Fatalf("delete lead: %v", err)
	}

	if _, _, err := svc.listActivities(created.ID, 1, 20); !errors.Is(err, ErrNotFound) {
		t.Errorf("listActivities on deleted lead = %v, want ErrNotFound", err)
	}
	if _, err := svc.listHistory(created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("listHistory on deleted lead = %v, want ErrNotFound", err)
	}
	if _, _, err := svc.listActivities("not-a-uuid", 1, 20); !errors.Is(err, ErrNotFound) {
		t.Errorf("listActivities malformed id = %v, want ErrNotFound", err)
	}
}

func TestDeletedContactIdentityMaskedIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "9876543210"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	if _, err := db.Exec(`UPDATE contacts SET deleted_at = now() WHERE id = $1`, created.ContactID); err != nil {
		t.Fatalf("soft-delete contact: %v", err)
	}

	got, err := svc.get(created.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.ContactName != "Deleted contact" {
		t.Errorf("contact name = %q, want masked", got.ContactName)
	}
	if got.ContactPhone != "" || got.ContactEmail != "" {
		t.Errorf("phone/email = %q/%q, want empty", got.ContactPhone, got.ContactEmail)
	}
	if got.DisplayName != "Deleted contact" {
		t.Errorf("display name = %q, want masked", got.DisplayName)
	}

	// The reminder feed masks the identity the same way.
	if _, err := db.Exec(
		`INSERT INTO lead_activities (lead_id, stage_id, type, remind_at) VALUES ($1, $2, 'call', $3)`,
		created.ID, stageID, time.Now().Add(time.Hour),
	); err != nil {
		t.Fatalf("seed reminder: %v", err)
	}
	reminders, err := svc.getPendingReminders("00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("pending reminders: %v", err)
	}
	if len(reminders) != 1 || reminders[0].LeadDisplayName != "Deleted contact" {
		t.Errorf("reminder display = %+v, want masked", reminders)
	}

	// A lead nickname still identifies the deal.
	nick := "Deal A"
	if _, err := svc.update(created.ID, UpdateRequest{Nickname: &nick}, ""); err != nil {
		t.Fatalf("set nickname: %v", err)
	}
	got, err = svc.get(created.ID)
	if err != nil {
		t.Fatalf("get lead after nickname: %v", err)
	}
	if got.DisplayName != "Deal A" {
		t.Errorf("display name = %q, want the nickname", got.DisplayName)
	}
}

// TestListHistoryDeletedLeadReturns404Integration pins the handler mapping:
// a soft-deleted lead's history must be 404, not a logged 500.
func TestListHistoryDeletedLeadReturns404Integration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	h := NewHandler(svc)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	if err := svc.delete(created.ID, ""); err != nil {
		t.Fatalf("delete lead: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/leads/"+created.ID+"/history", nil)
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("id", created.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
	rr := httptest.NewRecorder()

	h.ListHistory(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

// TestActivityStageNameSurvivesStageDeletionIntegration pins the snapshot: a
// deleted stage clears only the activity's link, and the stored name keeps the
// timeline readable.
func TestActivityStageNameSurvivesStageDeletionIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	var otherStageID string
	if err := db.QueryRow(
		`INSERT INTO lead_stages (pipeline_id, name, "order") VALUES ($1, 'Other', 1) RETURNING id`,
		pipelineID,
	).Scan(&otherStageID); err != nil {
		t.Fatalf("seed other stage: %v", err)
	}
	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	if _, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call"}); err != nil {
		t.Fatalf("create activity: %v", err)
	}

	// Move the lead out, then delete the stage directly: the guard blocks the
	// delete while a lead sits on it, and the snapshot must survive the FK
	// going null.
	if _, err := svc.update(created.ID, UpdateRequest{StageID: &otherStageID}, ""); err != nil {
		t.Fatalf("move lead: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM lead_stages WHERE id = $1`, stageID); err != nil {
		t.Fatalf("delete stage: %v", err)
	}

	acts, _, err := svc.listActivities(created.ID, 1, 20)
	if err != nil {
		t.Fatalf("list activities: %v", err)
	}
	if len(acts) != 1 {
		t.Fatalf("activities = %d, want 1", len(acts))
	}
	if acts[0].StageName != "New" {
		t.Errorf("stage name = %q, want the snapshot New", acts[0].StageName)
	}
	if acts[0].StageID != "" {
		t.Errorf("stage id = %q, want empty after the stage was deleted", acts[0].StageID)
	}
}

// TestRescheduleAfterStageDeletionUsesCurrentStageIntegration covers the new
// SET NULL window: the completed task's stage may be deleted, but the spawned
// follow-up must land in the lead's current stage.
func TestRescheduleAfterStageDeletionUsesCurrentStageIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageA := seedPipelineAndStage(t, db)
	var stageB string
	if err := db.QueryRow(
		`INSERT INTO lead_stages (pipeline_id, name, "order") VALUES ($1, 'Other', 1) RETURNING id`,
		pipelineID,
	).Scan(&stageB); err != nil {
		t.Fatalf("seed stage B: %v", err)
	}
	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageA,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	task, err := svc.createActivity(created.ID, stageA, "", CreateActivityRequest{Type: "Call"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	if _, err := svc.update(created.ID, UpdateRequest{StageID: &stageB}, ""); err != nil {
		t.Fatalf("move lead: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM lead_stages WHERE id = $1`, stageA); err != nil {
		t.Fatalf("delete stage: %v", err)
	}

	done := true
	next := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	if _, err := svc.updateActivity(created.ID, task.ID, "", UpdateActivityRequest{IsDone: &done, RescheduleAt: &next}); err != nil {
		t.Fatalf("reschedule after stage deletion: %v", err)
	}

	acts, _, err := svc.listActivities(created.ID, 1, 20)
	if err != nil {
		t.Fatalf("list activities: %v", err)
	}
	var nextTask *Activity
	for i := range acts {
		if acts[i].ScheduledAt != nil && acts[i].ScheduledAt.Equal(next) {
			nextTask = &acts[i]
		}
	}
	if nextTask == nil {
		t.Fatalf("spawned next task not found among %d activities", len(acts))
	}
	if nextTask.StageID != stageB {
		t.Errorf("next task stage = %q, want the lead's current stage %q", nextTask.StageID, stageB)
	}
}

// TestPatchLeadClosedToClosedReturns422Integration seeds an open, a lost and
// a won stage, moves a lead to lost, then PATCHes it to won and asserts the
// handler responds 422 (not 500) — ErrClosedToClosedMove must map cleanly.
func TestPatchLeadClosedToClosedReturns422Integration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	h := NewHandler(svc)

	var pipelineID string
	if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ('Closed Pipeline') RETURNING id`).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	var openStage, lostStage, wonStage string
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", outcome) VALUES ($1, 'Open', 0, 'open') RETURNING id`, pipelineID).Scan(&openStage); err != nil {
		t.Fatalf("seed open stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", outcome) VALUES ($1, 'Lost', 1, 'lost') RETURNING id`, pipelineID).Scan(&lostStage); err != nil {
		t.Fatalf("seed lost stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", outcome) VALUES ($1, 'Won', 2, 'won') RETURNING id`, pipelineID).Scan(&wonStage); err != nil {
		t.Fatalf("seed won stage: %v", err)
	}

	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Bob", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    openStage,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	if _, err := svc.update(created.ID, UpdateRequest{StageID: &lostStage}, ""); err != nil {
		t.Fatalf("move to lost: %v", err)
	}

	body, _ := json.Marshal(UpdateRequest{StageID: &wonStage})
	req := httptest.NewRequest(http.MethodPatch, "/api/leads/"+created.ID, strings.NewReader(string(body)))
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("id", created.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
	rr := httptest.NewRecorder()

	h.Update(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (not 500)", rr.Code)
	}
}

// TestBoardReturnsStageCountsIntegration seeds two stages, spreads leads
// across them, then asserts the kanban board groups by stage with the true
// per-stage count — this covers the board query, which had no prior test and
// regressed to a duplicate-FROM syntax error.
func TestBoardReturnsStageCountsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var pipelineID string
	if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ('Board Pipeline') RETURNING id`).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	var stageA, stageB string
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order") VALUES ($1, 'New', 0) RETURNING id`, pipelineID).Scan(&stageA); err != nil {
		t.Fatalf("seed stage A: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order") VALUES ($1, 'Contacted', 1) RETURNING id`, pipelineID).Scan(&stageB); err != nil {
		t.Fatalf("seed stage B: %v", err)
	}

	// Two leads in stage A, one in stage B.
	var rangedLeadID string
	for _, st := range []struct {
		stage string
		name  string
		phone string
	}{{stageA, "Alice", "1111111111"}, {stageA, "Bob", "2222222222"}, {stageB, "Carol", "3333333333"}} {
		created, err := svc.create(CreateRequest{
			NewContact: &NewContact{Name: st.name, Phone: st.phone},
			PipelineID: pipelineID,
			StageID:    st.stage,
		}, "")
		if err != nil {
			t.Fatalf("create lead %s: %v", st.name, err)
		}
		if st.name == "Alice" {
			rangedLeadID = created.ID
		}
	}

	scheduledAt := time.Date(2026, time.September, 8, 10, 0, 0, 0, time.UTC)
	scheduledEndAt := scheduledAt.Add(time.Hour)
	if _, err := svc.createActivity(rangedLeadID, stageA, "", CreateActivityRequest{
		Type:           "Call",
		ScheduledAt:    &scheduledAt,
		ScheduledEndAt: &scheduledEndAt,
	}); err != nil {
		t.Fatalf("create range task: %v", err)
	}

	board, err := svc.board(BoardFilters{PipelineID: pipelineID})
	if err != nil {
		t.Fatalf("board: %v", err)
	}

	if len(board.Stages) != 2 {
		t.Fatalf("stages = %d, want 2", len(board.Stages))
	}
	counts := map[string]int{}
	leadsByStage := map[string][]Lead{}
	for _, s := range board.Stages {
		counts[s.StageID] = s.Count
		leadsByStage[s.StageID] = s.Leads
	}
	if counts[stageA] != 2 {
		t.Errorf("count[stage A] = %d, want 2", counts[stageA])
	}
	if counts[stageB] != 1 {
		t.Errorf("count[stage B] = %d, want 1", counts[stageB])
	}
	if len(leadsByStage[stageA]) != 2 {
		t.Errorf("leads in stage A = %d, want 2", len(leadsByStage[stageA]))
	}
	if len(leadsByStage[stageB]) != 1 {
		t.Errorf("leads in stage B = %d, want 1", len(leadsByStage[stageB]))
	}
	for _, l := range leadsByStage[stageA] {
		if l.ID == rangedLeadID {
			if l.NextTaskEndAt == nil || !l.NextTaskEndAt.Equal(scheduledEndAt) {
				t.Errorf("next_task_end_at = %v, want %v", l.NextTaskEndAt, scheduledEndAt)
			}
			return
		}
	}
	t.Errorf("ranged lead %q missing from board", rangedLeadID)
}

// TestStatsReturnsStageCountsAndValueSumsIntegration seeds two pipelines,
// spreads leads valued from seeded program prices across stages, and asserts
// the dashboard aggregate: per-stage counts and summed values for the
// requested pipeline only, with valueless leads counted at zero and empty
// stages absent.
func TestStatsReturnsStageCountsAndValueSumsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	seedPipeline := func(name string) string {
		t.Helper()
		var id string
		if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
			t.Fatalf("seed pipeline %s: %v", name, err)
		}
		return id
	}
	seedStage := func(pipelineID, name string, order int) string {
		t.Helper()
		var id string
		if err := db.QueryRow(
			`INSERT INTO lead_stages (pipeline_id, name, "order") VALUES ($1, $2, $3) RETURNING id`,
			pipelineID, name, order,
		).Scan(&id); err != nil {
			t.Fatalf("seed stage %s: %v", name, err)
		}
		return id
	}

	// Lead values come from the program catalog; seed one price per value.
	program100 := seedProgram(t, db, "Stats Program 100", 100)
	program50 := seedProgram(t, db, "Stats Program 50", 50)
	program25 := seedProgram(t, db, "Stats Program 25", 25)

	pipelineA := seedPipeline("Stats Pipeline A")
	pipelineB := seedPipeline("Stats Pipeline B")
	stageA := seedStage(pipelineA, "New", 0)
	stageB := seedStage(pipelineA, "Contacted", 1)
	seedStage(pipelineA, "Empty", 2)
	stageOther := seedStage(pipelineB, "New", 0)

	for _, in := range []struct {
		name     string
		phone    string
		stage    string
		pipeline string
		program  *string
	}{
		{"Alice", "9000000001", stageA, pipelineA, &program100},
		{"Bob", "9000000002", stageA, pipelineA, &program50},
		{"Carol", "9000000003", stageB, pipelineA, &program25},
		{"Dave", "9000000004", stageB, pipelineA, nil},
		{"Erin", "9000000005", stageOther, pipelineB, &program100},
	} {
		if _, err := svc.create(CreateRequest{
			NewContact: &NewContact{Name: in.name, Phone: in.phone},
			PipelineID: in.pipeline,
			StageID:    in.stage,
			ProgramID:  in.program,
		}, ""); err != nil {
			t.Fatalf("create lead %s: %v", in.name, err)
		}
	}

	stats, err := svc.stats(pipelineA)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if len(stats.Stages) != 2 {
		t.Fatalf("stages = %d, want 2", len(stats.Stages))
	}
	got := map[string]StageStat{}
	for _, st := range stats.Stages {
		got[st.StageID] = st
	}
	want := map[string]StageStat{
		stageA: {StageID: stageA, Count: 2, ValueSum: 150},
		stageB: {StageID: stageB, Count: 2, ValueSum: 25},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stats = %+v, want %+v", got, want)
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
		`INSERT INTO lead_stages (pipeline_id, name, "order", outcome) VALUES ($1, 'Closed Lost', 0, 'lost') RETURNING id`,
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

// TestUpdateEmptyIdentityNoopIntegration asserts that PATCHing a lead with
// empty identity strings keeps the stored values: "" means "field absent" for
// contact_id, pipeline_id, and stage_id, so the request answers 200 instead of
// failing on the NOT NULL contact column or a misleading stage validation.
func TestUpdateEmptyIdentityNoopIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	h := NewHandler(svc)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/leads/"+created.ID,
		strings.NewReader(`{"contact_id":"","pipeline_id":""}`),
	)
	reqCtx := chi.NewRouteContext()
	reqCtx.URLParams.Add("id", created.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, reqCtx))
	rr := httptest.NewRecorder()

	h.Update(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}
	var payload struct {
		Data Lead `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.ContactID != created.ContactID {
		t.Errorf("contact_id = %q, want %q (unchanged)", payload.Data.ContactID, created.ContactID)
	}
	if payload.Data.PipelineID != created.PipelineID {
		t.Errorf("pipeline_id = %q, want %q (unchanged)", payload.Data.PipelineID, created.PipelineID)
	}
	if payload.Data.StageID != created.StageID {
		t.Errorf("stage_id = %q, want %q (unchanged)", payload.Data.StageID, created.StageID)
	}

	// The row was never written with a NULL contact: a fresh read still links
	// the original contact.
	getReq := httptest.NewRequest(http.MethodGet, "/api/leads/"+created.ID, nil)
	getCtx := chi.NewRouteContext()
	getCtx.URLParams.Add("id", created.ID)
	getReq = getReq.WithContext(context.WithValue(getReq.Context(), chi.RouteCtxKey, getCtx))
	getRR := httptest.NewRecorder()
	h.Get(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200; body = %s", getRR.Code, getRR.Body.String())
	}
	var got struct {
		Data Lead `json:"data"`
	}
	if err := json.Unmarshal(getRR.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if got.Data.ContactID != created.ContactID {
		t.Errorf("reloaded contact_id = %q, want %q", got.Data.ContactID, created.ContactID)
	}
}
