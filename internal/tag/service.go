package tag

import (
	"database/sql"
	"fmt"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List(tagType string) ([]Tag, error) {
	rows, err := s.db.Query(
		`SELECT id, name, type, COALESCE(color, ''), created_at FROM tags WHERE type = $1 ORDER BY name`,
		tagType,
	)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Type, &t.Color, &t.CreatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, nil
}

func (s *Service) Create(req CreateRequest) (*Tag, error) {
	var t Tag
	err := s.db.QueryRow(
		`INSERT INTO tags (name, type, color) VALUES ($1, $2, $3) RETURNING id, name, type, COALESCE(color, ''), created_at`,
		req.Name, req.Type, req.Color,
	).Scan(&t.ID, &t.Name, &t.Type, &t.Color, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create tag: %w", err)
	}
	return &t, nil
}

func (s *Service) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM tags WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	return nil
}
