package lead

import "time"

type Lead struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Email        string     `json:"email,omitempty"`
	Phone        string     `json:"phone,omitempty"`
	ContactID    *string    `json:"contact_id,omitempty"`
	ContactName  string     `json:"contact_name,omitempty"`
	PipelineID   string     `json:"pipeline_id"`
	StageID      string     `json:"stage_id"`
	StageName    string     `json:"stage_name,omitempty"`
	Value        *float64   `json:"value,omitempty"`
	Notes        string     `json:"notes,omitempty"`
	AssignedTo   *string    `json:"assigned_to,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
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

type Activity struct {
	ID          string     `json:"id"`
	LeadID      string     `json:"lead_id"`
	StageID     string     `json:"stage_id"`
	StageName   string     `json:"stage_name,omitempty"`
	UserID      *string    `json:"user_id,omitempty"`
	UserName    string     `json:"user_name,omitempty"`
	Type        string     `json:"type"`
	Description string     `json:"description"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	RemindAt    *time.Time `json:"remind_at,omitempty"`
	IsDone      bool       `json:"is_done"`
	IsReminded  bool       `json:"is_reminded"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type CreateActivityRequest struct {
	Type        string     `json:"type"`
	Description string     `json:"description"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	RemindAt    *time.Time `json:"remind_at,omitempty"`
}
