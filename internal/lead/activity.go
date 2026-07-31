package lead

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"crm/internal/ctxutil"
	"crm/internal/respond"

	"github.com/go-chi/chi/v5"
)

func (s *Service) ListActivities(leadID string) ([]Activity, error) {
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
	var activities []Activity
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.ID, &a.LeadID, &a.StageID, &a.StageName, &a.UserID, &a.UserName,
			&a.Type, &a.Description, &a.ScheduledAt, &a.RemindAt, &a.IsDone, &a.IsReminded,
			&a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		activities = append(activities, a)
	}
	return activities, nil
}

func (s *Service) CreateActivity(leadID, stageID, userID string, req CreateActivityRequest) (*Activity, error) {
	var a Activity
	err := s.db.QueryRow(`
		INSERT INTO lead_activities (lead_id, stage_id, user_id, type, description, scheduled_at, remind_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, lead_id, stage_id, '', user_id, '', type, description, scheduled_at, remind_at, is_done, is_reminded, created_at, updated_at`,
		leadID, stageID, userID, req.Type, req.Description, req.ScheduledAt, req.RemindAt,
	).Scan(&a.ID, &a.LeadID, &a.StageID, &a.StageName, &a.UserID, &a.UserName,
		&a.Type, &a.Description, &a.ScheduledAt, &a.RemindAt, &a.IsDone, &a.IsReminded,
		&a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create activity: %w", err)
	}
	return &a, nil
}

func (s *Service) DeleteActivity(activityID string) error {
	_, err := s.db.Exec(`DELETE FROM lead_activities WHERE id = $1`, activityID)
	if err != nil {
		return fmt.Errorf("delete activity: %w", err)
	}
	return nil
}

func (s *Service) GetPendingReminders() ([]Activity, error) {
	rows, err := s.db.Query(`
		SELECT la.id, la.lead_id, la.stage_id, COALESCE(ls.name, ''), la.user_id, COALESCE(u.name, ''),
			la.type, la.description, la.scheduled_at, la.remind_at, la.is_done, la.is_reminded,
			la.created_at, la.updated_at
		FROM lead_activities la
		LEFT JOIN users u ON u.id = la.user_id
		LEFT JOIN lead_stages ls ON ls.id = la.stage_id
		WHERE la.remind_at <= $1 AND NOT la.is_reminded AND NOT la.is_done
		ORDER BY la.remind_at ASC`,
		time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var activities []Activity
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.ID, &a.LeadID, &a.StageID, &a.StageName, &a.UserID, &a.UserName,
			&a.Type, &a.Description, &a.ScheduledAt, &a.RemindAt, &a.IsDone, &a.IsReminded,
			&a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		activities = append(activities, a)
	}
	return activities, nil
}

func (h *Handler) ListActivities(w http.ResponseWriter, r *http.Request) {
	leadID := chi.URLParam(r, "id")
	activities, err := h.svc.ListActivities(leadID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "An internal error occurred"}, nil)
		return
	}
	if activities == nil {
		activities = []Activity{}
	}
	respond.JSON(w, http.StatusOK, activities, nil, nil)
}

func (h *Handler) CreateActivity(w http.ResponseWriter, r *http.Request) {
	leadID := chi.URLParam(r, "id")
	userID := ctxutil.GetUserID(r)
	lead, err := h.svc.Get(leadID)
	if err != nil {
		respond.JSON(w, http.StatusNotFound, nil, &respond.Error{Code: "NOT_FOUND", Message: "Lead not found"}, nil)
		return
	}
	var req CreateActivityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"}, nil)
		return
	}
	if req.Type == "" {
		req.Type = "note"
	}
	activity, err := h.svc.CreateActivity(leadID, lead.StageID, userID, req)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "An internal error occurred"}, nil)
		return
	}
	respond.JSON(w, http.StatusCreated, activity, nil, nil)
}

func (h *Handler) DeleteActivity(w http.ResponseWriter, r *http.Request) {
	activityID := chi.URLParam(r, "activity_id")
	if err := h.svc.DeleteActivity(activityID); err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "An internal error occurred"}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"message": "deleted"}, nil, nil)
}

func (h *Handler) PendingReminders(w http.ResponseWriter, r *http.Request) {
	reminders, err := h.svc.GetPendingReminders()
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "An internal error occurred"}, nil)
		return
	}
	if reminders == nil {
		reminders = []Activity{}
	}
	respond.JSON(w, http.StatusOK, reminders, nil, nil)
}

func (h *Handler) DismissReminder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, err := h.svc.db.Exec(`UPDATE lead_activities SET is_reminded = true WHERE id = $1`, id)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "An internal error occurred"}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"message": "dismissed"}, nil, nil)
}
