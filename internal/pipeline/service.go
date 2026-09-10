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
)

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
		ORDER BY "order"`
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
	outcome, err := stageOutcome(req.Outcome)
	if err != nil {
		return nil, err
	}
	var st Stage
	err = s.db.QueryRow(
		`INSERT INTO lead_stages (pipeline_id, name, "order", color, outcome) VALUES ($1, $2, $3, $4, $5)
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
	// A missing stage surfaces as not-found before payload validation, matching
	// the delete path: editing a stage that was removed in another tab gets a
	// 404, not a validation error.
	var exists bool
	if err := s.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM lead_stages WHERE id = $1)`,
		stageID,
	).Scan(&exists); err != nil {
		return nil, fmt.Errorf("load stage for update: %w", err)
	}
	if !exists {
		return nil, ErrNotFound
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
	}

	var st Stage
	err := s.db.QueryRow(
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
	st.IsClosing = st.Outcome != OutcomeOpen
	return &st, nil
}

// deleteStage removes a stage and returns its name for the audit row with the
// same one-query and fallback rules as deletePipeline.
func (s *Service) deleteStage(stageID string) (string, error) {
	var name string
	err := s.db.QueryRow(`DELETE FROM lead_stages WHERE id = $1 RETURNING name`, stageID).Scan(&name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		if respond.IsForeignKeyViolation(err) {
			return "", ErrInUse
		}
		return "", fmt.Errorf("delete stage: %w", err)
	}
	if name == "" {
		name = stageID
	}
	return name, nil
}
