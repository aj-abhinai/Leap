package pipeline

import (
	"crm/internal/audit"
	"crm/internal/ctxutil"
	"crm/internal/respond"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// auditConfig writes the best-effort audit row for a pipeline or stage
// change — the only trace of who edited the sales configuration. Stages share
// the pipeline resource type; the description names which changed.
func (h *Handler) auditConfig(action, entity, name, id string, userID string) {
	audit.LogCustom(
		h.svc.db,
		fmt.Sprintf("%s %s %q", action, entity, name),
		"pipeline", id, auditAction(action), "", userID,
	)
}

func auditAction(action string) string {
	switch action {
	case "Created":
		return "create"
	case "Updated":
		return "update"
	default:
		return "delete"
	}
}

// respondError maps service errors onto the HTTP contract.
func respondError(w http.ResponseWriter, err error) {
	var stageInUse *StageInUseError
	var lastStage *LastStageError
	switch {
	case errors.Is(err, ErrNotFound), respond.IsNotFound(err):
		respond.JSON(
			w,
			http.StatusNotFound,
			nil,
			&respond.Error{Code: "NOT_FOUND", Message: ErrNotFound.Error()},
			nil,
		)
	case errors.As(err, &stageInUse):
		respond.JSON(
			w,
			http.StatusConflict,
			nil,
			&respond.Error{Code: "STAGE_IN_USE", Message: stageInUse.Error()},
			nil,
		)
	case errors.As(err, &lastStage):
		respond.JSON(
			w,
			http.StatusConflict,
			nil,
			&respond.Error{Code: "LAST_STAGE", Message: lastStage.Error()},
			nil,
		)
	case errors.Is(err, ErrInUse):
		respond.JSON(
			w,
			http.StatusConflict,
			nil,
			&respond.Error{Code: "CONFLICT", Message: "Delete the leads using this pipeline or stage first"},
			nil,
		)
	case errors.Is(err, ErrInvalidStageOutcome), errors.Is(err, ErrInvalidStageOrder):
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
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pipelines, err := h.svc.list()
	if err != nil {
		respondError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		pipelines,
		nil,
		nil,
	)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreatePipelineRequest
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
	if req.Name == "" {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "Name is required"},
			nil,
		)
		return
	}
	p, err := h.svc.createPipeline(req)
	if err != nil {
		respondError(w, err)
		return
	}
	h.auditConfig("Created", "pipeline", p.Name, p.ID, ctxutil.GetUserID(r))
	respond.JSON(
		w,
		http.StatusCreated,
		p,
		nil,
		nil,
	)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdatePipelineRequest
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
	p, err := h.svc.updatePipeline(id, req)
	if err != nil {
		respondError(w, err)
		return
	}
	h.auditConfig("Updated", "pipeline", p.Name, p.ID, ctxutil.GetUserID(r))
	respond.JSON(
		w,
		http.StatusOK,
		p,
		nil,
		nil,
	)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	name, err := h.svc.deletePipeline(id)
	if err != nil {
		respondError(w, err)
		return
	}
	h.auditConfig("Deleted", "pipeline", name, id, ctxutil.GetUserID(r))
	respond.JSON(
		w,
		http.StatusOK,
		map[string]string{"message": "Pipeline deleted"},
		nil,
		nil,
	)
}

func (h *Handler) CreateStage(w http.ResponseWriter, r *http.Request) {
	pipelineID := chi.URLParam(r, "id")
	var req CreateStageRequest
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
	if req.Name == "" {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "Name is required"},
			nil,
		)
		return
	}
	st, err := h.svc.createStage(pipelineID, req)
	if err != nil {
		respondError(w, err)
		return
	}
	h.auditConfig("Created", "stage", st.Name, st.ID, ctxutil.GetUserID(r))
	respond.JSON(
		w,
		http.StatusCreated,
		st,
		nil,
		nil,
	)
}

func (h *Handler) UpdateStage(w http.ResponseWriter, r *http.Request) {
	stageID := chi.URLParam(r, "stage_id")
	var req UpdateStageRequest
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
	st, err := h.svc.updateStage(stageID, req)
	if err != nil {
		respondError(w, err)
		return
	}
	h.auditConfig("Updated", "stage", st.Name, st.ID, ctxutil.GetUserID(r))
	respond.JSON(
		w,
		http.StatusOK,
		st,
		nil,
		nil,
	)
}

func (h *Handler) DeleteStage(w http.ResponseWriter, r *http.Request) {
	stageID := chi.URLParam(r, "stage_id")
	name, err := h.svc.deleteStage(stageID)
	if err != nil {
		respondError(w, err)
		return
	}
	h.auditConfig("Deleted", "stage", name, stageID, ctxutil.GetUserID(r))
	respond.JSON(
		w,
		http.StatusOK,
		map[string]string{"message": "Stage deleted"},
		nil,
		nil,
	)
}

// ReorderStages serves PUT /api/pipelines/{id}/stages/order. The request must
// list every stage of the pipeline exactly once; the array order becomes the
// stored order.
func (h *Handler) ReorderStages(w http.ResponseWriter, r *http.Request) {
	pipelineID := chi.URLParam(r, "id")
	var req ReorderStagesRequest
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
	if err := h.svc.reorderStages(pipelineID, req.StageIDs); err != nil {
		respondError(w, err)
		return
	}
	audit.LogCustom(h.svc.db, "Reordered pipeline stages", "pipeline", pipelineID, "update", "", ctxutil.GetUserID(r))
	respond.JSON(
		w,
		http.StatusOK,
		map[string]string{"message": "Stages reordered"},
		nil,
		nil,
	)
}
