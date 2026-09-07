package lead

import (
	"bytes"
	"crm/internal/testdb"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func openLeadConflict(t *testing.T, err error) *OpenLeadConflictError {
	t.Helper()
	var conflict *OpenLeadConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("err = %v, want *OpenLeadConflictError", err)
	}
	return conflict
}

func TestCreateRefusesDuplicateOpenLeadIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	programID := seedProgram(t, db, "Coaching", 25000)

	first, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
		ProgramID:  &programID,
		Nickname:   "Deal One",
	}, "")
	if err != nil {
		t.Fatalf("create first lead: %v", err)
	}

	_, err = svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
		ProgramID:  &programID,
	}, "")
	conflict := openLeadConflict(t, err)
	if conflict.Lead.ID != first.ID {
		t.Errorf("conflict lead id = %q, want %q", conflict.Lead.ID, first.ID)
	}
	if conflict.Lead.DisplayName != "Deal One" {
		t.Errorf("conflict display_name = %q, want Deal One (nickname preferred)", conflict.Lead.DisplayName)
	}
	if conflict.Lead.StageName != "New" {
		t.Errorf("conflict stage_name = %q, want New", conflict.Lead.StageName)
	}
	if conflict.Lead.ProgramName != "Coaching" {
		t.Errorf("conflict program_name = %q, want Coaching", conflict.Lead.ProgramName)
	}
	if conflict.Lead.PipelineID != pipelineID {
		t.Errorf("conflict pipeline_id = %q, want %q", conflict.Lead.PipelineID, pipelineID)
	}

	var leadCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM leads`).Scan(&leadCount); err != nil {
		t.Fatalf("count leads: %v", err)
	}
	if leadCount != 1 {
		t.Errorf("lead count = %d, want 1 (refused create must not write)", leadCount)
	}
}

func TestDifferentProgramSamePipelineAllowedIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	programA := seedProgram(t, db, "Coaching", 25000)
	programB := seedProgram(t, db, "Mentorship", 40000)

	for _, programID := range []string{programA, programB} {
		if _, err := svc.create(CreateRequest{
			NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
			PipelineID: pipelineID,
			StageID:    stageID,
			ProgramID:  &programID,
		}, ""); err != nil {
			t.Fatalf("create lead for program %q: %v", programID, err)
		}
	}
}

func TestSameProgramDifferentPipelineAllowedIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	programID := seedProgram(t, db, "Coaching", 25000)
	pipelineA, stageA := seedPipelineAndStage(t, db)
	pipelineB, stageB := seedPipelineAndStage(t, db)

	for _, slot := range []struct {
		pipeline, stage string
	}{{pipelineA, stageA}, {pipelineB, stageB}} {
		if _, err := svc.create(CreateRequest{
			NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
			PipelineID: slot.pipeline,
			StageID:    slot.stage,
			ProgramID:  &programID,
		}, ""); err != nil {
			t.Fatalf("create lead in pipeline %q: %v", slot.pipeline, err)
		}
	}
}

func TestProgramlessDuplicateRefusedIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)

	_, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create first lead: %v", err)
	}

	// A second program-less lead for the same contact and pipeline is the same
	// noise as a duplicate program lead.
	_, err = svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	openLeadConflict(t, err)

	// An explicit empty program targets the same program-less slot.
	empty := ""
	_, err = svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
		ProgramID:  &empty,
	}, "")
	openLeadConflict(t, err)
}

func TestCloseThenCreateAllowedIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var pipelineID string
	if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ('Cycle Pipeline') RETURNING id`).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	var openStage, closedStage string
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order") VALUES ($1, 'Open', 0) RETURNING id`, pipelineID).Scan(&openStage); err != nil {
		t.Fatalf("seed open stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", is_closing, outcome) VALUES ($1, 'Won', 1, true, 'won') RETURNING id`, pipelineID).Scan(&closedStage); err != nil {
		t.Fatalf("seed won stage: %v", err)
	}
	programID := seedProgram(t, db, "Coaching", 25000)

	first, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    openStage,
		ProgramID:  &programID,
	}, "")
	if err != nil {
		t.Fatalf("create first lead: %v", err)
	}

	// Closing the deal frees the slot: sequential repeat business is the cycle
	// model, not a duplicate.
	if _, err := svc.update(first.ID, UpdateRequest{StageID: &closedStage}, ""); err != nil {
		t.Fatalf("close lead: %v", err)
	}
	if _, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    openStage,
		ProgramID:  &programID,
	}, ""); err != nil {
		t.Fatalf("create after close: %v", err)
	}
}

func TestSoftDeletedLeadDoesNotBlockSlotIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	programID := seedProgram(t, db, "Coaching", 25000)

	first, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
		ProgramID:  &programID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}

	if err := svc.delete(first.ID, ""); err != nil {
		t.Fatalf("delete lead: %v", err)
	}

	// Only live leads hold the slot; a soft-deleted lead frees it.
	if _, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
		ProgramID:  &programID,
	}, ""); err != nil {
		t.Fatalf("create after soft delete: %v", err)
	}
}

func TestSpawnCycleBlockedWhenSlotHeldIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var pipelineID string
	if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ('Spawn Pipeline') RETURNING id`).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	var openStage, closedStage string
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order") VALUES ($1, 'Open', 0) RETURNING id`, pipelineID).Scan(&openStage); err != nil {
		t.Fatalf("seed open stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", is_closing, outcome) VALUES ($1, 'Lost', 1, true, 'lost') RETURNING id`, pipelineID).Scan(&closedStage); err != nil {
		t.Fatalf("seed lost stage: %v", err)
	}
	programID := seedProgram(t, db, "Coaching", 25000)

	// A closed lead whose slot was taken over by a newer open lead.
	closed, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    openStage,
		ProgramID:  &programID,
	}, "")
	if err != nil {
		t.Fatalf("create first lead: %v", err)
	}
	if _, err := svc.update(closed.ID, UpdateRequest{StageID: &closedStage}, ""); err != nil {
		t.Fatalf("close lead: %v", err)
	}
	holder, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    openStage,
		ProgramID:  &programID,
	}, "")
	if err != nil {
		t.Fatalf("create holder lead: %v", err)
	}

	// Drag-reopen of the closed lead targets a held slot and must refuse.
	_, err = svc.update(closed.ID, UpdateRequest{StageID: &openStage}, "")
	conflict := openLeadConflict(t, err)
	if conflict.Lead.ID != holder.ID {
		t.Errorf("conflict lead id = %q, want %q (the slot holder)", conflict.Lead.ID, holder.ID)
	}
}

func TestUpdateSlotKeyBlockedOnCollisionIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var pipelineID string
	if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ('Patch Pipeline') RETURNING id`).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	var stageA, stageB string
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order") VALUES ($1, 'New', 0) RETURNING id`, pipelineID).Scan(&stageA); err != nil {
		t.Fatalf("seed stage A: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order") VALUES ($1, 'Warm', 1) RETURNING id`, pipelineID).Scan(&stageB); err != nil {
		t.Fatalf("seed stage B: %v", err)
	}
	programX := seedProgram(t, db, "Coaching", 25000)
	programY := seedProgram(t, db, "Mentorship", 40000)

	holder, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageA,
		ProgramID:  &programX,
	}, "")
	if err != nil {
		t.Fatalf("create holder lead: %v", err)
	}
	mover, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageA,
		ProgramID:  &programY,
	}, "")
	if err != nil {
		t.Fatalf("create mover lead: %v", err)
	}

	// Re-setting the holder's own program is a no-op key change and stays legal.
	if _, err := svc.update(holder.ID, UpdateRequest{ProgramID: &programX}, ""); err != nil {
		t.Fatalf("no-op program patch = %v, want allowed", err)
	}

	// Moving the other lead into the held program slot must refuse.
	_, err = svc.update(mover.ID, UpdateRequest{ProgramID: &programX}, "")
	conflict := openLeadConflict(t, err)
	if conflict.Lead.ID != holder.ID {
		t.Errorf("conflict lead id = %q, want %q", conflict.Lead.ID, holder.ID)
	}

	// Changing the contact into one that holds the slot must refuse too: bob
	// already has an open lead in the mover's (pipeline, program) slot.
	other, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Bob", Phone: "9999999999"},
		PipelineID: pipelineID,
		StageID:    stageA,
		ProgramID:  &programY,
	}, "")
	if err != nil {
		t.Fatalf("create other-contact lead: %v", err)
	}
	_, err = svc.update(mover.ID, UpdateRequest{ContactID: &other.ContactID}, "")
	openLeadConflict(t, err)

	// Clearing the program into a held program-less slot must refuse.
	empty := ""
	if _, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageA,
	}, ""); err != nil {
		t.Fatalf("create program-less lead: %v", err)
	}
	_, err = svc.update(mover.ID, UpdateRequest{ProgramID: &empty}, "")
	openLeadConflict(t, err)

	// Stage moves never change the slot key and are always exempt.
	if _, err := svc.update(mover.ID, UpdateRequest{StageID: &stageB}, ""); err != nil {
		t.Fatalf("stage move = %v, want allowed", err)
	}
}

func TestUpdateSlotKeyIntoClosingStageAllowedIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var pipelineID string
	if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ('Close Pipeline') RETURNING id`).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	var openStage, openStageB, wonStage string
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order") VALUES ($1, 'New', 0) RETURNING id`, pipelineID).Scan(&openStage); err != nil {
		t.Fatalf("seed open stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order") VALUES ($1, 'Warm', 1) RETURNING id`, pipelineID).Scan(&openStageB); err != nil {
		t.Fatalf("seed open stage B: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", is_closing, outcome) VALUES ($1, 'Won', 2, true, 'won') RETURNING id`, pipelineID).Scan(&wonStage); err != nil {
		t.Fatalf("seed won stage: %v", err)
	}
	programX := seedProgram(t, db, "Coaching", 25000)
	programY := seedProgram(t, db, "Mentorship", 40000)

	// The slot (contact, pipeline, program X) is held by lead A; lead B sits
	// in program Y.
	if _, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    openStage,
		ProgramID:  &programX,
	}, ""); err != nil {
		t.Fatalf("create holder lead: %v", err)
	}
	b, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    openStage,
		ProgramID:  &programY,
	}, "")
	if err != nil {
		t.Fatalf("create mover lead: %v", err)
	}

	// A combined slot change + move into an OPEN stage still refuses: the
	// lead would stay open in the held slot.
	_, err = svc.update(b.ID, UpdateRequest{ProgramID: &programX, StageID: &openStageB}, "")
	openLeadConflict(t, err)

	// A combined slot change + move into a CLOSING stage is allowed: the lead
	// closes in the same update and never occupies the slot.
	updated, err := svc.update(b.ID, UpdateRequest{ProgramID: &programX, StageID: &wonStage}, "")
	if err != nil {
		t.Fatalf("close into held slot = %v, want allowed", err)
	}
	if updated.StageID != wonStage || updated.Outcome != "won" {
		t.Errorf("updated lead = stage %q outcome %q, want terminal won in the new slot", updated.StageID, updated.Outcome)
	}
}

func TestClosedLeadSlotKeyEditAllowedIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var pipelineID string
	if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ('Terminal Pipeline') RETURNING id`).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	var openStage, wonStage string
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order") VALUES ($1, 'Open', 0) RETURNING id`, pipelineID).Scan(&openStage); err != nil {
		t.Fatalf("seed open stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order", is_closing, outcome) VALUES ($1, 'Won', 1, true, 'won') RETURNING id`, pipelineID).Scan(&wonStage); err != nil {
		t.Fatalf("seed won stage: %v", err)
	}
	programX := seedProgram(t, db, "Coaching", 25000)
	programY := seedProgram(t, db, "Mentorship", 40000)

	// The slot (contact, pipeline, program X) is held by an OPEN lead.
	if _, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    openStage,
		ProgramID:  &programX,
	}, ""); err != nil {
		t.Fatalf("create holder lead: %v", err)
	}
	closed, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    openStage,
		ProgramID:  &programY,
	}, "")
	if err != nil {
		t.Fatalf("create second lead: %v", err)
	}
	if _, err := svc.update(closed.ID, UpdateRequest{StageID: &wonStage}, ""); err != nil {
		t.Fatalf("close second lead: %v", err)
	}

	// A closed (terminal) lead holds no slot, so re-sloting it into a held
	// program cannot create a duplicate open deal.
	updated, err := svc.update(closed.ID, UpdateRequest{ProgramID: &programX}, "")
	if err != nil {
		t.Fatalf("re-slot closed lead = %v, want allowed", err)
	}
	if updated.ProgramID == nil || *updated.ProgramID != programX {
		t.Errorf("closed lead program = %+v, want %q", updated.ProgramID, programX)
	}
	if updated.Outcome != "won" {
		t.Errorf("closed lead outcome = %q, want won (still terminal)", updated.Outcome)
	}
}

func TestConcurrentCreatesOneOpenLeadWinsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)

	const n = 8
	start := make(chan struct{})
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = svc.create(CreateRequest{
				NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
				PipelineID: pipelineID,
				StageID:    stageID,
			}, "")
		}(i)
	}
	close(start)
	wg.Wait()

	var conflict *OpenLeadConflictError
	wins, conflicts := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			wins++
		case errors.As(err, &conflict):
			conflicts++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if wins != 1 || conflicts != n-1 {
		t.Errorf("wins = %d, conflicts = %d; want 1 and %d", wins, conflicts, n-1)
	}
	var leadCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM leads`).Scan(&leadCount); err != nil {
		t.Fatalf("count leads: %v", err)
	}
	if leadCount != 1 {
		t.Errorf("lead count = %d, want exactly 1", leadCount)
	}
}

func TestCreateLeadConflictReturns409Integration(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))

	pipelineID, stageID := seedPipelineAndStage(t, db)
	programID := seedProgram(t, db, "Coaching", 25000)

	body := func() *bytes.Reader {
		raw, _ := json.Marshal(map[string]any{
			"new_contact": map[string]any{"name": "Alice", "phone": "1234567890"},
			"pipeline_id": pipelineID,
			"stage_id":    stageID,
			"program_id":  programID,
		})
		return bytes.NewReader(raw)
	}

	rr := httptest.NewRecorder()
	h.Create(rr, httptest.NewRequest(http.MethodPost, "/api/leads", body()))
	if rr.Code != http.StatusCreated {
		t.Fatalf("first create status = %d, want 201", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.Create(rr, httptest.NewRequest(http.MethodPost, "/api/leads", body()))
	if rr.Code != http.StatusConflict {
		t.Fatalf("duplicate create status = %d, want 409", rr.Code)
	}
	var payload struct {
		Data  map[string]any `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error.Code != "OPEN_LEAD_CONFLICT" {
		t.Errorf("error code = %q, want OPEN_LEAD_CONFLICT", payload.Error.Code)
	}
	existing, ok := payload.Data["existing_lead"].(map[string]any)
	if !ok {
		t.Fatalf("data = %+v, want existing_lead payload", payload.Data)
	}
	if id, _ := existing["id"].(string); id == "" {
		t.Errorf("existing_lead.id = %q, want the first lead's id", id)
	}
}