package pipeline

import (
	"crm/internal/respond"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// respondError maps service errors onto the HTTP contract.
func respondError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound), respond.IsNotFound(err):
		respond.JSON(
			w,
			http.StatusNotFound,
			nil,
			&respond.Error{Code: "NOT_FOUND", Message: ErrNotFound.Error()},
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
	if err := h.svc.deletePipeline(id); err != nil {
		respondError(w, err)
		return
	}
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
	if err := h.svc.deleteStage(stageID); err != nil {
		respondError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		map[string]string{"message": "Stage deleted"},
		nil,
		nil,
	)
}
