package lead

import (
	"crm/internal/util"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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
	// ErrContactRequired marks lead creation with neither a contact_id nor a
	// new_contact payload — a lead must reference a contact.
	ErrContactRequired = errors.New("a lead must reference a contact")
	// ErrNoContactDetail marks a new_contact with neither a phone nor an email.
	ErrNoContactDetail = errors.New("new contact must have at least one phone or one email")
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
		`SELECT l.id, COALESCE(l.nickname, ''), COALESCE(c.name, ''),
			l.contact_id, COALESCE(pcp.value, ''), COALESCE(pce.value, ''),
			l.pipeline_id, l.stage_id, COALESCE(ls.name, ''), COALESCE(l.outcome, ''),
			COALESCE(l.lost_reason, ''), l.value,
			l.program_id, COALESCE(p.name, ''), COALESCE(l.notes, ''), l.assigned_to, l.created_at, l.updated_at
		FROM leads l
		LEFT JOIN lead_stages ls ON l.stage_id = ls.id
		LEFT JOIN contacts c ON l.contact_id = c.id
		LEFT JOIN programs p ON l.program_id = p.id
		LEFT JOIN LATERAL (
			SELECT value FROM contact_phones WHERE contact_id = l.contact_id AND is_primary LIMIT 1
		) pcp ON true
		LEFT JOIN LATERAL (
			SELECT value FROM contact_emails WHERE contact_id = l.contact_id AND is_primary LIMIT 1
		) pce ON true
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
			&l.ID, &l.Nickname, &l.ContactName, &l.ContactID, &l.ContactPhone, &l.ContactEmail,
			&l.PipelineID, &l.StageID, &l.StageName, &l.Outcome, &l.LostReason, &l.Value, &l.ProgramID, &l.ProgramName,
			&l.Notes, &l.AssignedTo, &l.CreatedAt, &l.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		l.DisplayName = l.displayName()
		leads = append(leads, l)
	}
	return leads, total, nil
}

func (s *Service) get(id string) (*Lead, error) {
	var l Lead
	err := s.db.QueryRow(
		`SELECT l.id, COALESCE(l.nickname, ''), COALESCE(c.name, ''),
			l.contact_id, COALESCE(pcp.value, ''), COALESCE(pce.value, ''),
			l.pipeline_id, l.stage_id, COALESCE(ls.name, ''), COALESCE(l.outcome, ''),
			COALESCE(l.lost_reason, ''), l.value,
			l.program_id, COALESCE(p.name, ''), COALESCE(l.notes, ''), l.assigned_to, l.created_at, l.updated_at
		FROM leads l
		LEFT JOIN lead_stages ls ON l.stage_id = ls.id
		LEFT JOIN contacts c ON l.contact_id = c.id
		LEFT JOIN programs p ON l.program_id = p.id
		LEFT JOIN LATERAL (
			SELECT value FROM contact_phones WHERE contact_id = l.contact_id AND is_primary LIMIT 1
		) pcp ON true
		LEFT JOIN LATERAL (
			SELECT value FROM contact_emails WHERE contact_id = l.contact_id AND is_primary LIMIT 1
		) pce ON true
		WHERE l.id = $1 AND l.deleted_at IS NULL`, id,
	).Scan(
		&l.ID, &l.Nickname, &l.ContactName, &l.ContactID, &l.ContactPhone, &l.ContactEmail,
		&l.PipelineID, &l.StageID, &l.StageName, &l.Outcome, &l.LostReason, &l.Value, &l.ProgramID, &l.ProgramName,
		&l.Notes, &l.AssignedTo, &l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	l.DisplayName = l.displayName()
	return &l, nil
}

// displayName returns the lead's nickname when set, else the contact name.
func (l *Lead) displayName() string {
	if l.Nickname != "" {
		return l.Nickname
	}
	return l.ContactName
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

	// Resolve-or-create the backing contact.
	contactID, err := s.resolveOrCreateContactTx(tx, req.ContactID, req.NewContact)
	if err != nil {
		return nil, err
	}

	if err := s.validateStageForPipelineTx(tx, req.PipelineID, req.StageID); err != nil {
		return nil, err
	}
	programPrice, err := s.snapshotPriceTx(tx, req.ProgramID)
	if err != nil {
		return nil, err
	}
	var l Lead
	err = tx.QueryRow(
		`INSERT INTO leads (nickname, contact_id, pipeline_id, stage_id, program_id, value, notes, assigned_to)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, COALESCE(nickname, ''), contact_id, pipeline_id, stage_id,
			program_id, value, COALESCE(notes, ''), assigned_to, created_at, updated_at`,
		util.NullStr(req.Nickname),
		contactID,
		req.PipelineID,
		req.StageID,
		util.NullPtr(req.ProgramID),
		programPrice,
		util.NullStr(req.Notes),
		util.NullPtr(req.AssignedTo),
	).Scan(
		&l.ID, &l.Nickname, &l.ContactID, &l.PipelineID, &l.StageID, &l.ProgramID,
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
	l.DisplayName = l.displayName()
	s.logActivity(l.ID, "lead", "create", "", userID)
	return &l, nil
}

// resolveOrCreateContactTx links the lead to the contact_id when provided, else
// resolves an existing contact by phone (primary) / email (secondary) from the
// new_contact details, creating a new contact in the same transaction when none
// matches. This is the "contact is the single source of truth" entry flow.
func (s *Service) resolveOrCreateContactTx(tx *sql.Tx, contactID *string, nc *NewContact) (string, error) {
	if contactID != nil && *contactID != "" {
		return *contactID, nil
	}
	if nc == nil || nc.Name == "" {
		return "", ErrContactRequired
	}
	if nc.Phone == "" && nc.Email == "" {
		return "", ErrNoContactDetail
	}

	// Phone primary, email secondary (ADR 009). The child tables store the raw
	// value for display, so the lookup normalizes both sides: the incoming value
	// is stripped to digits and matched against the stored value with the same
	// transformation. Any phone/email on a contact counts as a match (not just
	// the primary), so an alternate number still resolves to the contact.
	if phone := util.NormalizePhone(nc.Phone); phone != "" {
		var found string
		err := tx.QueryRow(
			`SELECT contact_id FROM contact_phones WHERE regexp_replace(value, '\D', '', 'g') = $1 LIMIT 1`,
			phone,
		).Scan(&found)
		if err == nil {
			return found, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("resolve contact by phone: %w", err)
		}
	}
	if email := util.NormalizeEmail(nc.Email); email != "" {
		var found string
		err := tx.QueryRow(
			`SELECT contact_id FROM contact_emails WHERE lower(trim(value)) = $1 LIMIT 1`,
			email,
		).Scan(&found)
		if err == nil {
			return found, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("resolve contact by email: %w", err)
		}
	}

	// No match — create the contact and link the lead in the same transaction.
	var id string
	err := tx.QueryRow(
		`INSERT INTO contacts (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		nc.Name, util.NullStr(nc.Email), util.NullStr(nc.Phone),
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create contact from lead: %w", err)
	}
	if nc.Phone != "" {
		if _, err := tx.Exec(
			`INSERT INTO contact_phones (contact_id, value, is_primary) VALUES ($1, $2, true)`,
			id, nc.Phone,
		); err != nil {
			return "", fmt.Errorf("insert lead contact phone: %w", err)
		}
	}
	if nc.Email != "" {
		if _, err := tx.Exec(
			`INSERT INTO contact_emails (contact_id, value, is_primary) VALUES ($1, $2, true)`,
			id, nc.Email,
		); err != nil {
			return "", fmt.Errorf("insert lead contact email: %w", err)
		}
	}
	return id, nil
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

	// Resolve the outcome when the lead moves into or out of a closing stage.
	// outcome is set by the system on stage move (won/lost); lost_reason may be
	// supplied by the caller when the lead is marked lost.
	var outcome *string
	var lostReason *string
	var targetStage *stageInfo
	if req.StageID != nil && *req.StageID != "" && *req.StageID != old.StageID {
		info, err := s.stageInfoTx(tx, *req.StageID)
		if err != nil {
			return nil, err
		}
		targetStage = info
		if info.IsClosing {
			out := "lost"
			if strings.Contains(strings.ToLower(info.Name), "won") || strings.EqualFold(info.Name, "Converted") {
				out = "won"
			}
			outcome = &out
			if req.LostReason != nil && out == "lost" {
				lostReason = req.LostReason
			}
		} else {
			// Moving out of a closing stage clears the outcome.
			empty := ""
			outcome = &empty
			lostReason = &empty
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
			nickname = CASE WHEN $2::text IS NOT NULL THEN NULLIF($2::text, '') ELSE nickname END,
			contact_id = CASE WHEN $3::text IS NOT NULL THEN NULLIF($3::text, '')::uuid ELSE contact_id END,
			pipeline_id = COALESCE($4::uuid, pipeline_id),
			stage_id = COALESCE($5::uuid, stage_id),
			outcome = CASE WHEN $6::text IS NOT NULL THEN NULLIF($6::text, '') ELSE outcome END,
			lost_reason = CASE WHEN $7::text IS NOT NULL THEN NULLIF($7::text, '') ELSE lost_reason END,
			program_id = CASE WHEN $8::text IS NOT NULL THEN NULLIF($8::text, '')::uuid ELSE program_id END,
			value = CASE WHEN $8::text IS NOT NULL AND NULLIF($8::text, '') IS NULL THEN NULL ELSE COALESCE($9, value) END,
			notes = CASE WHEN $10::text IS NOT NULL THEN NULLIF($10::text, '') ELSE notes END,
			assigned_to = CASE WHEN $11::text IS NOT NULL THEN NULLIF($11::text, '')::uuid ELSE assigned_to END,
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, COALESCE(nickname, ''), contact_id, pipeline_id, stage_id,
			COALESCE(outcome, ''), COALESCE(lost_reason, ''),
			program_id, value, COALESCE(notes, ''), assigned_to, created_at, updated_at`,
		id,
		req.Nickname,
		req.ContactID,
		req.PipelineID,
		req.StageID,
		outcome,
		lostReason,
		req.ProgramID,
		programPrice,
		req.Notes,
		req.AssignedTo,
	).Scan(
		&l.ID, &l.Nickname, &l.ContactID, &l.PipelineID, &l.StageID, &l.Outcome, &l.LostReason,
		&l.ProgramID, &l.Value, &l.Notes, &l.AssignedTo, &l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update lead: %w", err)
	}

	// Record the stage move in history (same transaction, before commit).
	if targetStage != nil {
		if _, err := tx.Exec(
			`INSERT INTO lead_stage_history (lead_id, from_stage_id, to_stage_id, from_stage_name, to_stage_name, user_id)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::uuid)`,
			id, old.StageID, l.StageID, old.StageName, targetStage.Name, userID,
		); err != nil {
			return nil, fmt.Errorf("record stage history: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit lead update: %w", err)
	}
	if err := s.populateNames(&l); err != nil {
		return nil, err
	}
	l.DisplayName = l.displayName()

	action := "update"
	desc := "Updated lead"
	if old.StageID != l.StageID && old.StageID != "" {
		action = "move_stage"
		oldStage, err := s.stageName(old.StageID)
		if err != nil {
			return nil, err
		}
		desc = fmt.Sprintf("Moved lead from %q to %q", oldStage, l.StageName)
	} else if old.ContactID != "" && old.ContactID != l.ContactID {
		desc = fmt.Sprintf("Reassigned contact from %q to %q", old.ContactID, l.ContactID)
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
	if l.ContactID == "" {
		return nil
	}
	var contactName, contactPhone, contactEmail string
	err = s.db.QueryRow(
		`SELECT COALESCE(c.name, ''), COALESCE(pcp.value, ''), COALESCE(pce.value, '')
		FROM contacts c
		LEFT JOIN LATERAL (
			SELECT value FROM contact_phones WHERE contact_id = c.id AND is_primary LIMIT 1
		) pcp ON true
		LEFT JOIN LATERAL (
			SELECT value FROM contact_emails WHERE contact_id = c.id AND is_primary LIMIT 1
		) pce ON true
		WHERE c.id = $1`,
		l.ContactID,
	).Scan(&contactName, &contactPhone, &contactEmail)
	if err != nil {
		return fmt.Errorf("load contact name: %w", err)
	}
	l.ContactName = contactName
	l.ContactPhone = contactPhone
	l.ContactEmail = contactEmail
	return nil
}

// snapshotPriceTx resolves the catalog price of an optional program so it can
// be stored as the lead's immutable value snapshot. The program row is locked
// for the duration of the transaction so a concurrent archive cannot slip in
// between the check and the lead insert.
func (s *Service) snapshotPriceTx(tx *sql.Tx, programID *string) (*float64, error) {
	if programID == nil || *programID == "" {
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
	err := s.db.QueryRow(`SELECT name FROM lead_stages WHERE id = $1`, stageID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		// The stage row no longer exists (e.g. an old pipeline stage removed by
		// migration). Callers use the name only for display/history; an empty
		// name is safer than failing the whole update.
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("load stage name: %w", err)
	}
	return name, nil
}

// stageInfo is the minimal stage shape needed for outcome resolution and
// history snapshots.
type stageInfo struct {
	ID        string
	Name      string
	IsClosing bool
}

// stageInfoTx loads a stage's name and closing flag inside a transaction so
// the outcome resolution and history insert see a consistent view.
func (s *Service) stageInfoTx(tx *sql.Tx, stageID string) (*stageInfo, error) {
	var info stageInfo
	err := tx.QueryRow(
		`SELECT id, name, is_closing FROM lead_stages WHERE id = $1`,
		stageID,
	).Scan(&info.ID, &info.Name, &info.IsClosing)
	if err != nil {
		return nil, fmt.Errorf("load stage info: %w", err)
	}
	return &info, nil
}

// listHistory returns the chronological stage moves for a lead, oldest first.
func (s *Service) listHistory(leadID string) ([]StageHistory, error) {
	rows, err := s.db.Query(
		`SELECT id, lead_id, from_stage_id, to_stage_id,
			COALESCE(from_stage_name, ''), COALESCE(to_stage_name, ''),
			user_id, moved_at
		FROM lead_stage_history
		WHERE lead_id = $1
		ORDER BY moved_at ASC`,
		leadID,
	)
	if err != nil {
		return nil, fmt.Errorf("list stage history: %w", err)
	}
	defer rows.Close()
	history := []StageHistory{}
	for rows.Next() {
		var h StageHistory
		var fromStageID, toStageID sql.NullString
		if err := rows.Scan(
			&h.ID, &h.LeadID, &fromStageID, &toStageID,
			&h.FromStageName, &h.ToStageName, &h.UserID, &h.MovedAt,
		); err != nil {
			return nil, err
		}
		h.FromStageID = fromStageID.String
		h.ToStageID = toStageID.String
		history = append(history, h)
	}
	return history, rows.Err()
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
		VALUES ($1, $2, $3, $4, NULLIF($5, '')::uuid, NULLIF($6, ''))`,
		desc, resourceType, resourceID, action, userID, userName,
	); err != nil {
		slog.Error("log activity", "error", err, "resource_type", resourceType, "resource_id", resourceID)
	}
}
