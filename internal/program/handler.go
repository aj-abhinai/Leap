package program

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

// auditProgram writes the best-effort audit row for a catalog change — the
// only trace of who edited the programs a lead's value is snapshotted from.
func (h *Handler) auditProgram(action, name, id, userID string) {
	auditAction := "update"
	switch action {
	case "Created":
		auditAction = "create"
	case "Updated":
		auditAction = "update"
	default:
		auditAction = "update"
	}
	audit.LogCustom(
		h.svc.db,
		fmt.Sprintf("%s program %q", action, name),
		"program", id, auditAction, "", userID,
	)
}

func (h *Handler) ListActive(w http.ResponseWriter, r *http.Request) {
	programs, err := h.svc.listActive()
	if err != nil {
		respond.ServerError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, programs, nil, nil)
}

func (h *Handler) ListAll(w http.ResponseWriter, r *http.Request) {
	programs, err := h.svc.listAll()
	if err != nil {
		respond.ServerError(w, err)
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
		switch {
		case errors.Is(err, ErrNameRequired), errors.Is(err, ErrNegativePrice):
			respond.JSON(
				w,
				http.StatusBadRequest,
				nil,
				&respond.Error{Code: "BAD_REQUEST", Message: err.Error()},
				nil,
			)
		case errors.Is(err, ErrNotFound):
			respond.JSON(
				w,
				http.StatusNotFound,
				nil,
				&respond.Error{Code: "NOT_FOUND", Message: "Program not found"},
				nil,
			)
		default:
			respond.ServerError(w, err)
		}
		return
	}
	h.auditProgram("Created", p.Name, p.ID, ctxutil.GetUserID(r))
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
		switch {
		case errors.Is(err, ErrNotFound):
			respond.JSON(
				w,
				http.StatusNotFound,
				nil,
				&respond.Error{Code: "NOT_FOUND", Message: "Program not found"},
				nil,
			)
		case errors.Is(err, ErrNameRequired), errors.Is(err, ErrNegativePrice):
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
	h.auditProgram("Updated", p.Name, p.ID, ctxutil.GetUserID(r))
	respond.JSON(w, http.StatusOK, p, nil, nil)
}

func (h *Handler) Archive(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	name := h.svc.nameForAudit(id)
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
		respond.ServerError(w, err)
		return
	}
	h.auditProgram("Archived", name, id, ctxutil.GetUserID(r))
	respond.JSON(w, http.StatusOK, map[string]string{"message": "Program archived"}, nil, nil)
}

func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	name := h.svc.nameForAudit(id)
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
		respond.ServerError(w, err)
		return
	}
	h.auditProgram("Restored", name, id, ctxutil.GetUserID(r))
	respond.JSON(w, http.StatusOK, map[string]string{"message": "Program restored"}, nil, nil)
}
