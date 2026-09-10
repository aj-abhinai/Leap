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

// InUseError is returned when deleting a status or quick reply that history
// rows still reference. It carries the reference count so the UI can show it;
// the word lists that back history (statuses, quick replies) refuse deletion
// while referenced — the same rule the database enforces with its RESTRICT
// foreign keys.
type InUseError struct {
	Count int
	Type  string
	Name  string
}

func (e *InUseError) Error() string {
	if e.Type == "status" {
		return fmt.Sprintf("%d contacts still carry the status %q; move them first", e.Count, e.Name)
	}
	return fmt.Sprintf("%d tasks still use the quick reply %q; move them first", e.Count, e.Name)
}

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
		`SELECT t.id, t.name, t.type, COALESCE(t.color, ''), COALESCE(t.group_name, ''),
			t.sort_order, t.behavior, t.created_at, COUNT(ct.contact_id) AS usage_count
		FROM tags t
		LEFT JOIN contact_tags ct ON ct.tag_id = t.id
		WHERE t.type = $1
		GROUP BY t.id
		ORDER BY t.sort_order, t.name`,
		tagType,
	)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	tags := []Tag{}
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Type, &t.Color, &t.GroupName, &t.SortOrder, &t.Behavior, &t.CreatedAt, &t.UsageCount); err != nil {
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
	groupName := req.GroupName
	if req.Type != "quick_reply" {
		// group_name and behavior configure quick replies only; every other
		// kind stores the neutral defaults.
		groupName = ""
		behavior = "log"
	}
	var t Tag
	err = s.db.QueryRow(
		`INSERT INTO tags (name, type, color, group_name, sort_order, behavior)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, type, COALESCE(color, ''), COALESCE(group_name, ''), sort_order, behavior, created_at`,
		req.Name, req.Type, req.Color, groupName, req.SortOrder, behavior,
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
// group_name/behavior are quick-reply-only concepts: for every other type the
// row is canonicalized back to the neutral defaults.
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
			group_name = CASE WHEN type = 'quick_reply' THEN COALESCE($4, group_name) ELSE '' END,
			sort_order = COALESCE($5, sort_order),
			behavior = CASE WHEN type = 'quick_reply' THEN COALESCE($6, behavior) ELSE 'log' END
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

// delete removes a vocabulary entry. Statuses and quick replies refuse
// deletion while history rows reference them (contacts.status_id,
// lead_activities.quick_reply_id); the count is reported through InUseError.
// Task types and loss reasons are free-text snapshots, so they delete freely.
// Deleting a missing tag stays a no-op, as before. The deleted tag's type and
// name are returned so the handler can write a readable audit row.
func (s *Service) delete(id string) error {
	_, _, err := s.deleteWithTag(id)
	return err
}

func (s *Service) deleteWithTag(id string) (string, string, error) {
	var typ, name string
	err := s.db.QueryRow(`SELECT type, name FROM tags WHERE id = $1`, id).Scan(&typ, &name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("delete tag: %w", err)
	}
	var count int
	switch typ {
	case "status":
		err = s.db.QueryRow(`SELECT COUNT(*) FROM contacts WHERE status_id = $1`, id).Scan(&count)
	case "quick_reply":
		err = s.db.QueryRow(`SELECT COUNT(*) FROM lead_activities WHERE quick_reply_id = $1`, id).Scan(&count)
	}
	if err != nil {
		return "", "", fmt.Errorf("delete tag: count references: %w", err)
	}
	if count > 0 {
		return typ, name, &InUseError{Count: count, Type: typ, Name: name}
	}
	if _, err := s.db.Exec(`DELETE FROM tags WHERE id = $1`, id); err != nil {
		// A link can land between the count and the delete; the RESTRICT
		// foreign key refuses the delete, and the refusal is reported as the
		// same in-use error with the now-current count instead of a 500.
		if respond.IsForeignKeyViolation(err) {
			switch typ {
			case "status":
				err = s.db.QueryRow(`SELECT COUNT(*) FROM contacts WHERE status_id = $1`, id).Scan(&count)
			case "quick_reply":
				err = s.db.QueryRow(`SELECT COUNT(*) FROM lead_activities WHERE quick_reply_id = $1`, id).Scan(&count)
			}
			if err == nil && count > 0 {
				return typ, name, &InUseError{Count: count, Type: typ, Name: name}
			}
		}
		return "", "", fmt.Errorf("delete tag: %w", err)
	}
	return typ, name, nil
}

// TypeLabel renders a tag type as the user-facing vocabulary word for audit
// descriptions.
func TypeLabel(typ string) string {
	switch typ {
	case "status":
		return "status"
	case "quick_reply":
		return "quick reply"
	case "activity_type":
		return "task type"
	case "loss_reason":
		return "loss reason"
	default:
		return "tag"
	}
}
