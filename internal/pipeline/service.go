package pipeline

import (
	"database/sql"
	"fmt"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List() ([]Pipeline, error) {
	pipelines, err := s.listPipelines()
	if err != nil {
		return nil, err
	}
	for i := range pipelines {
		stages, err := s.ListStages(pipelines[i].ID)
		if err != nil {
			return nil, err
		}
		pipelines[i].Stages = stages
	}
	return pipelines, nil
}

func (s *Service) listPipelines() ([]Pipeline, error) {
	rows, err := s.db.Query(`SELECT id, name, COALESCE(description, ''), created_at, updated_at FROM pipelines ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}
	defer rows.Close()
	var pipelines []Pipeline
	for rows.Next() {
		var p Pipeline
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		pipelines = append(pipelines, p)
	}
	return pipelines, nil
}

func (s *Service) ListStages(pipelineID string) ([]Stage, error) {
	rows, err := s.db.Query(
		`SELECT id, pipeline_id, name, "order", COALESCE(color, ''), created_at, updated_at FROM lead_stages WHERE pipeline_id = $1 ORDER BY "order"`,
		pipelineID,
	)
	if err != nil {
		return nil, fmt.Errorf("list stages: %w", err)
	}
	defer rows.Close()
	var stages []Stage
	for rows.Next() {
		var st Stage
		if err := rows.Scan(&st.ID, &st.PipelineID, &st.Name, &st.Order, &st.Color, &st.CreatedAt, &st.UpdatedAt); err != nil {
			return nil, err
		}
		stages = append(stages, st)
	}
	return stages, nil
}

func (s *Service) CreatePipeline(req CreatePipelineRequest) (*Pipeline, error) {
	var p Pipeline
	err := s.db.QueryRow(
		`INSERT INTO pipelines (name, description) VALUES ($1, $2) RETURNING id, name, COALESCE(description, ''), created_at, updated_at`,
		req.Name, req.Description,
	).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create pipeline: %w", err)
	}
	return &p, nil
}

func (s *Service) UpdatePipeline(id string, req UpdatePipelineRequest) (*Pipeline, error) {
	var p Pipeline
	err := s.db.QueryRow(
		`UPDATE pipelines SET name = COALESCE($2, name), description = COALESCE($3, description), updated_at = now() WHERE id = $1 RETURNING id, name, COALESCE(description, ''), created_at, updated_at`,
		id, req.Name, req.Description,
	).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update pipeline: %w", err)
	}
	return &p, nil
}

func (s *Service) DeletePipeline(id string) error {
	_, err := s.db.Exec(`DELETE FROM pipelines WHERE id = $1`, id)
	return err
}

func (s *Service) CreateStage(pipelineID string, req CreateStageRequest) (*Stage, error) {
	var st Stage
	err := s.db.QueryRow(
		`INSERT INTO lead_stages (pipeline_id, name, "order", color) VALUES ($1, $2, $3, $4) RETURNING id, pipeline_id, name, "order", COALESCE(color, ''), created_at, updated_at`,
		pipelineID, req.Name, req.Order, nullStr(req.Color),
	).Scan(&st.ID, &st.PipelineID, &st.Name, &st.Order, &st.Color, &st.CreatedAt, &st.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create stage: %w", err)
	}
	return &st, nil
}

func (s *Service) UpdateStage(stageID string, req UpdateStageRequest) (*Stage, error) {
	var st Stage
	err := s.db.QueryRow(
		`UPDATE lead_stages SET name = COALESCE($2, name), "order" = COALESCE($3, "order"), color = COALESCE($4, color), updated_at = now() WHERE id = $1 RETURNING id, pipeline_id, name, "order", COALESCE(color, ''), created_at, updated_at`,
		stageID, req.Name, req.Order, req.Color,
	).Scan(&st.ID, &st.PipelineID, &st.Name, &st.Order, &st.Color, &st.CreatedAt, &st.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update stage: %w", err)
	}
	return &st, nil
}

func (s *Service) DeleteStage(stageID string) error {
	_, err := s.db.Exec(`DELETE FROM lead_stages WHERE id = $1`, stageID)
	return err
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
