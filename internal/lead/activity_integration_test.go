package lead

import (
	"context"
	"crm/internal/testdb"
	"database/sql"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func TestDismissReminderMissingIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	dismissed, err := svc.dismissReminder("00000000-0000-0000-0000-000000000000", "00000000-0000-0000-0000-000000000000", "")
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

	dismissed, err := svc.dismissReminder(leadID, activityID, "")
	if err != nil {
		t.Fatalf("dismissReminder: %v", err)
	}
	if !dismissed {
		t.Error("expected (true, nil) for an existing reminder")
	}

	dismissed, err = svc.dismissReminder(leadID, activityID, "")
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

	dismissed, err := svc.dismissReminder(leadB, activityID, "")
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
	if dismissed, err := svc.dismissReminder(created.ID, noReminderID, ""); err != nil || dismissed {
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
	if dismissed, err := svc.dismissReminder(created.ID, doneID, ""); err != nil || dismissed {
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
	if dismissed, err := svc.dismissReminder(created.ID, cancelledID, ""); err != nil || dismissed {
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

	if _, err := svc.snoozeReminder(created.ID, activityID, "", time.Now().Add(-time.Minute)); !errors.Is(err, ErrSnoozePast) {
		t.Errorf("snooze to the past = %v, want ErrSnoozePast", err)
	}

	tooFar := time.Now().Add(maxSnoozeHorizon + 24*time.Hour)
	if _, err := svc.snoozeReminder(created.ID, activityID, "", tooFar); !errors.Is(err, ErrSnoozeTooFar) {
		t.Errorf("snooze beyond horizon = %v, want ErrSnoozeTooFar", err)
	}

	// The row is untouched by the rejected attempts.
	snoozed, err := svc.snoozeReminder(created.ID, activityID, "", time.Now().Add(2*time.Hour).UTC().Truncate(time.Second))
	if err != nil || !snoozed {
		t.Errorf("valid snooze = %v, %v; want true, nil", snoozed, err)
	}
}

// A snooze anchors on COALESCE(remind_at, scheduled_at) and shifts the
// schedule by the same delta, so a schedule-only task moves out of overdue
// instead of keeping its stale start, and a ranged task keeps its window.
func TestSnoozeReminderShiftsScheduleFromSingleAnchorIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	// The recipient predicate compares against a uuid column, so the actor
	// must be a real user id even for genuinely unowned work.
	actorID := seedTestUser(t, db, "anchor@example.com")

	pipelineID, stageID := seedPipelineAndStage(t, db)
	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}

	// Schedule-only point task in the past: the schedule is the anchor.
	pointStart := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Second)
	var pointID string
	if err := db.QueryRow(
		`INSERT INTO lead_activities (lead_id, stage_id, type, scheduled_at)
		VALUES ($1, $2, 'call', $3) RETURNING id`,
		created.ID, stageID, pointStart,
	).Scan(&pointID); err != nil {
		t.Fatalf("seed point task: %v", err)
	}
	pointNew := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	if snoozed, err := svc.snoozeReminder(created.ID, pointID, actorID, pointNew); err != nil || !snoozed {
		t.Fatalf("snooze point task = %v, %v; want true, nil", snoozed, err)
	}
	var pointStartOut time.Time
	var pointRemind sql.NullTime
	if err := db.QueryRow(
		`SELECT scheduled_at, remind_at FROM lead_activities WHERE id = $1`, pointID,
	).Scan(&pointStartOut, &pointRemind); err != nil {
		t.Fatalf("load point task: %v", err)
	}
	if !pointStartOut.Equal(pointNew) {
		t.Errorf("schedule-only snooze scheduled_at = %v, want %v", pointStartOut, pointNew)
	}
	if !pointRemind.Valid || !pointRemind.Time.Equal(pointNew) {
		t.Errorf("schedule-only snooze remind_at = %v, want %v", pointRemind, pointNew)
	}

	// Range task with no reminder: both ends shift by the same delta.
	rangeStart := time.Now().Add(-4 * time.Hour).UTC().Truncate(time.Second)
	rangeEnd := rangeStart.Add(2 * time.Hour)
	var rangeID string
	if err := db.QueryRow(
		`INSERT INTO lead_activities (lead_id, stage_id, type, scheduled_at, scheduled_end_at)
		VALUES ($1, $2, 'call', $3, $4) RETURNING id`,
		created.ID, stageID, rangeStart, rangeEnd,
	).Scan(&rangeID); err != nil {
		t.Fatalf("seed range task: %v", err)
	}
	rangeNew := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	if snoozed, err := svc.snoozeReminder(created.ID, rangeID, actorID, rangeNew); err != nil || !snoozed {
		t.Fatalf("snooze range task = %v, %v; want true, nil", snoozed, err)
	}
	var rangeNewStart, rangeNewEnd time.Time
	if err := db.QueryRow(
		`SELECT scheduled_at, scheduled_end_at FROM lead_activities WHERE id = $1`, rangeID,
	).Scan(&rangeNewStart, &rangeNewEnd); err != nil {
		t.Fatalf("load range task: %v", err)
	}
	if !rangeNewStart.Equal(rangeNew) {
		t.Errorf("range snooze start = %v, want %v", rangeNewStart, rangeNew)
	}
	if got := rangeNewEnd.Sub(rangeNewStart); got != 2*time.Hour {
		t.Errorf("range snooze window = %v, want 2h", got)
	}

	// Task with an explicit reminder: the reminder is the anchor, so the
	// schedule shifts by the same delta as the reminder.
	bothStart := time.Now().Add(3 * time.Hour).UTC().Truncate(time.Second)
	bothRemind := bothStart.Add(-10 * time.Minute)
	var bothID string
	if err := db.QueryRow(
		`INSERT INTO lead_activities (lead_id, stage_id, type, scheduled_at, remind_at)
		VALUES ($1, $2, 'call', $3, $4) RETURNING id`,
		created.ID, stageID, bothStart, bothRemind,
	).Scan(&bothID); err != nil {
		t.Fatalf("seed anchored task: %v", err)
	}
	bothNew := bothStart.Add(5 * time.Hour)
	if snoozed, err := svc.snoozeReminder(created.ID, bothID, actorID, bothNew); err != nil || !snoozed {
		t.Fatalf("snooze anchored task = %v, %v; want true, nil", snoozed, err)
	}
	var bothStartOut time.Time
	var bothRemindOut time.Time
	if err := db.QueryRow(
		`SELECT scheduled_at, remind_at FROM lead_activities WHERE id = $1`, bothID,
	).Scan(&bothStartOut, &bothRemindOut); err != nil {
		t.Fatalf("load anchored task: %v", err)
	}
	wantStart := bothStart.Add(bothNew.Sub(bothRemind))
	if !bothStartOut.Equal(wantStart) {
		t.Errorf("anchored snooze scheduled_at = %v, want %v", bothStartOut, wantStart)
	}
	if !bothRemindOut.Equal(bothNew) {
		t.Errorf("anchored snooze remind_at = %v, want %v", bothRemindOut, bothNew)
	}
}

// A user may dismiss or snooze only the reminders they are responsible for:
// the lead's assignee, the task's creator on an unassigned lead, or genuinely
// unowned work (both null). An unrelated user gets a clean (false, nil) — the
// same recipient predicate as the bell's pending fetch, so the bell and the
// API can never disagree about whose nudge it is.
func TestReminderActionsScopedToResponsibleUserIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	assigneeID := seedTestUser(t, db, "assignee2@example.com")
	creatorID := seedTestUser(t, db, "creator2@example.com")
	otherID := seedTestUser(t, db, "other2@example.com")

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

	// Seed one open reminder per lead. The unowned lead's task has no creator.
	reminderIDs := map[string]string{}
	for _, leadID := range []string{assignedLead.ID, unassignedLead.ID, unownedLead.ID} {
		var id string
		if err := db.QueryRow(
			`INSERT INTO lead_activities (lead_id, stage_id, user_id, type, remind_at)
			VALUES ($1, $2, NULLIF($3, '')::uuid, 'call', $4) RETURNING id`,
			leadID, stageID, creatorID, time.Now().Add(time.Hour),
		).Scan(&id); err != nil {
			t.Fatalf("seed reminder: %v", err)
		}
		reminderIDs[leadID] = id
	}
	if _, err := db.Exec(`UPDATE lead_activities SET user_id = NULL WHERE lead_id = $1`, unownedLead.ID); err != nil {
		t.Fatalf("clear unowned task creator: %v", err)
	}

	// An unrelated user cannot dismiss or snooze a task assigned to someone
	// else or owned by its creator; the genuinely unowned task is visible to
	// everyone, so any viewer may clear it.
	for _, leadID := range []string{assignedLead.ID, unassignedLead.ID} {
		if dismissed, err := svc.dismissReminder(leadID, reminderIDs[leadID], otherID); err != nil || dismissed {
			t.Errorf("dismiss by other on %s = %v, %v; want false, nil", leadID, dismissed, err)
		}
		if snoozed, err := svc.snoozeReminder(leadID, reminderIDs[leadID], otherID, time.Now().Add(time.Hour)); err != nil || snoozed {
			t.Errorf("snooze by other on %s = %v, %v; want false, nil", leadID, snoozed, err)
		}
	}
	if snoozed, err := svc.snoozeReminder(unownedLead.ID, reminderIDs[unownedLead.ID], otherID, time.Now().Add(time.Hour)); err != nil || !snoozed {
		t.Errorf("snooze unowned by other = %v, %v; want true, nil", snoozed, err)
	}

	// The assignee can act on the assigned lead's task and the unowned one,
	// but not the unassigned lead's task (created by someone else).
	if dismissed, err := svc.dismissReminder(assignedLead.ID, reminderIDs[assignedLead.ID], assigneeID); err != nil || !dismissed {
		t.Errorf("dismiss by assignee on assigned lead = %v, %v; want true, nil", dismissed, err)
	}
	if dismissed, err := svc.dismissReminder(unassignedLead.ID, reminderIDs[unassignedLead.ID], assigneeID); err != nil || dismissed {
		t.Errorf("dismiss by assignee on unassigned lead = %v, %v; want false, nil", dismissed, err)
	}

	// The creator can act on their unassigned-lead task (and the unowned one).
	if snoozed, err := svc.snoozeReminder(unassignedLead.ID, reminderIDs[unassignedLead.ID], creatorID, time.Now().Add(2*time.Hour).UTC().Truncate(time.Second)); err != nil || !snoozed {
		t.Errorf("snooze by creator on unassigned lead = %v, %v; want true, nil", snoozed, err)
	}
	if dismissed, err := svc.dismissReminder(unownedLead.ID, reminderIDs[unownedLead.ID], creatorID); err != nil || !dismissed {
		t.Errorf("dismiss unowned by creator = %v, %v; want true, nil", dismissed, err)
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
	if !act.IsDone {
		t.Error("a saved quick reply should complete the activity")
	}
	if act.OccurredAt == nil {
		t.Error("occurred_at should be stamped when the quick reply completes the activity")
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
	if plain.IsDone {
		t.Error("an activity without an outcome stays open")
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
	if !updated.IsDone {
		t.Error("a recorded quick reply should complete the task")
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

// An un-complete on a task that carries a recorded quick reply is refused: the
// reply is the record of what happened, not a state flag.
func TestUpdateActivityRefusesUncompleteWithQuickReplyIntegration(t *testing.T) {
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
	qrID := seedQuickReplyTagBehavior(t, db, "Interested", "log")
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call", QuickReplyID: &qrID})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	undone := false
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{IsDone: &undone}); !errors.Is(err, ErrQuickReplyRecorded) {
		t.Fatalf("un-complete with a recorded reply = %v, want ErrQuickReplyRecorded", err)
	}

	var isDone bool
	var occurred, responded sql.NullTime
	if err := db.QueryRow(
		`SELECT is_done, occurred_at, responded_at FROM lead_activities WHERE id = $1`, act.ID,
	).Scan(&isDone, &occurred, &responded); err != nil {
		t.Fatalf("load row state: %v", err)
	}
	if !isDone {
		t.Error("a refused un-complete must leave the task done")
	}
	if !occurred.Valid || !responded.Valid {
		t.Error("a refused un-complete must leave the event stamps intact")
	}
}

// The refusal maps to 422 (not 500) at the HTTP boundary.
func TestUpdateActivityUncompleteWithQuickReplyReturns422Integration(t *testing.T) {
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
	qrID := seedQuickReplyTagBehavior(t, db, "Interested", "log")
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call", QuickReplyID: &qrID})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/leads/"+created.ID+"/activities/"+act.ID,
		strings.NewReader(`{"is_done":false}`),
	)
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("id", created.ID)
	ctx.URLParams.Add("activity_id", act.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
	rr := httptest.NewRecorder()

	h.UpdateActivity(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (not 500): %s", rr.Code, rr.Body.String())
	}
}

// Erasing a recorded reply maps to 422 (not 500) at the HTTP boundary.
func TestUpdateActivityEraseQuickReplyReturns422Integration(t *testing.T) {
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
	qrID := seedQuickReplyTagBehavior(t, db, "Interested", "log")
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call", QuickReplyID: &qrID})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/leads/"+created.ID+"/activities/"+act.ID,
		strings.NewReader(`{"quick_reply_id":""}`),
	)
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("id", created.ID)
	ctx.URLParams.Add("activity_id", act.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
	rr := httptest.NewRecorder()

	h.UpdateActivity(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (not 500): %s", rr.Code, rr.Body.String())
	}
}

// A task with no recorded reply can return to open; its event stamps clear so
// the open set and the timeline agree.
func TestUpdateActivityUncompleteClearsStampsIntegration(t *testing.T) {
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
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}
	done := true
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{IsDone: &done}); err != nil {
		t.Fatalf("complete activity: %v", err)
	}

	undone := false
	reopened, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{IsDone: &undone})
	if err != nil {
		t.Fatalf("un-complete plain task: %v", err)
	}
	if reopened.IsDone || reopened.OccurredAt != nil || reopened.RespondedAt != nil {
		t.Errorf(
			"reopened = done %v, occurred %v, responded %v; want open with nil stamps",
			reopened.IsDone, reopened.OccurredAt, reopened.RespondedAt,
		)
	}

	open, openTotal, err := svc.listAllActivities(ActivityListFilters{Status: "open", Page: 1, PerPage: 50})
	if err != nil {
		t.Fatalf("list open: %v", err)
	}
	if openTotal != 1 || len(open) != 1 || open[0].ID != act.ID {
		t.Errorf("open filter = %d rows %+v, want the reopened task", openTotal, open)
	}
	doneItems, doneTotal, err := svc.listAllActivities(ActivityListFilters{Status: "done", Page: 1, PerPage: 50})
	if err != nil {
		t.Fatalf("list done: %v", err)
	}
	if doneTotal != 0 || len(doneItems) != 0 {
		t.Errorf("done filter = %d rows %+v, want none", doneTotal, doneItems)
	}
}

// A task created with a quick reply reads as done on the list filters from the
// moment it is saved.
func TestListAllActivitiesQuickReplyRowsAreDoneIntegration(t *testing.T) {
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
	qrID := seedQuickReplyTagBehavior(t, db, "Share Details", "log")
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call", QuickReplyID: &qrID})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	open, openTotal, err := svc.listAllActivities(ActivityListFilters{Status: "open", Page: 1, PerPage: 50})
	if err != nil {
		t.Fatalf("list open: %v", err)
	}
	if openTotal != 0 || len(open) != 0 {
		t.Errorf("open filter = %d rows %+v, want none", openTotal, open)
	}
	doneItems, doneTotal, err := svc.listAllActivities(ActivityListFilters{Status: "done", Page: 1, PerPage: 50})
	if err != nil {
		t.Fatalf("list done: %v", err)
	}
	if doneTotal != 1 || len(doneItems) != 1 || doneItems[0].ID != act.ID {
		t.Errorf("done filter = %d rows %+v, want the logged attempt", doneTotal, doneItems)
	}
}

// A quick reply that completes the task with a follow-up spawns the next
// Open task: the completion comes from the reply, not from an explicit is_done.
func TestUpdateActivityQuickReplyWithFollowUpSpawnsNextIntegration(t *testing.T) {
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
	qrID := seedQuickReplyTagBehavior(t, db, "Rescheduled", "next")
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	next := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	updated, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{
		QuickReplyID: &qrID,
		FollowUp:     &FollowUpRequest{ScheduledAt: next},
	})
	if err != nil {
		t.Fatalf("follow-up with a quick reply: %v", err)
	}
	if !updated.IsDone {
		t.Error("the logged attempt should be done")
	}

	var nextCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM lead_activities WHERE lead_id = $1 AND type = 'Call' AND NOT is_done AND scheduled_at = $2`,
		created.ID, next,
	).Scan(&nextCount); err != nil {
		t.Fatalf("count next tasks: %v", err)
	}
	if nextCount != 1 {
		t.Errorf("next tasks = %d, want 1", nextCount)
	}
}

// The due_at sort uses the due boundary: the end for a range, the start for a
// point task, the reminder for reply-only entries, then creation as the last key.
func TestListAllActivitiesDueSortUsesBoundaryIntegration(t *testing.T) {
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

	now := time.Now().UTC()
	pointStart := now.Add(-2 * time.Hour)
	remindOnly := now.Add(-90 * time.Minute)
	rangeStart := now.Add(-3 * time.Hour)
	rangeEnd := now.Add(-30 * time.Minute)

	if _, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Point", ScheduledAt: &pointStart}); err != nil {
		t.Fatalf("create point task: %v", err)
	}
	if _, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Reminder", RemindAt: &remindOnly}); err != nil {
		t.Fatalf("create reminder-only task: %v", err)
	}
	if _, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Range", ScheduledAt: &rangeStart, ScheduledEndAt: &rangeEnd}); err != nil {
		t.Fatalf("create range task: %v", err)
	}

	items, total, err := svc.listAllActivities(ActivityListFilters{Sort: "due_at", Order: "asc", Page: 1, PerPage: 50})
	if err != nil {
		t.Fatalf("list due asc: %v", err)
	}
	if total != 3 || len(items) != 3 {
		t.Fatalf("due asc = %d rows (total %d), want 3", len(items), total)
	}
	want := []string{"Point", "Reminder", "Range"}
	for i, item := range items {
		if item.Type != want[i] {
			t.Errorf("due asc [%d] = %q, want %q", i, item.Type, want[i])
		}
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

func TestUpdateActivityFollowUpSpawnsNextIntegration(t *testing.T) {
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
		IsDone:   &done,
		FollowUp: &FollowUpRequest{ScheduledAt: next},
	})
	if err != nil {
		t.Fatalf("follow-up update: %v", err)
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

func TestCreateActivityFollowUpSpawnsNextIntegration(t *testing.T) {
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
		Type:     "Call 1",
		FollowUp: &FollowUpRequest{ScheduledAt: next},
	})
	if err != nil {
		t.Fatalf("create activity with follow-up: %v", err)
	}
	if !act.IsDone {
		t.Error("completed attempt should be marked done")
	}
	if act.RespondedAt == nil {
		t.Error("responded_at should be stamped on a follow-up attempt")
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

// An all-day follow-up carries its whole span: the day start, the day end,
// and an explicit 09:00 remind that must not be shifted by the nudge-lead
// default — the default would land it on the previous evening.
func TestUpdateActivityFollowUpAllDaySpanIntegration(t *testing.T) {
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
	qrID := seedQuickReplyTagBehavior(t, db, "Rescheduled", "next")
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	dayStart := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	dayEnd := dayStart.Add(24*time.Hour - time.Millisecond)
	remindAt := dayStart.Add(9 * time.Hour)
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{
		QuickReplyID: &qrID,
		FollowUp: &FollowUpRequest{
			ScheduledAt:    dayStart,
			ScheduledEndAt: &dayEnd,
			RemindAt:       &remindAt,
		},
	}); err != nil {
		t.Fatalf("all-day follow-up: %v", err)
	}

	var start, end, remind *time.Time
	if err := db.QueryRow(
		`SELECT scheduled_at, scheduled_end_at, remind_at FROM lead_activities WHERE lead_id = $1 AND is_done = false`,
		created.ID,
	).Scan(&start, &end, &remind); err != nil {
		t.Fatalf("load follow-up: %v", err)
	}
	if start == nil || !start.Equal(dayStart) {
		t.Errorf("follow-up scheduled_at = %v, want %v", start, dayStart)
	}
	if end == nil || !end.Equal(dayEnd) {
		t.Errorf("follow-up scheduled_end_at = %v, want %v", end, dayEnd)
	}
	if remind == nil || !remind.Equal(remindAt) {
		t.Errorf("follow-up remind_at = %v, want the explicit %v (not lead-shifted)", remind, remindAt)
	}
}

// The create path round-trips the same all-day span.
func TestCreateActivityFollowUpAllDaySpanIntegration(t *testing.T) {
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

	dayStart := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	dayEnd := dayStart.Add(24*time.Hour - time.Millisecond)
	remindAt := dayStart.Add(9 * time.Hour)
	if _, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{
		Type: "Call",
		FollowUp: &FollowUpRequest{
			ScheduledAt:    dayStart,
			ScheduledEndAt: &dayEnd,
			RemindAt:       &remindAt,
		},
	}); err != nil {
		t.Fatalf("create with all-day follow-up: %v", err)
	}

	var start, end, remind *time.Time
	if err := db.QueryRow(
		`SELECT scheduled_at, scheduled_end_at, remind_at FROM lead_activities WHERE lead_id = $1 AND is_done = false`,
		created.ID,
	).Scan(&start, &end, &remind); err != nil {
		t.Fatalf("load follow-up: %v", err)
	}
	if start == nil || !start.Equal(dayStart) {
		t.Errorf("follow-up scheduled_at = %v, want %v", start, dayStart)
	}
	if end == nil || !end.Equal(dayEnd) {
		t.Errorf("follow-up scheduled_end_at = %v, want %v", end, dayEnd)
	}
	if remind == nil || !remind.Equal(remindAt) {
		t.Errorf("follow-up remind_at = %v, want the explicit %v (not lead-shifted)", remind, remindAt)
	}
}

// A follow-up's type overrides the completed task's type when set.
func TestUpdateActivityFollowUpTypeOverrideIntegration(t *testing.T) {
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
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	done := true
	next := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{
		IsDone:   &done,
		FollowUp: &FollowUpRequest{Type: "Send Details", ScheduledAt: next},
	}); err != nil {
		t.Fatalf("typed follow-up: %v", err)
	}

	var nextType string
	if err := db.QueryRow(
		`SELECT type FROM lead_activities WHERE lead_id = $1 AND is_done = false`,
		created.ID,
	).Scan(&nextType); err != nil {
		t.Fatalf("load follow-up: %v", err)
	}
	if nextType != "Send Details" {
		t.Errorf("follow-up type = %q, want %q", nextType, "Send Details")
	}
}

// A follow-up without an explicit remind takes the nudge-lead default before
// its start.
func TestUpdateActivityFollowUpDefaultNudgeIntegration(t *testing.T) {
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
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	done := true
	next := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{
		IsDone:   &done,
		FollowUp: &FollowUpRequest{ScheduledAt: next},
	}); err != nil {
		t.Fatalf("follow-up without explicit remind: %v", err)
	}

	var remind *time.Time
	if err := db.QueryRow(
		`SELECT remind_at FROM lead_activities WHERE lead_id = $1 AND is_done = false`,
		created.ID,
	).Scan(&remind); err != nil {
		t.Fatalf("load follow-up: %v", err)
	}
	if remind == nil || !remind.Before(next) {
		t.Errorf("follow-up remind_at = %v, want a nudge before %v", remind, next)
	}
}

// A done row sorts by its happened stamp, not by the future date it was once
// scheduled for.
func TestListAllActivitiesDueSortDoneRowsKeyOnHappenedIntegration(t *testing.T) {
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

	// The legacy ghost shape: scheduled far ahead, completed now.
	future := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Second)
	ghost, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Ghost", ScheduledAt: &future})
	if err != nil {
		t.Fatalf("create ghost task: %v", err)
	}
	done := true
	if _, err := svc.updateActivity(created.ID, ghost.ID, "", UpdateActivityRequest{IsDone: &done}); err != nil {
		t.Fatalf("complete ghost task: %v", err)
	}
	// An open task due within the hour.
	soon := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	if _, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Soon", ScheduledAt: &soon}); err != nil {
		t.Fatalf("create soon task: %v", err)
	}

	items, _, err := svc.listAllActivities(ActivityListFilters{Sort: "due_at", Order: "asc", Page: 1, PerPage: 50})
	if err != nil {
		t.Fatalf("list due asc: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("due asc = %d rows, want 2", len(items))
	}
	if items[0].Type != "Ghost" {
		t.Errorf("due asc [0] = %q, want %q (the happened-now row first, not its future schedule)", items[0].Type, "Ghost")
	}
	if items[1].Type != "Soon" {
		t.Errorf("due asc [1] = %q, want %q", items[1].Type, "Soon")
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

	// No spawn: is_done without a follow-up is a plain completion.
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
		`INSERT INTO lead_stages (pipeline_id, name, "order", outcome) VALUES ($1, 'Closed', 99, 'lost') RETURNING id`,
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

	// "Reopen" the lead back into the open stage: under the one-row-one-cycle
	// model this spawns a NEW row; the old row stays terminal.
	spawned, err := svc.update(created.ID, UpdateRequest{StageID: &stageID}, "")
	if err != nil {
		t.Fatalf("reopen lead: %v", err)
	}
	if spawned.ID == created.ID {
		t.Error("expected a new cycle row for the reopened lead, got the same id")
	}

	// Editing the old closing touchpoint (description only) must not re-close
	// the old terminal row or touch the new cycle.
	desc := "corrected note"
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{Description: &desc}); err != nil {
		t.Fatalf("edit done close_lost activity: %v", err)
	}

	got, err := svc.get(created.ID)
	if err != nil {
		t.Fatalf("get old lead: %v", err)
	}
	if got.StageID == stageID {
		t.Errorf("old row stage_id = %q, want the closing stage (terminal, not reopened)", got.StageID)
	}
	if got.Outcome != "lost" {
		t.Errorf("old row outcome = %q, want lost (terminal)", got.Outcome)
	}

	spawnedGot, err := svc.get(spawned.ID)
	if err != nil {
		t.Fatalf("get new cycle: %v", err)
	}
	if spawnedGot.StageID != stageID {
		t.Errorf("new cycle stage_id = %q, want open stage %q", spawnedGot.StageID, stageID)
	}
	if spawnedGot.Outcome != "" {
		t.Errorf("new cycle outcome = %q, want empty", spawnedGot.Outcome)
	}
}

func TestCreateActivityOnClosedLeadRejectedIntegration(t *testing.T) {
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

	if _, err := svc.createActivity(created.ID, closingStage, "", CreateActivityRequest{Type: "Call"}); !errors.Is(err, ErrLeadClosed) {
		t.Errorf("create activity on a closed lead = %v, want ErrLeadClosed", err)
	}
}

func TestUpdateActivityOnClosedLeadAllowsRecordFixesIntegration(t *testing.T) {
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
	sched := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	remind := sched.Add(-15 * time.Minute)
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{
		Type:        "Call",
		Description: "original",
		ScheduledAt: &sched,
		RemindAt:    &remind,
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}
	// Closing cancels the open task; the row stays as permanent history.
	if _, err := svc.update(created.ID, UpdateRequest{StageID: &closingStage}, ""); err != nil {
		t.Fatalf("close lead: %v", err)
	}

	// Record fixes stay allowed: description edits, resubmitting the stored
	// schedule/reminder at the minute precision the edit form round-trips,
	// clearing the schedule, and completing a historical row.
	desc := "corrected"
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{Description: &desc}); err != nil {
		t.Fatalf("description edit on a closed lead: %v", err)
	}
	minuteSched := sched.Truncate(time.Minute)
	minuteRemind := remind.Truncate(time.Minute)
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{ScheduledAt: optTime(&minuteSched)}); err != nil {
		t.Fatalf("resubmitting the stored schedule on a closed lead: %v", err)
	}
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{RemindAt: optTime(&minuteRemind)}); err != nil {
		t.Fatalf("resubmitting the stored reminder on a closed lead: %v", err)
	}
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{ScheduledAt: optTime(nil)}); err != nil {
		t.Fatalf("clearing the schedule on a closed lead: %v", err)
	}
	done := true
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{IsDone: &done}); err != nil {
		t.Fatalf("completing a historical row on a closed lead: %v", err)
	}

	// A historical row whose flags are already false (seeded directly, as
	// legacy rows can be) accepts a resubmitted false: it is a no-op, not a
	// reactivation.
	var quietID string
	if err := db.QueryRow(
		`INSERT INTO lead_activities (lead_id, stage_id, type, is_done, is_cancelled)
		VALUES ($1, $2, 'Note', false, false) RETURNING id`,
		created.ID, closingStage,
	).Scan(&quietID); err != nil {
		t.Fatalf("seed historical open row: %v", err)
	}
	stillDone, stillCancelled := false, false
	if _, err := svc.updateActivity(created.ID, quietID, "", UpdateActivityRequest{IsDone: &stillDone}); err != nil {
		t.Fatalf("resubmitting stored is_done=false on a closed lead: %v", err)
	}
	if _, err := svc.updateActivity(created.ID, quietID, "", UpdateActivityRequest{IsCancelled: &stillCancelled}); err != nil {
		t.Fatalf("resubmitting stored is_cancelled=false on a closed lead: %v", err)
	}

	// Reactivation attempts are refused.
	uncancel := false
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{IsCancelled: &uncancel}); !errors.Is(err, ErrLeadClosed) {
		t.Errorf("un-cancel on a closed lead = %v, want ErrLeadClosed", err)
	}
	undone := false
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{IsDone: &undone}); !errors.Is(err, ErrLeadClosed) {
		t.Errorf("un-complete on a closed lead = %v, want ErrLeadClosed", err)
	}
	future := time.Now().Add(3 * time.Hour).UTC().Truncate(time.Second)
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{ScheduledAt: optTime(&future)}); !errors.Is(err, ErrLeadClosed) {
		t.Errorf("new schedule on a closed lead = %v, want ErrLeadClosed", err)
	}
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{ScheduledEndAt: optTime(&future)}); !errors.Is(err, ErrLeadClosed) {
		t.Errorf("new end on a closed lead = %v, want ErrLeadClosed", err)
	}
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{RemindAt: optTime(&future)}); !errors.Is(err, ErrLeadClosed) {
		t.Errorf("new reminder on a closed lead = %v, want ErrLeadClosed", err)
	}
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{FollowUp: &FollowUpRequest{ScheduledAt: future}}); !errors.Is(err, ErrLeadClosed) {
		t.Errorf("follow-up on a closed lead = %v, want ErrLeadClosed", err)
	}
}

func TestCloseLostTxIgnoresTerminalLeadIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	// Seed both a won and a lost closing stage: without the terminal guard,
	// close_lost would move the won lead into the lost stage.
	var wonStage string
	if err := db.QueryRow(
		`INSERT INTO lead_stages (pipeline_id, name, "order", outcome) VALUES ($1, 'Won', 1, 'won') RETURNING id`,
		pipelineID,
	).Scan(&wonStage); err != nil {
		t.Fatalf("seed won stage: %v", err)
	}
	_ = seedClosingStage(t, db, pipelineID)
	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	if _, err := svc.update(created.ID, UpdateRequest{StageID: &wonStage}, ""); err != nil {
		t.Fatalf("win lead: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	moved, err := svc.closeLostTx(tx, created.ID, "")
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("closeLostTx on a terminal lead: %v", err)
	}
	// Read through the transaction so a move that happened inside it would be
	// visible; reading after the rollback would make this assertion vacuous.
	var stageAfter string
	if err := tx.QueryRow(`SELECT stage_id FROM leads WHERE id = $1`, created.ID).Scan(&stageAfter); err != nil {
		_ = tx.Rollback()
		t.Fatalf("load stage after closeLostTx: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if moved {
		t.Error("closeLostTx moved a terminal lead; want a no-op")
	}
	if stageAfter != wonStage {
		t.Errorf("stage after closeLostTx = %q, want the won stage %q unchanged", stageAfter, wonStage)
	}
}

func TestPendingRemindersExcludeClosedLeadsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	closingStage := seedClosingStage(t, db, pipelineID)
	openLead, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create open lead: %v", err)
	}
	closedLead, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Bob", Phone: "0987654321"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create to-be-closed lead: %v", err)
	}
	if _, err := svc.update(closedLead.ID, UpdateRequest{StageID: &closingStage}, ""); err != nil {
		t.Fatalf("close lead: %v", err)
	}

	// Seed the reminders directly: the write guard refuses creating them
	// through the service on a closed lead, so this pins the read-side filter.
	for _, leadID := range []string{openLead.ID, closedLead.ID} {
		if _, err := db.Exec(
			`INSERT INTO lead_activities (lead_id, stage_id, type, remind_at)
			VALUES ($1, $2, 'call', $3)`,
			leadID, stageID, time.Now().Add(time.Hour),
		); err != nil {
			t.Fatalf("seed reminder: %v", err)
		}
	}

	reminders, err := svc.getPendingReminders("00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("get pending reminders: %v", err)
	}
	if len(reminders) != 1 || reminders[0].LeadID != openLead.ID {
		t.Errorf("pending reminders = %+v, want only the open lead's reminder", reminders)
	}
}

func TestUpdateActivityReturnsJoinedNamesIntegration(t *testing.T) {
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
	qrID := seedQuickReplyTagBehavior(t, db, "Interested", "log")
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call", QuickReplyID: &qrID})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	desc := "edited"
	updated, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{Description: &desc})
	if err != nil {
		t.Fatalf("update activity: %v", err)
	}
	// The edit response carries the joined display names the API advertises,
	// not the empty literals the old RETURNING produced.
	if updated.StageName != "New" {
		t.Errorf("stage name = %q, want New", updated.StageName)
	}
	if updated.QuickReplyName != "Interested" {
		t.Errorf("quick reply name = %q, want Interested", updated.QuickReplyName)
	}
}

// A recorded quick reply can be replaced but not erased: the reply is history.
func TestUpdateActivityRefusesQuickReplyErasureIntegration(t *testing.T) {
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
	qrID := seedQuickReplyTagBehavior(t, db, "Interested", "log")
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call", QuickReplyID: &qrID})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}
	if act.QuickReplyID != qrID {
		t.Fatalf("quick_reply_id = %q, want %q", act.QuickReplyID, qrID)
	}

	empty := ""
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{QuickReplyID: &empty}); !errors.Is(err, ErrQuickReplyErasure) {
		t.Fatalf("erase quick reply = %v, want ErrQuickReplyErasure", err)
	}

	var stored sql.NullString
	if err := db.QueryRow(`SELECT quick_reply_id FROM lead_activities WHERE id = $1`, act.ID).Scan(&stored); err != nil {
		t.Fatalf("load stored quick reply: %v", err)
	}
	if !stored.Valid || stored.String != qrID {
		t.Errorf("stored quick_reply_id = %q (valid %v), want %q", stored.String, stored.Valid, qrID)
	}
}

// A recorded quick reply is editable: a new reply replaces the old one, and the
// first response time stands.
func TestUpdateActivityReplacesQuickReplyIntegration(t *testing.T) {
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
	firstID := seedQuickReplyTagBehavior(t, db, "Interested", "log")
	secondID := seedQuickReplyTagBehavior(t, db, "Share Details", "log")
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call", QuickReplyID: &firstID})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}
	firstStamp := act.RespondedAt

	replaced, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{QuickReplyID: &secondID})
	if err != nil {
		t.Fatalf("replace quick reply: %v", err)
	}
	if replaced.QuickReplyID != secondID || replaced.QuickReplyName != "Share Details" {
		t.Errorf("reply = %q/%q, want %q/Share Details", replaced.QuickReplyID, replaced.QuickReplyName, secondID)
	}
	if firstStamp == nil || replaced.RespondedAt == nil || replaced.RespondedAt.Unix() != firstStamp.Unix() {
		t.Errorf("responded_at = %v, want the first response time %v", replaced.RespondedAt, firstStamp)
	}
}

// A cancelled row that still carries a reply (the shape the backfill leaves
// alone) must not return to open through un-cancel.
func TestUpdateActivityRefusesUncancelToOpenWithReplyIntegration(t *testing.T) {
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
	qrID := seedQuickReplyTagBehavior(t, db, "Interested", "log")
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}
	// The service cannot produce this legacy shape; the backfill leaves it as
	// it is, so seed it directly.
	if _, err := db.Exec(
		`UPDATE lead_activities SET quick_reply_id = $2, is_cancelled = true WHERE id = $1`,
		act.ID, qrID,
	); err != nil {
		t.Fatalf("seed legacy cancelled reply row: %v", err)
	}

	no := false
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{IsCancelled: &no}); !errors.Is(err, ErrQuickReplyRecorded) {
		t.Fatalf("un-cancel = %v, want ErrQuickReplyRecorded", err)
	}

	var isCancelled, isDone bool
	var storedReply sql.NullString
	if err := db.QueryRow(
		`SELECT is_cancelled, is_done, quick_reply_id FROM lead_activities WHERE id = $1`, act.ID,
	).Scan(&isCancelled, &isDone, &storedReply); err != nil {
		t.Fatalf("load row: %v", err)
	}
	if !isCancelled || isDone || !storedReply.Valid || storedReply.String != qrID {
		t.Errorf("row = cancelled %v / done %v / reply %q, want the refusal to leave it untouched", isCancelled, isDone, storedReply.String)
	}
}

// Un-cancelling a task without a reply stays allowed.
func TestUpdateActivityUncancelsWithoutReplyIntegration(t *testing.T) {
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
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	yes := true
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{IsCancelled: &yes}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	no := false
	uncancelled, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{IsCancelled: &no})
	if err != nil {
		t.Fatalf("un-cancel: %v", err)
	}
	if uncancelled.IsCancelled || uncancelled.IsDone {
		t.Errorf("row = cancelled %v / done %v, want a plain un-cancel", uncancelled.IsCancelled, uncancelled.IsDone)
	}
}

// An erasure racing a concurrently recorded reply must not erase it: the
// UPDATE's WHERE re-checks the refusal against the row it actually updates.
func TestUpdateActivityEraseRacingReplyRecordIsRefusedIntegration(t *testing.T) {
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
	qrID := seedQuickReplyTagBehavior(t, db, "Interested", "log")
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	// Hold a reply-recording write open so the erasure's UPDATE blocks on it
	// and re-evaluates the WHERE against the row the holder commits.
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin holder: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`UPDATE lead_activities SET quick_reply_id = $2, is_done = true WHERE id = $1`,
		act.ID, qrID,
	); err != nil {
		t.Fatalf("hold reply write: %v", err)
	}

	empty := ""
	ch := make(chan error, 1)
	go func() {
		_, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{QuickReplyID: &empty})
		ch <- err
	}()

	// Wait until the service's UPDATE waits on the held row lock. Filter to
	// this database so parallel package tests cannot match.
	blocked := false
	for i := 0; i < 100; i++ {
		var n int
		if err := db.QueryRow(
			`SELECT count(*) FROM pg_stat_activity
			 WHERE datname = current_database()
			   AND wait_event_type = 'Lock' AND query ILIKE '%UPDATE lead_activities%'`,
		).Scan(&n); err != nil {
			t.Fatalf("poll lock wait: %v", err)
		}
		if n > 0 {
			blocked = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("service update never waited on the row lock; cannot assert the re-check")
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit holder: %v", err)
	}

	select {
	case err := <-ch:
		if !errors.Is(err, ErrQuickReplyErasure) {
			t.Fatalf("erase racing reply = %v, want ErrQuickReplyErasure", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("erase call did not return after the holder committed")
	}

	var storedReply sql.NullString
	var isDone bool
	if err := db.QueryRow(
		`SELECT quick_reply_id, is_done FROM lead_activities WHERE id = $1`, act.ID,
	).Scan(&storedReply, &isDone); err != nil {
		t.Fatalf("load row: %v", err)
	}
	if !storedReply.Valid || storedReply.String != qrID || !isDone {
		t.Errorf("row = reply %q / done %v, want the concurrent reply intact", storedReply.String, isDone)
	}
}

// A no-op clear on a row without a reply still passes through: only erasing a
// recorded reply is refused.
func TestUpdateActivityEraseQuickReplyWithoutReplyPassesIntegration(t *testing.T) {
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
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	empty := ""
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{QuickReplyID: &empty}); err != nil {
		t.Fatalf("no-op clear = %v, want success", err)
	}

	var storedReply sql.NullString
	if err := db.QueryRow(`SELECT quick_reply_id FROM lead_activities WHERE id = $1`, act.ID).Scan(&storedReply); err != nil {
		t.Fatalf("load stored quick reply: %v", err)
	}
	if storedReply.Valid {
		t.Errorf("stored quick_reply_id = %q, want NULL", storedReply.String)
	}
}

// Un-cancelling a completed reply-bearing row is not a reopen: it stays done,
// so the update is allowed.
func TestUpdateActivityUncancelsCompletedReplyRowIntegration(t *testing.T) {
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
	qrID := seedQuickReplyTagBehavior(t, db, "Interested", "log")
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call", QuickReplyID: &qrID})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	yes := true
	if _, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{IsCancelled: &yes}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	no := false
	uncancelled, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{IsCancelled: &no})
	if err != nil {
		t.Fatalf("un-cancel completed reply row: %v", err)
	}
	if uncancelled.IsCancelled || !uncancelled.IsDone || uncancelled.QuickReplyID != qrID {
		t.Errorf("row = cancelled %v / done %v / reply %q, want the restored done row", uncancelled.IsCancelled, uncancelled.IsDone, uncancelled.QuickReplyID)
	}
}

// An un-cancel that completes the legacy row in the same request ends
// consistent and is allowed.
func TestUpdateActivityUncancelCompletingLegacyRowIntegration(t *testing.T) {
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
	qrID := seedQuickReplyTagBehavior(t, db, "Interested", "log")
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{Type: "Call"})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}
	// The legacy shape again: cancelled, open, reply carried.
	if _, err := db.Exec(
		`UPDATE lead_activities SET quick_reply_id = $2, is_cancelled = true WHERE id = $1`,
		act.ID, qrID,
	); err != nil {
		t.Fatalf("seed legacy cancelled reply row: %v", err)
	}

	no := false
	done := true
	updated, err := svc.updateActivity(created.ID, act.ID, "", UpdateActivityRequest{IsCancelled: &no, IsDone: &done})
	if err != nil {
		t.Fatalf("un-cancel + complete = %v, want success", err)
	}
	if updated.IsCancelled || !updated.IsDone || updated.QuickReplyID != qrID {
		t.Errorf("row = cancelled %v / done %v / reply %q, want the completed row", updated.IsCancelled, updated.IsDone, updated.QuickReplyID)
	}
}

func TestListAllActivitiesClampsHugePageIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	items, total, err := svc.listAllActivities(ActivityListFilters{Page: math.MaxInt, PerPage: 100})
	if err != nil {
		t.Fatalf("list with huge page: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Errorf("items/total = %d/%d, want 0/0 on an empty database", len(items), total)
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

func TestCreateCloseLostQuickReplyClosesLeadIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	pipelineID, stageID := seedPipelineAndStage(t, db)
	lostStageID := seedClosingStage(t, db, pipelineID)
	created, err := svc.create(CreateRequest{
		NewContact: &NewContact{Name: "Alice", Phone: "1234567890"},
		PipelineID: pipelineID,
		StageID:    stageID,
	}, "")
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	qrID := seedQuickReplyTagBehavior(t, db, "Closed Lost", "close_lost")

	// A saved close_lost reply completes the attempt, so the deal ends at save.
	act, err := svc.createActivity(created.ID, stageID, "", CreateActivityRequest{
		Type:         "Call 1",
		QuickReplyID: &qrID,
	})
	if err != nil {
		t.Fatalf("create close_lost activity: %v", err)
	}
	if !act.IsDone {
		t.Error("a saved close_lost reply should complete the activity")
	}

	got, err := svc.get(created.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.StageID != lostStageID {
		t.Errorf("stage_id = %q, want lost stage %q", got.StageID, lostStageID)
	}
	if got.Outcome != "lost" {
		t.Errorf("outcome = %q, want lost", got.Outcome)
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
	// the schedule (the default nudge lead time).
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
