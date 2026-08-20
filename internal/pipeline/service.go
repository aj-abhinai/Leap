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
	// ErrInvalidStageOutcome marks a requested outcome that a closing stage
	// cannot take; it is client-input validation, surfaced as a 400.
	ErrInvalidStageOutcome = errors.New("closing stages must have outcome 'won' or 'lost'")
)

// Stage outcome vocabulary (ADR 019 amendment). A stage's outcome is what
// reaching it means for a lead: open (in play), won, or lost.
const (
	OutcomeOpen = "open"
	OutcomeWon  = "won"
	OutcomeLost = "lost"
)

// stageOutcome resolves a stage's outcome from its closing flag and the
// requested value. Non-closing stages are always 'open'; closing stages must
// be 'won' or 'lost' (defaulting to 'lost' when not specified). An explicit
// 'open' on a closing stage is rejected — it is not a valid win/loss.
func stageOutcome(isClosing bool, outcome string) (string, error) {
	if !isClosing {
		return OutcomeOpen, nil
	}
	switch outcome {
	case "":
		return OutcomeLost, nil
	case OutcomeWon, OutcomeLost:
		return outcome, nil
	default:
		return "", ErrInvalidStageOutcome
	}
}

type Service struct {
	db *sql.DB
}

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
	query := `SELECT id, pipeline_id, name, "order", COALESCE(color, ''), is_closing, outcome, created_at, updated_at
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
			&st.ID, &st.PipelineID, &st.Name, &st.Order, &st.Color, &st.IsClosing, &st.Outcome, &st.CreatedAt, &st.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("list all stages: scan: %w", err)
		}
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

func (s *Service) deletePipeline(id string) error {
	_, err := s.db.Exec(`DELETE FROM pipelines WHERE id = $1`, id)
	if err != nil {
		if respond.IsForeignKeyViolation(err) {
			return ErrInUse
		}
		return fmt.Errorf("delete pipeline: %w", err)
	}
	return nil
}

func (s *Service) createStage(pipelineID string, req CreateStageRequest) (*Stage, error) {
	outcome, err := stageOutcome(req.IsClosing, req.Outcome)
	if err != nil {
		return nil, err
	}
	var st Stage
	err = s.db.QueryRow(
		`INSERT INTO lead_stages (pipeline_id, name, "order", color, is_closing, outcome) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, pipeline_id, name, "order", COALESCE(color, ''), is_closing, outcome, created_at, updated_at`,
		pipelineID,
		req.Name,
		req.Order,
		util.NullStr(req.Color),
		req.IsClosing,
		outcome,
	).Scan(&st.ID, &st.PipelineID, &st.Name, &st.Order, &st.Color, &st.IsClosing, &st.Outcome, &st.CreatedAt, &st.UpdatedAt)
	if err != nil {
		if respond.IsForeignKeyViolation(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("create stage: %w", err)
	}
	return &st, nil
}

func (s *Service) updateStage(stageID string, req UpdateStageRequest) (*Stage, error) {
	// Resolve the resulting closing flag and outcome together: only one may be
	// present in a partial update, and the outcome rules depend on the final
	// closing state (non-closing is always 'open', closing must be won/lost).
	curIsClosing := false
	var curOutcome string
	if err := s.db.QueryRow(
		`SELECT is_closing, outcome FROM lead_stages WHERE id = $1`,
		stageID,
	).Scan(&curIsClosing, &curOutcome); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("load stage for update: %w", err)
	}
	isClosing := curIsClosing
	if req.IsClosing != nil {
		isClosing = *req.IsClosing
	}
	outcomeVal := curOutcome
	if req.Outcome != nil {
		outcomeVal = *req.Outcome
	}
	// Promoting a previously-open stage to closing without an explicit win/loss
	// still carries outcome 'open' from the old row; treat that as unspecified
	// so it defaults to 'lost'. An explicit 'open' on an already-closing stage
	// is rejected by stageOutcome.
	if isClosing && outcomeVal == OutcomeOpen && !curIsClosing {
		outcomeVal = ""
	}
	outcome, err := stageOutcome(isClosing, outcomeVal)
	if err != nil {
		return nil, err
	}

	var st Stage
	err = s.db.QueryRow(
		`UPDATE lead_stages SET
			name = CASE WHEN NULLIF($2::text, '') IS NOT NULL THEN $2 ELSE name END,
			"order" = COALESCE($3::integer, "order"),
			color = CASE WHEN $4::text IS NOT NULL THEN NULLIF($4, '') ELSE color END,
			is_closing = $5,
			outcome = $6,
			updated_at = now()
		WHERE id = $1
		RETURNING id, pipeline_id, name, "order", COALESCE(color, ''), is_closing, outcome, created_at, updated_at`,
		stageID,
		req.Name,
		req.Order,
		req.Color,
		isClosing,
		outcome,
	).Scan(&st.ID, &st.PipelineID, &st.Name, &st.Order, &st.Color, &st.IsClosing, &st.Outcome, &st.CreatedAt, &st.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update stage: %w", err)
	}
	return &st, nil
}

func (s *Service) deleteStage(stageID string) error {
	_, err := s.db.Exec(`DELETE FROM lead_stages WHERE id = $1`, stageID)
	if err != nil {
		if respond.IsForeignKeyViolation(err) {
			return ErrInUse
		}
		return fmt.Errorf("delete stage: %w", err)
	}
	return nil
}
