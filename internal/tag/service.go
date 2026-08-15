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
	ErrInvalidType = errors.New("type must be 'tag', 'status', 'quick_reply', 'activity_type' or 'loss_reason'")
	// ErrInvalidBehavior marks behavior values outside the outcome actions.
	ErrInvalidBehavior = errors.New("behavior must be 'log', 'next' or 'close_lost'")
)

// validType reports whether a tag type is an allowed catalog kind.
func validType(t string) bool {
	switch t {
	case "tag", "status", "quick_reply", "activity_type", "loss_reason":
		return true
	}
	return false
}

// validBehavior reports whether a behavior is an allowed outcome action.
func validBehavior(b string) bool {
	switch b {
	case "log", "next", "close_lost":
		return true
	}
	return false
}

// normalizeBehavior defaults empty behaviors to "log"; an explicit invalid
// value is rejected so the caller can report a friendly error.
func normalizeBehavior(b string) (string, error) {
	if b == "" {
		return "log", nil
	}
	if !validBehavior(b) {
		return "", ErrInvalidBehavior
	}
	return b, nil
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
		`SELECT id, name, type, COALESCE(color, ''), COALESCE(group_name, ''), sort_order, behavior, created_at
		FROM tags WHERE type = $1 ORDER BY sort_order, name`,
		tagType,
	)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	tags := []Tag{}
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Type, &t.Color, &t.GroupName, &t.SortOrder, &t.Behavior, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("list tags: scan: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, nil
}

func (s *Service) create(req CreateRequest) (*Tag, error) {
	if !validType(req.Type) {
		return nil, ErrInvalidType
	}
	behavior, err := normalizeBehavior(req.Behavior)
	if err != nil {
		return nil, err
	}
	var t Tag
	err = s.db.QueryRow(
		`INSERT INTO tags (name, type, color, group_name, sort_order, behavior)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, type, COALESCE(color, ''), COALESCE(group_name, ''), sort_order, behavior, created_at`,
		req.Name, req.Type, req.Color, req.GroupName, req.SortOrder, behavior,
	).Scan(&t.ID, &t.Name, &t.Type, &t.Color, &t.GroupName, &t.SortOrder, &t.Behavior, &t.CreatedAt)
	if err != nil {
		if respond.IsDuplicate(err) {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("create tag: %w", err)
	}
	return &t, nil
}

// update applies partial edits (name, color, group, sort order, behavior) to a
// tag. Behavior is validated; an explicit invalid value is rejected.
func (s *Service) update(id string, req UpdateRequest) (*Tag, error) {
	if req.Behavior != nil {
		b, err := normalizeBehavior(*req.Behavior)
		if err != nil {
			return nil, err
		}
		req.Behavior = &b
	}
	var t Tag
	err := s.db.QueryRow(
		`UPDATE tags SET
			name = COALESCE($2, name),
			color = COALESCE($3, color),
			group_name = COALESCE($4, group_name),
			sort_order = COALESCE($5, sort_order),
			behavior = COALESCE($6, behavior)
		WHERE id = $1
		RETURNING id, name, type, COALESCE(color, ''), COALESCE(group_name, ''), sort_order, behavior, created_at`,
		id, req.Name, req.Color, req.GroupName, req.SortOrder, req.Behavior,
	).Scan(&t.ID, &t.Name, &t.Type, &t.Color, &t.GroupName, &t.SortOrder, &t.Behavior, &t.CreatedAt)
	if err != nil {
		if respond.IsDuplicate(err) {
			return nil, ErrDuplicate
		}
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("update tag: %w", err)
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
