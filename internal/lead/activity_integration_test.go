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

	reminders, err := svc.getPendingReminders("00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("get pending reminders: %v", err)
	}
	if len(reminders) != 1 || reminders[0].LeadID != live.ID {
		t.Errorf("pending reminders = %+v, want only the live lead's reminder", reminders)
	}
}

func TestPendingRemindersScopedToResponsibleUserIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	assigneeID := seedTestUser(t, db, "assignee@example.com")
	creatorID := seedTestUser(t, db, "creator@example.com")
	otherID := seedTestUser(t, db, "other@example.com")

	pipelineID, stageID := seedPipelineAndStage(t, db)

	// Lead assigned to assignee; task created by creator.
	assignedLead, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
		AssignedTo: &assigneeID,
	}, creatorID)
	if err != nil {
		t.Fatalf("create assigned lead: %v", err)
	}
	// Unassigned lead; task created by creator.
	unassignedLead, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Bob", Phone: "0987654321"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, creatorID)
	if err != nil {
		t.Fatalf("create unassigned lead: %v", err)
	}
	// Unowned lead: no assignee, task created by nobody (system).
	unownedLead, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Carol", Phone: "1112223333"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create unowned lead: %v", err)
	}

	for _, leadID := range []string{assignedLead.ID, unassignedLead.ID, unownedLead.ID} {
		if _, err := db.Exec(
			`INSERT INTO lead_activities (lead_id, stage_id, user_id, type, remind_at)
			VALUES ($1, $2, NULLIF($3, '')::uuid, 'call', $4)`,
			leadID, stageID, creatorID, time.Now().Add(time.Hour),
		); err != nil {
			t.Fatalf("seed reminder: %v", err)
		}
	}
	// The unowned lead's task has no creator.
	if _, err := db.Exec(
		`UPDATE lead_activities SET user_id = NULL WHERE lead_id = $1`,
		unownedLead.ID,
	); err != nil {
		t.Fatalf("clear unowned task creator: %v", err)
	}

	// The assignee sees their assigned lead's task (and the unowned one).
	assigned, err := svc.getPendingReminders(assigneeID)
	if err != nil {
		t.Fatalf("get pending reminders (assignee): %v", err)
	}
	assignedIDs := map[string]bool{}
	for _, r := range assigned {
		assignedIDs[r.LeadID] = true
	}
	if !assignedIDs[assignedLead.ID] || !assignedIDs[unownedLead.ID] {
		t.Errorf("assignee reminders = %v, want assigned + unowned leads", assignedIDs)
	}
	if assignedIDs[unassignedLead.ID] {
		t.Errorf("assignee reminders include unassigned lead's task, want excluded")
	}

	// The creator sees their task on the unassigned lead (and the unowned one),
	// but not the task on the assignee-owned lead.
	creator, err := svc.getPendingReminders(creatorID)
	if err != nil {
		t.Fatalf("get pending reminders (creator): %v", err)
	}
	creatorIDs := map[string]bool{}
	for _, r := range creator {
		creatorIDs[r.LeadID] = true
	}
	if !creatorIDs[unassignedLead.ID] || !creatorIDs[unownedLead.ID] {
		t.Errorf("creator reminders = %v, want unassigned + unowned leads", creatorIDs)
	}
	if creatorIDs[assignedLead.ID] {
		t.Errorf("creator reminders include assigned lead's task, want excluded")
	}

	// An unrelated user sees only the genuinely unowned work.
	other, err := svc.getPendingReminders(otherID)
	if err != nil {
		t.Fatalf("get pending reminders (other): %v", err)
	}
	otherIDs := map[string]bool{}
	for _, r := range other {
		otherIDs[r.LeadID] = true
	}
	if len(otherIDs) != 1 || !otherIDs[unownedLead.ID] {
		t.Errorf("other reminders = %v, want only the unowned lead", otherIDs)
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
	updated, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{ScheduledAt: optTime(&newTime)})
	if err != nil {
		t.Fatalf("update scheduled_at: %v", err)
	}
	if updated.ScheduledAt == nil || !updated.ScheduledAt.Equal(newTime) {
		t.Errorf("scheduled_at = %v, want %v", updated.ScheduledAt, newTime)
	}
}

func TestUpdateActivityClearScheduleIntegration(t *testing.T) {
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
	sched := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	remind := time.Now().Add(49 * time.Hour).UTC().Truncate(time.Second)
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{
		Type:        "Call 1",
		ScheduledAt: &sched,
		RemindAt:    &remind,
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}
	if act.ScheduledAt == nil || act.RemindAt == nil {
		t.Fatal("seed activity should carry both schedule and reminder")
	}

	// Explicit null clears both fields — the edit form sends null when the
	// date/time inputs are emptied.
	cleared, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{
		ScheduledAt: optTime(nil),
		RemindAt:    optTime(nil),
	})
	if err != nil {
		t.Fatalf("clear schedule: %v", err)
	}
	if cleared.ScheduledAt != nil || cleared.RemindAt != nil {
		t.Errorf("after clear scheduled_at/remind_at = %v/%v, want nil/nil", cleared.ScheduledAt, cleared.RemindAt)
	}
}

func TestUpdateActivityAbsentScheduleKeepsValueIntegration(t *testing.T) {
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
	sched := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{
		Type:        "Call 1",
		ScheduledAt: &sched,
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	// Updating only the description leaves the schedule untouched (absent key
	// is "keep"), unlike an explicit null which would clear it.
	desc := "touched"
	updated, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{Description: &desc})
	if err != nil {
		t.Fatalf("update description only: %v", err)
	}
	if updated.ScheduledAt == nil || !updated.ScheduledAt.Equal(sched) {
		t.Errorf("scheduled_at = %v, want unchanged %v when field absent", updated.ScheduledAt, sched)
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
		`INSERT INTO lead_stages (pipeline_id, name, "order", is_closing, outcome) VALUES ($1, 'Closed', 99, true, 'lost') RETURNING id`,
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

func TestListAllActivitiesMultiFilterArgBindingIntegration(t *testing.T) {
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

	// Two predicates that both carry arguments: a per-condition numbering
	// restart would bind la.type to the is_done flag's value and return nothing.
	doneList, total, err := svc.listAllActivities(ActivityListFilters{Status: "done", Type: "Call 2", Page: 1, PerPage: 50})
	if err != nil {
		t.Fatalf("list done+type: %v", err)
	}
	if total != 1 || len(doneList) != 1 || doneList[0].Type != "Call 2" {
		t.Errorf("done+type: total = %d, got %d rows; want the one done 'Call 2'", total, len(doneList))
	}

	// Search binds three arguments; the type predicate must continue after them.
	searched, total, err := svc.listAllActivities(ActivityListFilters{Search: "alice", Type: "Call 1", Page: 1, PerPage: 50})
	if err != nil {
		t.Fatalf("list search+type: %v", err)
	}
	if total != 1 || len(searched) != 1 || searched[0].Type != "Call 1" {
		t.Errorf("search+type: total = %d, got %d rows; want the open 'Call 1'", total, len(searched))
	}
}

func seedQuickReplyTagBehavior(t *testing.T, db interface {
	QueryRow(query string, args ...any) *sql.Row
}, name, behavior string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(`INSERT INTO tags (name, type, behavior) VALUES ($1, 'quick_reply', $2) RETURNING id`, name, behavior).Scan(&id); err != nil {
		t.Fatalf("seed quick reply tag: %v", err)
	}
	return id
}

func TestCreateActivityCloseLostMovesLeadIntegration(t *testing.T) {
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
	qrID := seedQuickReplyTagBehavior(t, db, "Closed Lost", "close_lost")

	done := true
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{
		Type:         "Call 2",
		QuickReplyID: &qrID,
		IsDone:       &done,
	})
	if err != nil {
		t.Fatalf("create close_lost activity: %v", err)
	}
	if !act.IsDone || act.OccurredAt == nil {
		t.Errorf("close_lost activity is_done = %v, occurred_at = %v; want done with occurred_at", act.IsDone, act.OccurredAt)
	}

	got, err := svc.get(created.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.StageID != closingStageID {
		t.Errorf("stage_id = %q, want closing stage %q", got.StageID, closingStageID)
	}
	if got.Outcome != "lost" {
		t.Errorf("outcome = %q, want lost", got.Outcome)
	}

	var cancelled, total int
	if err := db.QueryRow(
		`SELECT COUNT(*) FILTER (WHERE is_cancelled), COUNT(*) FROM lead_activities WHERE lead_id = $1`,
		created.ID,
	).Scan(&cancelled, &total); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if total != 2 || cancelled != 1 {
		t.Errorf("expected 1 of 2 tasks cancelled, got %d of %d", cancelled, total)
	}

	var history int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM lead_stage_history WHERE lead_id = $1 AND to_stage_id = $2`,
		created.ID, closingStageID,
	).Scan(&history); err != nil {
		t.Fatalf("count history: %v", err)
	}
	if history != 1 {
		t.Errorf("history rows into closing stage = %d, want 1", history)
	}
}

func TestUpdateActivityCloseLostMovesLeadIntegration(t *testing.T) {
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
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call 1"})
	if err != nil {
		t.Fatalf("create open task: %v", err)
	}
	qrID := seedQuickReplyTagBehavior(t, db, "Closed Lost", "close_lost")

	done := true
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{QuickReplyID: &qrID, IsDone: &done}); err != nil {
		t.Fatalf("complete with close_lost quick reply: %v", err)
	}

	got, err := svc.get(created.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.StageID != closingStageID {
		t.Errorf("stage_id = %q, want closing stage %q", got.StageID, closingStageID)
	}
	if got.Outcome != "lost" {
		t.Errorf("outcome = %q, want lost", got.Outcome)
	}

	// Completing the closing touchpoint must not have been cancelled by the move.
	var cancelled, total int
	if err := db.QueryRow(
		`SELECT COUNT(*) FILTER (WHERE is_cancelled), COUNT(*) FROM lead_activities WHERE lead_id = $1`,
		created.ID,
	).Scan(&cancelled, &total); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if total != 1 || cancelled != 0 {
		t.Errorf("expected 0 of 1 tasks cancelled, got %d of %d", cancelled, total)
	}
}

func TestEditDoneCloseLostActivityDoesNotRecloseLeadIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	_ = seedClosingStage(t, db, pipelineID)
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
		t.Fatalf("create open task: %v", err)
	}
	qrID := seedQuickReplyTagBehavior(t, db, "Closed Lost", "close_lost")

	done := true
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{QuickReplyID: &qrID, IsDone: &done}); err != nil {
		t.Fatalf("complete with close_lost quick reply: %v", err)
	}

	// Reopen the lead back into the open stage.
	if _, err := svc.update(created.ID, UpdateRequest{StageID: &stageID}, ""); err != nil {
		t.Fatalf("reopen lead: %v", err)
	}

	// Editing the old closing touchpoint (description only) must not re-close.
	desc := "corrected note"
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{Description: &desc}); err != nil {
		t.Fatalf("edit done close_lost activity: %v", err)
	}

	got, err := svc.get(created.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.StageID != stageID {
		t.Errorf("stage_id = %q, want open stage %q (edit must not re-close)", got.StageID, stageID)
	}
	if got.Outcome != "" {
		t.Errorf("outcome = %q, want cleared", got.Outcome)
	}
}

func TestCloseLostWithoutLostStageIntegration(t *testing.T) {
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
	qrID := seedQuickReplyTagBehavior(t, db, "Closed Lost", "close_lost")

	done := true
	if _, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{
		Type:         "Call 1",
		QuickReplyID: &qrID,
		IsDone:       &done,
	}); !errors.Is(err, ErrNoLostStage) {
		t.Fatalf("create close_lost without lost stage = %v, want ErrNoLostStage", err)
	}

	got, err := svc.get(created.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.StageID != stageID {
		t.Errorf("stage_id = %q, want unchanged %q", got.StageID, stageID)
	}
}

func TestCreateCloseLostActivityNotDoneDoesNotCloseLeadIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	_ = seedClosingStage(t, db, pipelineID)
	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	qrID := seedQuickReplyTagBehavior(t, db, "Closed Lost", "close_lost")

	// A scheduled close_lost task (no is_done) must not close the lead.
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{
		Type:         "Call 1",
		QuickReplyID: &qrID,
	})
	if err != nil {
		t.Fatalf("create scheduled close_lost activity: %v", err)
	}
	if act.IsDone || act.IsCancelled {
		t.Errorf("activity is_done = %v, is_cancelled = %v; want open", act.IsDone, act.IsCancelled)
	}

	got, err := svc.get(created.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.StageID != stageID {
		t.Errorf("stage_id = %q, want open stage %q (scheduled close_lost task must not close)", got.StageID, stageID)
	}
	if got.Outcome != "" {
		t.Errorf("outcome = %q, want empty", got.Outcome)
	}
}

func TestCreateActivityDefaultsReminderToNudgeLeadIntegration(t *testing.T) {
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

	// With no explicit remind time, the reminder defaults to 5 minutes before
	// the schedule (ADR 004 default nudge).
	start := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{
		Type:        "call",
		ScheduledAt: &start,
	})
	if err != nil {
		t.Fatalf("create scheduled activity: %v", err)
	}
	want := start.Add(-5 * time.Minute)
	if act.RemindAt == nil || !act.RemindAt.Equal(want) {
		t.Errorf("remind_at = %v, want %v (5 minutes before start)", act.RemindAt, want)
	}

	// An explicit remind time always wins over the default.
	explicit := start.Add(-30 * time.Minute)
	act2, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{
		Type:        "call",
		ScheduledAt: &start,
		RemindAt:    &explicit,
	})
	if err != nil {
		t.Fatalf("create activity with explicit remind: %v", err)
	}
	if act2.RemindAt == nil || !act2.RemindAt.Equal(explicit) {
		t.Errorf("remind_at = %v, want explicit %v", act2.RemindAt, explicit)
	}
}

func TestCreateActivityRejectsInvalidRangeIntegration(t *testing.T) {
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

	start := time.Now().Add(time.Hour).Truncate(time.Second)
	end := start.Add(-time.Minute) // before the start
	_, err = svc.createActivity(created.ID, stageID, "", CreateActivityRequest{
		Type:           "call",
		ScheduledAt:    &start,
		ScheduledEndAt: &end,
	})
	if !errors.Is(err, ErrInvalidRange) {
		t.Errorf("end before start = %v, want ErrInvalidRange", err)
	}

	// A range task with end after start is accepted and the end is stored.
	goodEnd := start.Add(time.Hour)
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{
		Type:           "call",
		ScheduledAt:    &start,
		ScheduledEndAt: &goodEnd,
	})
	if err != nil {
		t.Fatalf("create range task: %v", err)
	}
	if act.ScheduledEndAt == nil || !act.ScheduledEndAt.Equal(goodEnd) {
		t.Errorf("scheduled_end_at = %v, want %v", act.ScheduledEndAt, goodEnd)
	}
}
