package program

import (
	"crm/internal/util"
	"database/sql"
	"errors"
	"fmt"
)

var (
	ErrNotFound      = errors.New("program not found")
	ErrNameRequired  = errors.New("name is required")
	ErrNegativePrice = errors.New("price cannot be negative")
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

const selectColumns = `id, name, COALESCE(description, ''), price, (deleted_at IS NOT NULL), created_at, updated_at`

func (s *Service) scanProgram(row interface{ Scan(...any) error }) (*Program, error) {
	var p Program
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Archived, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan program: %w", err)
	}
	return &p, nil
}

// listActive returns non-archived programs for lead-form selection.
func (s *Service) listActive() ([]Program, error) {
	rows, err := s.db.Query(
		`SELECT ` + selectColumns + ` FROM programs WHERE deleted_at IS NULL ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list programs: %w", err)
	}
	defer rows.Close()
	return collect(rows)
}

// listAll returns every program including archived ones for settings.
func (s *Service) listAll() ([]Program, error) {
	rows, err := s.db.Query(
		`SELECT ` + selectColumns + ` FROM programs ORDER BY deleted_at IS NULL, name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list all programs: %w", err)
	}
	defer rows.Close()
	return collect(rows)
}

func collect(rows *sql.Rows) ([]Program, error) {
	programs := []Program{}
	for rows.Next() {
		var p Program
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Archived, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan program: %w", err)
		}
		programs = append(programs, p)
	}
	return programs, rows.Err()
}

// getActive loads a single non-archived program; used by lead price snapshot.
func (s *Service) getActive(id string) (*Program, error) {
	return s.scanProgram(s.db.QueryRow(
		`SELECT `+selectColumns+` FROM programs WHERE id = $1 AND deleted_at IS NULL`,
		id,
	))
}

func (s *Service) create(req CreateRequest) (*Program, error) {
	if req.Name == "" {
		return nil, ErrNameRequired
	}
	if req.Price < 0 {
		return nil, ErrNegativePrice
	}
	return s.scanProgram(s.db.QueryRow(
		`INSERT INTO programs (name, description, price) VALUES ($1, $2, $3)
		RETURNING `+selectColumns,
		req.Name, util.NullStr(req.Description), req.Price,
	))
}

func (s *Service) update(id string, req UpdateRequest) (*Program, error) {
	if req.Name != nil && *req.Name == "" {
		return nil, ErrNameRequired
	}
	if req.Price != nil && *req.Price < 0 {
		return nil, ErrNegativePrice
	}
	p, err := s.scanProgram(s.db.QueryRow(
		`UPDATE programs SET
			name = CASE WHEN NULLIF($2, '') IS NOT NULL THEN $2 ELSE name END,
			description = CASE WHEN $3 IS NOT NULL THEN NULLIF($3, '') ELSE description END,
			price = COALESCE($4, price),
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+selectColumns,
		id,
		req.Name,
		req.Description,
		req.Price,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update program: %w", err)
	}
	return p, nil
}

// archive soft-deletes a program so historical leads keep their reference;
// restore clears deleted_at again.
func (s *Service) archive(id string) error {
	res, err := s.db.Exec(`UPDATE programs SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("archive program: %w", err)
	}
	if rows, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("archive program: rows affected: %w", err)
	} else if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) restore(id string) error {
	res, err := s.db.Exec(`UPDATE programs SET deleted_at = NULL, updated_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("restore program: %w", err)
	}
	if rows, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("restore program: rows affected: %w", err)
	} else if rows == 0 {
		return ErrNotFound
	}
	return nil
}
