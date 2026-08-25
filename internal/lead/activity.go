package lead

import (
	"crm/internal/util"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ErrInvalidQuickReply marks a quick_reply_id that does not reference a
// quick-reply tag (type 'quick_reply').
var ErrInvalidQuickReply = errors.New("quick_reply_id must reference a quick_reply tag")

// ErrSnoozePast marks a snooze whose remind_at is not in the future.
var ErrSnoozePast = errors.New("remind_at must be in the future")

// ErrSnoozeTooFar marks a snooze beyond the allowed horizon.
var ErrSnoozeTooFar = errors.New("remind_at is too far in the future")

// maxSnoozeHorizon bounds how far a snooze may push a reminder forward so a
// misbehaving client cannot queue tasks years out. The frontend presets cap at
// 24 hours; a year is far beyond any legitimate manual entry.
const maxSnoozeHorizon = 365 * 24 * time.Hour

const activitySelect = `
	SELECT la.id, la.lead_id, la.stage_id, COALESCE(ls.name, ''), la.user_id, COALESCE(u.name, ''),
		la.type, la.description, la.quick_reply_id, t.name,
		la.scheduled_at, la.remind_at, la.responded_at, la.occurred_at,
		la.is_done, la.is_cancelled, la.is_reminded, la.created_at, la.updated_at
	FROM lead_activities la
	LEFT JOIN users u ON u.id = la.user_id
	LEFT JOIN lead_stages ls ON ls.id = la.stage_id
	LEFT JOIN tags t ON t.id = la.quick_reply_id`

// validateQuickReplyTx rejects quick-reply ids that do not reference a
// quick-reply tag.
func (s *Service) validateQuickReplyTx(q interface {
	QueryRow(query string, args ...any) *sql.Row
}, quickReplyID *string) error {
	if quickReplyID == nil || *quickReplyID == "" {
		return nil
	}
	var exists bool
	err := q.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM tags WHERE id = $1 AND type = 'quick_reply')`,
		*quickReplyID,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("validate quick reply: %w", err)
	}
	if !exists {
		return ErrInvalidQuickReply
	}
	return nil
}

// ErrEmptyType marks an activity request with a blank type. Descriptions are
// optional — a quick "Call 1 / Busy, reschedule" entry needs no prose.
var ErrEmptyType = errors.New("activity type cannot be empty")

// ErrNothingToUpdate marks an activity update request with no fields set.
var ErrNothingToUpdate = errors.New("nothing to update")

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
	var quickReplyID sql.NullString
	var quickReplyName sql.NullString
	if err := scan.Scan(
		&a.ID, &a.LeadID, &a.StageID, &a.StageName, &a.UserID, &a.UserName,
		&a.Type, &a.Description, &quickReplyID, &quickReplyName,
		&a.ScheduledAt, &a.RemindAt, &a.RespondedAt, &a.OccurredAt,
		&a.IsDone, &a.IsCancelled, &a.IsReminded, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return a, err
	}
	a.QuickReplyID = quickReplyID.String
	a.QuickReplyName = quickReplyName.String
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
	offset := util.Offset(page, perPage)
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
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list activities: iterate: %w", err)
	}
	return activities, total, nil
}

func (s *Service) createActivity(leadID, stageID, userID string, req CreateActivityRequest) (*Activity, error) {
	if err := validateActivityFields(req.Type); err != nil {
		return nil, err
	}
	if err := s.validateQuickReplyTx(s.db, req.QuickReplyID); err != nil {
		return nil, err
	}
	// An activity created with a quick reply is already "responded" — log the time.
	// A reschedule_at also implies the attempt happened, so it is done too.
	// IsDone completes an activity created in one shot (close_lost from the
	// create form), stamping occurred_at so it survives the closing-stage move.
	var respondedAt any
	if (req.QuickReplyID != nil && *req.QuickReplyID != "") || req.RescheduleAt != nil {
		respondedAt = time.Now()
	}
	isDone := req.RescheduleAt != nil || (req.IsDone != nil && *req.IsDone)
	var occurredAt any
	if isDone {
		occurredAt = time.Now()
	}
	desc := strings.TrimSpace(req.Description)
	var a Activity
	var quickReplyID sql.NullString
	var quickReplyName sql.NullString
	err := s.db.QueryRow(`
		WITH ins AS (
			INSERT INTO lead_activities (lead_id, stage_id, user_id, type, description, quick_reply_id, scheduled_at, remind_at, responded_at, occurred_at, is_done)
			VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5, NULLIF($6, '')::uuid, $7, $8, $9, $10, $11)
			RETURNING id, lead_id, stage_id, user_id, type, description, quick_reply_id,
				scheduled_at, remind_at, responded_at, occurred_at, is_done, is_cancelled, is_reminded, created_at, updated_at
		)
		SELECT ins.id, ins.lead_id, ins.stage_id, COALESCE(ls.name, ''), ins.user_id, COALESCE(u.name, ''),
			ins.type, ins.description, ins.quick_reply_id, COALESCE(t.name, ''),
			ins.scheduled_at, ins.remind_at, ins.responded_at, ins.occurred_at,
			ins.is_done, ins.is_cancelled, ins.is_reminded, ins.created_at, ins.updated_at
		FROM ins
		LEFT JOIN users u ON u.id = ins.user_id
		LEFT JOIN lead_stages ls ON ls.id = ins.stage_id
		LEFT JOIN tags t ON t.id = ins.quick_reply_id`,
		leadID, stageID, userID, req.Type, desc, req.QuickReplyID, req.ScheduledAt, req.RemindAt, respondedAt, occurredAt, isDone,
	).Scan(
		&a.ID, &a.LeadID, &a.StageID, &a.StageName, &a.UserID, &a.UserName,
		&a.Type, &a.Description, &quickReplyID, &quickReplyName,
		&a.ScheduledAt, &a.RemindAt, &a.RespondedAt, &a.OccurredAt,
		&a.IsDone, &a.IsCancelled, &a.IsReminded, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create activity: %w", err)
	}
	a.QuickReplyID = quickReplyID.String
	a.QuickReplyName = quickReplyName.String

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
// scheduled_at, remind_at), lifecycle state (done, cancelled), and quick reply.
// Description is optional and can be cleared by sending "". Setting a quick reply
// (or marking done) on an activity that had neither auto-logs the response
// time; occurred_at is stamped on completion unless supplied explicitly.
// Editing remind_at re-opens the reminder (is_reminded = false).
//
// The "log attempt + next" reschedule flow: when is_done=true and a
// reschedule_at is supplied, the completed attempt is logged and a new task of
// the same type is created for reschedule_at, defaulting its reminder to the
// same time.
func (s *Service) updateActivity(leadID, activityID, userID string, req UpdateActivityRequest) (*Activity, error) {
	if err := s.validateQuickReplyTx(s.db, req.QuickReplyID); err != nil {
		return nil, err
	}
	if req.QuickReplyID == nil && req.IsDone == nil && req.Type == nil && req.Description == nil &&
		!req.ScheduledAt.Set && !req.RemindAt.Set && req.OccurredAt == nil &&
		req.IsCancelled == nil && req.RescheduleAt == nil {
		return nil, ErrNothingToUpdate
	}

	// Load the current row so we only stamp response times on the null->set edge.
	var cur Activity
	var curQuickReplyID sql.NullString
	err := s.db.QueryRow(`
		SELECT id, quick_reply_id, responded_at, is_done, type, description FROM lead_activities WHERE id = $1 AND lead_id = $2`,
		activityID, leadID,
	).Scan(&cur.ID, &curQuickReplyID, &cur.RespondedAt, &cur.IsDone, &cur.Type, &cur.Description)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load activity: %w", err)
	}
	cur.QuickReplyID = curQuickReplyID.String

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

	newQuickReply := cur.QuickReplyID
	if req.QuickReplyID != nil {
		newQuickReply = *req.QuickReplyID
	}
	willHaveQuickReply := newQuickReply != ""
	markDone := false
	if req.IsDone != nil {
		markDone = *req.IsDone
	}

	// Stamp the response time when the activity gains a quick reply or is marked
	// done for the first time. Clear is never applied to responded_at.
	var respondedAt any
	if cur.RespondedAt == nil && (willHaveQuickReply || markDone) {
		respondedAt = time.Now()
	}

	var a Activity
	var quickReplyID sql.NullString
	var quickReplyName sql.NullString
	err = s.db.QueryRow(`
		UPDATE lead_activities SET
			quick_reply_id = COALESCE($3, quick_reply_id),
			is_done = COALESCE($4, is_done),
			responded_at = COALESCE($5, responded_at),
			type = COALESCE($6, type),
			description = COALESCE($7, description),
			scheduled_at = CASE WHEN $8 THEN $9 ELSE scheduled_at END,
			remind_at = CASE WHEN $10 THEN $11 ELSE remind_at END,
			is_reminded = CASE WHEN $10 AND $11::timestamptz IS NOT NULL THEN false ELSE is_reminded END,
			occurred_at = COALESCE($12, occurred_at, CASE WHEN $4 = true THEN now() ELSE NULL END),
			is_cancelled = COALESCE($13, is_cancelled),
			updated_at = now()
		WHERE id = $1 AND lead_id = $2
		RETURNING id, lead_id, stage_id, '', user_id, '', type, description, quick_reply_id, '',
			scheduled_at, remind_at, responded_at, occurred_at, is_done, is_cancelled, is_reminded, created_at, updated_at`,
		activityID, leadID, req.QuickReplyID, req.IsDone, respondedAt, req.Type, desc,
		req.ScheduledAt.Set, req.ScheduledAt.Value, req.RemindAt.Set, req.RemindAt.Value,
		req.OccurredAt, req.IsCancelled,
	).Scan(
		&a.ID, &a.LeadID, &a.StageID, &a.StageName, &a.UserID, &a.UserName,
		&a.Type, &a.Description, &quickReplyID, &quickReplyName,
		&a.ScheduledAt, &a.RemindAt, &a.RespondedAt, &a.OccurredAt,
		&a.IsDone, &a.IsCancelled, &a.IsReminded, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update activity: %w", err)
	}
	a.QuickReplyID = quickReplyID.String
	a.QuickReplyName = quickReplyName.String

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
// actually changed. Only open, non-cancelled activities that carry a reminder
// or schedule are eligible, so dismiss stays aligned with reminder semantics;
// a missing, already-reminded, done, cancelled, or reminder-less id is a clean
// (false, nil) instead of an error.
func (s *Service) dismissReminder(leadID, activityID string) (bool, error) {
	res, err := s.db.Exec(
		`UPDATE lead_activities SET is_reminded = true
		WHERE id = $1 AND lead_id = $2
			AND NOT is_reminded
			AND NOT is_done
			AND NOT is_cancelled
			AND (remind_at IS NOT NULL OR scheduled_at IS NOT NULL)`,
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
// reschedule of an open task. The new time must be future-only and
// within the snooze horizon. Only open, non-cancelled activities that carry a
// reminder or schedule are eligible — matching dismissReminder — so a missing
// or reminder-less id is a clean (false, nil) instead of an error.
func (s *Service) snoozeReminder(leadID, activityID string, remindAt time.Time) (bool, error) {
	now := time.Now()
	if !remindAt.After(now) {
		return false, ErrSnoozePast
	}
	if remindAt.After(now.Add(maxSnoozeHorizon)) {
		return false, ErrSnoozeTooFar
	}
	res, err := s.db.Exec(
		`UPDATE lead_activities SET
			remind_at = $2,
			is_reminded = false,
			scheduled_at = CASE
				WHEN scheduled_at IS NOT NULL AND remind_at IS NOT NULL THEN scheduled_at + ($2 - remind_at)
				ELSE scheduled_at
			END
		WHERE id = $1 AND lead_id = $3 AND NOT is_done AND NOT is_cancelled
			AND (remind_at IS NOT NULL OR scheduled_at IS NOT NULL)`,
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
// Each row carries the lead display name and contact id so reminder surfaces
// can show whose lead the task belongs to and open the lead drawer.
func (s *Service) getPendingReminders() ([]ActivityListItem, error) {
	rows, err := s.db.Query(`
		SELECT la.id, la.lead_id, la.stage_id, COALESCE(ls.name, ''), la.user_id, COALESCE(u.name, ''),
			la.type, la.description, la.quick_reply_id, COALESCE(t.name, ''),
			la.scheduled_at, la.remind_at, la.responded_at, la.occurred_at,
			la.is_done, la.is_cancelled, la.is_reminded, la.created_at, la.updated_at,
			COALESCE(NULLIF(l.nickname, ''), c.name, ''), l.contact_id
		FROM lead_activities la
		JOIN leads l ON l.id = la.lead_id AND l.deleted_at IS NULL
		LEFT JOIN contacts c ON c.id = l.contact_id
		LEFT JOIN lead_stages ls ON ls.id = la.stage_id
		LEFT JOIN users u ON u.id = la.user_id
		LEFT JOIN tags t ON t.id = la.quick_reply_id
		WHERE NOT la.is_done AND NOT la.is_cancelled
			AND (la.remind_at IS NOT NULL OR la.scheduled_at IS NOT NULL)
		ORDER BY COALESCE(la.remind_at, la.scheduled_at) ASC
		LIMIT 100`,
	)
	if err != nil {
		return nil, fmt.Errorf("get pending reminders: %w", err)
	}
	defer rows.Close()
	items := []ActivityListItem{}
	for rows.Next() {
		var a Activity
		var quickReplyID, quickReplyName sql.NullString
		var displayName, contactID string
		if err := rows.Scan(
			&a.ID, &a.LeadID, &a.StageID, &a.StageName, &a.UserID, &a.UserName,
			&a.Type, &a.Description, &quickReplyID, &quickReplyName,
			&a.ScheduledAt, &a.RemindAt, &a.RespondedAt, &a.OccurredAt,
			&a.IsDone, &a.IsCancelled, &a.IsReminded, &a.CreatedAt, &a.UpdatedAt,
			&displayName, &contactID,
		); err != nil {
			return nil, fmt.Errorf("get pending reminders: scan: %w", err)
		}
		a.QuickReplyID = quickReplyID.String
		a.QuickReplyName = quickReplyName.String
		items = append(items, ActivityListItem{
			Activity:        a,
			LeadDisplayName: displayName,
			ContactID:       contactID,
		})
	}
	return items, rows.Err()
}

// listAllActivities returns every activity across leads with optional filters
// (status bucket, overdue window, owner, type, text search, date range), an
// explicit sort, and page/per_page pagination. Each row carries the lead
// display name and linked contact id for list context.
func (s *Service) listAllActivities(f ActivityListFilters) ([]ActivityListItem, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 {
		f.PerPage = 50
	}
	if f.PerPage > 100 {
		f.PerPage = 100
	}
	order := strings.ToLower(f.Order)
	if order != "asc" {
		order = "desc"
	}

	// Conditions use a "$?" placeholder; renumber() assigns real $n after all
	// args are collected so multi-arg predicates (search) stay consistent.
	// One counter is shared across all conditions — each condition continues
	// from the previous one's last number.
	where := []string{"l.deleted_at IS NULL"}
	args := []any{}
	add := func(cond string, a ...any) {
		where = append(where, cond)
		args = append(args, a...)
	}
	argIdx := 0
	renumber := func(cond string) string {
		var b strings.Builder
		for {
			i := strings.Index(cond, "$?")
			if i < 0 {
				b.WriteString(cond)
				break
			}
			b.WriteString(cond[:i])
			argIdx++
			b.WriteString(fmt.Sprintf("$%d", argIdx))
			cond = cond[i+2:]
		}
		return b.String()
	}
	apply := func() {
		for i, c := range where {
			where[i] = renumber(c)
		}
	}

	switch f.Status {
	case "done":
		add("la.is_done = $?", true)
	case "cancelled":
		add("la.is_cancelled = $?", true)
	case "open":
		add("NOT la.is_done AND NOT la.is_cancelled")
	default:
		// "all"
	}
	if f.Overdue {
		add("la.remind_at IS NOT NULL AND la.remind_at < now() AND NOT la.is_done AND NOT la.is_cancelled")
	}
	if f.Mine {
		userID := f.UserID
		if userID == "" {
			add("la.user_id IS NOT NULL")
		} else {
			add("la.user_id = $?", userID)
		}
	}
	if f.Type != "" {
		add("la.type = $?", f.Type)
	}
	if f.Search != "" {
		pat := "%" + f.Search + "%"
		add("(la.description ILIKE $? OR COALESCE(c.name, '') ILIKE $? OR COALESCE(l.nickname, '') ILIKE $?)", pat, pat, pat)
	}
	if f.From != nil {
		add("COALESCE(la.occurred_at, la.responded_at, la.scheduled_at, la.created_at) >= $?", *f.From)
	}
	if f.To != nil {
		add("COALESCE(la.occurred_at, la.responded_at, la.scheduled_at, la.created_at) <= $?", *f.To)
	}
	apply()

	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRow(
		`SELECT COUNT(*)
		FROM lead_activities la
		JOIN leads l ON l.id = la.lead_id
		LEFT JOIN contacts c ON c.id = l.contact_id
		WHERE `+whereSQL, args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count all activities: %w", err)
	}

	var orderBy string
	switch f.Sort {
	case "type":
		orderBy = "la.type"
	case "created_at":
		orderBy = "la.created_at"
	case "due_at":
		orderBy = "COALESCE(la.remind_at, la.scheduled_at, la.created_at)"
	default:
		orderBy = "COALESCE(la.remind_at, la.scheduled_at, la.created_at)"
	}
	offset := (f.Page - 1) * f.PerPage

	// The WHERE placeholders were renumbered 1..N; LIMIT/OFFSET follow after.
	limitArg := len(args) + 1
	rows, err := s.db.Query(`
		SELECT la.id, la.lead_id, la.stage_id, COALESCE(ls.name, ''), la.user_id, COALESCE(u.name, ''),
			la.type, la.description, la.quick_reply_id, COALESCE(t.name, ''),
			la.scheduled_at, la.remind_at, la.responded_at, la.occurred_at,
			la.is_done, la.is_cancelled, la.is_reminded, la.created_at, la.updated_at,
			COALESCE(NULLIF(l.nickname, ''), c.name, ''), l.contact_id
		FROM lead_activities la
		JOIN leads l ON l.id = la.lead_id
		LEFT JOIN contacts c ON c.id = l.contact_id
		LEFT JOIN lead_stages ls ON ls.id = la.stage_id
		LEFT JOIN users u ON u.id = la.user_id
		LEFT JOIN tags t ON t.id = la.quick_reply_id
		WHERE `+whereSQL+`
		ORDER BY `+orderBy+` `+strings.ToUpper(order)+`
		LIMIT $`+strconv.Itoa(limitArg)+` OFFSET $`+strconv.Itoa(limitArg+1),
		append(args, f.PerPage, offset)...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list all activities: %w", err)
	}
	defer rows.Close()

	items := []ActivityListItem{}
	for rows.Next() {
		var a Activity
		var quickReplyID sql.NullString
		var quickReplyName sql.NullString
		var displayName string
		var contactID string
		if err := rows.Scan(
			&a.ID, &a.LeadID, &a.StageID, &a.StageName, &a.UserID, &a.UserName,
			&a.Type, &a.Description, &quickReplyID, &quickReplyName,
			&a.ScheduledAt, &a.RemindAt, &a.RespondedAt, &a.OccurredAt,
			&a.IsDone, &a.IsCancelled, &a.IsReminded, &a.CreatedAt, &a.UpdatedAt,
			&displayName, &contactID,
		); err != nil {
			return nil, 0, fmt.Errorf("list all activities: scan: %w", err)
		}
		a.QuickReplyID = quickReplyID.String
		a.QuickReplyName = quickReplyName.String
		items = append(items, ActivityListItem{
			Activity:        a,
			LeadDisplayName: displayName,
			ContactID:       contactID,
		})
	}
	return items, total, nil
}
