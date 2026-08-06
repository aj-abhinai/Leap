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
		la.scheduled_at, la.remind_at, la.responded_at, la.is_done, la.is_reminded,
		la.created_at, la.updated_at
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

// ErrEmptyType marks an activity request with a blank type.
var ErrEmptyType = errors.New("activity type cannot be empty")

// ErrEmptyDescription marks an activity request with a blank description.
var ErrEmptyDescription = errors.New("activity description is required")

// validateActivityFields enforces the shared create/update invariants: a type
// is never blank and a description is always required.
func validateActivityFields(typeValue, description string) error {
	if typeValue == "" {
		return ErrEmptyType
	}
	if strings.TrimSpace(description) == "" {
		return ErrEmptyDescription
	}
	return nil
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
		var a Activity
		var outcomeID sql.NullString
		var outcomeName sql.NullString
		if err := rows.Scan(
			&a.ID, &a.LeadID, &a.StageID, &a.StageName, &a.UserID, &a.UserName,
			&a.Type, &a.Description, &outcomeID, &outcomeName,
			&a.ScheduledAt, &a.RemindAt, &a.RespondedAt, &a.IsDone, &a.IsReminded,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		a.OutcomeID = outcomeID.String
		a.OutcomeName = outcomeName.String
		activities = append(activities, a)
	}
	return activities, total, nil
}

func (s *Service) createActivity(leadID, stageID, userID string, req CreateActivityRequest) (*Activity, error) {
	if err := validateActivityFields(req.Type, req.Description); err != nil {
		return nil, err
	}
	if err := s.validateOutcomeTx(s.db, req.OutcomeID); err != nil {
		return nil, err
	}
	// An activity created with an outcome is already "responded" — log the time.
	var respondedAt any
	if req.OutcomeID != nil && *req.OutcomeID != "" {
		respondedAt = time.Now()
	}
	var a Activity
	var outcomeID sql.NullString
	var outcomeName sql.NullString
	err := s.db.QueryRow(`
		WITH ins AS (
			INSERT INTO lead_activities (lead_id, stage_id, user_id, type, description, outcome_id, scheduled_at, remind_at, responded_at)
			VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5, NULLIF($6, '')::uuid, $7, $8, $9)
			RETURNING id, lead_id, stage_id, user_id, type, description, outcome_id,
				scheduled_at, remind_at, responded_at, is_done, is_reminded, created_at, updated_at
		)
		SELECT ins.id, ins.lead_id, ins.stage_id, COALESCE(ls.name, ''), ins.user_id, COALESCE(u.name, ''),
			ins.type, ins.description, ins.outcome_id, COALESCE(t.name, ''),
			ins.scheduled_at, ins.remind_at, ins.responded_at, ins.is_done, ins.is_reminded,
			ins.created_at, ins.updated_at
		FROM ins
		LEFT JOIN users u ON u.id = ins.user_id
		LEFT JOIN lead_stages ls ON ls.id = ins.stage_id
		LEFT JOIN tags t ON t.id = ins.outcome_id`,
		leadID, stageID, userID, req.Type, req.Description, req.OutcomeID, req.ScheduledAt, req.RemindAt, respondedAt,
	).Scan(
		&a.ID, &a.LeadID, &a.StageID, &a.StageName, &a.UserID, &a.UserName,
		&a.Type, &a.Description, &outcomeID, &outcomeName,
		&a.ScheduledAt, &a.RemindAt, &a.RespondedAt, &a.IsDone, &a.IsReminded,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create activity: %w", err)
	}
	a.OutcomeID = outcomeID.String
	a.OutcomeName = outcomeName.String
	return &a, nil
}

// updateActivity updates an activity's outcome, done flag, and/or editable
// task fields (type, description, remind_at). Setting an outcome (or marking
// done) on an activity that had neither auto-logs the response time; the
// original responded_at is never cleared or moved. Editing remind_at re-opens
// the reminder (is_reminded = false) so it re-enters the pending pool.
func (s *Service) updateActivity(leadID, activityID string, req UpdateActivityRequest) (*Activity, error) {
	if err := s.validateOutcomeTx(s.db, req.OutcomeID); err != nil {
		return nil, err
	}
	if req.OutcomeID == nil && req.IsDone == nil && req.Type == nil && req.Description == nil && req.RemindAt == nil {
		return nil, errors.New("nothing to update")
	}

	var a Activity
	// Load the current row so we only stamp responded_at on the null->set edge.
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

	// Validate the merged row so a partial update can't leave type/description blank.
	mergedType, mergedDescription := cur.Type, cur.Description
	if req.Type != nil {
		mergedType = *req.Type
	}
	if req.Description != nil {
		mergedDescription = *req.Description
	}
	if err := validateActivityFields(mergedType, mergedDescription); err != nil {
		return nil, err
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

	var outcomeID sql.NullString
	var outcomeName sql.NullString
	err = s.db.QueryRow(`
		UPDATE lead_activities SET
			outcome_id = COALESCE($3, outcome_id),
			is_done = COALESCE($4, is_done),
			responded_at = COALESCE($5, responded_at),
			type = COALESCE($6, type),
			description = COALESCE($7, description),
			remind_at = COALESCE($8, remind_at),
			is_reminded = CASE WHEN $8::timestamptz IS NOT NULL THEN false ELSE is_reminded END,
			updated_at = now()
		WHERE id = $1 AND lead_id = $2
		RETURNING id, lead_id, stage_id, '', user_id, '', type, description, outcome_id, '',
			scheduled_at, remind_at, responded_at, is_done, is_reminded, created_at, updated_at`,
		activityID, leadID, req.OutcomeID, req.IsDone, respondedAt, req.Type, req.Description, req.RemindAt,
	).Scan(
		&a.ID, &a.LeadID, &a.StageID, &a.StageName, &a.UserID, &a.UserName,
		&a.Type, &a.Description, &outcomeID, &outcomeName,
		&a.ScheduledAt, &a.RemindAt, &a.RespondedAt, &a.IsDone, &a.IsReminded,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update activity: %w", err)
	}
	a.OutcomeID = outcomeID.String
	a.OutcomeName = outcomeName.String
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
func (s *Service) dismissReminder(activityID string) (bool, error) {
	res, err := s.db.Exec(`UPDATE lead_activities SET is_reminded = true WHERE id = $1 AND NOT is_reminded`, activityID)
	if err != nil {
		return false, fmt.Errorf("dismiss reminder: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("dismiss reminder: rows affected: %w", err)
	}
	return rows > 0, nil
}

// snoozeReminder pushes an activity's reminder time forward and re-opens it
// (is_reminded = false) so it re-enters the pending pool. It reports whether a
// row actually changed; a missing id is a clean (false, nil) instead of an
// error.
func (s *Service) snoozeReminder(activityID string, remindAt time.Time) (bool, error) {
	res, err := s.db.Exec(
		`UPDATE lead_activities SET remind_at = $2, is_reminded = false WHERE id = $1`,
		activityID, remindAt,
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

// getPendingReminders returns every activity that has a reminder and is not
// done — both overdue (remind_at in the past) and upcoming (remind_at in the
// future) — so the reminders page can render Overdue / Upcoming / Done
// sections. Dismissed (is_reminded) rows are included too so the Done section
// can list them.
func (s *Service) getPendingReminders() ([]Activity, error) {
	rows, err := s.db.Query(activitySelect+`
		WHERE la.remind_at IS NOT NULL AND NOT la.is_done
		ORDER BY la.remind_at ASC
		LIMIT 100`,
	)
	if err != nil {
		return nil, fmt.Errorf("get pending reminders: %w", err)
	}
	defer rows.Close()
	activities := []Activity{}
	for rows.Next() {
		var a Activity
		var outcomeID sql.NullString
		var outcomeName sql.NullString
		if err := rows.Scan(
			&a.ID, &a.LeadID, &a.StageID, &a.StageName, &a.UserID, &a.UserName,
			&a.Type, &a.Description, &outcomeID, &outcomeName,
			&a.ScheduledAt, &a.RemindAt, &a.RespondedAt, &a.IsDone, &a.IsReminded,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("get pending reminders: scan: %w", err)
		}
		a.OutcomeID = outcomeID.String
		a.OutcomeName = outcomeName.String
		activities = append(activities, a)
	}
	return activities, nil
}
