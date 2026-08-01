package contact

import (
	"crm/internal/util"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) list(page, perPage int, search string) ([]Contact, int, error) {
	var total int
	baseWhere := "WHERE deleted_at IS NULL"
	args := []any{}
	argIdx := 1

	if search != "" {
		baseWhere += fmt.Sprintf(
			" AND (name ILIKE $%d OR email ILIKE $%d OR phone ILIKE $%d)",
			argIdx, argIdx+1, argIdx+2,
		)
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm, searchTerm)
		argIdx += 3
	}

	countQuery := "SELECT COUNT(*) FROM contacts " + baseWhere
	err := s.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count contacts: %w", err)
	}

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	selectQuery := fmt.Sprintf(
		`SELECT id, name, COALESCE(email, ''), COALESCE(phone, ''), COALESCE(location, ''), age, created_at, updated_at
		FROM contacts %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`,
		baseWhere, argIdx, argIdx+1,
	)
	args = append(args, perPage, offset)
	rows, err := s.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list contacts: %w", err)
	}
	defer rows.Close()

	contacts := []Contact{}
	contactIDs := []string{}
	for rows.Next() {
		var c Contact
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Email, &c.Phone, &c.Location, &c.Age, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		contacts = append(contacts, c)
		contactIDs = append(contactIDs, c.ID)
	}

	if len(contacts) > 0 {
		contacts, err = s.populateTagsAndStatus(contacts, contactIDs)
		if err != nil {
			return nil, 0, err
		}
	}

	return contacts, total, nil
}

func (s *Service) populateTagsAndStatus(contacts []Contact, contactIDs []string) ([]Contact, error) {
	tagsByContact := make(map[string][]TagRef, len(contactIDs))
	statusByContact := make(map[string]*TagRef)

	for _, id := range contactIDs {
		tagsByContact[id] = []TagRef{}
	}

	tagRows, err := s.db.Query(
		`SELECT ct.contact_id, t.id, t.name, COALESCE(t.color, '')
		FROM contact_tags ct
		JOIN tags t ON t.id = ct.tag_id
		WHERE ct.contact_id = ANY($1)
		ORDER BY t.name`,
		contactIDs,
	)
	if err != nil {
		return contacts, fmt.Errorf("load contact tags: %w", err)
	}
	defer tagRows.Close()
	for tagRows.Next() {
		var contactID string
		var ref TagRef
		if err := tagRows.Scan(&contactID, &ref.ID, &ref.Name, &ref.Color); err != nil {
			return contacts, fmt.Errorf("scan contact tag: %w", err)
		}
		tagsByContact[contactID] = append(tagsByContact[contactID], ref)
	}

	statusRows, err := s.db.Query(
		`SELECT c.id, COALESCE(t.id::text, ''), COALESCE(t.name, ''), COALESCE(t.color, '')
		FROM contacts c
		LEFT JOIN tags t ON t.id = c.status_id
		WHERE c.id = ANY($1)`,
		contactIDs,
	)
	if err != nil {
		return contacts, fmt.Errorf("load contact statuses: %w", err)
	}
	defer statusRows.Close()
	for statusRows.Next() {
		var contactID string
		var ref TagRef
		if err := statusRows.Scan(&contactID, &ref.ID, &ref.Name, &ref.Color); err != nil {
			return contacts, fmt.Errorf("scan contact status: %w", err)
		}
		if ref.ID != "" {
			statusByContact[contactID] = &ref
		}
	}

	for i := range contacts {
		contacts[i].Tags = tagsByContact[contacts[i].ID]
		contacts[i].Status = statusByContact[contacts[i].ID]
	}
	return contacts, nil
}

func (s *Service) get(id string) (*Contact, error) {
	var c Contact
	err := s.db.QueryRow(
		`SELECT id, name, COALESCE(email, ''), COALESCE(phone, ''), COALESCE(location, ''), age, created_at, updated_at
		FROM contacts
		WHERE id = $1 AND deleted_at IS NULL`,
		id,
	).Scan(
		&c.ID, &c.Name, &c.Email, &c.Phone, &c.Location, &c.Age, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	populated, err := s.populateTagsAndStatus([]Contact{c}, []string{c.ID})
	if err != nil {
		return nil, err
	}
	return &populated[0], nil
}

func (s *Service) create(req CreateRequest) (*Contact, error) {
	keys, err := s.loadContactKeys()
	if err != nil {
		slog.Error("load contact keys for duplicate check", "error", err)
		keys = contactKeys{phones: map[string]bool{}, emails: map[string]bool{}}
	}

	var c Contact
	err = s.db.QueryRow(
		`INSERT INTO contacts (name, email, phone, location, age, status_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, COALESCE(email, ''), COALESCE(phone, ''), COALESCE(location, ''), age, created_at, updated_at`,
		req.Name, util.NullStr(req.Email), util.NullStr(req.Phone), util.NullStr(req.Location), req.Age, req.StatusID,
	).Scan(
		&c.ID, &c.Name, &c.Email, &c.Phone, &c.Location, &c.Age, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create contact: %w", err)
	}

	for _, tagID := range req.TagIDs {
		_, _ = s.db.Exec(
			`INSERT INTO contact_tags (contact_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			c.ID, tagID,
		)
	}

	populated, err := s.populateTagsAndStatus([]Contact{c}, []string{c.ID})
	if err != nil {
		return nil, err
	}
	if reason := keys.duplicateReason(req.Phone, req.Email); reason != "" {
		populated[0].Warnings = []string{reason}
	}
	return &populated[0], nil
}

func (s *Service) update(id string, req UpdateRequest, userID string) (*Contact, error) {
	old, err := s.get(id)
	if err != nil {
		return nil, err
	}

	var c Contact
	err = s.db.QueryRow(
		`UPDATE contacts SET
			name = COALESCE($2, name),
			email = COALESCE($3, email),
			phone = COALESCE($4, phone),
			location = COALESCE($5, location),
			age = COALESCE($6, age),
			status_id = COALESCE($7, status_id),
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, name, COALESCE(email, ''), COALESCE(phone, ''), COALESCE(location, ''), age, created_at, updated_at`,
		id,
		req.Name,
		util.StrPtr(req.Email),
		util.StrPtr(req.Phone),
		util.StrPtr(req.Location),
		req.Age,
		req.StatusID,
	).Scan(
		&c.ID, &c.Name, &c.Email, &c.Phone, &c.Location, &c.Age, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update contact: %w", err)
	}

	if req.TagIDs != nil {
		_, _ = s.db.Exec(`DELETE FROM contact_tags WHERE contact_id = $1`, id)
		for _, tagID := range req.TagIDs {
			_, _ = s.db.Exec(
				`INSERT INTO contact_tags (contact_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				id, tagID,
			)
		}
	}

	populated, err := s.populateTagsAndStatus([]Contact{c}, []string{c.ID})
	if err != nil {
		return nil, err
	}
	c = populated[0]

	changes := diffContact(old, &c)
	if changes != "" {
		s.logActivity(id, "contact", "update", changes, userID)
	}
	return &c, nil
}

func (s *Service) delete(id string, userID string) error {
	_, err := s.db.Exec(`UPDATE contacts SET deleted_at = now() WHERE id = $1`, id)
	if err != nil {
		return err
	}
	s.logActivity(id, "contact", "delete", `{"action":"deleted"}`, userID)
	return nil
}

func (s *Service) logActivity(resourceID, resourceType, action, changes, userID string) {
	userName := ""
	if userID != "" {
		if err := s.db.QueryRow(`SELECT name FROM users WHERE id = $1`, userID).Scan(&userName); err != nil {
			slog.Error("resolve audit actor name", "error", err, "user_id", userID)
		}
	}
	if _, err := s.db.Exec(
		`INSERT INTO audit_logs (description, resource_type, resource_id, action, changes, user_id, user_name)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''))`,
		action+" "+resourceType+" "+resourceID, resourceType, resourceID, changes, userID, userName,
	); err != nil {
		slog.Error("log activity", "error", err, "resource_type", resourceType, "resource_id", resourceID)
	}
}

func diffContact(old, new *Contact) string {
	diff := map[string]any{}
	if old.Name != new.Name {
		diff["name"] = map[string]string{"old": old.Name, "new": new.Name}
	}
	if old.Email != new.Email {
		diff["email"] = map[string]string{"old": old.Email, "new": new.Email}
	}
	if old.Phone != new.Phone {
		diff["phone"] = map[string]string{"old": old.Phone, "new": new.Phone}
	}
	if old.Location != new.Location {
		diff["location"] = map[string]string{"old": old.Location, "new": new.Location}
	}
	if (old.Age == nil) != (new.Age == nil) || (old.Age != nil && *old.Age != *new.Age) {
		diff["age"] = map[string]any{"old": old.Age, "new": new.Age}
	}

	oldTags := tagsToSet(old.Tags)
	newTags := tagsToSet(new.Tags)
	if tagsChanged(oldTags, newTags) {
		diff["tags"] = map[string]any{"old": old.Tags, "new": new.Tags}
	}

	oldStatus := ""
	if old.Status != nil {
		oldStatus = old.Status.Name
	}
	newStatus := ""
	if new.Status != nil {
		newStatus = new.Status.Name
	}
	if oldStatus != newStatus {
		diff["status"] = map[string]string{"old": oldStatus, "new": newStatus}
	}

	if len(diff) == 0 {
		return ""
	}
	b, _ := json.Marshal(diff)
	return string(b)
}

func tagsToSet(tags []TagRef) map[string]bool {
	s := make(map[string]bool)
	for _, t := range tags {
		s[t.ID] = true
	}
	return s
}

func tagsChanged(old, new map[string]bool) bool {
	if len(old) != len(new) {
		return true
	}
	for k := range old {
		if !new[k] {
			return true
		}
	}
	return false
}
