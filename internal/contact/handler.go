package contact

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
		perPage = 20
	}
	search := r.URL.Query().Get("q")

	contacts, total, err := h.svc.List(page, perPage, search)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "An internal error occurred"}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, contacts, nil, &respond.Meta{Page: page, PerPage: perPage, Total: total})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"}, nil)
		return
	}
	if req.Name == "" {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Name is required"}, nil)
		return
	}
	c, err := h.svc.Create(req)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "An internal error occurred"}, nil)
		return
	}
	respond.JSON(w, http.StatusCreated, c, nil, nil)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"}, nil)
		return
	}
	c, err := h.svc.Update(id, req)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "An internal error occurred"}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, c, nil, nil)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(id); err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "An internal error occurred"}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"message": "Contact deleted"}, nil, nil)
}
