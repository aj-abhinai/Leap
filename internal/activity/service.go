package activity

import (
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

// LogLogin records a successful sign-in in the audit log.
func (s *Service) LogLogin(userID, userName, email string) {
	desc := "user logged in"
	if email != "" {
		desc = email + " logged in"
	}
	if _, err := s.db.Exec(
		`INSERT INTO audit_logs (description, user_id, user_name, action, resource_type)
		VALUES ($1, $2, $3, 'login', 'user')`,
		desc, userID, userName,
	); err != nil {
		slog.Error("log login", "error", err, "user_id", userID)
	}
}

// LogLogout records a sign-out in the audit log.
func (s *Service) LogLogout(userID, userName, email string) {
	desc := "user logged out"
	if email != "" {
		desc = email + " logged out"
	}
	if _, err := s.db.Exec(
		`INSERT INTO audit_logs (description, user_id, user_name, action, resource_type)
		VALUES ($1, $2, $3, 'logout', 'user')`,
		desc, userID, userName,
	); err != nil {
		slog.Error("log logout", "error", err, "user_id", userID)
	}
}

func (s *Service) list(page, perPage int, filters ActivityFilters) ([]Entry, int, error) {
	var total int
	baseWhere := "WHERE 1=1"
	args := []any{}
	argIdx := 1

	if filters.UserID != "" {
		baseWhere += fmt.Sprintf(" AND user_id = $%d", argIdx)
		args = append(args, filters.UserID)
		argIdx++
	}
	if filters.Action != "" {
		baseWhere += fmt.Sprintf(" AND action = $%d", argIdx)
		args = append(args, filters.Action)
		argIdx++
	}
	if filters.ResourceType != "" {
		baseWhere += fmt.Sprintf(" AND resource_type = $%d", argIdx)
		args = append(args, filters.ResourceType)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM audit_logs " + baseWhere
	err := s.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count activity: %w", err)
	}

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	selectQuery := fmt.Sprintf(`
		SELECT id, description, user_id, user_name, action, resource_type, resource_id, changes, created_at
		FROM audit_logs %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, baseWhere, argIdx, argIdx+1)
	args = append(args, perPage, offset)

	rows, err := s.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list activity: %w", err)
	}
	defer rows.Close()

	entries := []Entry{}
	for rows.Next() {
		var e Entry
		var changes []byte
		if err := rows.Scan(
			&e.ID,
			&e.Description,
			&e.UserID,
			&e.UserName,
			&e.Action,
			&e.ResourceType,
			&e.ResourceID,
			&changes,
			&e.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("list activity: scan: %w", err)
		}
		if len(changes) > 0 {
			raw := json.RawMessage(changes)
			e.Changes = &raw
		}
		entries = append(entries, e)
	}
	return entries, total, nil
}
