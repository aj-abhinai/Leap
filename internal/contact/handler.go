package contact

import (
	"crm/internal/ctxutil"
	"crm/internal/respond"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

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

// respondError writes a mapped error response for mutation handlers.
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
	case errors.Is(err, ErrInvalidStatus), errors.Is(err, ErrNoContactDetail):
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
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	search := r.URL.Query().Get("q")

	contacts, total, err := h.svc.list(page, perPage, search)
	if err != nil {
		respond.ServerError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		contacts,
		nil,
		&respond.Meta{Page: page, PerPage: perPage, Total: total},
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
	c, err := h.svc.create(req)
	if err != nil {
		respondError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusCreated,
		c,
		nil,
		nil,
	)
}

func (h *Handler) BulkCreate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	var req BulkCreateRequest
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
	if len(req.Contacts) == 0 {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "no contacts provided"},
			nil,
		)
		return
	}
	if len(req.Contacts) > 500 {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "maximum 500 contacts per import"},
			nil,
		)
		return
	}
	resp, err := h.svc.bulkCreate(req)
	if err != nil {
		respond.ServerError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		resp,
		nil,
		nil,
	)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	c, err := h.svc.get(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respond.JSON(
				w,
				http.StatusNotFound,
				nil,
				&respond.Error{Code: "NOT_FOUND", Message: "Contact not found"},
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
		c,
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
	userID := ctxutil.GetUserID(r)
	c, err := h.svc.update(id, req, userID)
	if err != nil {
		respondError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		c,
		nil,
		nil,
	)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := ctxutil.GetUserID(r)
	if err := h.svc.delete(id, userID); err != nil {
		respondError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		map[string]string{"message": "Contact deleted"},
		nil,
		nil,
	)
}

func (h *Handler) ListNotes(w http.ResponseWriter, r *http.Request) {
	contactID := chi.URLParam(r, "id")
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
	notes, total, err := h.svc.listNotes(contactID, page, perPage)
	if err != nil {
		respondError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		notes,
		nil,
		&respond.Meta{Page: page, PerPage: perPage, Total: total},
	)
}

func (h *Handler) CreateNote(w http.ResponseWriter, r *http.Request) {
	contactID := chi.URLParam(r, "id")
	userID := ctxutil.GetUserID(r)
	var req CreateNoteRequest
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
	if req.Note == "" {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "note is required"},
			nil,
		)
		return
	}
	note, err := h.svc.createNote(contactID, userID, req.Note)
	if err != nil {
		respond.ServerError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusCreated,
		note,
		nil,
		nil,
	)
}

func (h *Handler) DeleteNote(w http.ResponseWriter, r *http.Request) {
	contactID := chi.URLParam(r, "id")
	noteID := chi.URLParam(r, "note_id")
	userID := ctxutil.GetUserID(r)
	canDeleteAny, err := h.perms.UserCan(userID, "*")
	if err != nil {
		respondError(w, err)
		return
	}
	if err := h.svc.deleteNote(contactID, noteID, userID, canDeleteAny); err != nil {
		respondError(w, err)
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
