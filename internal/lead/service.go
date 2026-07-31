package lead

import (
	"crm/internal/util"
	"database/sql"
	"fmt"
	"log/slog"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List(pipelineID, stageID string, page, perPage int) ([]Lead, int, error) {
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

	selectQuery := fmt.Sprintf(`
		SELECT l.id, l.name, COALESCE(l.email, ''), COALESCE(l.phone, ''),
			l.contact_id, COALESCE(c.name, ''), l.pipeline_id, l.stage_id, COALESCE(ls.name, ''), l.value,
			COALESCE(l.notes, ''), l.assigned_to, l.created_at, l.updated_at
		FROM leads l
		LEFT JOIN lead_stages ls ON l.stage_id = ls.id
		LEFT JOIN contacts c ON l.contact_id = c.id
		%s
		ORDER BY l.created_at DESC
		LIMIT $%d OFFSET $%d`, baseWhere, argIdx, argIdx+1)
	args = append(args, perPage, offset)

	rows, err := s.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list leads: %w", err)
	}
	defer rows.Close()

	leads := []Lead{}
	for rows.Next() {
		var l Lead
		if err := rows.Scan(&l.ID, &l.Name, &l.Email, &l.Phone, &l.ContactID, &l.ContactName, &l.PipelineID, &l.StageID, &l.StageName, &l.Value, &l.Notes, &l.AssignedTo, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, 0, err
		}
		leads = append(leads, l)
	}
	return leads, total, nil
}

func (s *Service) Get(id string) (*Lead, error) {
	var l Lead
	err := s.db.QueryRow(`
		SELECT l.id, l.name, COALESCE(l.email, ''), COALESCE(l.phone, ''),
			l.contact_id, COALESCE(c.name, ''), l.pipeline_id, l.stage_id, COALESCE(ls.name, ''), l.value,
			COALESCE(l.notes, ''), l.assigned_to, l.created_at, l.updated_at
		FROM leads l
		LEFT JOIN lead_stages ls ON l.stage_id = ls.id
		LEFT JOIN contacts c ON l.contact_id = c.id
		WHERE l.id = $1 AND l.deleted_at IS NULL`, id,
	).Scan(&l.ID, &l.Name, &l.Email, &l.Phone, &l.ContactID, &l.ContactName, &l.PipelineID, &l.StageID, &l.StageName, &l.Value, &l.Notes, &l.AssignedTo, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *Service) Create(req CreateRequest) (*Lead, error) {
	var l Lead
	err := s.db.QueryRow(`
		INSERT INTO leads (name, email, phone, contact_id, pipeline_id, stage_id, value, notes, assigned_to)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, name, COALESCE(email, ''), COALESCE(phone, ''), contact_id, pipeline_id, stage_id, value, COALESCE(notes, ''), assigned_to, created_at, updated_at`,
		req.Name, util.NullStr(req.Email), util.NullStr(req.Phone), req.ContactID, req.PipelineID, req.StageID, req.Value, util.NullStr(req.Notes), req.AssignedTo,
	).Scan(&l.ID, &l.Name, &l.Email, &l.Phone, &l.ContactID, &l.PipelineID, &l.StageID, &l.Value, &l.Notes, &l.AssignedTo, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create lead: %w", err)
	}
	var stageName string
	s.db.QueryRow(`SELECT name FROM lead_stages WHERE id = $1`, l.StageID).Scan(&stageName)
	l.StageName = stageName
	var contactName string
	if l.ContactID != nil {
		s.db.QueryRow(`SELECT name FROM contacts WHERE id = $1`, *l.ContactID).Scan(&contactName)
		l.ContactName = contactName
	}
	s.logActivity(l.ID, "lead", "create", "")
	return &l, nil
}

func (s *Service) Update(id string, req UpdateRequest) (*Lead, error) {
	old, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	var l Lead
	err = s.db.QueryRow(`
		UPDATE leads SET
			name = COALESCE($2, name),
			email = COALESCE($3, email),
			phone = COALESCE($4, phone),
			contact_id = COALESCE($5, contact_id),
			pipeline_id = COALESCE($6, pipeline_id),
			stage_id = COALESCE($7, stage_id),
			value = COALESCE($8, value),
			notes = COALESCE($9, notes),
			assigned_to = COALESCE($10, assigned_to),
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, name, COALESCE(email, ''), COALESCE(phone, ''), contact_id, pipeline_id, stage_id, value, COALESCE(notes, ''), assigned_to, created_at, updated_at`,
		id, req.Name, util.StrPtr(req.Email), util.StrPtr(req.Phone), req.ContactID, req.PipelineID, req.StageID, req.Value, util.StrPtr(req.Notes), req.AssignedTo,
	).Scan(&l.ID, &l.Name, &l.Email, &l.Phone, &l.ContactID, &l.PipelineID, &l.StageID, &l.Value, &l.Notes, &l.AssignedTo, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update lead: %w", err)
	}
	var stageName string
	s.db.QueryRow(`SELECT name FROM lead_stages WHERE id = $1`, l.StageID).Scan(&stageName)
	l.StageName = stageName
	var contactName string
	if l.ContactID != nil {
		s.db.QueryRow(`SELECT name FROM contacts WHERE id = $1`, *l.ContactID).Scan(&contactName)
		l.ContactName = contactName
	}

	action := "update"
	desc := "Updated lead"
	if old.StageID != l.StageID && old.StageID != "" {
		action = "move_stage"
		var oldStage string
		s.db.QueryRow(`SELECT name FROM lead_stages WHERE id = $1`, old.StageID).Scan(&oldStage)
		desc = fmt.Sprintf("Moved lead from '%s' to '%s'", oldStage, stageName)
	}
	s.logActivity(l.ID, "lead", action, desc)
	return &l, nil
}

func (s *Service) MoveStage(leadID, newStageID string) error {
	var oldStage string
	s.db.QueryRow(`SELECT COALESCE(ls.name, '') FROM leads l LEFT JOIN lead_stages ls ON l.stage_id = ls.id WHERE l.id = $1`, leadID).Scan(&oldStage)

	_, err := s.db.Exec(`UPDATE leads SET stage_id = $1, updated_at = now() WHERE id = $2`, newStageID, leadID)
	if err != nil {
		return err
	}
	var newStage string
	s.db.QueryRow(`SELECT name FROM lead_stages WHERE id = $1`, newStageID).Scan(&newStage)
	desc := fmt.Sprintf("Moved lead from '%s' to '%s'", oldStage, newStage)
	s.logActivity(leadID, "lead", "move_stage", desc)
	return nil
}

func (s *Service) Delete(id string) error {
	_, err := s.db.Exec(`UPDATE leads SET deleted_at = now() WHERE id = $1`, id)
	if err != nil {
		return err
	}
	s.logActivity(id, "lead", "delete", "deleted")
	return nil
}

func (s *Service) logActivity(resourceID, resourceType, action, desc string) {
	if _, err := s.db.Exec(
		`INSERT INTO audit_logs (description, resource_type, resource_id, action) VALUES ($1, $2, $3, $4)`,
		desc, resourceType, resourceID, action,
	); err != nil {
		slog.Error("log activity", "error", err, "resource_type", resourceType, "resource_id", resourceID)
	}
}
