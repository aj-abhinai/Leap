package lead

import "time"

type Lead struct {
	ID           string     `json:"id"`
	Nickname     string     `json:"nickname,omitempty"`
	DisplayName  string     `json:"display_name"`
	ContactID    string     `json:"contact_id"`
	ContactName  string     `json:"contact_name,omitempty"`
	ContactPhone string     `json:"contact_phone,omitempty"`
	ContactEmail string     `json:"contact_email,omitempty"`
	PipelineID   string     `json:"pipeline_id"`
	StageID      string     `json:"stage_id"`
	StageName    string     `json:"stage_name,omitempty"`
	ProgramID    *string    `json:"program_id,omitempty"`
	ProgramName  string     `json:"program_name,omitempty"`
	Value        *float64   `json:"value,omitempty"`
	Notes        string     `json:"notes,omitempty"`
	AssignedTo   *string    `json:"assigned_to,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}

// NewContact holds the contact details to resolve-or-create when a lead is
// created without an explicit contact_id.
type NewContact struct {
	Name  string `json:"name"`
	Phone string `json:"phone,omitempty"`
	Email string `json:"email,omitempty"`
}

type CreateRequest struct {
	Nickname   string      `json:"nickname,omitempty"`
	ContactID  *string     `json:"contact_id,omitempty"`
	NewContact *NewContact `json:"new_contact,omitempty"`
	PipelineID string      `json:"pipeline_id"`
	StageID    string      `json:"stage_id"`
	ProgramID  *string     `json:"program_id,omitempty"`
	Value      *float64    `json:"value,omitempty"`
	Notes      string      `json:"notes,omitempty"`
	AssignedTo *string     `json:"assigned_to,omitempty"`
}

type UpdateRequest struct {
	Nickname   *string  `json:"nickname,omitempty"`
	ContactID  *string  `json:"contact_id,omitempty"`
	PipelineID *string  `json:"pipeline_id,omitempty"`
	StageID    *string  `json:"stage_id,omitempty"`
	ProgramID  *string  `json:"program_id,omitempty"`
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
