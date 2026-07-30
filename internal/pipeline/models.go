package pipeline

import "time"

type Pipeline struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Stages      []Stage   `json:"stages,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Stage struct {
	ID         string    `json:"id"`
	PipelineID string    `json:"pipeline_id"`
	Name       string    `json:"name"`
	Order      int       `json:"order"`
	Color      string    `json:"color,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type CreatePipelineRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type UpdatePipelineRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type CreateStageRequest struct {
	Name  string `json:"name"`
	Order int    `json:"order"`
	Color string `json:"color,omitempty"`
}

type UpdateStageRequest struct {
	Name  *string `json:"name,omitempty"`
	Order *int    `json:"order,omitempty"`
	Color *string `json:"color,omitempty"`
}
