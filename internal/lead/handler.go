package lead

import (
	"crm/internal/respond"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *Service
}

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
	pipelineID := r.URL.Query().Get("pipeline_id")
	stageID := r.URL.Query().Get("stage_id")
	contactID := r.URL.Query().Get("contact_id")

	leads, total, err := h.svc.List(pipelineID, stageID, contactID, page, perPage)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "An internal error occurred"}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, leads, nil, &respond.Meta{Page: page, PerPage: perPage, Total: total})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"}, nil)
		return
	}
	if req.Name == "" || req.PipelineID == "" || req.StageID == "" {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Name, pipeline_id and stage_id are required"}, nil)
		return
	}
	l, err := h.svc.Create(req)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "An internal error occurred"}, nil)
		return
	}
	respond.JSON(w, http.StatusCreated, l, nil, nil)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"}, nil)
		return
	}
	l, err := h.svc.Update(id, req)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "An internal error occurred"}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, l, nil, nil)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(id); err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "An internal error occurred"}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"message": "Lead deleted"}, nil, nil)
}
