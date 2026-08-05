package tag

import (
	"crm/internal/respond"
	"database/sql"
	"errors"
	"fmt"
)

var (
	// ErrDuplicate marks tag names that already exist.
	ErrDuplicate = errors.New("tag name already exists")
	// ErrInvalidType marks tag types outside the allowed catalog kinds.
	ErrInvalidType = errors.New("type must be 'tag', 'status', 'activity_type' or 'loss_reason'")
)

// validType reports whether a tag type is an allowed catalog kind.
func validType(t string) bool {
	switch t {
	case "tag", "status", "activity_type", "loss_reason":
		return true
	}
	return false
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) list(tagType string) ([]Tag, error) {
	if tagType == "" {
		tagType = "tag"
	}
	if !validType(tagType) {
		return nil, ErrInvalidType
	}
	rows, err := s.db.Query(
		`SELECT id, name, type, COALESCE(color, ''), created_at FROM tags WHERE type = $1 ORDER BY name`,
		tagType,
	)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	tags := []Tag{}
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Type, &t.Color, &t.CreatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, nil
}

func (s *Service) create(req CreateRequest) (*Tag, error) {
	if !validType(req.Type) {
		return nil, ErrInvalidType
	}
	var t Tag
	err := s.db.QueryRow(
		`INSERT INTO tags (name, type, color) VALUES ($1, $2, $3) RETURNING id, name, type, COALESCE(color, ''), created_at`,
		req.Name, req.Type, req.Color,
	).Scan(&t.ID, &t.Name, &t.Type, &t.Color, &t.CreatedAt)
	if err != nil {
		if respond.IsDuplicate(err) {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("create tag: %w", err)
	}
	return &t, nil
}

func (s *Service) delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM tags WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	return nil
}
