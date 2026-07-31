package contact

import (
	"encoding/json"
	"fmt"
	"net/http"

	"crm/internal/ctxutil"
	"crm/internal/respond"

	"github.com/go-chi/chi/v5"
)

func (s *Service) ListNotes(contactID string) ([]Note, error) {
	rows, err := s.db.Query(`
		SELECT cn.id, cn.contact_id, cn.user_id, COALESCE(u.name, ''), cn.note, cn.created_at, cn.updated_at
		FROM contact_notes cn
		LEFT JOIN users u ON u.id = cn.user_id
		WHERE cn.contact_id = $1
		ORDER BY cn.created_at DESC`, contactID)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()
	var notes []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.ContactID, &n.UserID, &n.UserName, &n.Note, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}
	return notes, nil
}

func (s *Service) CreateNote(contactID, userID, note string) (*Note, error) {
	var n Note
	err := s.db.QueryRow(`
		INSERT INTO contact_notes (contact_id, user_id, note)
		VALUES ($1, $2, $3)
		RETURNING id, contact_id, user_id, '', note, created_at, updated_at`,
		contactID, userID, note,
	).Scan(&n.ID, &n.ContactID, &n.UserID, &n.UserName, &n.Note, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create note: %w", err)
	}
	if userID != "" {
		_ = s.db.QueryRow(`SELECT name FROM users WHERE id = $1`, userID).Scan(&n.UserName)
	}
	preview := note
	if len(preview) > 50 {
		preview = preview[:50]
	}
	changes, _ := json.Marshal(map[string]string{"note": preview})
	s.logActivity(contactID, "contact_note", "create", string(changes))
	return &n, nil
}

func (s *Service) DeleteNote(contactID, noteID string) error {
	_, err := s.db.Exec(`DELETE FROM contact_notes WHERE id = $1 AND contact_id = $2`, noteID, contactID)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	changes, _ := json.Marshal(map[string]string{"note_id": noteID})
	s.logActivity(contactID, "contact_note", "delete", string(changes))
	return nil
}

func (h *Handler) ListNotes(w http.ResponseWriter, r *http.Request) {
	contactID := chi.URLParam(r, "id")
	notes, err := h.svc.ListNotes(contactID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "An internal error occurred"}, nil)
		return
	}
	if notes == nil {
		notes = []Note{}
	}
	respond.JSON(w, http.StatusOK, notes, nil, nil)
}

func (h *Handler) CreateNote(w http.ResponseWriter, r *http.Request) {
	contactID := chi.URLParam(r, "id")
	userID := ctxutil.GetUserID(r)
	var req CreateNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"}, nil)
		return
	}
	if req.Note == "" {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "note is required"}, nil)
		return
	}
	note, err := h.svc.CreateNote(contactID, userID, req.Note)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "An internal error occurred"}, nil)
		return
	}
	respond.JSON(w, http.StatusCreated, note, nil, nil)
}

func (h *Handler) DeleteNote(w http.ResponseWriter, r *http.Request) {
	contactID := chi.URLParam(r, "id")
	noteID := chi.URLParam(r, "note_id")
	if err := h.svc.DeleteNote(contactID, noteID); err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "An internal error occurred"}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"message": "deleted"}, nil, nil)
}
