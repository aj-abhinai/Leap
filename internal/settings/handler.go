package settings

import (
	"crm/internal/respond"
	"encoding/json"
	"net/http"
)

// Handler serves the org settings endpoints. All routes are gated by
// settings:manage at registration.
type Handler struct {
	svc *Service
}

// NewHandler creates a settings Handler for the given service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetNudgeLeadMinutes serves GET /api/settings/nudge-lead-minutes.
func (h *Handler) GetNudgeLeadMinutes(w http.ResponseWriter, r *http.Request) {
	minutes, err := h.svc.GetNudgeLeadMinutes()
	if err != nil {
		respond.ServerError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]int{"minutes": minutes}, nil, nil)
}

// SetNudgeLeadMinutes serves PUT /api/settings/nudge-lead-minutes.
func (h *Handler) SetNudgeLeadMinutes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Minutes int `json:"minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"},
			nil,
		)
		return
	}
	if err := h.svc.SetNudgeLeadMinutes(req.Minutes); err != nil {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: err.Error()},
			nil,
		)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]int{"minutes": req.Minutes}, nil, nil)
}
