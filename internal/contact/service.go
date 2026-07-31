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

func (s *Service) List(page, perPage int, search string) ([]Contact, int, error) {
	var total int
	baseWhere := "WHERE deleted_at IS NULL"
	args := []any{}
	argIdx := 1

	if search != "" {
		baseWhere += fmt.Sprintf(" AND (name ILIKE $%d OR email ILIKE $%d OR phone ILIKE $%d)", argIdx, argIdx+1, argIdx+2)
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

	selectQuery := fmt.Sprintf("SELECT id, name, COALESCE(email, ''), COALESCE(phone, ''), COALESCE(location, ''), age, created_at, updated_at FROM contacts %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d", baseWhere, argIdx, argIdx+1)
	args = append(args, perPage, offset)
	rows, err := s.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list contacts: %w", err)
	}
	defer rows.Close()

	contacts := []Contact{}
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.ID, &c.Name, &c.Email, &c.Phone, &c.Location, &c.Age, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, 0, err
		}
		contacts = append(contacts, c)
	}
	return contacts, total, nil
}

func (s *Service) Get(id string) (*Contact, error) {
	var c Contact
	err := s.db.QueryRow(
		`SELECT id, name, COALESCE(email, ''), COALESCE(phone, ''), COALESCE(location, ''), age, created_at, updated_at FROM contacts WHERE id = $1 AND deleted_at IS NULL`,
		id,
	).Scan(&c.ID, &c.Name, &c.Email, &c.Phone, &c.Location, &c.Age, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Service) Create(req CreateRequest) (*Contact, error) {
	var c Contact
	err := s.db.QueryRow(
		`INSERT INTO contacts (name, email, phone, location, age) VALUES ($1, $2, $3, $4, $5) RETURNING id, name, COALESCE(email, ''), COALESCE(phone, ''), COALESCE(location, ''), age, created_at, updated_at`,
		req.Name, util.NullStr(req.Email), util.NullStr(req.Phone), util.NullStr(req.Location), req.Age,
	).Scan(&c.ID, &c.Name, &c.Email, &c.Phone, &c.Location, &c.Age, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create contact: %w", err)
	}
	return &c, nil
}

func (s *Service) Update(id string, req UpdateRequest) (*Contact, error) {
	old, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	var c Contact
	err = s.db.QueryRow(`
		UPDATE contacts SET
			name = COALESCE($2, name),
			email = COALESCE($3, email),
			phone = COALESCE($4, phone),
			location = COALESCE($5, location),
			age = COALESCE($6, age),
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, name, COALESCE(email, ''), COALESCE(phone, ''), COALESCE(location, ''), age, created_at, updated_at`,
		id, req.Name, util.StrPtr(req.Email), util.StrPtr(req.Phone), util.StrPtr(req.Location), req.Age,
	).Scan(&c.ID, &c.Name, &c.Email, &c.Phone, &c.Location, &c.Age, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update contact: %w", err)
	}

	changes := diffContact(old, &c)
	if changes != "" {
		s.logActivity(id, "contact", "update", changes)
	}
	return &c, nil
}

func (s *Service) Delete(id string) error {
	_, err := s.db.Exec(`UPDATE contacts SET deleted_at = now() WHERE id = $1`, id)
	if err != nil {
		return err
	}
	s.logActivity(id, "contact", "delete", `{"action":"deleted"}`)
	return nil
}

func (s *Service) logActivity(resourceID, resourceType, action, changes string) {
	if _, err := s.db.Exec(
		`INSERT INTO audit_logs (description, resource_type, resource_id, action, changes) VALUES ($1, $2, $3, $4, $5)`,
		action+" "+resourceType+" "+resourceID, resourceType, resourceID, action, changes,
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
	if len(diff) == 0 {
		return ""
	}
	b, _ := json.Marshal(diff)
	return string(b)
}

