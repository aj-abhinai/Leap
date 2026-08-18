package lead

import (
	"crm/internal/testdb"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestDismissReminderMissingIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	dismissed, err := svc.dismissReminder("00000000-0000-0000-0000-000000000000", "00000000-0000-0000-0000-000000000000")
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
	var contactID string
	if err := db.QueryRow(`INSERT INTO contacts (name) VALUES ('Alice') RETURNING id`).Scan(&contactID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	var leadID string
	if err := db.QueryRow(
		`INSERT INTO leads (contact_id, pipeline_id, stage_id) VALUES ($1, $2, $3) RETURNING id`,
		contactID, pipelineID, stageID,
	).Scan(&leadID); err != nil {
		t.Fatalf("seed lead: %v", err)
	}
	var activityID string
	if err := db.QueryRow(
		`INSERT INTO lead_activities (lead_id, stage_id, type, description, scheduled_at)
		VALUES ($1, $2, 'call', 'Follow up', $3) RETURNING id`,
		leadID, stageID, time.Now().Add(time.Hour),
	).Scan(&activityID); err != nil {
		t.Fatalf("seed activity: %v", err)
	}

	dismissed, err := svc.dismissReminder(leadID, activityID)
	if err != nil {
		t.Fatalf("dismissReminder: %v", err)
	}
	if !dismissed {
		t.Error("expected (true, nil) for an existing reminder")
	}

	dismissed, err = svc.dismissReminder(leadID, activityID)
	if err != nil {
		t.Fatalf("second dismiss: %v", err)
	}
	if dismissed {
		t.Error("second dismiss should report false; is_reminded is idempotent")
	}
}

func TestDismissReminderWrongLeadScopedIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	_, stageID := seedPipelineAndStage(t, db)
	_, otherStageID := seedPipelineAndStage(t, db)
	var contactID string
	if err := db.QueryRow(`INSERT INTO contacts (name) VALUES ('Alice') RETURNING id`).Scan(&contactID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	var leadA, leadB string
	if err := db.QueryRow(
		`INSERT INTO leads (contact_id, pipeline_id, stage_id) VALUES ($1, (SELECT pipeline_id FROM lead_stages WHERE id = $2), $2) RETURNING id`,
		contactID, stageID,
	).Scan(&leadA); err != nil {
		t.Fatalf("seed lead A: %v", err)
	}
	if err := db.QueryRow(
		`INSERT INTO leads (contact_id, pipeline_id, stage_id) VALUES ($1, (SELECT pipeline_id FROM lead_stages WHERE id = $2), $2) RETURNING id`,
		contactID, otherStageID,
	).Scan(&leadB); err != nil {
		t.Fatalf("seed lead B: %v", err)
	}
	var activityID string
	if err := db.QueryRow(
		`INSERT INTO lead_activities (lead_id, stage_id, type, description) VALUES ($1, $2, 'call', 'Follow up') RETURNING id`,
		leadA, stageID,
	).Scan(&activityID); err != nil {
		t.Fatalf("seed activity: %v", err)
	}

	dismissed, err := svc.dismissReminder(leadB, activityID)
	if err != nil {
		t.Fatalf("dismissReminder wrong lead: %v", err)
	}
	if dismissed {
		t.Error("dismiss on another lead should be scoped out (false, nil)")
	}
}

func TestDismissReminderRequiresOpenReminderIntegration(t *testing.T) {
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

	// A reminder-less open task is not a reminder and cannot be dismissed.
	var noReminderID string
	if err := db.QueryRow(
		`INSERT INTO lead_activities (lead_id, stage_id, type) VALUES ($1, $2, 'call') RETURNING id`,
		created.ID, stageID,
	).Scan(&noReminderID); err != nil {
		t.Fatalf("seed reminder-less activity: %v", err)
	}
	if dismissed, err := svc.dismissReminder(created.ID, noReminderID); err != nil || dismissed {
		t.Errorf("dismiss reminder-less = %v, %v; want false, nil", dismissed, err)
	}

	// A done task cannot be dismissed.
	var doneID string
	if err := db.QueryRow(
		`INSERT INTO lead_activities (lead_id, stage_id, type, scheduled_at, is_done)
		VALUES ($1, $2, 'call', $3, true) RETURNING id`,
		created.ID, stageID, time.Now().Add(time.Hour),
	).Scan(&doneID); err != nil {
		t.Fatalf("seed done activity: %v", err)
	}
	if dismissed, err := svc.dismissReminder(created.ID, doneID); err != nil || dismissed {
		t.Errorf("dismiss done = %v, %v; want false, nil", dismissed, err)
	}

	// A cancelled task cannot be dismissed.
	var cancelledID string
	if err := db.QueryRow(
		`INSERT INTO lead_activities (lead_id, stage_id, type, scheduled_at, is_cancelled)
		VALUES ($1, $2, 'call', $3, true) RETURNING id`,
		created.ID, stageID, time.Now().Add(time.Hour),
	).Scan(&cancelledID); err != nil {
		t.Fatalf("seed cancelled activity: %v", err)
	}
	if dismissed, err := svc.dismissReminder(created.ID, cancelledID); err != nil || dismissed {
		t.Errorf("dismiss cancelled = %v, %v; want false, nil", dismissed, err)
	}
}

func TestSnoozeReminderBoundsIntegration(t *testing.T) {
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
	var activityID string
	if err := db.QueryRow(
		`INSERT INTO lead_activities (lead_id, stage_id, type, scheduled_at)
		VALUES ($1, $2, 'call', $3) RETURNING id`,
		created.ID, stageID, time.Now().Add(time.Hour),
	).Scan(&activityID); err != nil {
		t.Fatalf("seed activity: %v", err)
	}

	if _, err := svc.snoozeReminder(created.ID, activityID, time.Now().Add(-time.Minute)); !errors.Is(err, ErrSnoozePast) {
		t.Errorf("snooze to the past = %v, want ErrSnoozePast", err)
	}

	tooFar := time.Now().Add(maxSnoozeHorizon + 24*time.Hour)
	if _, err := svc.snoozeReminder(created.ID, activityID, tooFar); !errors.Is(err, ErrSnoozeTooFar) {
		t.Errorf("snooze beyond horizon = %v, want ErrSnoozeTooFar", err)
	}

	// The row is untouched by the rejected attempts.
	snoozed, err := svc.snoozeReminder(created.ID, activityID, time.Now().Add(2*time.Hour).UTC().Truncate(time.Second))
	if err != nil || !snoozed {
		t.Errorf("valid snooze = %v, %v; want true, nil", snoozed, err)
	}
}

func TestPendingRemindersExcludeDeletedLeadsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	live, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create live lead: %v", err)
	}
	deleted, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Bob", Phone: "0987654321"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create to-be-deleted lead: %v", err)
	}

	for _, leadID := range []string{live.ID, deleted.ID} {
		if _, err := db.Exec(
			`INSERT INTO lead_activities (lead_id, stage_id, type, remind_at)
			VALUES ($1, $2, 'call', $3)`,
			leadID, stageID, time.Now().Add(time.Hour),
		); err != nil {
			t.Fatalf("seed reminder: %v", err)
		}
	}
	if _, err := db.Exec(`UPDATE leads SET deleted_at = now() WHERE id = $1`, deleted.ID); err != nil {
		t.Fatalf("soft-delete lead: %v", err)
	}

	reminders, err := svc.getPendingReminders()
	if err != nil {
		t.Fatalf("get pending reminders: %v", err)
	}
	if len(reminders) != 1 || reminders[0].LeadID != live.ID {
		t.Errorf("pending reminders = %+v, want only the live lead's reminder", reminders)
	}
}

func TestUpdateActivityEmptyRequestRejectedIntegration(t *testing.T) {
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
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call 1"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{}); !errors.Is(err, ErrNothingToUpdate) {
		t.Errorf("empty update = %v, want ErrNothingToUpdate", err)
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
	statusID := seedQuickReplyTag(t, db, "No Reply")

	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{
		Type:         "Call 1",
		QuickReplyID: &statusID,
	})
	if err != nil {
		t.Fatalf("create activity with outcome: %v", err)
	}
	if act.RespondedAt == nil {
		t.Error("responded_at should be set when activity is created with an outcome")
	}
	if act.QuickReplyName != "No Reply" {
		t.Errorf("quick_reply_name = %q, want No Reply", act.QuickReplyName)
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
	statusA := seedQuickReplyTag(t, db, "No Reply")
	statusB := seedQuickReplyTag(t, db, "Share Details WA")

	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call 1"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	// First mark: responded_at is set.
	updated, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{QuickReplyID: &statusA})
	if err != nil {
		t.Fatalf("mark first outcome: %v", err)
	}
	if updated.RespondedAt == nil {
		t.Fatal("responded_at should be set on first outcome mark")
	}
	first := updated.RespondedAt.Unix()

	// Change outcome again: responded_at must NOT move.
	changed, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{QuickReplyID: &statusB})
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
	// A tag, not a quick reply.
	var tagID string
	if err := db.QueryRow(`INSERT INTO tags (name, type) VALUES ('VIP', 'tag') RETURNING id`).Scan(&tagID); err != nil {
		t.Fatalf("seed tag: %v", err)
	}

	_, err = svc.createActivity(created.ID, stageID, "", CreateActivityRequest{
		Type:         "Call 1",
		QuickReplyID: &tagID,
	})
	if !errors.Is(err, ErrInvalidQuickReply) {
		t.Errorf("create with non-quick-reply = %v, want ErrInvalidQuickReply", err)
	}
}

func seedQuickReplyTag(t *testing.T, db interface {
	QueryRow(query string, args ...any) *sql.Row
}, name string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(`INSERT INTO tags (name, type) VALUES ($1, 'quick_reply') RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("seed quick reply tag: %v", err)
	}
	return id
}

func TestCreateActivityDescriptionOptionalIntegration(t *testing.T) {
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

	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call 1"})
	if err != nil {
		t.Fatalf("create activity without description: %v", err)
	}
	if act.Type != "Call 1" {
		t.Errorf("type = %q, want Call 1", act.Type)
	}
}

func TestUpdateActivityScheduledAtEditableIntegration(t *testing.T) {
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
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call 1"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}
	if act.ScheduledAt != nil {
		t.Fatal("fresh activity should have no scheduled_at")
	}

	newTime := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	updated, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{ScheduledAt: &newTime})
	if err != nil {
		t.Fatalf("update scheduled_at: %v", err)
	}
	if updated.ScheduledAt == nil || !updated.ScheduledAt.Equal(newTime) {
		t.Errorf("scheduled_at = %v, want %v", updated.ScheduledAt, newTime)
	}
}

func TestUpdateActivityRescheduleSpawnsNextIntegration(t *testing.T) {
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
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call 1"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	done := true
	next := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Second)
	updated, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{
		IsDone:       &done,
		RescheduleAt: &next,
	})
	if err != nil {
		t.Fatalf("reschedule update: %v", err)
	}
	if !updated.IsDone {
		t.Error("original activity should be marked done")
	}
	if updated.OccurredAt == nil {
		t.Error("occurred_at should be stamped when completing")
	}

	// The next occurrence must exist with the new time, same type, open.
	var nextID, nextType string
	var nextScheduled *time.Time
	if err := db.QueryRow(
		`SELECT id, type, scheduled_at FROM lead_activities WHERE lead_id = $1 AND type = 'Call 1' AND is_done = false ORDER BY created_at DESC LIMIT 1`,
		created.ID,
	).Scan(&nextID, &nextType, &nextScheduled); err != nil {
		t.Fatalf("load next activity: %v", err)
	}
	if nextID == act.ID {
		t.Error("next activity should be a distinct row")
	}
	if nextScheduled == nil || !nextScheduled.Equal(next) {
		t.Errorf("next scheduled_at = %v, want %v", nextScheduled, next)
	}
}

func TestCreateActivityRescheduleSpawnsNextIntegration(t *testing.T) {
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

	next := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{
		Type:         "Call 1",
		RescheduleAt: &next,
	})
	if err != nil {
		t.Fatalf("create activity with reschedule: %v", err)
	}
	if !act.IsDone {
		t.Error("completed attempt should be marked done")
	}
	if act.RespondedAt == nil {
		t.Error("responded_at should be stamped on a rescheduled attempt")
	}

	var nextID string
	var nextScheduled *time.Time
	if err := db.QueryRow(
		`SELECT id, scheduled_at FROM lead_activities WHERE lead_id = $1 AND type = 'Call 1' AND is_done = false ORDER BY created_at DESC LIMIT 1`,
		created.ID,
	).Scan(&nextID, &nextScheduled); err != nil {
		t.Fatalf("load next activity: %v", err)
	}
	if nextID == act.ID {
		t.Error("next activity should be a distinct row")
	}
	if nextScheduled == nil || !nextScheduled.Equal(next) {
		t.Errorf("next scheduled_at = %v, want %v", nextScheduled, next)
	}
}

func TestCreateActivityDoneWithOutcomeIntegration(t *testing.T) {
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
	statusID := seedQuickReplyTag(t, db, "Closed Lost")

	done := true
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{
		Type:         "Call 1",
		QuickReplyID: &statusID,
		IsDone:       &done,
	})
	if err != nil {
		t.Fatalf("create done activity: %v", err)
	}
	if !act.IsDone {
		t.Error("activity created with is_done should be marked done")
	}
	if act.RespondedAt == nil {
		t.Error("responded_at should be stamped when created done with an outcome")
	}
	if act.OccurredAt == nil {
		t.Error("occurred_at should be stamped when created done")
	}

	// No spawn: is_done without reschedule_at is a plain completion.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM lead_activities WHERE lead_id = $1`, created.ID).Scan(&count); err != nil {
		t.Fatalf("count activities: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly one activity, got %d", count)
	}
}

func TestCloseLeadCancelsOpenTasksIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	closingStageID := seedClosingStage(t, db, pipelineID)
	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	if _, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call 1"}); err != nil {
		t.Fatalf("create open task: %v", err)
	}

	closing := closingStageID
	if _, err := svc.update(created.ID, UpdateRequest{StageID: &closing}, ""); err != nil {
		t.Fatalf("move lead to closing stage: %v", err)
	}

	var cancelled, total int
	if err := db.QueryRow(
		`SELECT COUNT(*) FILTER (WHERE is_cancelled), COUNT(*) FROM lead_activities WHERE lead_id = $1`,
		created.ID,
	).Scan(&cancelled, &total); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if total != 1 || cancelled != 1 {
		t.Errorf("expected 1 of 1 tasks cancelled, got %d of %d", cancelled, total)
	}
}

func seedClosingStage(t *testing.T, db *sql.DB, pipelineID string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(
		`INSERT INTO lead_stages (pipeline_id, name, "order", is_closing) VALUES ($1, 'Closed', 99, true) RETURNING id`,
		pipelineID,
	).Scan(&id); err != nil {
		t.Fatalf("seed closing stage: %v", err)
	}
	return id
}

func TestListAllActivitiesFiltersIntegration(t *testing.T) {
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
	if _, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call 1"}); err != nil {
		t.Fatalf("create open task: %v", err)
	}
	done := true
	if _, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call 2", IsDone: &done}); err != nil {
		t.Fatalf("create done task: %v", err)
	}

	all, total, err := svc.listAllActivities(ActivityListFilters{Page: 1, PerPage: 50})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if total != 2 {
		t.Errorf("all total = %d, want 2", total)
	}
	if len(all) != 2 || all[0].LeadDisplayName == "" {
		t.Errorf("expected 2 items with lead display name, got %d", len(all))
	}

	open, total, err := svc.listAllActivities(ActivityListFilters{Status: "open", Page: 1, PerPage: 50})
	if err != nil {
		t.Fatalf("list open: %v", err)
	}
	if total != 1 || open[0].IsDone {
		t.Errorf("open filter: total = %d, first is_done = %v; want 1 open task", total, open[0].IsDone)
	}

	doneList, total, err := svc.listAllActivities(ActivityListFilters{Status: "done", Page: 1, PerPage: 50})
	if err != nil {
		t.Fatalf("list done: %v", err)
	}
	if total != 1 || !doneList[0].IsDone {
		t.Errorf("done filter: total = %d", total)
	}

	searched, total, err := svc.listAllActivities(ActivityListFilters{Search: "alice", Page: 1, PerPage: 50})
	if err != nil {
		t.Fatalf("list search: %v", err)
	}
	if total != 2 {
		t.Errorf("search 'alice' total = %d, want 2", total)
	}
	_ = searched
}
