package pipeline

import (
	"crm/internal/respond"
	"crm/internal/util"
	"database/sql"
	"errors"
	"fmt"
)

var (
	// ErrNotFound marks mutations targeting a pipeline or stage that does
	// not exist.
	ErrNotFound = errors.New("pipeline or stage not found")
	// ErrInUse marks deletions blocked because leads or activities still
	// reference the pipeline or stage.
	ErrInUse = errors.New("resource is in use and cannot be deleted")
	// ErrInvalidStageOutcome marks a requested outcome outside the open/won/
	// lost vocabulary; it is client-input validation, surfaced as a 400.
	ErrInvalidStageOutcome = errors.New("outcome must be 'open', 'won', or 'lost'")
	// ErrInvalidStageOrder marks a reorder request that does not list every
	// stage of the pipeline exactly once.
	ErrInvalidStageOrder = errors.New("stage order must list every stage of the pipeline exactly once")
)

// StageInUseError refuses a stage change while leads reference the stage; the
// count lets the client name the blocker.
type StageInUseError struct {
	LeadCount int
}

func (e *StageInUseError) Error() string {
	return fmt.Sprintf("%d leads use this stage; move them before changing or deleting it", e.LeadCount)
}

// LastStageError refuses removing the pipeline's last open or lost stage:
// lead entry needs an open stage and close-lost needs a lost one.
type LastStageError struct {
	Outcome string
}

func (e *LastStageError) Error() string {
	return fmt.Sprintf("the pipeline needs at least one %s stage", e.Outcome)
}

// Stage outcome vocabulary. A stage's outcome is what
// reaching it means for a lead: open (in play), won, or lost. A stage with
// outcome 'won' or 'lost' closes the lead; 'open' does not.
const (
	OutcomeOpen = "open"
	OutcomeWon  = "won"
	OutcomeLost = "lost"
)

// stageOutcome validates a requested stage outcome, defaulting an omitted
// value to 'open'. 'won' and 'lost' make the stage a closing stage.
func stageOutcome(outcome string) (string, error) {
	switch outcome {
	case "":
		return OutcomeOpen, nil
	case OutcomeOpen, OutcomeWon, OutcomeLost:
		return outcome, nil
	default:
		return "", ErrInvalidStageOutcome
	}
}

// Service provides database-backed pipeline and stage operations, including
// stage outcome metadata used for lead win/loss resolution.
type Service struct {
	db *sql.DB
}

// NewService creates a pipeline Service backed by db.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) list() ([]Pipeline, error) {
	pipelines, err := s.listPipelines()
	if err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}
	if len(pipelines) == 0 {
		return pipelines, nil
	}

	pipelineIDs := make([]string, len(pipelines))
	for i, p := range pipelines {
		pipelineIDs[i] = p.ID
	}
	stageMap, err := s.listAllStages(pipelineIDs)
	if err != nil {
		return nil, fmt.Errorf("list stages: %w", err)
	}
	for i := range pipelines {
		pipelines[i].Stages = stageMap[pipelines[i].ID]
	}
	return pipelines, nil
}

func (s *Service) listAllStages(pipelineIDs []string) (map[string][]Stage, error) {
	query := `SELECT id, pipeline_id, name, "order", COALESCE(color, ''), outcome, created_at, updated_at
		FROM lead_stages
		WHERE pipeline_id = ANY($1)
		ORDER BY "order", created_at, id`
	rows, err := s.db.Query(query, pipelineIDs)
	if err != nil {
		return nil, fmt.Errorf("list all stages: %w", err)
	}
	defer rows.Close()

	stageMap := map[string][]Stage{}
	for rows.Next() {
		var st Stage
		if err := rows.Scan(
			&st.ID, &st.PipelineID, &st.Name, &st.Order, &st.Color, &st.Outcome, &st.CreatedAt, &st.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("list all stages: scan: %w", err)
		}
		st.IsClosing = st.Outcome != OutcomeOpen
		stageMap[st.PipelineID] = append(stageMap[st.PipelineID], st)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list all stages: iterate: %w", err)
	}
	return stageMap, nil
}

func (s *Service) listPipelines() ([]Pipeline, error) {
	rows, err := s.db.Query(
		`SELECT id, name, COALESCE(description, ''), created_at, updated_at FROM pipelines ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}
	defer rows.Close()
	pipelines := []Pipeline{}
	for rows.Next() {
		var p Pipeline
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("list pipelines: scan: %w", err)
		}
		pipelines = append(pipelines, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list pipelines: iterate: %w", err)
	}
	return pipelines, nil
}

func (s *Service) createPipeline(req CreatePipelineRequest) (*Pipeline, error) {
	var p Pipeline
	err := s.db.QueryRow(
		`INSERT INTO pipelines (name, description) VALUES ($1, $2)
		RETURNING id, name, COALESCE(description, ''), created_at, updated_at`,
		req.Name, req.Description,
	).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create pipeline: %w", err)
	}
	return &p, nil
}

func (s *Service) updatePipeline(id string, req UpdatePipelineRequest) (*Pipeline, error) {
	if !util.IsUUID(id) {
		return nil, ErrNotFound
	}
	var p Pipeline
	err := s.db.QueryRow(
		`UPDATE pipelines SET
			name = CASE WHEN NULLIF($2::text, '') IS NOT NULL THEN $2 ELSE name END,
			description = CASE WHEN $3::text IS NOT NULL THEN NULLIF($3, '') ELSE description END,
			updated_at = now()
		WHERE id = $1
		RETURNING id, name, COALESCE(description, ''), created_at, updated_at`,
		id,
		req.Name,
		req.Description,
	).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update pipeline: %w", err)
	}
	return &p, nil
}

// deletePipeline removes a pipeline and returns its name for the audit row, so
// the snapshot and the mutation are one query and cannot race a concurrent
// rename. An empty name falls back to the id so the audit row still names
// something.
func (s *Service) deletePipeline(id string) (string, error) {
	if !util.IsUUID(id) {
		return "", ErrNotFound
	}
	var name string
	err := s.db.QueryRow(`DELETE FROM pipelines WHERE id = $1 RETURNING name`, id).Scan(&name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		if respond.IsForeignKeyViolation(err) {
			return "", ErrInUse
		}
		return "", fmt.Errorf("delete pipeline: %w", err)
	}
	if name == "" {
		name = id
	}
	return name, nil
}

func (s *Service) createStage(pipelineID string, req CreateStageRequest) (*Stage, error) {
	if !util.IsUUID(pipelineID) {
		return nil, ErrNotFound
	}
	outcome, err := stageOutcome(req.Outcome)
	if err != nil {
		return nil, err
	}
	var st Stage
	err = s.db.QueryRow(
		`INSERT INTO lead_stages (pipeline_id, name, "order", color, outcome)
		VALUES ($1, $2,
			COALESCE(NULLIF($3::integer, 0), (SELECT COALESCE(MAX("order"), -1) + 1 FROM lead_stages WHERE pipeline_id = $1)),
			$4, $5)
		RETURNING id, pipeline_id, name, "order", COALESCE(color, ''), outcome, created_at, updated_at`,
		pipelineID,
		req.Name,
		req.Order,
		util.NullStr(req.Color),
		outcome,
	).Scan(&st.ID, &st.PipelineID, &st.Name, &st.Order, &st.Color, &st.Outcome, &st.CreatedAt, &st.UpdatedAt)
	if err != nil {
		if respond.IsForeignKeyViolation(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("create stage: %w", err)
	}
	st.IsClosing = st.Outcome != OutcomeOpen
	return &st, nil
}

func (s *Service) updateStage(stageID string, req UpdateStageRequest) (*Stage, error) {
	if !util.IsUUID(stageID) {
		return nil, ErrNotFound
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("update stage: %w", err)
	}
	defer tx.Rollback()

	// Lock the stage so concurrent outcome edits serialize. A missing stage
	// surfaces as not-found before payload validation, matching the delete
	// path: editing a stage that was removed in another tab gets a 404.
	var pipelineID, currentOutcome string
	err = tx.QueryRow(
		`SELECT pipeline_id, outcome FROM lead_stages WHERE id = $1 FOR UPDATE`,
		stageID,
	).Scan(&pipelineID, &currentOutcome)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load stage for update: %w", err)
	}

	// A partial update may change the outcome alone: the value is validated
	// against the open/won/lost vocabulary, and closing is derived from it. An
	// omitted outcome (nil) keeps the stored value; an explicit empty string is
	// malformed rather than a quiet reset to open.
	var outcome *string
	if req.Outcome != nil {
		if *req.Outcome == "" {
			return nil, ErrInvalidStageOutcome
		}
		resolved, err := stageOutcome(*req.Outcome)
		if err != nil {
			return nil, err
		}
		outcome = &resolved
		if resolved != currentOutcome {
			if err := s.guardOutcomeChangeTx(tx, stageID, pipelineID, currentOutcome); err != nil {
				return nil, err
			}
		}
	}

	var st Stage
	err = tx.QueryRow(
		`UPDATE lead_stages SET
			name = CASE WHEN NULLIF($2::text, '') IS NOT NULL THEN $2 ELSE name END,
			"order" = COALESCE($3::integer, "order"),
			color = CASE WHEN $4::text IS NOT NULL THEN NULLIF($4, '') ELSE color END,
			outcome = COALESCE($5::text, outcome),
			updated_at = now()
		WHERE id = $1
		RETURNING id, pipeline_id, name, "order", COALESCE(color, ''), outcome, created_at, updated_at`,
		stageID,
		req.Name,
		req.Order,
		req.Color,
		outcome,
	).Scan(&st.ID, &st.PipelineID, &st.Name, &st.Order, &st.Color, &st.Outcome, &st.CreatedAt, &st.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update stage: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit stage update: %w", err)
	}
	st.IsClosing = st.Outcome != OutcomeOpen
	return &st, nil
}

// guardOutcomeChangeTx refuses an outcome change that would strand live leads
// in the stage or leave the pipeline without a usable open/lost stage.
func (s *Service) guardOutcomeChangeTx(tx *sql.Tx, stageID, pipelineID, currentOutcome string) error {
	var leads int
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM leads WHERE stage_id = $1 AND deleted_at IS NULL`,
		stageID,
	).Scan(&leads); err != nil {
		return fmt.Errorf("count stage leads: %w", err)
	}
	if leads > 0 {
		return &StageInUseError{LeadCount: leads}
	}
	return guardLastStageTx(tx, stageID, pipelineID, currentOutcome)
}

// guardLastStageTx refuses removing (or stopping use of) the pipeline's last
// open or lost stage: lead entry needs an open stage and close-lost needs a
// lost one.
func guardLastStageTx(tx *sql.Tx, stageID, pipelineID, outcome string) error {
	if outcome != OutcomeOpen && outcome != OutcomeLost {
		return nil
	}
	var others int
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM lead_stages WHERE pipeline_id = $1 AND id <> $2 AND outcome = $3`,
		pipelineID, stageID, outcome,
	).Scan(&others); err != nil {
		return fmt.Errorf("count sibling stages: %w", err)
	}
	if others == 0 {
		return &LastStageError{Outcome: outcome}
	}
	return nil
}

// deleteStage removes a stage and returns its name for the audit row. Any
// lead row pins the stage (live leads show it, soft-deleted rows still hold
// the FK), and the pipeline must keep its last open and lost stages.
func (s *Service) deleteStage(stageID string) (string, error) {
	if !util.IsUUID(stageID) {
		return "", ErrNotFound
	}
	tx, err := s.db.Begin()
	if err != nil {
		return "", fmt.Errorf("delete stage: %w", err)
	}
	defer tx.Rollback()

	var name, pipelineID, outcome string
	err = tx.QueryRow(
		`SELECT name, pipeline_id, outcome FROM lead_stages WHERE id = $1 FOR UPDATE`,
		stageID,
	).Scan(&name, &pipelineID, &outcome)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("delete stage: %w", err)
	}

	var leads int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM leads WHERE stage_id = $1`, stageID).Scan(&leads); err != nil {
		return "", fmt.Errorf("count stage leads: %w", err)
	}
	if leads > 0 {
		return "", &StageInUseError{LeadCount: leads}
	}
	if err := guardLastStageTx(tx, stageID, pipelineID, outcome); err != nil {
		return "", err
	}

	if _, err := tx.Exec(`DELETE FROM lead_stages WHERE id = $1`, stageID); err != nil {
		return "", fmt.Errorf("delete stage: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit stage delete: %w", err)
	}
	if name == "" {
		name = stageID
	}
	return name, nil
}

// reorderStages sets the pipeline's stage order to the given ids in one
// transaction. The pipeline's stages are locked first, and the request must
// list every stage exactly once; a mismatch is rejected so a stale client
// cannot drop or duplicate columns.
func (s *Service) reorderStages(pipelineID string, stageIDs []string) error {
	if !util.IsUUID(pipelineID) {
		return ErrNotFound
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("reorder stages: %w", err)
	}
	defer tx.Rollback()

	var pipelineExists bool
	if err := tx.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM pipelines WHERE id = $1)`,
		pipelineID,
	).Scan(&pipelineExists); err != nil {
		return fmt.Errorf("reorder stages: check pipeline: %w", err)
	}
	if !pipelineExists {
		return ErrNotFound
	}

	rows, err := tx.Query(
		`SELECT id FROM lead_stages WHERE pipeline_id = $1 FOR UPDATE`,
		pipelineID,
	)
	if err != nil {
		return fmt.Errorf("reorder stages: lock stages: %w", err)
	}
	existing := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("reorder stages: scan stage: %w", err)
		}
		existing[id] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("reorder stages: iterate stages: %w", err)
	}
	rows.Close()

	if len(stageIDs) == 0 || len(stageIDs) != len(existing) {
		return ErrInvalidStageOrder
	}
	seen := map[string]bool{}
	for _, id := range stageIDs {
		if !existing[id] || seen[id] {
			return ErrInvalidStageOrder
		}
		seen[id] = true
	}
	for i, id := range stageIDs {
		if _, err := tx.Exec(
			`UPDATE lead_stages SET "order" = $2, updated_at = now() WHERE id = $1 AND pipeline_id = $3`,
			id, i, pipelineID,
		); err != nil {
			return fmt.Errorf("reorder stages: update: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("reorder stages: commit: %w", err)
	}
	return nil
}
