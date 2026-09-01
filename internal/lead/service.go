package lead

import (
	"crm/internal/audit"
	"crm/internal/util"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

var (
	// ErrCustomValueRejected marks lead create/update requests that try to set
	// value directly; the value always comes from the program catalog price.
	ErrCustomValueRejected = errors.New("lead value is set from the program catalog price")
	// ErrProgramNotActive marks program_id values that do not reference a
	// live (non-archived) program.
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
	// ErrClosingStageAtCreate marks lead creation into a closing stage; closing
	// is reachable only by moving an existing lead.
	ErrClosingStageAtCreate = errors.New("a lead cannot be created in a closing stage")
	// ErrNoLostStage marks a close_lost quick reply whose pipeline has no lost
	// closing stage; there is no target to move the lead to.
	ErrNoLostStage = errors.New("no lost closing stage configured in this pipeline")
	// ErrContactNotActive marks a contact_id that does not exist or has been
	// deleted; leads must not link to soft-deleted contacts.
	ErrContactNotActive = errors.New("contact not found or deleted")
	// ErrInvalidContactID marks a contact_id that is not a well-formed UUID, so
	// a malformed value surfaces as a clean 400 instead of a server error.
	ErrInvalidContactID = errors.New("contact_id must be a valid contact reference")
	// ErrInvalidAssignee marks an assigned_to that does not reference a live
	// (non-deleted) user, or that is not a well-formed UUID; deleted users must
	// not remain assignable and malformed values must not surface as server
	// errors.
	ErrInvalidAssignee = errors.New("assigned_to must reference an active user")
	// ErrClosedToClosedMove marks a stage move from one closing stage to
	// another (e.g. lost → won). A closed lead is terminal; a mislabel is fixed
	// by starting a new cycle, not by re-closing the old row.
	ErrClosedToClosedMove = errors.New("a closed lead cannot move to another closing stage")
)

// Service provides database-backed lead operations: list/get/create/update/
// delete, stage moves with history, activities, reminders, and the
// resolve-or-create contact flow for lead entry.
type Service struct {
	db *sql.DB
}

// NewService creates a lead Service backed by db.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// leadSelect is the canonical lead row projection shared by list, board, and
// get. The FROM/joins are appended by each caller (board and get add the
// phone/email laterals unconditionally; list only adds them for searches).
const leadSelect = `
	SELECT l.id, COALESCE(l.nickname, ''), COALESCE(c.name, ''),
		l.contact_id, COALESCE(pcp.value, ''), COALESCE(pce.value, ''),
		l.pipeline_id, l.stage_id, COALESCE(ls.name, ''), COALESCE(ls.outcome, 'open'),
		COALESCE(l.outcome, ''),
		COALESCE(l.lost_reason, ''), l.value,
		l.program_id, COALESCE(p.name, ''), COALESCE(l.notes, ''), l.assigned_to, l.created_at, l.updated_at,
		COALESCE(nt.type, ''), nt.scheduled_at, COALESCE(lt.type, ''), lt.touched_at
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
	LEFT JOIN LATERAL (
		SELECT type, scheduled_at FROM lead_activities
		WHERE lead_id = l.id AND NOT is_done AND NOT is_cancelled AND scheduled_at IS NOT NULL
		ORDER BY scheduled_at ASC LIMIT 1
	) nt ON true
	LEFT JOIN LATERAL (
		SELECT type, COALESCE(occurred_at, responded_at) AS touched_at FROM lead_activities
		WHERE lead_id = l.id AND COALESCE(occurred_at, responded_at) IS NOT NULL
		ORDER BY COALESCE(occurred_at, responded_at) DESC LIMIT 1
	) lt ON true`

// scanLead scans one row produced by leadSelect into a Lead.
func scanLead(scan interface {
	Scan(dest ...any) error
}) (Lead, error) {
	var l Lead
	err := scan.Scan(
		&l.ID, &l.Nickname, &l.ContactName, &l.ContactID, &l.ContactPhone, &l.ContactEmail,
		&l.PipelineID, &l.StageID, &l.StageName, &l.StageOutcome, &l.Outcome, &l.LostReason, &l.Value,
		&l.ProgramID, &l.ProgramName, &l.Notes, &l.AssignedTo, &l.CreatedAt, &l.UpdatedAt,
		&l.NextTaskType, &l.NextTaskAt, &l.LastTouchType, &l.LastTouchAt,
	)
	if err != nil {
		return l, err
	}
	l.DisplayName = l.displayName()
	return l, nil
}

// leadFilters builds the shared lead WHERE clause used by list and board:
// search (nickname, contact name, primary phone/email, program name),
// outcome (the linked stage's outcome), and assigned_to ("none" = unassigned,
// "" = no filter, otherwise a user id). The search clause references the
// pcp/pce laterals, so callers must include them in the FROM when search is
// non-empty.
func leadFilters(search, outcome, assignedTo string) *util.WhereBuilder {
	w := util.NewWhereBuilder("l.deleted_at IS NULL")
	if search != "" {
		pat := util.LikePattern(search)
		w.Add(`(COALESCE(l.nickname, '') ILIKE $? ESCAPE '\' OR c.name ILIKE $? ESCAPE '\' OR pcp.value ILIKE $? ESCAPE '\' OR pce.value ILIKE $? ESCAPE '\' OR COALESCE(p.name, '') ILIKE $? ESCAPE '\')`,
			pat, pat, pat, pat, pat)
	}
	switch outcome {
	case "open", "won", "lost":
		w.Add("ls.outcome = $?", outcome)
	}
	switch assignedTo {
	case "none":
		w.Add("l.assigned_to IS NULL")
	case "":
		// no filter
	default:
		w.Add("l.assigned_to = $?", assignedTo)
	}
	return w
}

// leadSearchFrom is the extra FROM fragment (phone/email laterals) that the
// search clause references; it is appended only when a search is present.
const leadSearchFrom = `
	LEFT JOIN LATERAL (
		SELECT value FROM contact_phones WHERE contact_id = l.contact_id AND is_primary LIMIT 1
	) pcp ON true
	LEFT JOIN LATERAL (
		SELECT value FROM contact_emails WHERE contact_id = l.contact_id AND is_primary LIMIT 1
	) pce ON true`

func (s *Service) list(f ListFilters, page, perPage int) ([]Lead, int, error) {
	w := leadFilters(f.Search, f.Outcome, f.AssignedTo)
	if f.PipelineID != "" {
		w.Add("l.pipeline_id = $?", f.PipelineID)
	}
	if f.StageID != "" {
		w.Add("l.stage_id = $?", f.StageID)
	}
	if f.ContactID != "" {
		w.Add("l.contact_id = $?", f.ContactID)
	}
	whereSQL := w.SQL()

	// The count needs the same joins the where clauses reference (contacts for
	// search names, stage for outcome). The phone/email laterals are only
	// referenced by the search clause, so add them just for searches to keep the
	// near-universal no-search count from paying for per-row correlated lookups.
	countFrom := `FROM leads l
		LEFT JOIN lead_stages ls ON l.stage_id = ls.id
		LEFT JOIN contacts c ON l.contact_id = c.id
		LEFT JOIN programs p ON l.program_id = p.id`
	if f.Search != "" {
		countFrom += leadSearchFrom
	}
	var total int
	err := s.db.QueryRow("SELECT COUNT(*) "+countFrom+" WHERE "+whereSQL, w.Args()...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count leads: %w", err)
	}

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}
	offset := util.Offset(page, perPage)

	limitArg := w.NextArg()
	// leadSelect already carries the full FROM with the phone/email laterals,
	// so the page query needs only the WHERE appended — appending countFrom
	// would duplicate the FROM (syntax error).
	selectQuery := leadSelect + `
		WHERE ` + whereSQL + `
		ORDER BY l.created_at DESC
		LIMIT $` + strconv.Itoa(limitArg) + ` OFFSET $` + strconv.Itoa(limitArg+1)
	rows, err := s.db.Query(selectQuery, append(w.Args(), perPage, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list leads: %w", err)
	}
	defer rows.Close()

	leads := []Lead{}
	for rows.Next() {
		l, err := scanLead(rows)
		if err != nil {
			return nil, 0, err
		}
		leads = append(leads, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list leads: iterate: %w", err)
	}
	return leads, total, nil
}

// board returns the kanban payload: for every stage in the pipeline, the true
// count of live leads in that stage plus only the newest BoardWindow leads.
// Older leads are never deleted, just not rendered; a created_at from/to
// filter brings them back into the window.
func (s *Service) board(f BoardFilters) (*Board, error) {
	w := leadFilters(f.Search, f.Outcome, f.AssignedTo)
	if f.PipelineID != "" {
		w.Add("l.pipeline_id = $?", f.PipelineID)
	}
	if f.From != nil {
		w.Add("l.created_at >= $?", *f.From)
	}
	if f.To != nil {
		w.Add("l.created_at <= $?", *f.To)
	}
	whereSQL := w.SQL()

	// One pass: windowed rows per stage (ROW_NUMBER newest-first, capped at
	// BoardWindow) joined with the true per-stage count. The stage_leads CTE
	// carries the filter joins (contacts, stage, phones/emails, program) so the
	// WHERE can reference them; the outer query adds the display joins.
	filterFrom := `FROM leads l
		LEFT JOIN lead_stages ls ON l.stage_id = ls.id
		LEFT JOIN contacts c ON l.contact_id = c.id
		LEFT JOIN programs p ON l.program_id = p.id`
	if f.Search != "" {
		filterFrom += leadSearchFrom
	}
	rows, err := s.db.Query(`
		WITH stage_leads AS (
			SELECT l.*,
				ROW_NUMBER() OVER (PARTITION BY l.stage_id ORDER BY l.created_at DESC) AS rn
			`+filterFrom+`
			WHERE `+whereSQL+`
		),
		counts AS (
			SELECT stage_id, COUNT(*) AS total FROM stage_leads GROUP BY stage_id
		)
		SELECT sl.stage_id, c.total`+strings.Replace(leadSelect, "\n\tFROM leads l", "\n\tFROM stage_leads sl", 1)+`
		JOIN counts c ON c.stage_id = sl.stage_id
		LEFT JOIN lead_stages ls ON sl.stage_id = ls.id
		LEFT JOIN contacts ct ON sl.contact_id = ct.id
		LEFT JOIN programs p ON sl.program_id = p.id
		LEFT JOIN LATERAL (
			SELECT value FROM contact_phones WHERE contact_id = sl.contact_id AND is_primary LIMIT 1
		) pcp ON true
		LEFT JOIN LATERAL (
			SELECT value FROM contact_emails WHERE contact_id = sl.contact_id AND is_primary LIMIT 1
		) pce ON true
		LEFT JOIN LATERAL (
			SELECT type, scheduled_at FROM lead_activities
			WHERE lead_id = sl.id AND NOT is_done AND NOT is_cancelled AND scheduled_at IS NOT NULL
			ORDER BY scheduled_at ASC LIMIT 1
		) nt ON true
		LEFT JOIN LATERAL (
			SELECT type, COALESCE(occurred_at, responded_at) AS touched_at FROM lead_activities
			WHERE lead_id = sl.id AND COALESCE(occurred_at, responded_at) IS NOT NULL
			ORDER BY COALESCE(occurred_at, responded_at) DESC LIMIT 1
		) lt ON true
		WHERE sl.rn <= `+strconv.Itoa(BoardWindow)+`
		ORDER BY sl.stage_id, sl.created_at DESC`,
		w.Args()...,
	)
	if err != nil {
		return nil, fmt.Errorf("load board: %w", err)
	}
	defer rows.Close()

	stages := map[string]*BoardStage{}
	order := []string{}
	for rows.Next() {
		var stageID string
		var total int
		l, err := scanLead(rows)
		if err != nil {
			return nil, fmt.Errorf("load board: scan: %w", err)
		}
		if _, ok := stages[stageID]; !ok {
			stages[stageID] = &BoardStage{StageID: stageID, Count: total}
			order = append(order, stageID)
		}
		stages[stageID].Leads = append(stages[stageID].Leads, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load board: iterate: %w", err)
	}
	board := &Board{Stages: make([]BoardStage, 0, len(order))}
	for _, id := range order {
		board.Stages = append(board.Stages, *stages[id])
	}
	return board, nil
}

func (s *Service) get(id string) (*Lead, error) {
	l, err := scanLead(s.db.QueryRow(leadSelect+`
		WHERE l.id = $1 AND l.deleted_at IS NULL`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get lead: %w", err)
	}
	return &l, nil
}

// displayName returns the lead's nickname when set, else the contact name.
func (l *Lead) displayName() string {
	if l.Nickname != "" {
		return l.Nickname
	}
	return l.ContactName
}

// validateAssignedToTx rejects assigned_to values that do not reference a live
// (non-deleted) user, so deleted users cannot remain assignable and a
// non-UUID value surfaces as a clean 400 instead of a server error.
func (s *Service) validateAssignedToTx(tx *sql.Tx, assignedTo *string) error {
	if assignedTo == nil || *assignedTo == "" {
		return nil
	}
	if !util.IsUUID(*assignedTo) {
		return ErrInvalidAssignee
	}
	var live bool
	if err := tx.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL)`,
		*assignedTo,
	).Scan(&live); err != nil {
		return fmt.Errorf("validate assigned_to: %w", err)
	}
	if !live {
		return ErrInvalidAssignee
	}
	return nil
}

// spawnCycle starts a new lead row for a closed lead's contact when the user
// drags the closed card back to an open stage. The new row carries the
// contact (always), a fresh program price snapshot, and the nickname; the
// assignee starts unassigned and notes/tasks do not
// carry. The old row stays terminal and untouched.
func (s *Service) spawnCycle(old *Lead, targetStageID, userID string) (*Lead, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("spawn cycle: %w", err)
	}
	defer tx.Rollback()

	// Validate the target stage belongs to the old lead's pipeline.
	if err := s.validateStageForPipelineTx(tx, old.PipelineID, targetStageID); err != nil {
		return nil, err
	}

	// A fresh price snapshot from the program catalog (may be nil if the lead
	// has no program).
	var price *float64
	if old.ProgramID != nil && *old.ProgramID != "" {
		p, err := s.activeProgramPriceTx(tx, *old.ProgramID)
		if err != nil {
			return nil, err
		}
		price = &p
	}

	var l Lead
	err = tx.QueryRow(
		`INSERT INTO leads (nickname, contact_id, pipeline_id, stage_id, program_id, value)
		VALUES (NULLIF($1, ''), $2, $3, $4, NULLIF($5, '')::uuid, $6)
		RETURNING id, COALESCE(nickname, ''), contact_id, pipeline_id, stage_id,
			(SELECT COALESCE(outcome, 'open') FROM lead_stages WHERE id = leads.stage_id),
			COALESCE(outcome, ''), COALESCE(lost_reason, ''),
			program_id, value, COALESCE(notes, ''), assigned_to, created_at, updated_at`,
		old.Nickname, old.ContactID, old.PipelineID, targetStageID, old.ProgramID, price,
	).Scan(
		&l.ID, &l.Nickname, &l.ContactID, &l.PipelineID, &l.StageID, &l.StageOutcome, &l.Outcome, &l.LostReason,
		&l.ProgramID, &l.Value, &l.Notes, &l.AssignedTo, &l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("spawn cycle: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("spawn cycle: commit: %w", err)
	}

	if err := s.populateNames(&l); err != nil {
		return nil, err
	}
	l.DisplayName = l.displayName()
	// Audit with the closed lead's display name, not its UUID, so the log
	// reads like every other audit description.
	name := old.DisplayName
	if name == "" {
		name = old.ID
	}
	s.logActivity(l.ID, "lead", "create", fmt.Sprintf("Started new cycle from closed lead %q", name), userID)
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

	// Resolve-or-create the backing contact.
	contactID, err := s.resolveOrCreateContactTx(tx, req.ContactID, req.NewContact, userID)
	if err != nil {
		return nil, err
	}

	if err := s.validateAssignedToTx(tx, req.AssignedTo); err != nil {
		return nil, err
	}

	if err := s.validateStageForPipelineTx(tx, req.PipelineID, req.StageID); err != nil {
		return nil, err
	}
	// Closing stages are unreachable at create: a lead must be moved into them
	// so the stage history records the move.
	info, err := s.stageInfoTx(tx, req.StageID)
	if err != nil {
		return nil, err
	}
	if info.IsClosing {
		return nil, ErrClosingStageAtCreate
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
			(SELECT COALESCE(outcome, 'open') FROM lead_stages WHERE id = leads.stage_id),
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
		&l.ID, &l.Nickname, &l.ContactID, &l.PipelineID, &l.StageID, &l.StageOutcome, &l.ProgramID,
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
	s.logActivity(l.ID, "lead", "create", fmt.Sprintf("Created lead %q", l.DisplayName), userID)
	return &l, nil
}

// resolveOrCreateContactTx links the lead to the contact_id when provided, else
// resolves an existing contact by phone (primary) / email (secondary) from the
// new_contact details, creating a new contact in the same transaction when none
// matches. This is the "contact is the single source of truth" entry flow.
func (s *Service) resolveOrCreateContactTx(tx *sql.Tx, contactID *string, nc *NewContact, userID string) (string, error) {
	if contactID != nil && *contactID != "" {
		// Reject deleted or unknown contacts so a lead can never be attached
		// to a soft-deleted contact's row. Malformed
		// ids are rejected up front so a non-UUID value cannot surface as a
		// Postgres cast error.
		if !util.IsUUID(*contactID) {
			return "", ErrInvalidContactID
		}
		var live bool
		err := tx.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM contacts WHERE id = $1 AND deleted_at IS NULL)`,
			*contactID,
		).Scan(&live)
		if err != nil {
			return "", fmt.Errorf("validate contact: %w", err)
		}
		if !live {
			return "", ErrContactNotActive
		}
		return *contactID, nil
	}
	if nc == nil || nc.Name == "" {
		return "", ErrContactRequired
	}
	if nc.Phone == "" && nc.Email == "" {
		return "", ErrNoContactDetail
	}

	// Serialize concurrent lead entries that could create a duplicate contact
	// for the same phone/email. The lock is namespaced by the lookup key and
	// released automatically at commit/rollback.
	key := util.NormalizePhone(nc.Phone)
	if key == "" {
		key = util.NormalizeEmail(nc.Email)
	}
	if key != "" {
		if _, err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtext($1))`, "lead_resolve:"+key); err != nil {
			return "", fmt.Errorf("lock contact resolve: %w", err)
		}
	}

	// Phone primary, email secondary. The child tables store the raw
	// value for display, so the lookup normalizes both sides: the incoming value
	// is stripped to digits and matched against the stored value with the same
	// transformation. Any phone/email on a contact counts as a match (not just
	// the primary), so an alternate number still resolves to the contact.
	if phone := util.NormalizePhone(nc.Phone); phone != "" {
		var found string
		err := tx.QueryRow(
			`SELECT cp.contact_id FROM contact_phones cp
			JOIN contacts c ON c.id = cp.contact_id AND c.deleted_at IS NULL
			WHERE regexp_replace(cp.value, '\D', '', 'g') IN ($1, '91' || $1) LIMIT 1`,
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
			`SELECT ce.contact_id FROM contact_emails ce
			JOIN contacts c ON c.id = ce.contact_id AND c.deleted_at IS NULL
			WHERE lower(trim(ce.value)) = $1 LIMIT 1`,
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
		`INSERT INTO contacts (name) VALUES ($1) RETURNING id`,
		nc.Name,
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

	// Audit the contact creation inside the same transaction. Best-effort: a
	// failed audit must never roll back the business mutation.
	userName := ""
	if userID != "" {
		_ = tx.QueryRow(`SELECT name FROM users WHERE id = $1`, userID).Scan(&userName)
	}
	if _, err := tx.Exec(
		`INSERT INTO audit_logs (description, resource_type, resource_id, action, user_id, user_name)
		VALUES ($1, 'contact', $2, 'create', NULLIF($3, '')::uuid, NULLIF($4, ''))`,
		"Contact created from lead entry", id, userID, userName,
	); err != nil {
		slog.Warn("log contact create from lead", "error", err)
	}

	return id, nil
}

func (s *Service) update(id string, req UpdateRequest, userID string) (*Lead, error) {
	if req.Value != nil {
		return nil, ErrCustomValueRejected
	}
	old, err := s.get(id)
	if err != nil {
		return nil, fmt.Errorf("update lead: load current: %w", err)
	}

	// A closed lead (one in a closing stage) is terminal. Dragging it to an
	// open stage must not mutate the row — it spawns a new lead row for the
	// same contact in the target stage, carrying the contact, a fresh program
	// price snapshot, and the nickname; the assignee starts unassigned and
	// notes/tasks do not carry.
	// Moving a closed lead into another closing stage (lost → won) is rejected:
	// a mislabel is fixed by a new cycle, not by re-closing the old row.
	if req.StageID != nil && *req.StageID != "" && *req.StageID != old.StageID &&
		old.StageOutcome != "open" {
		// The target stage's closing flag decides: closed → open spawns a new
		// cycle; closed → closed is rejected.
		var targetClosing bool
		if err := s.db.QueryRow(
			`SELECT is_closing FROM lead_stages WHERE id = $1`,
			*req.StageID,
		).Scan(&targetClosing); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrStageNotInPipeline
			}
			return nil, fmt.Errorf("update lead: load target stage: %w", err)
		}
		if targetClosing {
			return nil, ErrClosedToClosedMove
		}
		return s.spawnCycle(old, *req.StageID, userID)
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
		// The lead keeps its current stage; verify it still belongs to the
		// pipeline the client is switching the lead into.
		if err := tx.QueryRow(
			`SELECT id FROM lead_stages WHERE pipeline_id = $1 AND id = $2`,
			*req.PipelineID, old.StageID,
		).Scan(new(string)); err != nil {
			return nil, ErrStageNotInPipeline
		}
	}
	if req.ContactID != nil && *req.ContactID != "" {
		if !util.IsUUID(*req.ContactID) {
			return nil, ErrInvalidContactID
		}
		var live bool
		err := tx.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM contacts WHERE id = $1 AND deleted_at IS NULL)`,
			*req.ContactID,
		).Scan(&live)
		if err != nil {
			return nil, fmt.Errorf("update lead: validate contact: %w", err)
		}
		if !live {
			return nil, ErrContactNotActive
		}
	}
	if err := s.validateAssignedToTx(tx, req.AssignedTo); err != nil {
		return nil, err
	}

	// Resolve the outcome when the lead moves into or out of a closing stage.
	// outcome is set from the target stage's declared outcome, so 'won' and
	// 'lost' come from stage metadata rather than
	// matching stage names by text; lost_reason may be supplied by the caller
	// when the lead is marked lost.
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
			// The stage declares its outcome ('won' or 'lost'); both are valid.
			// The pipeline package's stageOutcome enforces that closing stages
			// are never 'open', but guard anyway for malformed data.
			if info.Outcome == "" || info.Outcome == "open" {
				info.Outcome = "lost"
			}
			outcome = &info.Outcome
			if req.LostReason != nil && info.Outcome == "lost" {
				lostReason = req.LostReason
			}
		}
		// Moving out of a closing stage never reaches here: the top-of-update
		// check spawns a new cycle or rejects the move.
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
			(SELECT COALESCE(outcome, 'open') FROM lead_stages WHERE id = leads.stage_id),
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
		&l.ID, &l.Nickname, &l.ContactID, &l.PipelineID, &l.StageID, &l.StageOutcome, &l.Outcome, &l.LostReason,
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
		// Reaching a closing stage resolves the deal: cancel every open task so
		// reminders stop nagging on won/lost leads.
		if targetStage.IsClosing {
			if _, err := tx.Exec(
				`UPDATE lead_activities SET is_cancelled = true
				WHERE lead_id = $1 AND NOT is_done AND NOT is_cancelled`,
				id,
			); err != nil {
				return nil, fmt.Errorf("cancel open tasks: %w", err)
			}
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
		oldName, newName := old.ContactName, l.ContactName
		if oldName == "" {
			oldName = old.ContactID
		}
		if newName == "" {
			newName = l.ContactID
		}
		desc = fmt.Sprintf("Reassigned contact from %q to %q", oldName, newName)
	}
	s.logActivity(l.ID, "lead", action, desc, userID)
	return &l, nil
}

func (s *Service) delete(id string, userID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("delete lead: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.Exec(`UPDATE leads SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("delete lead: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete lead: rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	// Cancel open tasks in the same transaction, mirroring the close path, so
	// a soft-deleted lead stops generating reminders.
	if _, err := tx.Exec(
		`UPDATE lead_activities SET is_cancelled = true
		WHERE lead_id = $1 AND NOT is_done AND NOT is_cancelled`,
		id,
	); err != nil {
		return fmt.Errorf("delete lead: cancel open tasks: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit lead delete: %w", err)
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
		return fmt.Errorf("populate names: load stage name: %w", err)
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
	// Outcome is the stage's declared outcome ('open' | 'won' | 'lost'),
	// authoritative for lead outcome resolution.
	Outcome string
}

// stageInfoTx loads a stage's name, closing flag, and outcome inside a
// transaction so the outcome resolution and history insert see a consistent
// view.
func (s *Service) stageInfoTx(tx *sql.Tx, stageID string) (*stageInfo, error) {
	var info stageInfo
	err := tx.QueryRow(
		`SELECT id, name, is_closing, outcome FROM lead_stages WHERE id = $1`,
		stageID,
	).Scan(&info.ID, &info.Name, &info.IsClosing, &info.Outcome)
	if err != nil {
		return nil, fmt.Errorf("load stage info: %w", err)
	}
	return &info, nil
}

// closeLostTx executes a close_lost quick reply inside a transaction: the
// lead moves to its pipeline's lost closing stage, outcome resolves to
// 'lost', open tasks are cancelled, and the move is recorded in stage
// history. It returns false (no move) when the lead already sits in the
// target stage, and ErrNoLostStage when the pipeline has no lost closing
// stage. The outcome rule mirrors update()'s: closing stages that carry the
// column default 'open' count as lost.
func (s *Service) closeLostTx(tx *sql.Tx, leadID, userID string) (bool, error) {
	var pipelineID, currentStageID string
	if err := tx.QueryRow(
		`SELECT pipeline_id, stage_id FROM leads WHERE id = $1 AND deleted_at IS NULL`,
		leadID,
	).Scan(&pipelineID, &currentStageID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrNotFound
		}
		return false, fmt.Errorf("close lost: load lead: %w", err)
	}

	var target stageInfo
	err := tx.QueryRow(
		`SELECT id, name, is_closing, outcome FROM lead_stages
		WHERE pipeline_id = $1 AND is_closing AND outcome <> 'won'
		ORDER BY "order" ASC, created_at ASC
		LIMIT 1`,
		pipelineID,
	).Scan(&target.ID, &target.Name, &target.IsClosing, &target.Outcome)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNoLostStage
	}
	if err != nil {
		return false, fmt.Errorf("close lost: find lost stage: %w", err)
	}
	if target.ID == currentStageID {
		return false, nil
	}

	outcome := target.Outcome
	if outcome == "" || outcome == "open" {
		outcome = "lost"
	}
	if _, err := tx.Exec(
		`UPDATE leads SET stage_id = $2, outcome = $3, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`,
		leadID, target.ID, outcome,
	); err != nil {
		return false, fmt.Errorf("close lost: move lead: %w", err)
	}

	var fromStageName string
	if err := tx.QueryRow(`SELECT name FROM lead_stages WHERE id = $1`, currentStageID).Scan(&fromStageName); err != nil {
		return false, fmt.Errorf("close lost: load current stage name: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO lead_stage_history (lead_id, from_stage_id, to_stage_id, from_stage_name, to_stage_name, user_id)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::uuid)`,
		leadID, currentStageID, target.ID, fromStageName, target.Name, userID,
	); err != nil {
		return false, fmt.Errorf("close lost: record stage history: %w", err)
	}

	// Reaching a closing stage resolves the deal: cancel every open task so
	// reminders stop nagging on lost leads.
	if _, err := tx.Exec(
		`UPDATE lead_activities SET is_cancelled = true
		WHERE lead_id = $1 AND NOT is_done AND NOT is_cancelled`,
		leadID,
	); err != nil {
		return false, fmt.Errorf("close lost: cancel open tasks: %w", err)
	}
	return true, nil
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
	audit.LogCustom(s.db, desc, resourceType, resourceID, action, "", userID)
}
