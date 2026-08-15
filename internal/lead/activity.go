package lead

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrInvalidOutcome marks an outcome_id that does not reference a status tag.
var ErrInvalidOutcome = errors.New("outcome_id must reference a status tag")

const activitySelect = `
	SELECT la.id, la.lead_id, la.stage_id, COALESCE(ls.name, ''), la.user_id, COALESCE(u.name, ''),
		la.type, la.description, la.outcome_id, t.name,
		la.scheduled_at, la.remind_at, la.responded_at, la.occurred_at,
		la.is_done, la.is_cancelled, la.is_reminded, la.created_at, la.updated_at
	FROM lead_activities la
	LEFT JOIN users u ON u.id = la.user_id
	LEFT JOIN lead_stages ls ON ls.id = la.stage_id
	LEFT JOIN tags t ON t.id = la.outcome_id`

// validateOutcomeTx rejects outcome ids that do not reference a status tag.
func (s *Service) validateOutcomeTx(q interface {
	QueryRow(query string, args ...any) *sql.Row
}, outcomeID *string) error {
	if outcomeID == nil || *outcomeID == "" {
		return nil
	}
	var exists bool
	err := q.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM tags WHERE id = $1 AND type = 'status')`,
		*outcomeID,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("validate outcome: %w", err)
	}
	if !exists {
		return ErrInvalidOutcome
	}
	return nil
}

// ErrEmptyType marks an activity request with a blank type. Descriptions are
// optional (ADR 018) — a quick "Call 1 / Busy, reschedule" entry needs no prose.
var ErrEmptyType = errors.New("activity type cannot be empty")

// validateActivityFields enforces the shared create/update invariant: a type
// is never blank. Descriptions are optional.
func validateActivityFields(typeValue string) error {
	if strings.TrimSpace(typeValue) == "" {
		return ErrEmptyType
	}
	return nil
}

// scanActivity scans one row produced by activitySelect into an Activity.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanActivity(scan rowScanner) (Activity, error) {
	var a Activity
	var outcomeID sql.NullString
	var outcomeName sql.NullString
	if err := scan.Scan(
		&a.ID, &a.LeadID, &a.StageID, &a.StageName, &a.UserID, &a.UserName,
		&a.Type, &a.Description, &outcomeID, &outcomeName,
		&a.ScheduledAt, &a.RemindAt, &a.RespondedAt, &a.OccurredAt,
		&a.IsDone, &a.IsCancelled, &a.IsReminded, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return a, err
	}
	a.OutcomeID = outcomeID.String
	a.OutcomeName = outcomeName.String
	return a, nil
}

func (s *Service) listActivities(leadID string, page, perPage int) ([]Activity, int, error) {
	var total int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM lead_activities WHERE lead_id = $1`,
		leadID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count activities: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := s.db.Query(activitySelect+`
		WHERE la.lead_id = $1
		ORDER BY la.created_at DESC
		LIMIT $2 OFFSET $3`, leadID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list activities: %w", err)
	}
	defer rows.Close()
	activities := []Activity{}
	for rows.Next() {
		a, err := scanActivity(rows)
		if err != nil {
			return nil, 0, err
		}
		activities = append(activities, a)
	}
	return activities, total, nil
}

func (s *Service) createActivity(leadID, stageID, userID string, req CreateActivityRequest) (*Activity, error) {
	if err := validateActivityFields(req.Type); err != nil {
		return nil, err
	}
	if err := s.validateOutcomeTx(s.db, req.OutcomeID); err != nil {
		return nil, err
	}
	// An activity created with an outcome is already "responded" — log the time.
	// A reschedule_at also implies the attempt happened, so it is done too.
	// IsDone completes an activity created in one shot (close_lost from the
	// create form), stamping occurred_at so it survives the closing-stage move.
	var respondedAt any
	if (req.OutcomeID != nil && *req.OutcomeID != "") || req.RescheduleAt != nil {
		respondedAt = time.Now()
	}
	isDone := req.RescheduleAt != nil || (req.IsDone != nil && *req.IsDone)
	var occurredAt any
	if isDone {
		occurredAt = time.Now()
	}
	desc := strings.TrimSpace(req.Description)
	var a Activity
	var outcomeID sql.NullString
	var outcomeName sql.NullString
	err := s.db.QueryRow(`
		WITH ins AS (
			INSERT INTO lead_activities (lead_id, stage_id, user_id, type, description, outcome_id, scheduled_at, remind_at, responded_at, occurred_at, is_done)
			VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5, NULLIF($6, '')::uuid, $7, $8, $9, $10, $11)
			RETURNING id, lead_id, stage_id, user_id, type, description, outcome_id,
				scheduled_at, remind_at, responded_at, occurred_at, is_done, is_cancelled, is_reminded, created_at, updated_at
		)
		SELECT ins.id, ins.lead_id, ins.stage_id, COALESCE(ls.name, ''), ins.user_id, COALESCE(u.name, ''),
			ins.type, ins.description, ins.outcome_id, COALESCE(t.name, ''),
			ins.scheduled_at, ins.remind_at, ins.responded_at, ins.occurred_at,
			ins.is_done, ins.is_cancelled, ins.is_reminded, ins.created_at, ins.updated_at
		FROM ins
		LEFT JOIN users u ON u.id = ins.user_id
		LEFT JOIN lead_stages ls ON ls.id = ins.stage_id
		LEFT JOIN tags t ON t.id = ins.outcome_id`,
		leadID, stageID, userID, req.Type, desc, req.OutcomeID, req.ScheduledAt, req.RemindAt, respondedAt, occurredAt, isDone,
	).Scan(
		&a.ID, &a.LeadID, &a.StageID, &a.StageName, &a.UserID, &a.UserName,
		&a.Type, &a.Description, &outcomeID, &outcomeName,
		&a.ScheduledAt, &a.RemindAt, &a.RespondedAt, &a.OccurredAt,
		&a.IsDone, &a.IsCancelled, &a.IsReminded, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create activity: %w", err)
	}
	a.OutcomeID = outcomeID.String
	a.OutcomeName = outcomeName.String

	// "Log attempt + next": a created-and-completed activity with a reschedule
	// time spawns the next occurrence of the same type at the new time.
	if req.RescheduleAt != nil {
		if _, err := s.createActivity(leadID, a.StageID, userID, CreateActivityRequest{
			Type:        req.Type,
			ScheduledAt: req.RescheduleAt,
			RemindAt:    req.RescheduleAt,
		}); err != nil {
			return nil, fmt.Errorf("create next activity: %w", err)
		}
	}
	return &a, nil
}

// updateActivity updates an activity's task fields (type, description,
// scheduled_at, remind_at), lifecycle state (done, cancelled), and outcome.
// Description is optional and can be cleared by sending "". Setting an outcome
// (or marking done) on an activity that had neither auto-logs the response
// time; occurred_at is stamped on completion unless supplied explicitly.
// Editing remind_at re-opens the reminder (is_reminded = false).
//
// The "log attempt + next" reschedule flow (ADR 018): when is_done=true and a
// reschedule_at is supplied, the completed attempt is logged and a new task of
// the same type is created for reschedule_at, defaulting its reminder to the
// same time.
func (s *Service) updateActivity(leadID, activityID, userID string, req UpdateActivityRequest) (*Activity, error) {
	if err := s.validateOutcomeTx(s.db, req.OutcomeID); err != nil {
		return nil, err
	}
	if req.OutcomeID == nil && req.IsDone == nil && req.Type == nil && req.Description == nil &&
		req.ScheduledAt == nil && req.RemindAt == nil && req.OccurredAt == nil &&
		req.IsCancelled == nil && req.RescheduleAt == nil {
		return nil, errors.New("nothing to update")
	}

	// Load the current row so we only stamp response times on the null->set edge.
	var cur Activity
	var curOutcomeID sql.NullString
	err := s.db.QueryRow(`
		SELECT id, outcome_id, responded_at, is_done, type, description FROM lead_activities WHERE id = $1 AND lead_id = $2`,
		activityID, leadID,
	).Scan(&cur.ID, &curOutcomeID, &cur.RespondedAt, &cur.IsDone, &cur.Type, &cur.Description)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load activity: %w", err)
	}
	cur.OutcomeID = curOutcomeID.String

	// Validate the merged row so a partial update can't leave the type blank.
	mergedType := cur.Type
	if req.Type != nil {
		mergedType = *req.Type
	}
	if err := validateActivityFields(mergedType); err != nil {
		return nil, err
	}

	// Descriptions are optional and stored trimmed.
	var desc any
	if req.Description != nil {
		desc = strings.TrimSpace(*req.Description)
	}

	newOutcome := cur.OutcomeID
	if req.OutcomeID != nil {
		newOutcome = *req.OutcomeID
	}
	willHaveOutcome := newOutcome != ""
	markDone := false
	if req.IsDone != nil {
		markDone = *req.IsDone
	}

	// Stamp the response time when the activity gains an outcome or is marked
	// done for the first time. Clear is never applied to responded_at.
	var respondedAt any
	if cur.RespondedAt == nil && (willHaveOutcome || markDone) {
		respondedAt = time.Now()
	}

	var a Activity
	var outcomeID sql.NullString
	var outcomeName sql.NullString
	err = s.db.QueryRow(`
		UPDATE lead_activities SET
			outcome_id = COALESCE($3, outcome_id),
			is_done = COALESCE($4, is_done),
			responded_at = COALESCE($5, responded_at),
			type = COALESCE($6, type),
			description = COALESCE($7, description),
			scheduled_at = COALESCE($8, scheduled_at),
			remind_at = COALESCE($9, remind_at),
			is_reminded = CASE WHEN $9::timestamptz IS NOT NULL THEN false ELSE is_reminded END,
			occurred_at = COALESCE($10, occurred_at, CASE WHEN $4 = true THEN now() ELSE NULL END),
			is_cancelled = COALESCE($11, is_cancelled),
			updated_at = now()
		WHERE id = $1 AND lead_id = $2
		RETURNING id, lead_id, stage_id, '', user_id, '', type, description, outcome_id, '',
			scheduled_at, remind_at, responded_at, occurred_at, is_done, is_cancelled, is_reminded, created_at, updated_at`,
		activityID, leadID, req.OutcomeID, req.IsDone, respondedAt, req.Type, desc,
		req.ScheduledAt, req.RemindAt, req.OccurredAt, req.IsCancelled,
	).Scan(
		&a.ID, &a.LeadID, &a.StageID, &a.StageName, &a.UserID, &a.UserName,
		&a.Type, &a.Description, &outcomeID, &outcomeName,
		&a.ScheduledAt, &a.RemindAt, &a.RespondedAt, &a.OccurredAt,
		&a.IsDone, &a.IsCancelled, &a.IsReminded, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update activity: %w", err)
	}
	a.OutcomeID = outcomeID.String
	a.OutcomeName = outcomeName.String

	// "Log attempt + next": a completed activity with a reschedule time spawns
	// the next occurrence of the same type at the new time.
	if req.RescheduleAt != nil && (req.IsDone != nil && *req.IsDone) && !a.IsCancelled {
		if _, err := s.createActivity(leadID, a.StageID, userID, CreateActivityRequest{
			Type:        mergedType,
			ScheduledAt: req.RescheduleAt,
			RemindAt:    req.RescheduleAt,
		}); err != nil {
			return nil, fmt.Errorf("create next activity: %w", err)
		}
	}
	return &a, nil
}

func (s *Service) deleteActivity(leadID, activityID string) error {
	res, err := s.db.Exec(`DELETE FROM lead_activities WHERE id = $1 AND lead_id = $2`, activityID, leadID)
	if err != nil {
		return fmt.Errorf("delete activity: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete activity: rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// dismissReminder marks an activity as reminded and reports whether a row
// actually changed; a missing or already-reminded id is a clean (false, nil)
// instead of an error.
func (s *Service) dismissReminder(leadID, activityID string) (bool, error) {
	res, err := s.db.Exec(
		`UPDATE lead_activities SET is_reminded = true WHERE id = $1 AND lead_id = $2 AND NOT is_reminded`,
		activityID, leadID,
	)
	if err != nil {
		return false, fmt.Errorf("dismiss reminder: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("dismiss reminder: rows affected: %w", err)
	}
	return rows > 0, nil
}

// snoozeReminder pushes an activity's reminder forward and re-opens it
// (is_reminded = false) so it re-enters the pending pool. The task's scheduled
// time shifts by the same delta as the reminder, so snooze behaves as a quick
// reschedule of an open task (ADR 018). It reports whether a row actually
// changed; a missing id is a clean (false, nil) instead of an error.
func (s *Service) snoozeReminder(leadID, activityID string, remindAt time.Time) (bool, error) {
	res, err := s.db.Exec(
		`UPDATE lead_activities SET
			remind_at = $2,
			is_reminded = false,
			scheduled_at = CASE
				WHEN scheduled_at IS NOT NULL AND remind_at IS NOT NULL THEN scheduled_at + ($2 - remind_at)
				ELSE scheduled_at
			END
		WHERE id = $1 AND lead_id = $3 AND NOT is_done`,
		activityID, remindAt, leadID,
	)
	if err != nil {
		return false, fmt.Errorf("snooze reminder: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("snooze reminder: rows affected: %w", err)
	}
	return rows > 0, nil
}

// getPendingReminders returns every open task — an activity that is neither
// done nor cancelled and has a due time (scheduled_at, or remind_at for a
// reminder-only entry). Both overdue and upcoming are included so the
// reminders page can render Overdue / Upcoming / Done sections; dismissed
// (is_reminded) rows are included too so the Dismissed section can list them.
func (s *Service) getPendingReminders() ([]Activity, error) {
	rows, err := s.db.Query(activitySelect+`
		WHERE NOT la.is_done AND NOT la.is_cancelled
			AND (la.remind_at IS NOT NULL OR la.scheduled_at IS NOT NULL)
		ORDER BY COALESCE(la.remind_at, la.scheduled_at) ASC
		LIMIT 100`,
	)
	if err != nil {
		return nil, fmt.Errorf("get pending reminders: %w", err)
	}
	defer rows.Close()
	activities := []Activity{}
	for rows.Next() {
		a, err := scanActivity(rows)
		if err != nil {
			return nil, fmt.Errorf("get pending reminders: scan: %w", err)
		}
		activities = append(activities, a)
	}
	return activities, nil
}
