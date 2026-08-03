package lead

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"crm/internal/ctxutil"
	"crm/internal/respond"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc   *Service
	perms PermissionChecker
}

type PermissionChecker interface {
	UserCan(userID, permission string) (bool, error)
}

func NewHandler(svc *Service, perms PermissionChecker) *Handler {
	return &Handler{svc: svc, perms: perms}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}
	pipelineID := r.URL.Query().Get("pipeline_id")
	stageID := r.URL.Query().Get("stage_id")
	contactID := r.URL.Query().Get("contact_id")

	leads, total, err := h.svc.list(pipelineID, stageID, contactID, page, perPage)
	if err != nil {
		respond.JSON(
			w,
			http.StatusInternalServerError,
			nil,
			&respond.Error{Code: "INTERNAL", Message: "An internal error occurred"},
			nil,
		)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		leads,
		nil,
		&respond.Meta{Page: page, PerPage: perPage, Total: total},
	)
}

func respondLeadMutationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrCustomValueRejected), errors.Is(err, ErrProgramNotActive):
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: err.Error()},
			nil,
		)
	case errors.Is(err, ErrNotFound), respond.IsNotFound(err):
		respond.JSON(
			w,
			http.StatusNotFound,
			nil,
			&respond.Error{Code: "NOT_FOUND", Message: ErrNotFound.Error()},
			nil,
		)
	case errors.Is(err, ErrStageNotInPipeline):
		respond.JSON(
			w,
			http.StatusUnprocessableEntity,
			nil,
			&respond.Error{Code: "UNPROCESSABLE", Message: err.Error()},
			nil,
		)
	default:
		respond.JSON(
			w,
			http.StatusInternalServerError,
			nil,
			&respond.Error{Code: "INTERNAL", Message: "An internal error occurred"},
			nil,
		)
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
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
	if req.Name == "" || req.PipelineID == "" || req.StageID == "" {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "Name, pipeline_id and stage_id are required"},
			nil,
		)
		return
	}
	userID := ctxutil.GetUserID(r)
	l, err := h.svc.create(req, userID)
	if err != nil {
		respondLeadMutationError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusCreated,
		l,
		nil,
		nil,
	)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := ctxutil.GetUserID(r)
	var req UpdateRequest
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
	if req.StageID != nil && *req.StageID != "" {
		canMove, err := h.perms.UserCan(userID, "lead:move_stage")
		if err != nil {
			respondLeadMutationError(w, err)
			return
		}
		if !canMove {
			respond.JSON(
				w,
				http.StatusForbidden,
				nil,
				&respond.Error{Code: "FORBIDDEN", Message: "lead:move_stage permission required to move leads between stages"},
				nil,
			)
			return
		}
	}
	l, err := h.svc.update(id, req, userID)
	if err != nil {
		respondLeadMutationError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		l,
		nil,
		nil,
	)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := ctxutil.GetUserID(r)
	if err := h.svc.delete(id, userID); err != nil {
		respondLeadMutationError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		map[string]string{"message": "Lead deleted"},
		nil,
		nil,
	)
}

func (h *Handler) ListActivities(w http.ResponseWriter, r *http.Request) {
	leadID := chi.URLParam(r, "id")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 100 {
		perPage = 100
	}
	activities, total, err := h.svc.listActivities(leadID, page, perPage)
	if err != nil {
		respondLeadMutationError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		activities,
		nil,
		&respond.Meta{Page: page, PerPage: perPage, Total: total},
	)
}

func (h *Handler) CreateActivity(w http.ResponseWriter, r *http.Request) {
	leadID := chi.URLParam(r, "id")
	userID := ctxutil.GetUserID(r)
	lead, err := h.svc.get(leadID)
	if err != nil {
		respond.JSON(
			w,
			http.StatusNotFound,
			nil,
			&respond.Error{Code: "NOT_FOUND", Message: "Lead not found"},
			nil,
		)
		return
	}
	var req CreateActivityRequest
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
	if req.Type == "" {
		req.Type = "note"
	}
	activity, err := h.svc.createActivity(leadID, lead.StageID, userID, req)
	if err != nil {
		respond.JSON(
			w,
			http.StatusInternalServerError,
			nil,
			&respond.Error{Code: "INTERNAL", Message: "An internal error occurred"},
			nil,
		)
		return
	}
	respond.JSON(
		w,
		http.StatusCreated,
		activity,
		nil,
		nil,
	)
}

func (h *Handler) DeleteActivity(w http.ResponseWriter, r *http.Request) {
	leadID := chi.URLParam(r, "id")
	activityID := chi.URLParam(r, "activity_id")
	if err := h.svc.deleteActivity(leadID, activityID); err != nil {
		respondLeadMutationError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		map[string]string{"message": "deleted"},
		nil,
		nil,
	)
}

func (h *Handler) PendingReminders(w http.ResponseWriter, r *http.Request) {
	reminders, err := h.svc.getPendingReminders()
	if err != nil {
		respond.JSON(
			w,
			http.StatusInternalServerError,
			nil,
			&respond.Error{Code: "INTERNAL", Message: "An internal error occurred"},
			nil,
		)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		reminders,
		nil,
		nil,
	)
}

func (h *Handler) DismissReminder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	dismissed, err := h.svc.dismissReminder(id)
	if err != nil {
		respond.JSON(
			w,
			http.StatusInternalServerError,
			nil,
			&respond.Error{Code: "INTERNAL", Message: "An internal error occurred"},
			nil,
		)
		return
	}
	if !dismissed {
		respond.JSON(
			w,
			http.StatusNotFound,
			nil,
			&respond.Error{Code: "NOT_FOUND", Message: "Reminder not found"},
			nil,
		)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		map[string]string{"message": "dismissed"},
		nil,
		nil,
	)
}
