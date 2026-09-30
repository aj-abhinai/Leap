package lead

import (
	"crm/internal/audit"
	"crm/internal/settings"
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
	ErrProgramNotActive = errors.New("program not found or archived")
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
	// ErrLeadClosed marks writes that would create or reactivate working state
	// on a lead sitting in a closing stage: a new task, or an edit that puts a
	// task back into working state. Record-only edits to existing tasks stay
	// allowed on a terminal deal.
	ErrLeadClosed = errors.New("a closed lead does not accept new or reactivated tasks")
	// ErrSpawnOnlyStage marks a reopen of a closed lead that carries fields
	// besides stage_id. The new cycle copies the contact, program, and
	// nickname from the closed row by design; sibling edits are refused
	// instead of silently dropped.
	ErrSpawnOnlyStage = errors.New("reopening a closed lead accepts only stage_id")
)

// OpenLeadConflictError refuses a write that would open a second lead in an
// occupied (contact, pipeline, program) slot. Lead carries the open lead that
// already holds the slot so the UI can offer the resolve-or-log path without
// a second fetch.
type OpenLeadConflictError struct {
	Lead OpenLeadRef
}

// Error returns the human-readable refusal message.
func (e *OpenLeadConflictError) Error() string {
	return "an open lead already exists for this contact, pipeline, and program"
}

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
	SELECT l.id, COALESCE(l.nickname, ''),
		CASE WHEN c.deleted_at IS NOT NULL THEN 'Deleted contact' ELSE COALESCE(c.name, '') END,
		l.contact_id,
		CASE WHEN c.deleted_at IS NULL THEN COALESCE(pcp.value, '') ELSE '' END,
		CASE WHEN c.deleted_at IS NULL THEN COALESCE(pce.value, '') ELSE '' END,
		l.pipeline_id, l.stage_id, COALESCE(ls.name, ''), COALESCE(ls.outcome, 'open'),
		COALESCE(l.outcome, ''),
		COALESCE(l.lost_reason, ''), l.value,
		l.program_id, COALESCE(p.name, ''), COALESCE(l.notes, ''), l.assigned_to, l.created_at, l.updated_at,
		COALESCE(nt.type, ''), nt.scheduled_at, nt.scheduled_end_at, COALESCE(lt.type, ''), lt.touched_at
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
		SELECT type, scheduled_at, scheduled_end_at FROM lead_activities
		WHERE lead_id = l.id AND NOT is_done AND NOT is_cancelled AND scheduled_at IS NOT NULL
		ORDER BY scheduled_at ASC LIMIT 1
	) nt ON true
	LEFT JOIN LATERAL (
		SELECT type, COALESCE(occurred_at, responded_at) AS touched_at FROM lead_activities
		WHERE lead_id = l.id AND COALESCE(occurred_at, responded_at) IS NOT NULL
		ORDER BY COALESCE(occurred_at, responded_at) DESC LIMIT 1
	) lt ON true`

// scanLead scans one row produced by leadSelect (or a prefix of extra columns
// followed by the 24 lead columns, as the board query does) into a Lead.
func scanLead(scan interface {
	Scan(dest ...any) error
}, prefix ...any) (Lead, error) {
	var l Lead
	dests := append(prefix,
		&l.ID, &l.Nickname, &l.ContactName, &l.ContactID, &l.ContactPhone, &l.ContactEmail,
		&l.PipelineID, &l.StageID, &l.StageName, &l.StageOutcome, &l.Outcome, &l.LostReason, &l.Value,
		&l.ProgramID, &l.ProgramName, &l.Notes, &l.AssignedTo, &l.CreatedAt, &l.UpdatedAt,
		&l.NextTaskType, &l.NextTaskAt, &l.NextTaskEndAt, &l.LastTouchType, &l.LastTouchAt,
	)
	err := scan.Scan(dests...)
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
		w.Add(`(COALESCE(l.nickname, '') ILIKE $? ESCAPE '\'
			OR (c.deleted_at IS NULL AND c.name ILIKE $? ESCAPE '\')
			OR (c.deleted_at IS NULL AND pcp.value ILIKE $? ESCAPE '\')
			OR (c.deleted_at IS NULL AND pce.value ILIKE $? ESCAPE '\')
			OR COALESCE(p.name, '') ILIKE $? ESCAPE '\')`,
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

// leadBaseFrom is the plain joins shared by every lead query: the stage
// (for outcome/name), the contact (for display name) and the program. It is
// the FROM of the outer board query (over stage_leads) as well as the CTE.
const leadBaseFrom = `
	LEFT JOIN lead_stages ls ON l.stage_id = ls.id
	LEFT JOIN contacts c ON l.contact_id = c.id
	LEFT JOIN programs p ON l.program_id = p.id`

// leadSearchFrom is the extra FROM fragment (phone/email laterals) that the
// search clause references; it is appended only when a search is present.
const leadSearchFrom = `
	LEFT JOIN LATERAL (
		SELECT value FROM contact_phones WHERE contact_id = l.contact_id AND is_primary LIMIT 1
	) pcp ON true
	LEFT JOIN LATERAL (
		SELECT value FROM contact_emails WHERE contact_id = l.contact_id AND is_primary LIMIT 1
	) pce ON true`

// leadSelectOuter is the board query's column list over the stage_leads CTE
// (aliased sl, contact aliased ct); it joins the leading stage_id/count
// columns, and the outer FROM/joins follow it.
const leadSelectOuter = `
	sl.id, COALESCE(sl.nickname, ''),
		CASE WHEN ct.deleted_at IS NOT NULL THEN 'Deleted contact' ELSE COALESCE(ct.name, '') END,
		sl.contact_id,
		CASE WHEN ct.deleted_at IS NULL THEN COALESCE(pcp.value, '') ELSE '' END,
		CASE WHEN ct.deleted_at IS NULL THEN COALESCE(pce.value, '') ELSE '' END,
		sl.pipeline_id, sl.stage_id, COALESCE(ls.name, ''), COALESCE(ls.outcome, 'open'),
		COALESCE(sl.outcome, ''),
		COALESCE(sl.lost_reason, ''), sl.value,
		sl.program_id, COALESCE(p.name, ''), COALESCE(sl.notes, ''), sl.assigned_to, sl.created_at, sl.updated_at,
		COALESCE(nt.type, ''), nt.scheduled_at, nt.scheduled_end_at, COALESCE(lt.type, ''), lt.touched_at`

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
	filterFrom := `FROM leads l` + leadBaseFrom
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
		SELECT sl.stage_id, c.total,`+leadSelectOuter+`
		FROM stage_leads sl
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
			SELECT type, scheduled_at, scheduled_end_at FROM lead_activities
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
		l, err := scanLead(rows, &stageID, &total)
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

// getForUpdateTx loads a lead inside a transaction with the row locked, so
// concurrent lead writes serialize and every decision reads the state the
// write will replace. It returns ErrNotFound when the lead is missing or
// soft-deleted.
func (s *Service) getForUpdateTx(tx *sql.Tx, id string) (*Lead, error) {
	l, err := scanLead(tx.QueryRow(leadSelect+`
		WHERE l.id = $1 AND l.deleted_at IS NULL
		FOR UPDATE OF l`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get lead for update: %w", err)
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

// spawnCycleTx starts a new lead row for a closed lead's contact when the user
// reopens the deal. The new row carries the contact (always), a fresh program
// price snapshot, and the nickname; the assignee starts unassigned and
// notes/tasks do not carry. The old row stays terminal and untouched. The
// caller commits and then calls afterSpawn for the display names and audit row.
func (s *Service) spawnCycleTx(tx *sql.Tx, old *Lead, targetStageID string) (*Lead, error) {
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

	// The reopened lead occupies the same (contact, pipeline, program) slot as
	// the closed one it replaces; refuse when another open lead already holds
	// it.
	if err := s.refuseOpenLeadConflictTx(tx, old.ContactID, old.PipelineID, old.ProgramID, ""); err != nil {
		return nil, err
	}

	var l Lead
	err := tx.QueryRow(
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
	return &l, nil
}

// afterSpawn resolves the new cycle's display names and writes the spawn audit
// row naming the closed lead it replaces. The cycle is already committed, so an
// enrichment failure is logged without undoing it.
func (s *Service) afterSpawn(l, old *Lead, userID string) error {
	if err := s.populateNames(l); err != nil {
		// The new cycle is committed; a display-name read must not prevent the
		// spawn audit row below from being written.
		slog.Error("populate lead names", "error", err, "lead_id", l.ID)
	}
	l.DisplayName = l.displayName()
	name := old.DisplayName
	if name == "" {
		name = old.ID
	}
	s.logActivity(l.ID, "lead", "create", fmt.Sprintf("Started new cycle from closed lead %q", name), userID)
	return nil
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

	// One open lead per (contact, pipeline, program): a repeat enquiry for an
	// open deal is a touchpoint, not a new opportunity, so refuse the create
	// when the contact already holds the slot. Runs after request validation
	// so invalid payloads surface their own errors first.
	if err := s.refuseOpenLeadConflictTx(tx, contactID, req.PipelineID, req.ProgramID, ""); err != nil {
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
		// The lead is committed; a display-name read must not fail the write.
		// The response degrades to empty names and the next read repairs it.
		slog.Error("populate lead names", "error", err, "lead_id", l.ID)
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
		// The shared row lock pairs with the lock a contact delete takes, so
		// a contact cannot be hidden between this check and the commit.
		var live bool
		err := tx.QueryRow(
			`SELECT true FROM contacts WHERE id = $1 AND deleted_at IS NULL FOR SHARE`,
			*contactID,
		).Scan(&live)
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrContactNotActive
		}
		if err != nil {
			return "", fmt.Errorf("validate contact: %w", err)
		}
		return *contactID, nil
	}
	if nc == nil || nc.Name == "" {
		return "", ErrContactRequired
	}
	// The "at least one contact detail" invariant is checked on the canonical
	// forms, like the contact module does: a phone that carries no digits
	// ("+", "call me") does not count as a detail.
	defaultCC, err := settings.DefaultCountryCode(tx)
	if err != nil {
		return "", fmt.Errorf("resolve contact: %w", err)
	}
	phoneKey, codedKey := util.PhoneLookupKeys(nc.Phone, defaultCC)
	emailKey := util.NormalizeEmail(nc.Email)
	if phoneKey == "" && emailKey == "" {
		return "", ErrNoContactDetail
	}

	// Serialize concurrent lead entries that could create a duplicate contact
	// for the same phone/email. The lock is namespaced by the national-form
	// lookup key (both storage generations collapse to it) and released
	// automatically at commit/rollback.
	lookupKey := phoneKey
	if lookupKey == "" {
		lookupKey = emailKey
	}
	if lookupKey != "" {
		if _, err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtext($1))`, "lead_resolve:"+lookupKey); err != nil {
			return "", fmt.Errorf("lock contact resolve: %w", err)
		}
	}

	// Phone primary, email secondary. The child tables store the canonical
	// value ('+' + digits), so the lookup compares both storage generations:
	// the incoming value is keyed in national form and matched against the
	// stored value with the same transformation. Any phone/email on a contact
	// counts as a match (not just the primary), so an alternate number still
	// resolves to the contact. Both match queries take the shared row lock a
	// contact delete pairs with: a contact hidden while resolving is dropped
	// on re-check, so the lead falls through to a fresh contact instead of
	// linking to a deleted one.
	if phoneKey != "" {
		var found string
		err := tx.QueryRow(
			`SELECT cp.contact_id FROM contact_phones cp
			JOIN contacts c ON c.id = cp.contact_id AND c.deleted_at IS NULL
			WHERE `+util.PhoneMatchCond("cp.value", "$1", "$2")+`
			LIMIT 1
			FOR SHARE OF c`,
			phoneKey, codedKey,
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
			WHERE lower(trim(ce.value)) = $1 LIMIT 1
			FOR SHARE OF c`,
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
	// The phone is canonicalized to one stored form ('+' + digits), matching
	// every other entry point.
	var id string
	err = tx.QueryRow(
		`INSERT INTO contacts (name) VALUES ($1) RETURNING id`,
		nc.Name,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create contact from lead: %w", err)
	}
	if phoneKey != "" {
		if _, err := tx.Exec(
			`INSERT INTO contact_phones (contact_id, value, is_primary) VALUES ($1, $2, true)`,
			id, util.CanonicalPhone(nc.Phone, defaultCC),
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

// normalizeProgram maps a program reference onto its slot key: an explicit
// empty string (the clear-program signal used by updates) is the program-less
// slot, same as nil.
func normalizeProgram(programID *string) *string {
	if programID == nil || *programID == "" {
		return nil
	}
	return programID
}

// sameProgram reports whether two program references denote the same slot key,
// treating nil and an empty string as the program-less slot.
func sameProgram(a, b *string) bool {
	an, bn := normalizeProgram(a), normalizeProgram(b)
	if an == nil || bn == nil {
		return an == bn
	}
	return *an == *bn
}

// lockLeadSlotTx serializes concurrent writes to the same open-lead slot
// (contact, pipeline, program) so exactly one of two racing creates wins. The
// lock is scoped to the transaction and released at commit/rollback, mirroring
// the contact-resolve lock used by lead entry.
func lockLeadSlotTx(tx *sql.Tx, contactID, pipelineID string, programID *string) error {
	slot := "lead_slot:" + contactID + ":" + pipelineID + ":"
	if programID != nil {
		slot += *programID
	}
	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtext($1))`, slot); err != nil {
		return fmt.Errorf("lock lead slot: %w", err)
	}
	return nil
}

// openLeadForSlotTx returns the open lead holding the (contact, pipeline,
// program) slot, or nil when the slot is free. A lead holds the slot while it
// is live and its linked stage declares outcome 'open' — the same stage
// metadata the rest of the product reads — so a closed or deleted lead never
// blocks a new cycle. excludeID is a lead id to ignore (the row being
// updated, whose own key change must not count as a holder).
func (s *Service) openLeadForSlotTx(tx *sql.Tx, contactID, pipelineID string, programID *string, excludeID string) (*OpenLeadRef, error) {
	var ref OpenLeadRef
	var programIDNull sql.NullString
	err := tx.QueryRow(
		`SELECT l.id,
			CASE WHEN c.deleted_at IS NOT NULL THEN COALESCE(NULLIF(l.nickname, ''), 'Deleted contact')
				ELSE COALESCE(NULLIF(l.nickname, ''), c.name, '') END,
			COALESCE(ls.name, ''),
			COALESCE(p.name, ''), l.program_id, COALESCE(pl.name, '')
		FROM leads l
		JOIN contacts c ON c.id = l.contact_id
		JOIN lead_stages ls ON ls.id = l.stage_id AND ls.outcome = 'open'
		LEFT JOIN programs p ON p.id = l.program_id
		LEFT JOIN pipelines pl ON pl.id = l.pipeline_id
		WHERE l.contact_id = $1 AND l.pipeline_id = $2
			AND l.program_id IS NOT DISTINCT FROM $3
			AND l.deleted_at IS NULL
			AND ($4 = '' OR l.id::text <> $4)
		ORDER BY l.created_at ASC
		LIMIT 1`,
		contactID, pipelineID, programID, excludeID,
	).Scan(&ref.ID, &ref.DisplayName, &ref.StageName, &ref.ProgramName, &programIDNull, &ref.PipelineName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("check open lead slot: %w", err)
	}
	if programIDNull.Valid {
		ref.ProgramID = &programIDNull.String
	}
	ref.PipelineID = pipelineID
	return &ref, nil
}

// refuseOpenLeadConflictTx enforces the one-open-lead rule for a slot: it
// locks the slot, then refuses the write when an open lead already holds it.
// Returns an *OpenLeadConflictError carrying the existing lead; nil when the
// slot is free. An empty program reference targets the program-less slot.
// excludeID ignores the lead being written (the update path), so a lead never
// conflicts with its own key change.
func (s *Service) refuseOpenLeadConflictTx(tx *sql.Tx, contactID, pipelineID string, programID *string, excludeID string) error {
	programID = normalizeProgram(programID)
	if err := lockLeadSlotTx(tx, contactID, pipelineID, programID); err != nil {
		return err
	}
	existing, err := s.openLeadForSlotTx(tx, contactID, pipelineID, programID, excludeID)
	if err != nil {
		return err
	}
	if existing != nil {
		return &OpenLeadConflictError{Lead: *existing}
	}
	return nil
}

func (s *Service) update(id string, req UpdateRequest, userID string) (*Lead, error) {
	if req.Value != nil {
		return nil, ErrCustomValueRejected
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("update lead: %w", err)
	}
	defer tx.Rollback()

	// Load under a row lock: concurrent moves serialize, so every decision
	// below reads the state the UPDATE will replace, not a stale snapshot.
	old, err := s.getForUpdateTx(tx, id)
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
		// The target stage's outcome decides: closed → open spawns a new
		// cycle; closed → closed is rejected.
		var targetOutcome string
		if err := tx.QueryRow(
			`SELECT outcome FROM lead_stages WHERE id = $1`,
			*req.StageID,
		).Scan(&targetOutcome); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrStageNotInPipeline
			}
			return nil, fmt.Errorf("update lead: load target stage: %w", err)
		}
		if targetOutcome != "open" {
			return nil, ErrClosedToClosedMove
		}
		if !req.spawnCarriesOnlyStage() {
			return nil, ErrSpawnOnlyStage
		}
		l, err := s.spawnCycleTx(tx, old, *req.StageID)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("update lead: commit spawn: %w", err)
		}
		if err := s.afterSpawn(l, old, userID); err != nil {
			return nil, err
		}
		return l, nil
	}

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
		// The shared row lock pairs with the lock a contact delete takes, so
		// a contact cannot be hidden between this check and the commit.
		var live bool
		err := tx.QueryRow(
			`SELECT true FROM contacts WHERE id = $1 AND deleted_at IS NULL FOR SHARE`,
			*req.ContactID,
		).Scan(&live)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrContactNotActive
		}
		if err != nil {
			return nil, fmt.Errorf("update lead: validate contact: %w", err)
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
			// The pipeline package enforces that outcomes stay in the
			// open/won/lost vocabulary, so a closing stage is never 'open'.
			outcome = &info.Outcome
			if req.LostReason != nil && info.Outcome == "lost" {
				lostReason = req.LostReason
			}
		}
		// Moving out of a closing stage never reaches here: the top-of-update
		// check spawns a new cycle or rejects the move.
	}

	// One open lead per (contact, pipeline, program): only a slot-key change
	// can create a duplicate, so the guard runs when contact_id, pipeline_id,
	// or program_id change; stage moves never touch the key. An explicit empty
	// program clears the program and targets the program-less slot. Two
	// exemptions: a move into a closing stage (the lead closes in this same
	// update and never occupies the new slot) and an already-closed lead
	// (terminal rows hold no slot, so re-sloting them cannot create a
	// duplicate open deal).
	contactID := old.ContactID
	if req.ContactID != nil && *req.ContactID != "" {
		contactID = *req.ContactID
	}
	pipelineID := old.PipelineID
	if req.PipelineID != nil {
		pipelineID = *req.PipelineID
	}
	programID := normalizeProgram(old.ProgramID)
	if req.ProgramID != nil {
		programID = normalizeProgram(req.ProgramID)
	}
	if old.StageOutcome == "open" &&
		(targetStage == nil || !targetStage.IsClosing) &&
		(contactID != old.ContactID || pipelineID != old.PipelineID || !sameProgram(programID, old.ProgramID)) {
		if err := s.refuseOpenLeadConflictTx(tx, contactID, pipelineID, programID, id); err != nil {
			return nil, err
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
		if err := s.insertStageHistoryTx(tx, id, old.StageID, l.StageID, old.StageName, targetStage.Name, userID); err != nil {
			return nil, err
		}
		// Reaching a closing stage resolves the deal: cancel every open task so
		// reminders stop nagging on won/lost leads.
		if targetStage.IsClosing {
			if err := s.cancelOpenTasksTx(tx, id); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit lead update: %w", err)
	}
	if err := s.populateNames(&l); err != nil {
		// The update is committed; a display-name read must not fail it.
		// The response degrades to empty names and the next read repairs it.
		slog.Error("populate lead names", "error", err, "lead_id", l.ID)
	}
	l.DisplayName = l.displayName()

	// Audit only what no other surface records: a stage move is owned by the
	// stage history, so a pure move writes no row. Field edits carry their
	// before → after in the description.
	parts := []string{}
	if old.PipelineID != l.PipelineID {
		parts = append(parts, fmt.Sprintf("pipeline %q → %q", s.pipelineIDToName(old.PipelineID), s.pipelineIDToName(l.PipelineID)))
	}
	if old.ProgramName != l.ProgramName {
		parts = append(parts, fmt.Sprintf("program %q → %q", old.ProgramName, l.ProgramName))
	}
	if (old.Value == nil) != (l.Value == nil) || (old.Value != nil && l.Value != nil && *old.Value != *l.Value) {
		parts = append(parts, fmt.Sprintf("value %s → %s", valueLabel(old.Value), valueLabel(l.Value)))
	}
	if !sameID(old.AssignedTo, l.AssignedTo) {
		parts = append(parts, fmt.Sprintf("assignee %q → %q", s.userIDToName(old.AssignedTo), s.userIDToName(l.AssignedTo)))
	}
	if old.ContactID != l.ContactID && old.ContactID != "" {
		parts = append(parts, fmt.Sprintf("contact %q → %q", contactLabel(old.ContactName, old.ContactID), contactLabel(l.ContactName, l.ContactID)))
	}
	if old.LostReason != l.LostReason {
		parts = append(parts, fmt.Sprintf("lost reason %q → %q", old.LostReason, l.LostReason))
	}
	if len(parts) > 0 {
		s.logActivity(l.ID, "lead", "update", fmt.Sprintf("Updated lead %q: %s", l.DisplayName, strings.Join(parts, ", ")), userID)
	} else if old.Nickname != l.Nickname || old.Notes != l.Notes {
		// A notes/nickname edit leaves no other trace, so it still earns a
		// bare attribution row; a pure stage move writes nothing.
		s.logActivity(l.ID, "lead", "update", fmt.Sprintf("Updated lead %q", l.DisplayName), userID)
	}
	return &l, nil
}

// valueLabel renders an optional lead value for an audit description.
func valueLabel(v *float64) string {
	if v == nil {
		return "none"
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

// sameID reports whether two optional uuids are equal, treating nil as "".
func sameID(a, b *string) bool {
	av, bv := "", ""
	if a != nil {
		av = *a
	}
	if b != nil {
		bv = *b
	}
	return av == bv
}

// contactLabel renders a contact by name with a uuid fallback for audit text.
func contactLabel(name, id string) string {
	if name != "" {
		return name
	}
	return id
}

// userIDToName resolves a user uuid to a display name for audit text; an
// empty or unknown id falls back to the id itself (or "none" when empty).
func (s *Service) userIDToName(id *string) string {
	if id == nil || *id == "" {
		return "none"
	}
	var name string
	if err := s.db.QueryRow(`SELECT name FROM users WHERE id = $1`, *id).Scan(&name); err != nil || name == "" {
		return *id
	}
	return name
}

// pipelineIDToName resolves a pipeline uuid to a display name for audit text;
// an unknown or empty id falls back to the id itself.
func (s *Service) pipelineIDToName(id string) string {
	if id == "" {
		return "none"
	}
	var name string
	if err := s.db.QueryRow(`SELECT name FROM pipelines WHERE id = $1`, id).Scan(&name); err != nil || name == "" {
		return id
	}
	return name
}

func (s *Service) delete(id string, userID string) error {
	// The audit row is the last public trace of the deleted cycle (the journey
	// hides soft-deleted leads), so capture the display name before hiding it.
	var displayName string
	err := s.db.QueryRow(
		`SELECT COALESCE(NULLIF(l.nickname, ''), c.name, '') FROM leads l
		LEFT JOIN contacts c ON c.id = l.contact_id
		WHERE l.id = $1 AND l.deleted_at IS NULL`,
		id,
	).Scan(&displayName)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("delete lead: load display name: %w", err)
	}
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
	if err := s.cancelOpenTasksTx(tx, id); err != nil {
		return fmt.Errorf("delete lead: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit lead delete: %w", err)
	}
	s.logActivity(id, "lead", "delete", fmt.Sprintf("Deleted lead %q", displayName), userID)
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
		`SELECT CASE WHEN c.deleted_at IS NOT NULL THEN 'Deleted contact' ELSE COALESCE(c.name, '') END,
			CASE WHEN c.deleted_at IS NULL THEN COALESCE(pcp.value, '') ELSE '' END,
			CASE WHEN c.deleted_at IS NULL THEN COALESCE(pce.value, '') ELSE '' END
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
	ID   string
	Name string
	// IsClosing is derived from Outcome (outcome != 'open').
	IsClosing bool
	// Outcome is the stage's declared outcome ('open' | 'won' | 'lost'),
	// authoritative for lead outcome resolution.
	Outcome string
}

// leadStageOutcomeTx loads a live lead's current stage outcome inside a
// transaction, deriving open versus closing from stage metadata. It takes the
// same row lock a plain UPDATE takes (FOR NO KEY UPDATE), so a concurrent
// close serializes behind the check instead of deadlocking on a lock upgrade
// shared-lock holders would hit when they update the lead. It returns
// ErrNotFound when the lead is missing or soft-deleted.
func (s *Service) leadStageOutcomeTx(tx *sql.Tx, leadID string) (string, error) {
	var outcome string
	err := tx.QueryRow(`
		SELECT ls.outcome
		FROM leads l
		JOIN lead_stages ls ON ls.id = l.stage_id
		WHERE l.id = $1 AND l.deleted_at IS NULL
		FOR NO KEY UPDATE OF l`,
		leadID,
	).Scan(&outcome)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("load lead stage outcome: %w", err)
	}
	return outcome, nil
}

// stageInfoTx loads a stage's name and outcome inside a transaction so the
// outcome resolution and history insert see a consistent view. Closing is
// derived from the outcome — the single stored source of truth.
func (s *Service) stageInfoTx(tx *sql.Tx, stageID string) (*stageInfo, error) {
	var info stageInfo
	err := tx.QueryRow(
		`SELECT id, name, outcome FROM lead_stages WHERE id = $1`,
		stageID,
	).Scan(&info.ID, &info.Name, &info.Outcome)
	if err != nil {
		return nil, fmt.Errorf("load stage info: %w", err)
	}
	info.IsClosing = info.Outcome != "open"
	return &info, nil
}

// insertStageHistoryTx records one stage move with the stage names captured at
// move time, so later renames or deletions never rewrite history. Every lead
// transition that changes stages uses this one writer.
func (s *Service) insertStageHistoryTx(tx *sql.Tx, leadID, fromStageID, toStageID, fromName, toName, userID string) error {
	if _, err := tx.Exec(
		`INSERT INTO lead_stage_history (lead_id, from_stage_id, to_stage_id, from_stage_name, to_stage_name, user_id)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::uuid)`,
		leadID, fromStageID, toStageID, fromName, toName, userID,
	); err != nil {
		return fmt.Errorf("record stage history: %w", err)
	}
	return nil
}

// cancelOpenTasksTx cancels every open task on a lead: reaching a closing
// stage or deleting the lead ends its working state, so reminders stop.
func (s *Service) cancelOpenTasksTx(tx *sql.Tx, leadID string) error {
	if _, err := tx.Exec(
		`UPDATE lead_activities SET is_cancelled = true
		WHERE lead_id = $1 AND NOT is_done AND NOT is_cancelled`,
		leadID,
	); err != nil {
		return fmt.Errorf("cancel open tasks: %w", err)
	}
	return nil
}

// closeLostTx executes a close_lost quick reply inside a transaction: the
// lead moves to its pipeline's lost closing stage, outcome resolves to
// 'lost', open tasks are cancelled, and the move is recorded in stage
// history. It returns false (no move) when the lead already sits in the
// target stage or when the lead is terminal (a closed row never re-closes),
// and ErrNoLostStage when the pipeline has no lost closing stage. The outcome
// rule mirrors update()'s: closing stages that carry the column default
// 'open' count as lost.
func (s *Service) closeLostTx(tx *sql.Tx, leadID, userID string) (bool, error) {
	var pipelineID, currentStageID, currentOutcome string
	if err := tx.QueryRow(
		`SELECT l.pipeline_id, l.stage_id, ls.outcome
		FROM leads l
		JOIN lead_stages ls ON ls.id = l.stage_id
		WHERE l.id = $1 AND l.deleted_at IS NULL`,
		leadID,
	).Scan(&pipelineID, &currentStageID, &currentOutcome); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrNotFound
		}
		return false, fmt.Errorf("close lost: load lead: %w", err)
	}
	if currentOutcome != "open" {
		return false, nil
	}

	var target stageInfo
	err := tx.QueryRow(
		`SELECT id, name, outcome FROM lead_stages
		WHERE pipeline_id = $1 AND outcome = 'lost'
		ORDER BY "order" ASC, created_at ASC
		LIMIT 1`,
		pipelineID,
	).Scan(&target.ID, &target.Name, &target.Outcome)
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
	if err := s.insertStageHistoryTx(tx, leadID, currentStageID, target.ID, fromStageName, target.Name, userID); err != nil {
		return false, fmt.Errorf("close lost: %w", err)
	}

	// Reaching a closing stage resolves the deal: cancel every open task so
	// reminders stop nagging on lost leads.
	if err := s.cancelOpenTasksTx(tx, leadID); err != nil {
		return false, fmt.Errorf("close lost: %w", err)
	}
	return true, nil
}

// listHistory returns the chronological stage moves for a lead, oldest first.
// requireLiveLead returns ErrNotFound unless the id is a live (non-deleted)
// lead, so nested reads cannot surface rows of hidden leads. Malformed ids
// report not-found rather than a database cast error.
func (s *Service) requireLiveLead(leadID string) error {
	if !util.IsUUID(leadID) {
		return ErrNotFound
	}
	var live bool
	if err := s.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM leads WHERE id = $1 AND deleted_at IS NULL)`,
		leadID,
	).Scan(&live); err != nil {
		return fmt.Errorf("check lead: %w", err)
	}
	if !live {
		return ErrNotFound
	}
	return nil
}

func (s *Service) listHistory(leadID string) ([]StageHistory, error) {
	if err := s.requireLiveLead(leadID); err != nil {
		return nil, err
	}
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
