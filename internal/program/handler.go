package program

import (
	"encoding/json"
	"errors"
	"net/http"

	"crm/internal/respond"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ListActive(w http.ResponseWriter, r *http.Request) {
	programs, err := h.svc.listActive()
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
	respond.JSON(w, http.StatusOK, programs, nil, nil)
}

func (h *Handler) ListAll(w http.ResponseWriter, r *http.Request) {
	programs, err := h.svc.listAll()
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
	respond.JSON(w, http.StatusOK, programs, nil, nil)
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
	p, err := h.svc.create(req)
	if err != nil {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: err.Error()},
			nil,
		)
		return
	}
	respond.JSON(w, http.StatusCreated, p, nil, nil)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
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
	p, err := h.svc.update(id, req)
	if errors.Is(err, ErrNotFound) {
		respond.JSON(
			w,
			http.StatusNotFound,
			nil,
			&respond.Error{Code: "NOT_FOUND", Message: "Program not found"},
			nil,
		)
		return
	}
	if err != nil {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: err.Error()},
			nil,
		)
		return
	}
	respond.JSON(w, http.StatusOK, p, nil, nil)
}

func (h *Handler) Archive(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.archive(id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.JSON(
				w,
				http.StatusNotFound,
				nil,
				&respond.Error{Code: "NOT_FOUND", Message: "Program not found"},
				nil,
			)
			return
		}
		respond.JSON(
			w,
			http.StatusInternalServerError,
			nil,
			&respond.Error{Code: "INTERNAL", Message: "An internal error occurred"},
			nil,
		)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"message": "Program archived"}, nil, nil)
}

func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.restore(id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.JSON(
				w,
				http.StatusNotFound,
				nil,
				&respond.Error{Code: "NOT_FOUND", Message: "Program not found"},
				nil,
			)
			return
		}
		respond.JSON(
			w,
			http.StatusInternalServerError,
			nil,
			&respond.Error{Code: "INTERNAL", Message: "An internal error occurred"},
			nil,
		)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"message": "Program restored"}, nil, nil)
}
