package lead

import (
	"crm/internal/testdb"
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
