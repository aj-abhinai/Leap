package lead

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"crm/internal/ctxutil"
	"crm/internal/respond"
	"crm/internal/util"

	"github.com/go-chi/chi/v5"
)

// Handler serves the lead HTTP endpoints: leads, activities, reminders, and
// the global activities list.
type Handler struct {
	svc *Service
}

// NewHandler creates a lead Handler for the given service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
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
	f := ListFilters{
		PipelineID: r.URL.Query().Get("pipeline_id"),
		StageID:    r.URL.Query().Get("stage_id"),
		ContactID:  r.URL.Query().Get("contact_id"),
		Search:     r.URL.Query().Get("q"),
		Outcome:    r.URL.Query().Get("outcome"),
		AssignedTo: r.URL.Query().Get("assigned_to"),
	}
	for name, value := range map[string]string{
		"pipeline_id": f.PipelineID, "stage_id": f.StageID,
		"contact_id": f.ContactID, "assigned_to": f.AssignedTo,
	} {
		if value != "" && value != "none" && !util.IsUUID(value) {
			respond.JSON(
				w,
				http.StatusBadRequest,
				nil,
				&respond.Error{Code: "BAD_REQUEST", Message: name + " must be a valid id"},
				nil,
			)
			return
		}
	}
	switch f.Outcome {
	case "", "open", "won", "lost":
	default:
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "outcome must be open, won, or lost"},
			nil,
		)
		return
	}

	leads, total, err := h.svc.list(f, page, perPage)
	if err != nil {
		respond.ServerError(w, err)
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

// Board returns the kanban payload: per-stage newest-window leads plus true
// counts, with an optional created_at from/to filter. The pipeline is
// required; without one there is no meaningful board.
func (h *Handler) Board(w http.ResponseWriter, r *http.Request) {
	pipelineID := r.URL.Query().Get("pipeline_id")
	if pipelineID == "" {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "pipeline_id is required"},
			nil,
		)
		return
	}
	if !util.IsUUID(pipelineID) {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "pipeline_id must be a valid id"},
			nil,
		)
		return
	}
	f := BoardFilters{
		PipelineID: pipelineID,
		Search:     r.URL.Query().Get("q"),
		Outcome:    r.URL.Query().Get("outcome"),
		AssignedTo: r.URL.Query().Get("assigned_to"),
	}
	if v := r.URL.Query().Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			respond.JSON(
				w,
				http.StatusBadRequest,
				nil,
				&respond.Error{Code: "BAD_REQUEST", Message: "from must be an RFC3339 timestamp"},
				nil,
			)
			return
		}
		f.From = &t
	}
	if v := r.URL.Query().Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			respond.JSON(
				w,
				http.StatusBadRequest,
				nil,
				&respond.Error{Code: "BAD_REQUEST", Message: "to must be an RFC3339 timestamp"},
				nil,
			)
			return
		}
		f.To = &t
	}
	board, err := h.svc.board(f)
	if err != nil {
		respond.ServerError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, board, nil, nil)
}

func respondLeadMutationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrCustomValueRejected), errors.Is(err, ErrProgramNotActive),
		errors.Is(err, ErrContactRequired), errors.Is(err, ErrNoContactDetail),
		errors.Is(err, ErrInvalidQuickReply), errors.Is(err, ErrEmptyType),
		errors.Is(err, ErrContactNotActive), errors.Is(err, ErrInvalidAssignee),
		errors.Is(err, ErrInvalidContactID), errors.Is(err, ErrNothingToUpdate):
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: err.Error()},
			nil,
		)
	case errors.Is(err, ErrNotFound), errors.Is(err, sql.ErrNoRows), respond.IsNotFound(err):
		respond.JSON(
			w,
			http.StatusNotFound,
			nil,
			&respond.Error{Code: "NOT_FOUND", Message: ErrNotFound.Error()},
			nil,
		)
	case errors.Is(err, ErrStageNotInPipeline), errors.Is(err, ErrClosingStageAtCreate), errors.Is(err, ErrNoLostStage), errors.Is(err, ErrClosedToClosedMove):
		respond.JSON(
			w,
			http.StatusUnprocessableEntity,
			nil,
			&respond.Error{Code: "UNPROCESSABLE", Message: err.Error()},
			nil,
		)
	default:
		respond.ServerError(w, err)
	}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	l, err := h.svc.get(id)
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
	if req.PipelineID == "" || req.StageID == "" {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "pipeline_id and stage_id are required"},
			nil,
		)
		return
	}
	if (req.ContactID == nil || *req.ContactID == "") && req.NewContact == nil {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "a lead must reference a contact (contact_id or new_contact)"},
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
	// Stage moves are a lead-module write capability; the route
	// gate already enforces lead:write, so Update proceeds directly.
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

func (h *Handler) ListHistory(w http.ResponseWriter, r *http.Request) {
	leadID := chi.URLParam(r, "id")
	history, err := h.svc.listHistory(leadID)
	if err != nil {
		respond.ServerError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, history, nil, nil)
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
		if errors.Is(err, ErrNotFound) || respond.IsNotFound(err) {
			respond.JSON(
				w,
				http.StatusNotFound,
				nil,
				&respond.Error{Code: "NOT_FOUND", Message: "Lead not found"},
				nil,
			)
			return
		}
		respond.ServerError(w, err)
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
		respondLeadMutationError(w, err)
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
	userID := ctxutil.GetUserID(r)
	if err := h.svc.deleteActivity(leadID, activityID, userID); err != nil {
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

func (h *Handler) UpdateActivity(w http.ResponseWriter, r *http.Request) {
	leadID := chi.URLParam(r, "id")
	activityID := chi.URLParam(r, "activity_id")
	userID := ctxutil.GetUserID(r)
	var req UpdateActivityRequest
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
	activity, err := h.svc.updateActivity(leadID, activityID, userID, req)
	if err != nil {
		respondLeadMutationError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		activity,
		nil,
		nil,
	)
}

func (h *Handler) PendingReminders(w http.ResponseWriter, r *http.Request) {
	userID := ctxutil.GetUserID(r)
	reminders, err := h.svc.getPendingReminders(userID)
	if err != nil {
		respond.ServerError(w, err)
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

// ListAllActivities serves the global activities list (GET /api/activities)
// with filters, sort, and pagination. "mine" scopes to the requesting user.
func (h *Handler) ListAllActivities(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var from, to *time.Time
	for _, name := range []string{"from", "to"} {
		v := q.Get(name)
		if v == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			respond.JSON(
				w,
				http.StatusBadRequest,
				nil,
				&respond.Error{Code: "BAD_REQUEST", Message: name + " must be an RFC3339 timestamp"},
				nil,
			)
			return
		}
		if name == "from" {
			from = &t
		} else {
			to = &t
		}
	}

	f := ActivityListFilters{
		Status:  q.Get("status"),
		Overdue: q.Get("overdue") == "true",
		Mine:    q.Get("mine") == "true",
		UserID:  ctxutil.GetUserID(r),
		Type:    q.Get("type"),
		Search:  q.Get("q"),
		From:    from,
		To:      to,
		Sort:    q.Get("sort"),
		Order:   q.Get("order"),
	}

	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 100 {
		perPage = 100
	}
	f.Page = page
	f.PerPage = perPage

	items, total, err := h.svc.listAllActivities(f)
	if err != nil {
		respond.ServerError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		items,
		nil,
		&respond.Meta{Page: page, PerPage: perPage, Total: total},
	)
}

func (h *Handler) DismissReminder(w http.ResponseWriter, r *http.Request) {
	leadID := chi.URLParam(r, "lead_id")
	id := chi.URLParam(r, "id")
	dismissed, err := h.svc.dismissReminder(leadID, id)
	if err != nil {
		respond.ServerError(w, err)
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

func (h *Handler) SnoozeReminder(w http.ResponseWriter, r *http.Request) {
	leadID := chi.URLParam(r, "lead_id")
	id := chi.URLParam(r, "id")
	var req SnoozeRequest
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
	if req.RemindAt.IsZero() {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "remind_at is required"},
			nil,
		)
		return
	}
	snoozed, err := h.svc.snoozeReminder(leadID, id, req.RemindAt)
	if err != nil {
		switch {
		case errors.Is(err, ErrSnoozePast), errors.Is(err, ErrSnoozeTooFar):
			respond.JSON(
				w,
				http.StatusBadRequest,
				nil,
				&respond.Error{Code: "BAD_REQUEST", Message: err.Error()},
				nil,
			)
		default:
			respond.ServerError(w, err)
		}
		return
	}
	if !snoozed {
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
		map[string]string{"message": "snoozed"},
		nil,
		nil,
	)
}
