package lead

import (
	"crm/internal/testdb"
	"database/sql"
	"errors"
	"testing"
)

func TestDismissReminderMissingIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	dismissed, err := svc.dismissReminder("00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("dismissReminder missing: %v", err)
	}
	if dismissed {
		t.Error("expected (false, nil) for a nonexistent reminder")
	}
}

func TestDismissReminderFoundIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	var leadID string
	if err := db.QueryRow(
		`INSERT INTO leads (name, pipeline_id, stage_id) VALUES ('Lead A', $1, $2) RETURNING id`,
		pipelineID, stageID,
	).Scan(&leadID); err != nil {
		t.Fatalf("seed lead: %v", err)
	}
	var activityID string
	if err := db.QueryRow(
		`INSERT INTO lead_activities (lead_id, stage_id, type, description) VALUES ($1, $2, 'call', 'Follow up') RETURNING id`,
		leadID, stageID,
	).Scan(&activityID); err != nil {
		t.Fatalf("seed activity: %v", err)
	}

	dismissed, err := svc.dismissReminder(activityID)
	if err != nil {
		t.Fatalf("dismissReminder: %v", err)
	}
	if !dismissed {
		t.Error("expected (true, nil) for an existing reminder")
	}

	dismissed, err = svc.dismissReminder(activityID)
	if err != nil {
		t.Fatalf("second dismiss: %v", err)
	}
	if dismissed {
		t.Error("second dismiss should report false; is_reminded is idempotent")
	}
}

func TestCreateActivityWithOutcomeSetsRespondedAtIntegration(t *testing.T) {
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
	statusID := seedStatusTag(t, db, "No Reply")

	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{
		Type:      "Call 1",
		OutcomeID: &statusID,
	})
	if err != nil {
		t.Fatalf("create activity with outcome: %v", err)
	}
	if act.RespondedAt == nil {
		t.Error("responded_at should be set when activity is created with an outcome")
	}
	if act.OutcomeName != "No Reply" {
		t.Errorf("outcome_name = %q, want No Reply", act.OutcomeName)
	}

	// Activity created without an outcome has no responded_at.
	plain, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Note"})
	if err != nil {
		t.Fatalf("create plain activity: %v", err)
	}
	if plain.RespondedAt != nil {
		t.Error("responded_at should be nil for an activity without an outcome")
	}
}

func TestUpdateActivityMarksResponseOnceIntegration(t *testing.T) {
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
	statusA := seedStatusTag(t, db, "No Reply")
	statusB := seedStatusTag(t, db, "Share Details WA")

	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call 1"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	// First mark: responded_at is set.
	updated, err := svc.updateActivity(created.ID, act.ID, UpdateActivityRequest{OutcomeID: &statusA})
	if err != nil {
		t.Fatalf("mark first outcome: %v", err)
	}
	if updated.RespondedAt == nil {
		t.Fatal("responded_at should be set on first outcome mark")
	}
	first := updated.RespondedAt.Unix()

	// Change outcome again: responded_at must NOT move.
	changed, err := svc.updateActivity(created.ID, act.ID, UpdateActivityRequest{OutcomeID: &statusB})
	if err != nil {
		t.Fatalf("change outcome: %v", err)
	}
	if changed.RespondedAt == nil || changed.RespondedAt.Unix() != first {
		t.Errorf("responded_at moved on outcome change: got %v, want %v", changed.RespondedAt, first)
	}
}

func TestUpdateActivityRejectsNonStatusOutcomeIntegration(t *testing.T) {
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
	// A tag, not a status.
	var tagID string
	if err := db.QueryRow(`INSERT INTO tags (name, type) VALUES ('VIP', 'tag') RETURNING id`).Scan(&tagID); err != nil {
		t.Fatalf("seed tag: %v", err)
	}

	_, err = svc.createActivity(created.ID, stageID, "", CreateActivityRequest{
		Type:      "Call 1",
		OutcomeID: &tagID,
	})
	if !errors.Is(err, ErrInvalidOutcome) {
		t.Errorf("create with non-status outcome = %v, want ErrInvalidOutcome", err)
	}
}

func seedStatusTag(t *testing.T, db interface {
	QueryRow(query string, args ...any) *sql.Row
}, name string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(`INSERT INTO tags (name, type) VALUES ($1, 'status') RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("seed status tag: %v", err)
	}
	return id
}
