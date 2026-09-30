package lead

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"crm/internal/util"
)

// bulkContactLimit caps one bulk lead entry run; it mirrors the contact
// import limit so one imported batch becomes leads in one action.
const bulkContactLimit = 500

var (
	// ErrBulkNoContacts refuses a bulk run without contact ids.
	ErrBulkNoContacts = errors.New("contact_ids is required")
	// ErrBulkLimit refuses a bulk run above the 500-contact cap.
	ErrBulkLimit = errors.New("too many contacts (limit 500)")
	// ErrBulkInvalidID refuses a malformed contact id before any write: a
	// non-UUID id is a client bug, not a per-row outcome.
	ErrBulkInvalidID = errors.New("contact_ids must be valid ids")
	// ErrBulkInvalidField refuses malformed shared-field ids before any
	// write: a non-UUID pipeline, stage, or program is a client bug, not a
	// per-row outcome.
	ErrBulkInvalidField = errors.New("pipeline_id, stage_id, and program_id must be valid ids")
)

// BulkCreateRequest is the bulk lead entry payload: one shared pipeline,
// stage, program, and assignee; one lead per contact id.
type BulkCreateRequest struct {
	ContactIDs []string `json:"contact_ids"`
	PipelineID string   `json:"pipeline_id"`
	StageID    string   `json:"stage_id"`
	ProgramID  string   `json:"program_id,omitempty"`
	AssignedTo string   `json:"assigned_to,omitempty"`
}

// BulkCreateResponse reports one bulk run: created counts the new leads,
// skipped counts contacts whose open lead already holds the slot, failed
// counts contact-level errors, and Errors lists every non-created contact.
type BulkCreateResponse struct {
	Created int            `json:"created"`
	Skipped int            `json:"skipped"`
	Failed  int            `json:"failed"`
	Errors  []BulkRowError `json:"errors,omitempty"`
}

// BulkRowError is one non-created contact from a bulk run. Outcome is
// "skipped" (an open lead already holds the slot) or "failed" (the contact
// could not take a lead); Reason is the human-readable cause.
type BulkRowError struct {
	ContactID string `json:"contact_id"`
	Name      string `json:"name"`
	Outcome   string `json:"outcome"`
	Reason    string `json:"message"`
}

// bulkCreate creates one lead per contact id under the shared pipeline,
// stage, program, and assignee. The shared fields are validated once before
// any write, so an invalid request creates nothing; each row then runs the
// same create path as a single lead, skipping contacts that already hold an
// open lead in the slot. A row-level failure is reported and does not stop
// the remaining rows. Returns ErrBulkNoContacts, ErrBulkLimit, or
// ErrBulkInvalidID for a malformed contact id list, and ErrBulkInvalidField
// for a malformed pipeline, stage, or program id.
func (s *Service) bulkCreate(req BulkCreateRequest, userID string) (*BulkCreateResponse, error) {
	ids, err := normalizeBulkContactIDs(req.ContactIDs)
	if err != nil {
		return nil, err
	}
	// A malformed shared id would otherwise reach PostgreSQL as a cast error
	// and surface as a server error instead of the promised fail-fast 4xx.
	if !util.IsUUID(req.PipelineID) || !util.IsUUID(req.StageID) ||
		(req.ProgramID != "" && !util.IsUUID(req.ProgramID)) {
		return nil, ErrBulkInvalidField
	}

	var programID *string
	if req.ProgramID != "" {
		programID = &req.ProgramID
	}
	var assignedTo *string
	if req.AssignedTo != "" {
		assignedTo = &req.AssignedTo
	}

	if err := s.validateBulkSharedTx(req.PipelineID, req.StageID, programID, assignedTo); err != nil {
		return nil, err
	}

	names, err := s.contactNames(ids)
	if err != nil {
		return nil, fmt.Errorf("bulk create: load contact names: %w", err)
	}

	// The run's start time (database clock) bounds the post-error
	// verification below: only a lead created at or after it can be this
	// run's committed work.
	var runStart time.Time
	if err := s.db.QueryRow(`SELECT now()`).Scan(&runStart); err != nil {
		return nil, fmt.Errorf("bulk create: load run start: %w", err)
	}

	resp := &BulkCreateResponse{}
	loggedUnexpected := false
	for _, id := range ids {
		contactID := id
		_, err := s.create(CreateRequest{
			ContactID:  &contactID,
			PipelineID: req.PipelineID,
			StageID:    req.StageID,
			ProgramID:  programID,
			AssignedTo: assignedTo,
		}, userID)
		if err == nil {
			resp.Created++
			continue
		}

		var conflict *OpenLeadConflictError
		row := BulkRowError{ContactID: id, Name: names[id]}
		switch {
		case errors.As(err, &conflict):
			row.Outcome = "skipped"
			row.Reason = "already has an open deal: " + openLeadContext(&conflict.Lead)
			resp.Skipped++
		case errors.Is(err, ErrContactNotActive), errors.Is(err, ErrInvalidContactID):
			row.Outcome = "failed"
			row.Reason = err.Error()
			resp.Failed++
		case errors.Is(err, ErrStageNotInPipeline), errors.Is(err, ErrClosingStageAtCreate),
			errors.Is(err, ErrProgramNotActive), errors.Is(err, ErrInvalidAssignee):
			// Shared-field state changed between the fail-fast pass and this
			// row (a concurrent settings edit): the curated refusal is the
			// honest reason.
			row.Outcome = "failed"
			row.Reason = err.Error()
			resp.Failed++
		default:
			// An unexpected failure: create can fail after its transaction
			// committed (display-name enrichment), so the lead may exist
			// despite the error. Verify the slot before reporting the row.
			if committed, verifyErr := s.openLeadCreatedSince(id, req.PipelineID, programID, runStart); verifyErr == nil && committed {
				resp.Created++
				continue
			}
			row.Outcome = "failed"
			row.Reason = "internal error — re-run to check this contact"
			resp.Failed++
			// The technical detail goes to the server log, never into the
			// response: internal errors are not operator-facing text.
			if !loggedUnexpected {
				loggedUnexpected = true
				slog.Error("bulk lead create: unexpected row error", "contact_id", id, "error", err)
			}
		}
		if row.Name == "" {
			row.Name = id
		}
		resp.Errors = append(resp.Errors, row)
	}
	return resp, nil
}

// normalizeBulkContactIDs rejects empty, oversized, and malformed id lists
// and removes duplicates while preserving order, so one run never creates two
// leads for the same contact.
func normalizeBulkContactIDs(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, ErrBulkNoContacts
	}
	if len(ids) > bulkContactLimit {
		return nil, ErrBulkLimit
	}
	seen := make(map[string]bool, len(ids))
	normalized := make([]string, 0, len(ids))
	for _, id := range ids {
		if !util.IsUUID(id) {
			return nil, ErrBulkInvalidID
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		normalized = append(normalized, id)
	}
	return normalized, nil
}

// validateBulkSharedTx runs the shared-field checks once, before any lead is
// written: an unknown pipeline, foreign or closing stage, archived program,
// or deleted assignee fails the whole request with nothing created. The same
// checks run again inside every row's create (the single writer path); this
// pass exists to fail fast, not to replace them.
func (s *Service) validateBulkSharedTx(pipelineID, stageID string, programID, assignedTo *string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("bulk create: validate: %w", err)
	}
	defer tx.Rollback()

	if err := s.validateAssignedToTx(tx, assignedTo); err != nil {
		return err
	}
	if err := s.validateStageForPipelineTx(tx, pipelineID, stageID); err != nil {
		return err
	}
	info, err := s.stageInfoTx(tx, stageID)
	if err != nil {
		return err
	}
	if info.IsClosing {
		return ErrClosingStageAtCreate
	}
	if _, err := s.snapshotPriceTx(tx, programID); err != nil {
		return err
	}
	return nil
}

// contactNames loads the display name of every requested contact in one
// query, including soft-deleted rows: a skipped or failed row must be
// identified by name in the report, not by an opaque id. The uuid[] cast
// keeps the lookup on the primary-key index.
func (s *Service) contactNames(ids []string) (map[string]string, error) {
	rows, err := s.db.Query(`SELECT id, name FROM contacts WHERE id = ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := make(map[string]string, len(ids))
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		names[id] = name
	}
	return names, rows.Err()
}

// openLeadCreatedSince reports whether a live open lead occupies the
// (contact, pipeline, program) slot with created_at at or after since — the
// proof that a create which returned an error still committed its lead (the
// transaction commits before display names are loaded). The time window keeps
// a pre-existing lead in the slot from being counted as this run's work.
func (s *Service) openLeadCreatedSince(contactID, pipelineID string, programID *string, since time.Time) (bool, error) {
	var exists bool
	err := s.db.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM leads l
			JOIN lead_stages ls ON ls.id = l.stage_id AND ls.outcome = 'open'
			WHERE l.contact_id = $1 AND l.pipeline_id = $2
				AND l.program_id IS NOT DISTINCT FROM $3
				AND l.deleted_at IS NULL
				AND l.created_at >= $4)`,
		contactID, pipelineID, programID, since,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check created lead: %w", err)
	}
	return exists, nil
}

// openLeadContext renders the deal already holding a slot for a skip reason:
// stage, program (or "No program"), and pipeline, most specific first. Empty
// names are omitted rather than rendered as blanks.
func openLeadContext(ref *OpenLeadRef) string {
	parts := make([]string, 0, 3)
	if ref.StageName != "" {
		parts = append(parts, ref.StageName)
	}
	if ref.ProgramName != "" {
		parts = append(parts, ref.ProgramName)
	} else {
		parts = append(parts, "No program")
	}
	if ref.PipelineName != "" {
		parts = append(parts, ref.PipelineName)
	}
	return strings.Join(parts, " · ")
}
