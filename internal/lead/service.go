package lead

import (
	"crm/internal/util"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
)

var (
	ErrCustomValueRejected = errors.New("lead value is set from the program catalog price")
	ErrProgramNotActive    = errors.New("program not found or archived")
	// ErrNotFound marks mutations targeting a lead that does not exist or
	// has been deleted.
	ErrNotFound = errors.New("lead not found")
	// ErrStageNotInPipeline marks leads whose stage does not belong to the
	// pipeline they are assigned to.
	ErrStageNotInPipeline = errors.New("stage_id does not belong to the pipeline")
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) list(pipelineID, stageID, contactID string, page, perPage int) ([]Lead, int, error) {
	var total int
	baseWhere := "WHERE l.deleted_at IS NULL"
	args := []any{}
	argIdx := 1

	if pipelineID != "" {
		baseWhere += fmt.Sprintf(" AND l.pipeline_id = $%d", argIdx)
		args = append(args, pipelineID)
		argIdx++
	}
	if stageID != "" {
		baseWhere += fmt.Sprintf(" AND l.stage_id = $%d", argIdx)
		args = append(args, stageID)
		argIdx++
	}
	if contactID != "" {
		baseWhere += fmt.Sprintf(" AND l.contact_id = $%d", argIdx)
		args = append(args, contactID)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM leads l " + baseWhere
	err := s.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count leads: %w", err)
	}

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	selectQuery := fmt.Sprintf(
		`SELECT l.id, l.name, COALESCE(l.email, ''), COALESCE(l.phone, ''),
			l.contact_id, COALESCE(c.name, ''), l.pipeline_id, l.stage_id, COALESCE(ls.name, ''), l.value,
			l.program_id, COALESCE(p.name, ''), COALESCE(l.notes, ''), l.assigned_to, l.created_at, l.updated_at
		FROM leads l
		LEFT JOIN lead_stages ls ON l.stage_id = ls.id
		LEFT JOIN contacts c ON l.contact_id = c.id
		LEFT JOIN programs p ON l.program_id = p.id
		%s
		ORDER BY l.created_at DESC
		LIMIT $%d OFFSET $%d`,
		baseWhere, argIdx, argIdx+1,
	)
	args = append(args, perPage, offset)

	rows, err := s.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list leads: %w", err)
	}
	defer rows.Close()

	leads := []Lead{}
	for rows.Next() {
		var l Lead
		if err := rows.Scan(
			&l.ID, &l.Name, &l.Email, &l.Phone, &l.ContactID, &l.ContactName, &l.PipelineID, &l.StageID,
			&l.StageName, &l.Value, &l.ProgramID, &l.ProgramName, &l.Notes, &l.AssignedTo,
			&l.CreatedAt, &l.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		leads = append(leads, l)
	}
	return leads, total, nil
}

func (s *Service) get(id string) (*Lead, error) {
	var l Lead
	err := s.db.QueryRow(
		`SELECT l.id, l.name, COALESCE(l.email, ''), COALESCE(l.phone, ''),
			l.contact_id, COALESCE(c.name, ''), l.pipeline_id, l.stage_id, COALESCE(ls.name, ''), l.value,
			l.program_id, COALESCE(p.name, ''), COALESCE(l.notes, ''), l.assigned_to, l.created_at, l.updated_at
		FROM leads l
		LEFT JOIN lead_stages ls ON l.stage_id = ls.id
		LEFT JOIN contacts c ON l.contact_id = c.id
		LEFT JOIN programs p ON l.program_id = p.id
		WHERE l.id = $1 AND l.deleted_at IS NULL`, id,
	).Scan(
		&l.ID, &l.Name, &l.Email, &l.Phone, &l.ContactID, &l.ContactName, &l.PipelineID, &l.StageID,
		&l.StageName, &l.Value, &l.ProgramID, &l.ProgramName, &l.Notes, &l.AssignedTo,
		&l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *Service) create(req CreateRequest, userID string) (*Lead, error) {
	if req.Value != nil {
		return nil, ErrCustomValueRejected
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("create lead: %w", err)
	}
	defer tx.Rollback()

	if err := s.validateStageForPipelineTx(tx, req.PipelineID, req.StageID); err != nil {
		return nil, err
	}
	programPrice, err := s.snapshotPriceTx(tx, req.ProgramID)
	if err != nil {
		return nil, err
	}
	var l Lead
	err = tx.QueryRow(
		`INSERT INTO leads (name, email, phone, contact_id, pipeline_id, stage_id, program_id, value, notes, assigned_to)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, name, COALESCE(email, ''), COALESCE(phone, ''), contact_id, pipeline_id, stage_id,
			program_id, value, COALESCE(notes, ''), assigned_to, created_at, updated_at`,
		req.Name,
		util.NullStr(req.Email),
		util.NullStr(req.Phone),
		util.NullPtr(req.ContactID),
		req.PipelineID,
		req.StageID,
		util.NullPtr(req.ProgramID),
		programPrice,
		util.NullStr(req.Notes),
		util.NullPtr(req.AssignedTo),
	).Scan(
		&l.ID, &l.Name, &l.Email, &l.Phone, &l.ContactID, &l.PipelineID, &l.StageID, &l.ProgramID,
		&l.Value, &l.Notes, &l.AssignedTo, &l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create lead: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit lead: %w", err)
	}
	if err := s.populateNames(&l); err != nil {
		return nil, err
	}
	s.logActivity(l.ID, "lead", "create", "", userID)
	return &l, nil
}

func (s *Service) update(id string, req UpdateRequest, userID string) (*Lead, error) {
	if req.Value != nil {
		return nil, ErrCustomValueRejected
	}
	old, err := s.get(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("update lead: %w", err)
	}
	defer tx.Rollback()

	if req.StageID != nil {
		pipelineID := old.PipelineID
		if req.PipelineID != nil {
			pipelineID = *req.PipelineID
		}
		if err := s.validateStageForPipelineTx(tx, pipelineID, *req.StageID); err != nil {
			return nil, err
		}
	}
	if req.PipelineID != nil && req.StageID == nil {
		var stageID string
		if err := tx.QueryRow(
			`SELECT id FROM lead_stages WHERE pipeline_id = $1 AND id = $2`,
			*req.PipelineID, old.StageID,
		).Scan(&stageID); err != nil {
			return nil, ErrStageNotInPipeline
		}
	}

	var programPrice *float64
	if req.ProgramID != nil && *req.ProgramID != "" {
		if old.ProgramID != nil && *old.ProgramID == *req.ProgramID {
			programPrice = old.Value
		} else {
			price, err := s.activeProgramPriceTx(tx, *req.ProgramID)
			if err != nil {
				return nil, err
			}
			programPrice = &price
		}
	}

	var l Lead
	err = tx.QueryRow(
		`UPDATE leads SET
			name = CASE WHEN NULLIF($2, '') IS NOT NULL THEN $2 ELSE name END,
			email = CASE WHEN $3 IS NOT NULL THEN NULLIF($3, '') ELSE email END,
			phone = CASE WHEN $4 IS NOT NULL THEN NULLIF($4, '') ELSE phone END,
			contact_id = CASE WHEN $5 IS NOT NULL THEN NULLIF($5, '')::uuid ELSE contact_id END,
			pipeline_id = COALESCE($6, pipeline_id),
			stage_id = COALESCE($7, stage_id),
			program_id = CASE WHEN $8 IS NOT NULL THEN NULLIF($8, '')::uuid ELSE program_id END,
			value = CASE WHEN $8 IS NOT NULL AND NULLIF($8, '') IS NULL THEN NULL ELSE COALESCE($9, value) END,
			notes = CASE WHEN $10 IS NOT NULL THEN NULLIF($10, '') ELSE notes END,
			assigned_to = CASE WHEN $11 IS NOT NULL THEN NULLIF($11, '')::uuid ELSE assigned_to END,
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, name, COALESCE(email, ''), COALESCE(phone, ''), contact_id, pipeline_id, stage_id,
			program_id, value, COALESCE(notes, ''), assigned_to, created_at, updated_at`,
		id,
		req.Name,
		req.Email,
		req.Phone,
		req.ContactID,
		req.PipelineID,
		req.StageID,
		req.ProgramID,
		programPrice,
		req.Notes,
		req.AssignedTo,
	).Scan(
		&l.ID, &l.Name, &l.Email, &l.Phone, &l.ContactID, &l.PipelineID, &l.StageID, &l.ProgramID,
		&l.Value, &l.Notes, &l.AssignedTo, &l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update lead: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit lead update: %w", err)
	}
	if err := s.populateNames(&l); err != nil {
		return nil, err
	}

	action := "update"
	desc := "Updated lead"
	if old.StageID != l.StageID && old.StageID != "" {
		action = "move_stage"
		oldStage, err := s.stageName(old.StageID)
		if err != nil {
			return nil, err
		}
		desc = fmt.Sprintf("Moved lead from %q to %q", oldStage, l.StageName)
	}
	s.logActivity(l.ID, "lead", action, desc, userID)
	return &l, nil
}

func (s *Service) delete(id string, userID string) error {
	res, err := s.db.Exec(`UPDATE leads SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	s.logActivity(id, "lead", "delete", "deleted", userID)
	return nil
}

// validateStageForPipelineTx rejects stage ids that do not belong to the
// given pipeline so kanban columns can always display a lead's stage.
func (s *Service) validateStageForPipelineTx(tx *sql.Tx, pipelineID, stageID string) error {
	var exists bool
	err := tx.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM lead_stages WHERE id = $1 AND pipeline_id = $2)`,
		stageID, pipelineID,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("validate stage pipeline: %w", err)
	}
	if !exists {
		return ErrStageNotInPipeline
	}
	return nil
}

func (s *Service) populateNames(l *Lead) error {
	stageName, err := s.stageName(l.StageID)
	if err != nil {
		return err
	}
	l.StageName = stageName
	if l.ProgramID != nil {
		var programName string
		if err := s.db.QueryRow(`SELECT name FROM programs WHERE id = $1`, *l.ProgramID).Scan(&programName); err != nil {
			return fmt.Errorf("load program name: %w", err)
		}
		l.ProgramName = programName
	}
	if l.ContactID == nil {
		return nil
	}
	var contactName string
	if err := s.db.QueryRow(`SELECT name FROM contacts WHERE id = $1`, *l.ContactID).Scan(&contactName); err != nil {
		return fmt.Errorf("load contact name: %w", err)
	}
	l.ContactName = contactName
	return nil
}

// snapshotPriceTx resolves the catalog price of an optional program so it can
// be stored as the lead's immutable value snapshot. The program row is locked
// for the duration of the transaction so a concurrent archive cannot slip in
// between the check and the lead insert.
func (s *Service) snapshotPriceTx(tx *sql.Tx, programID *string) (*float64, error) {
	if programID == nil {
		return nil, nil
	}
	price, err := s.activeProgramPriceTx(tx, *programID)
	if err != nil {
		return nil, err
	}
	return &price, nil
}

// activeProgramPriceTx rejects archived or unknown programs so historical
// catalog entries can never be attached to new leads.
func (s *Service) activeProgramPriceTx(tx *sql.Tx, programID string) (float64, error) {
	var price float64
	err := tx.QueryRow(
		`SELECT price FROM programs WHERE id = $1 AND deleted_at IS NULL FOR SHARE`,
		programID,
	).Scan(&price)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrProgramNotActive
	}
	if err != nil {
		return 0, fmt.Errorf("load program price: %w", err)
	}
	return price, nil
}

func (s *Service) stageName(stageID string) (string, error) {
	var name string
	if err := s.db.QueryRow(`SELECT name FROM lead_stages WHERE id = $1`, stageID).Scan(&name); err != nil {
		return "", fmt.Errorf("load stage name: %w", err)
	}
	return name, nil
}

func (s *Service) logActivity(resourceID, resourceType, action, desc, userID string) {
	userName := ""
	if userID != "" {
		if err := s.db.QueryRow(`SELECT name FROM users WHERE id = $1`, userID).Scan(&userName); err != nil {
			slog.Error("resolve audit actor name", "error", err, "user_id", userID)
		}
	}
	if _, err := s.db.Exec(
		`INSERT INTO audit_logs (description, resource_type, resource_id, action, user_id, user_name)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''))`,
		desc, resourceType, resourceID, action, userID, userName,
	); err != nil {
		slog.Error("log activity", "error", err, "resource_type", resourceType, "resource_id", resourceID)
	}
}
