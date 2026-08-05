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
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) list() ([]Pipeline, error) {
	pipelines, err := s.listPipelines()
	if err != nil {
		return nil, err
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
		return nil, err
	}
	for i := range pipelines {
		pipelines[i].Stages = stageMap[pipelines[i].ID]
	}
	return pipelines, nil
}

func (s *Service) listAllStages(pipelineIDs []string) (map[string][]Stage, error) {
	query := `SELECT id, pipeline_id, name, "order", COALESCE(color, ''), is_closing, created_at, updated_at
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
			&st.ID, &st.PipelineID, &st.Name, &st.Order, &st.Color, &st.IsClosing, &st.CreatedAt, &st.UpdatedAt,
		); err != nil {
			return nil, err
		}
		stageMap[st.PipelineID] = append(stageMap[st.PipelineID], st)
	}
	if err := rows.Err(); err != nil {
		return nil, err
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
			return nil, err
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
			name = CASE WHEN NULLIF($2, '') IS NOT NULL THEN $2 ELSE name END,
			description = CASE WHEN $3 IS NOT NULL THEN NULLIF($3, '') ELSE description END,
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
	var st Stage
	err := s.db.QueryRow(
		`INSERT INTO lead_stages (pipeline_id, name, "order", color, is_closing) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, pipeline_id, name, "order", COALESCE(color, ''), is_closing, created_at, updated_at`,
		pipelineID,
		req.Name,
		req.Order,
		util.NullStr(req.Color),
		req.IsClosing,
	).Scan(&st.ID, &st.PipelineID, &st.Name, &st.Order, &st.Color, &st.IsClosing, &st.CreatedAt, &st.UpdatedAt)
	if err != nil {
		if respond.IsForeignKeyViolation(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("create stage: %w", err)
	}
	return &st, nil
}

func (s *Service) updateStage(stageID string, req UpdateStageRequest) (*Stage, error) {
	var st Stage
	err := s.db.QueryRow(
		`UPDATE lead_stages SET
			name = CASE WHEN NULLIF($2, '') IS NOT NULL THEN $2 ELSE name END,
			"order" = COALESCE($3, "order"),
			color = CASE WHEN $4 IS NOT NULL THEN NULLIF($4, '') ELSE color END,
			is_closing = COALESCE($5, is_closing),
			updated_at = now()
		WHERE id = $1
		RETURNING id, pipeline_id, name, "order", COALESCE(color, ''), is_closing, created_at, updated_at`,
		stageID,
		req.Name,
		req.Order,
		req.Color,
		req.IsClosing,
	).Scan(&st.ID, &st.PipelineID, &st.Name, &st.Order, &st.Color, &st.IsClosing, &st.CreatedAt, &st.UpdatedAt)
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
