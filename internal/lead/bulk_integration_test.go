package lead

import (
	"crm/internal/testdb"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// seedContact inserts a bare contact row — the shape a CSV import leaves
// behind before leads are attached.
func seedContact(t *testing.T, db *sql.DB, name string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(`INSERT INTO contacts (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	return id
}

func countLeads(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM leads`).Scan(&n); err != nil {
		t.Fatalf("count leads: %v", err)
	}
	return n
}

func TestBulkCreateOneLeadPerContactIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	userID := seedTestUser(t, db, "bulk@example.com")
	pipelineID, stageID := seedPipelineAndStage(t, db)
	programID := seedProgram(t, db, "Coaching", 25000)
	contacts := []string{
		seedContact(t, db, "Alice"),
		seedContact(t, db, "Bob"),
		seedContact(t, db, "Carol"),
	}

	resp, err := svc.bulkCreate(BulkCreateRequest{
		ContactIDs: contacts,
		PipelineID: pipelineID,
		StageID:    stageID,
		ProgramID:  programID,
	}, userID)
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}
	if resp.Created != 3 || resp.Skipped != 0 || resp.Failed != 0 || len(resp.Errors) != 0 {
		t.Fatalf("resp = %+v, want 3 created and no row errors", resp)
	}

	// Every lead sits in the requested slot with the program price snapshot.
	var leadCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM leads WHERE stage_id = $1 AND program_id = $2 AND value = 25000`,
		stageID, programID,
	).Scan(&leadCount); err != nil {
		t.Fatalf("count leads: %v", err)
	}
	if leadCount != 3 {
		t.Errorf("leads with stage, program, and price snapshot = %d, want 3", leadCount)
	}

	var auditCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM audit_logs WHERE resource_type = 'lead' AND action = 'create'`,
	).Scan(&auditCount); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if auditCount != 3 {
		t.Errorf("audit rows = %d, want one per created lead", auditCount)
	}
}

func TestBulkCreateSkipsContactHoldingOpenLeadIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	programID := seedProgram(t, db, "Coaching", 25000)
	heldContact := seedContact(t, db, "Alice")
	freshContact := seedContact(t, db, "Bob")

	// A first run gives Alice a lead in the slot; the second run must refuse
	// a duplicate for her and still create Bob's lead.
	first, err := svc.bulkCreate(BulkCreateRequest{
		ContactIDs: []string{heldContact},
		PipelineID: pipelineID,
		StageID:    stageID,
		ProgramID:  programID,
	}, "")
	if err != nil || first.Created != 1 {
		t.Fatalf("seed run = %+v, %v; want 1 created", first, err)
	}

	resp, err := svc.bulkCreate(BulkCreateRequest{
		ContactIDs: []string{heldContact, freshContact},
		PipelineID: pipelineID,
		StageID:    stageID,
		ProgramID:  programID,
	}, "")
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}
	if resp.Created != 1 || resp.Skipped != 1 || resp.Failed != 0 {
		t.Fatalf("resp = %+v, want 1 created, 1 skipped, 0 failed", resp)
	}
	row := resp.Errors[0]
	if row.ContactID != heldContact || row.Outcome != "skipped" || row.Name != "Alice" {
		t.Errorf("skip row = %+v", row)
	}
	if !strings.Contains(row.Reason, "New") || !strings.Contains(row.Reason, "Coaching") {
		t.Errorf("skip reason = %q, want the existing deal's stage and program", row.Reason)
	}
	if got := countLeads(t, db); got != 2 {
		t.Errorf("leads = %d, want 2 (a skip must not write)", got)
	}
}

func TestBulkCreateDifferentProgramAllowedIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	programA := seedProgram(t, db, "Coaching", 25000)
	programB := seedProgram(t, db, "Mentorship", 40000)
	contactID := seedContact(t, db, "Alice")

	for _, programID := range []string{programA, programB} {
		resp, err := svc.bulkCreate(BulkCreateRequest{
			ContactIDs: []string{contactID},
			PipelineID: pipelineID,
			StageID:    stageID,
			ProgramID:  programID,
		}, "")
		if err != nil || resp.Created != 1 {
			t.Fatalf("bulk create for program %q = %+v, %v; want 1 created", programID, resp, err)
		}
	}
}

func TestBulkCreateAfterClosedLeadStartsNewCycleIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	closingStage := seedClosingStage(t, db, pipelineID)
	contactID := seedContact(t, db, "Alice")

	closed, err := svc.create(CreateRequest{
		ContactID:  &contactID,
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	if _, err := svc.update(closed.ID, UpdateRequest{StageID: &closingStage}, ""); err != nil {
		t.Fatalf("close lead: %v", err)
	}

	resp, err := svc.bulkCreate(BulkCreateRequest{
		ContactIDs: []string{contactID},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}
	if resp.Created != 1 || resp.Skipped != 0 || resp.Failed != 0 {
		t.Fatalf("resp = %+v, want a new cycle created", resp)
	}
	if got := countLeads(t, db); got != 2 {
		t.Errorf("leads = %d, want 2 (the closed row plus the new cycle)", got)
	}
}

func TestBulkCreateReportsDeletedContactIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	deleted := seedContact(t, db, "Gone")
	live := seedContact(t, db, "Alice")
	if _, err := db.Exec(`UPDATE contacts SET deleted_at = now() WHERE id = $1`, deleted); err != nil {
		t.Fatalf("soft delete contact: %v", err)
	}

	resp, err := svc.bulkCreate(BulkCreateRequest{
		ContactIDs: []string{deleted, live},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}
	if resp.Created != 1 || resp.Failed != 1 || resp.Skipped != 0 {
		t.Fatalf("resp = %+v, want 1 created, 1 failed", resp)
	}
	row := resp.Errors[0]
	if row.ContactID != deleted || row.Outcome != "failed" || row.Name != "Gone" {
		t.Errorf("failed row = %+v", row)
	}
	if row.Reason != "contact not found or deleted" {
		t.Errorf("reason = %q, want the deleted-contact refusal", row.Reason)
	}
	if got := countLeads(t, db); got != 1 {
		t.Errorf("leads = %d, want 1", got)
	}
}

func TestBulkCreateValidatesSharedFieldsBeforeWritingIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	_, foreignStage := seedPipelineAndStage(t, db)
	closingStage := seedClosingStage(t, db, pipelineID)
	archivedProgram := seedProgram(t, db, "Archived", 5000)
	if _, err := db.Exec(`UPDATE programs SET deleted_at = now() WHERE id = $1`, archivedProgram); err != nil {
		t.Fatalf("archive program: %v", err)
	}
	deadUser := seedTestUser(t, db, "dead@example.com")
	if _, err := db.Exec(`UPDATE users SET deleted_at = now() WHERE id = $1`, deadUser); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	contactID := seedContact(t, db, "Alice")

	cases := []struct {
		name string
		req  BulkCreateRequest
		want error
	}{
		{"stage from another pipeline", BulkCreateRequest{ContactIDs: []string{contactID}, PipelineID: pipelineID, StageID: foreignStage}, ErrStageNotInPipeline},
		{"closing stage", BulkCreateRequest{ContactIDs: []string{contactID}, PipelineID: pipelineID, StageID: closingStage}, ErrClosingStageAtCreate},
		{"archived program", BulkCreateRequest{ContactIDs: []string{contactID}, PipelineID: pipelineID, StageID: stageID, ProgramID: archivedProgram}, ErrProgramNotActive},
		{"deleted assignee", BulkCreateRequest{ContactIDs: []string{contactID}, PipelineID: pipelineID, StageID: stageID, AssignedTo: deadUser}, ErrInvalidAssignee},
		{"malformed assignee", BulkCreateRequest{ContactIDs: []string{contactID}, PipelineID: pipelineID, StageID: stageID, AssignedTo: "not-a-uuid"}, ErrInvalidAssignee},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.bulkCreate(tc.req, ""); !errors.Is(err, tc.want) {
				t.Fatalf("bulk create = %v, want %v", err, tc.want)
			}
			if got := countLeads(t, db); got != 0 {
				t.Errorf("leads = %d, want 0 (fail fast must not write)", got)
			}
		})
	}

	// The same request with valid shared fields still creates, so the
	// refusals above are about the fields, not the fixture.
	if resp, err := svc.bulkCreate(BulkCreateRequest{ContactIDs: []string{contactID}, PipelineID: pipelineID, StageID: stageID}, ""); err != nil || resp.Created != 1 {
		t.Fatalf("valid bulk create = %+v, %v; want 1 created", resp, err)
	}
}

func TestBulkCreateDedupesContactIDsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	contactID := seedContact(t, db, "Alice")

	resp, err := svc.bulkCreate(BulkCreateRequest{
		ContactIDs: []string{contactID, contactID},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}
	if resp.Created != 1 || len(resp.Errors) != 0 {
		t.Fatalf("resp = %+v, want one lead and no row errors", resp)
	}
	if got := countLeads(t, db); got != 1 {
		t.Errorf("leads = %d, want 1 (duplicate ids collapse)", got)
	}
}

func TestBulkCreateRejectsInvalidContactIDListsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	newReq := func(ids []string) BulkCreateRequest {
		return BulkCreateRequest{ContactIDs: ids, PipelineID: pipelineID, StageID: stageID}
	}

	if _, err := svc.bulkCreate(newReq(nil), ""); !errors.Is(err, ErrBulkNoContacts) {
		t.Errorf("empty list = %v, want ErrBulkNoContacts", err)
	}
	if _, err := svc.bulkCreate(newReq([]string{"not-a-uuid"}), ""); !errors.Is(err, ErrBulkInvalidID) {
		t.Errorf("malformed id = %v, want ErrBulkInvalidID", err)
	}

	oversized := make([]string, bulkContactLimit+1)
	for i := range oversized {
		oversized[i] = fmt.Sprintf("00000000-0000-0000-0000-%012d", i)
	}
	if _, err := svc.bulkCreate(newReq(oversized), ""); !errors.Is(err, ErrBulkLimit) {
		t.Errorf("oversized list = %v, want ErrBulkLimit", err)
	}
	if got := countLeads(t, db); got != 0 {
		t.Errorf("leads = %d, want 0 for refused lists", got)
	}
}

func TestBulkCreateRejectsMalformedSharedIDsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	contactID := seedContact(t, db, "Alice")

	cases := []struct {
		name string
		req  BulkCreateRequest
	}{
		{"malformed pipeline", BulkCreateRequest{ContactIDs: []string{contactID}, PipelineID: "not-a-uuid", StageID: stageID}},
		{"malformed stage", BulkCreateRequest{ContactIDs: []string{contactID}, PipelineID: pipelineID, StageID: "not-a-uuid"}},
		{"malformed program", BulkCreateRequest{ContactIDs: []string{contactID}, PipelineID: pipelineID, StageID: stageID, ProgramID: "not-a-uuid"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.bulkCreate(tc.req, ""); !errors.Is(err, ErrBulkInvalidField) {
				t.Fatalf("bulk create = %v, want ErrBulkInvalidField", err)
			}
			if got := countLeads(t, db); got != 0 {
				t.Errorf("leads = %d, want 0 (a malformed id must create nothing)", got)
			}
		})
	}
}

func TestOpenLeadCreatedSinceIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	contactID := seedContact(t, db, "Alice")

	var before time.Time
	if err := db.QueryRow(`SELECT now()`).Scan(&before); err != nil {
		t.Fatalf("read database clock: %v", err)
	}

	created, err := svc.create(CreateRequest{
		ContactID:  &contactID,
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}

	// A lead committed inside the window counts as this run's work — the
	// post-error verification that keeps a committed lead from being
	// reported as failed.
	if committed, err := svc.openLeadCreatedSince(contactID, pipelineID, nil, before); err != nil || !committed {
		t.Errorf("openLeadCreatedSince = %v, %v; want true for the lead just created", committed, err)
	}

	// A window starting after the lead's creation does not match.
	after := created.CreatedAt.Add(time.Second)
	if committed, err := svc.openLeadCreatedSince(contactID, pipelineID, nil, after); err != nil || committed {
		t.Errorf("openLeadCreatedSince after creation = %v, %v; want false", committed, err)
	}

	// Another program's slot does not match.
	programID := seedProgram(t, db, "Coaching", 25000)
	if committed, err := svc.openLeadCreatedSince(contactID, pipelineID, &programID, before); err != nil || committed {
		t.Errorf("openLeadCreatedSince for a different program = %v, %v; want false", committed, err)
	}

	// A soft-deleted lead frees the slot.
	if _, err := db.Exec(`UPDATE leads SET deleted_at = now() WHERE id = $1`, created.ID); err != nil {
		t.Fatalf("soft delete lead: %v", err)
	}
	if committed, err := svc.openLeadCreatedSince(contactID, pipelineID, nil, before); err != nil || committed {
		t.Errorf("openLeadCreatedSince after soft delete = %v, %v; want false", committed, err)
	}
}

func TestBulkCreateFullBatchIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)

	rows, err := db.Query(`INSERT INTO contacts (name)
		SELECT 'Bulk ' || g.i FROM generate_series(1, 500) AS g(i) RETURNING id`)
	if err != nil {
		t.Fatalf("seed contacts: %v", err)
	}
	ids := make([]string, 0, 500)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan contact id: %v", err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate contact ids: %v", err)
	}

	start := time.Now()
	resp, err := svc.bulkCreate(BulkCreateRequest{
		ContactIDs: ids,
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}
	// The measured wall time is the evidence for the 30-second write-timeout
	// budget at the 500-row cap; the test asserts the batch, not a duration.
	t.Logf("500-row bulk run completed in %s", elapsed)
	if resp.Created != 500 || resp.Skipped != 0 || resp.Failed != 0 {
		t.Fatalf("resp = %+v, want the full batch created", resp)
	}
	if got := countLeads(t, db); got != 500 {
		t.Errorf("leads = %d, want 500", got)
	}
}

func TestBulkCreateHandlerReportsOutcomesIntegration(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))

	pipelineID, stageID := seedPipelineAndStage(t, db)
	contactID := seedContact(t, db, "Alice")

	body := fmt.Sprintf(`{"contact_ids":[%q],"pipeline_id":%q,"stage_id":%q}`, contactID, pipelineID, stageID)
	req := httptest.NewRequest(http.MethodPost, "/api/leads/bulk", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.BulkCreate(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rr.Code, rr.Body.String())
	}
	var envelope struct {
		Data BulkCreateResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	resp := envelope.Data
	if resp.Created != 1 {
		t.Errorf("created = %d, want 1 (body: %s)", resp.Created, rr.Body.String())
	}

	for _, tc := range []struct {
		name string
		body string
	}{
		{"invalid json", "{"},
		{"missing pipeline and stage", fmt.Sprintf(`{"contact_ids":[%q]}`, contactID)},
		{"empty contact ids", fmt.Sprintf(`{"contact_ids":[],"pipeline_id":%q,"stage_id":%q}`, pipelineID, stageID)},
		{"malformed pipeline id", fmt.Sprintf(`{"contact_ids":[%q],"pipeline_id":"nope","stage_id":%q}`, contactID, stageID)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/leads/bulk", strings.NewReader(tc.body))
			rr := httptest.NewRecorder()
			h.BulkCreate(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400 (%s)", rr.Code, rr.Body.String())
			}
		})
	}
}
