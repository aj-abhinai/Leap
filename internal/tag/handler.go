package tag

import (
	"crm/internal/audit"
	"crm/internal/ctxutil"
	"crm/internal/respond"
	"database/sql"
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

// auditVocabulary writes the best-effort audit row for a vocabulary change —
// the only trace of who edited Settings words.
func (h *Handler) auditVocabulary(action, typ, name, id string, userID string) {
	audit.LogCustom(
		h.svc.db,
		fmt.Sprintf("%s %s %q", action, TypeLabel(typ), name),
		"tag", id, actionLabel(action), "", userID,
	)
}

func actionLabel(action string) string {
	if action == "Deleted" {
		return "delete"
	}
	if action == "Created" {
		return "create"
	}
	return "update"
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	tagType := r.URL.Query().Get("type")
	tags, err := h.svc.list(tagType)
	if err != nil {
		if errors.Is(err, ErrInvalidType) {
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
	respond.JSON(
		w,
		http.StatusOK,
		tags,
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
	if req.Type == "" {
		req.Type = "tag"
	}
	t, err := h.svc.create(req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidType), errors.Is(err, ErrNameRequired), errors.Is(err, ErrInvalidColor):
			respond.JSON(
				w,
				http.StatusBadRequest,
				nil,
				&respond.Error{Code: "BAD_REQUEST", Message: err.Error()},
				nil,
			)
		case errors.Is(err, ErrDuplicate):
			respond.JSON(
				w,
				http.StatusConflict,
				nil,
				&respond.Error{Code: "CONFLICT", Message: err.Error()},
				nil,
			)
		default:
			respond.ServerError(w, err)
		}
		return
	}
	h.auditVocabulary("Created", t.Type, t.Name, t.ID, ctxutil.GetUserID(r))
	respond.JSON(
		w,
		http.StatusCreated,
		t,
		nil,
		nil,
	)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	typ, name, err := h.svc.deleteWithTag(id)
	if err != nil {
		var inUse *InUseError
		if errors.As(err, &inUse) {
			respond.JSON(
				w,
				http.StatusConflict,
				nil,
				&respond.Error{Code: "ERR_IN_USE", Message: err.Error()},
				nil,
			)
			return
		}
		respond.ServerError(w, err)
		return
	}
	if typ != "" {
		h.auditVocabulary("Deleted", typ, name, id, ctxutil.GetUserID(r))
	}
	respond.JSON(
		w,
		http.StatusOK,
		map[string]string{"message": "deleted"},
		nil,
		nil,
	)
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
	t, err := h.svc.update(id, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidBehavior), errors.Is(err, ErrNameRequired), errors.Is(err, ErrInvalidColor):
			respond.JSON(
				w,
				http.StatusBadRequest,
				nil,
				&respond.Error{Code: "BAD_REQUEST", Message: err.Error()},
				nil,
			)
		case errors.Is(err, ErrDuplicate):
			respond.JSON(
				w,
				http.StatusConflict,
				nil,
				&respond.Error{Code: "CONFLICT", Message: err.Error()},
				nil,
			)
		case errors.Is(err, sql.ErrNoRows):
			respond.JSON(
				w,
				http.StatusNotFound,
				nil,
				&respond.Error{Code: "NOT_FOUND", Message: "Tag not found"},
				nil,
			)
		default:
			respond.ServerError(w, err)
		}
		return
	}
	h.auditVocabulary("Updated", t.Type, t.Name, t.ID, ctxutil.GetUserID(r))
	respond.JSON(
		w,
		http.StatusOK,
		t,
		nil,
		nil,
	)
}
