package lead

import "time"

type Lead struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Email      string     `json:"email,omitempty"`
	Phone      string     `json:"phone,omitempty"`
	ContactID  *string    `json:"contact_id,omitempty"`
	PipelineID string     `json:"pipeline_id"`
	StageID    string     `json:"stage_id"`
	StageName  string     `json:"stage_name,omitempty"`
	Value      *float64   `json:"value,omitempty"`
	Notes      string     `json:"notes,omitempty"`
	AssignedTo *string    `json:"assigned_to,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

type CreateRequest struct {
	Name       string   `json:"name"`
	Email      string   `json:"email,omitempty"`
	Phone      string   `json:"phone,omitempty"`
	ContactID  *string  `json:"contact_id,omitempty"`
	PipelineID string   `json:"pipeline_id"`
	StageID    string   `json:"stage_id"`
	Value      *float64 `json:"value,omitempty"`
	Notes      string   `json:"notes,omitempty"`
	AssignedTo *string  `json:"assigned_to,omitempty"`
}

type UpdateRequest struct {
	Name       *string  `json:"name,omitempty"`
	Email      *string  `json:"email,omitempty"`
	Phone      *string  `json:"phone,omitempty"`
	ContactID  *string  `json:"contact_id,omitempty"`
	PipelineID *string  `json:"pipeline_id,omitempty"`
	StageID    *string  `json:"stage_id,omitempty"`
	Value      *float64 `json:"value,omitempty"`
	Notes      *string  `json:"notes,omitempty"`
	AssignedTo *string  `json:"assigned_to,omitempty"`
}
