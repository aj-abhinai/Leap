package settings

import (
	"crm/internal/audit"
	"crm/internal/ctxutil"
	"crm/internal/respond"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Handler serves the org settings endpoints. Reads are open to every
// signed-in user; writes are gated by settings:manage at registration.
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

// SetNudgeLeadMinutes serves PUT /api/settings/nudge-lead-minutes. The
// payload must carry an explicit minutes value: absent or null is rejected so
// a malformed client cannot silently reset the setting to zero. The change is
// audited with its before â†’ after value.
func (h *Handler) SetNudgeLeadMinutes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Minutes *int `json:"minutes"`
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
	if req.Minutes == nil {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "minutes is required"},
			nil,
		)
		return
	}
	old, err := h.svc.GetNudgeLeadMinutes()
	if err != nil {
		respond.ServerError(w, err)
		return
	}
	if err := h.svc.SetNudgeLeadMinutes(*req.Minutes); err != nil {
		if errors.Is(err, ErrInvalidNudgeMinutes) {
			respond.JSON(
				w,
				http.StatusBadRequest,
				nil,
				&respond.Error{Code: "BAD_REQUEST", Message: err.Error()},
				nil,
			)
			return
		}
		respond.ServerError(w, err)
		return
	}
	audit.LogCustom(
		h.svc.db,
		fmt.Sprintf("Changed %q from %d to %d minutes", "Remind before tasks start", old, *req.Minutes),
		"settings", "", "update", "", ctxutil.GetUserID(r),
	)
	respond.JSON(w, http.StatusOK, map[string]int{"minutes": *req.Minutes}, nil, nil)
}

// GetDefaultCountryCode serves GET /api/settings/default-country-code.
func (h *Handler) GetDefaultCountryCode(w http.ResponseWriter, r *http.Request) {
	code, err := h.svc.GetDefaultCountryCode()
	if err != nil {
		respond.ServerError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"country_code": code}, nil, nil)
}

// SetDefaultCountryCode serves PUT /api/settings/default-country-code. The
// change is audited with its before â†’ after value.
func (h *Handler) SetDefaultCountryCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CountryCode string `json:"country_code"`
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
	old, err := h.svc.GetDefaultCountryCode()
	if err != nil {
		respond.ServerError(w, err)
		return
	}
	normalized, err := h.svc.SetDefaultCountryCode(req.CountryCode)
	if err != nil {
		if errors.Is(err, ErrInvalidCountryCode) {
			respond.JSON(
				w,
				http.StatusBadRequest,
				nil,
				&respond.Error{Code: "BAD_REQUEST", Message: err.Error()},
				nil,
			)
			return
		}
		respond.ServerError(w, err)
		return
	}
	audit.LogCustom(
		h.svc.db,
		fmt.Sprintf("Changed %q from %s to %s", "Default country code", old, normalized),
		"settings", "", "update", "", ctxutil.GetUserID(r),
	)
	respond.JSON(w, http.StatusOK, map[string]string{"country_code": normalized}, nil, nil)
}
