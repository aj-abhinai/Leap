package contact

import (
	"crm/internal/util"
	"database/sql"
	"errors"
	"fmt"
)

// requireLiveContact returns ErrNotFound unless the id is a live (non-deleted)
// contact, so notes cannot leak from or attach to hidden rows. Malformed ids
// report not-found rather than a database cast error.
func (s *Service) requireLiveContact(contactID string) error {
	if !util.IsUUID(contactID) {
		return ErrNotFound
	}
	var live bool
	if err := s.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM contacts WHERE id = $1 AND deleted_at IS NULL)`,
		contactID,
	).Scan(&live); err != nil {
		return fmt.Errorf("check contact: %w", err)
	}
	if !live {
		return ErrNotFound
	}
	return nil
}

func (s *Service) listNotes(contactID string, page, perPage int) ([]Note, int, error) {
	if err := s.requireLiveContact(contactID); err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM contact_notes WHERE contact_id = $1`,
		contactID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count notes: %w", err)
	}
	offset := util.Offset(page, perPage)
	rows, err := s.db.Query(`
		SELECT cn.id, cn.contact_id, cn.user_id, COALESCE(u.name, ''), cn.note, cn.created_at, cn.updated_at
		FROM contact_notes cn
		LEFT JOIN users u ON u.id = cn.user_id
		WHERE cn.contact_id = $1
		ORDER BY cn.created_at DESC
		LIMIT $2 OFFSET $3`, contactID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()
	notes := []Note{}
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.ContactID, &n.UserID, &n.UserName, &n.Note, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("list notes: scan: %w", err)
		}
		notes = append(notes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list notes: iterate: %w", err)
	}
	return notes, total, nil
}

func (s *Service) createNote(contactID, userID, note string) (*Note, error) {
	if err := s.requireLiveContact(contactID); err != nil {
		return nil, err
	}
	var n Note
	err := s.db.QueryRow(`
		INSERT INTO contact_notes (contact_id, user_id, note)
		SELECT $1, $2, $3
		WHERE EXISTS (SELECT 1 FROM contacts WHERE id = $1 AND deleted_at IS NULL)
		RETURNING id, contact_id, user_id,
			(SELECT COALESCE(name, '') FROM users WHERE id = $2),
			note, created_at, updated_at`,
		contactID, userID, note,
	).Scan(&n.ID, &n.ContactID, &n.UserID, &n.UserName, &n.Note, &n.CreatedAt, &n.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		// The contact was deleted between the check and the insert.
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("create note: %w", err)
	}
	// No audit row: the note itself is visible in the contact detail and
	// carries its author.
	return &n, nil
}

// deleteNote hard-deletes a note; the audit row is the only surviving trace,
// so it quotes the note's opening words instead of logging raw ids.
func (s *Service) deleteNote(contactID, noteID, userID string, canDeleteAny bool) error {
	if err := s.requireLiveContact(contactID); err != nil {
		return err
	}
	var preview string
	err := s.db.QueryRow(`SELECT note FROM contact_notes WHERE id = $1 AND contact_id = $2`, noteID, contactID).Scan(&preview)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("delete note: load preview: %w", err)
	}
	if len(preview) > 50 {
		preview = preview[:50] + "…"
	}
	res, err := s.db.Exec(
		`DELETE FROM contact_notes WHERE id = $1 AND contact_id = $2 AND (user_id = $3 OR $4)`,
		noteID, contactID, userID, canDeleteAny,
	)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete note: rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	s.auditLogDesc(fmt.Sprintf("Deleted note %q", preview), "contact_note", contactID, "delete", userID)
	return nil
}
