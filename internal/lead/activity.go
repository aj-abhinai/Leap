package lead

import (
	"fmt"
	"time"
)

func (s *Service) listActivities(leadID string) ([]Activity, error) {
	rows, err := s.db.Query(`
		SELECT la.id, la.lead_id, la.stage_id, COALESCE(ls.name, ''), la.user_id, COALESCE(u.name, ''),
			la.type, la.description, la.scheduled_at, la.remind_at, la.is_done, la.is_reminded,
			la.created_at, la.updated_at
		FROM lead_activities la
		LEFT JOIN users u ON u.id = la.user_id
		LEFT JOIN lead_stages ls ON ls.id = la.stage_id
		WHERE la.lead_id = $1
		ORDER BY la.created_at DESC`, leadID)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	defer rows.Close()
	activities := []Activity{}
	for rows.Next() {
		var a Activity
		if err := rows.Scan(
			&a.ID, &a.LeadID, &a.StageID, &a.StageName, &a.UserID, &a.UserName,
			&a.Type, &a.Description, &a.ScheduledAt, &a.RemindAt, &a.IsDone, &a.IsReminded,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		activities = append(activities, a)
	}
	return activities, nil
}

func (s *Service) createActivity(leadID, stageID, userID string, req CreateActivityRequest) (*Activity, error) {
	var a Activity
	err := s.db.QueryRow(`
		INSERT INTO lead_activities (lead_id, stage_id, user_id, type, description, scheduled_at, remind_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, lead_id, stage_id, '', user_id, '', type, description, scheduled_at, remind_at,
			is_done, is_reminded, created_at, updated_at`,
		leadID, stageID, userID, req.Type, req.Description, req.ScheduledAt, req.RemindAt,
	).Scan(
		&a.ID, &a.LeadID, &a.StageID, &a.StageName, &a.UserID, &a.UserName,
		&a.Type, &a.Description, &a.ScheduledAt, &a.RemindAt, &a.IsDone, &a.IsReminded,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create activity: %w", err)
	}
	return &a, nil
}

func (s *Service) deleteActivity(activityID string) error {
	_, err := s.db.Exec(`DELETE FROM lead_activities WHERE id = $1`, activityID)
	if err != nil {
		return fmt.Errorf("delete activity: %w", err)
	}
	return nil
}

// dismissReminder marks an activity as reminded and reports whether a row
// actually existed; a missing id is a clean (false, nil) instead of an error.
func (s *Service) dismissReminder(activityID string) (bool, error) {
	res, err := s.db.Exec(`UPDATE lead_activities SET is_reminded = true WHERE id = $1`, activityID)
	if err != nil {
		return false, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (s *Service) getPendingReminders() ([]Activity, error) {
	rows, err := s.db.Query(`
		SELECT la.id, la.lead_id, la.stage_id, COALESCE(ls.name, ''), la.user_id, COALESCE(u.name, ''),
			la.type, la.description, la.scheduled_at, la.remind_at, la.is_done, la.is_reminded,
			la.created_at, la.updated_at
		FROM lead_activities la
		LEFT JOIN users u ON u.id = la.user_id
		LEFT JOIN lead_stages ls ON ls.id = la.stage_id
		WHERE la.remind_at <= $1 AND NOT la.is_reminded AND NOT la.is_done
		ORDER BY la.remind_at ASC
		LIMIT 100`,
		time.Now(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	activities := []Activity{}
	for rows.Next() {
		var a Activity
		if err := rows.Scan(
			&a.ID, &a.LeadID, &a.StageID, &a.StageName, &a.UserID, &a.UserName,
			&a.Type, &a.Description, &a.ScheduledAt, &a.RemindAt, &a.IsDone, &a.IsReminded,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		activities = append(activities, a)
	}
	return activities, nil
}
