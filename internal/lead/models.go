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
	Outcome      string     `json:"outcome,omitempty"`
	LostReason   string     `json:"lost_reason,omitempty"`
	ProgramID    *string    `json:"program_id,omitempty"`
	ProgramName  string     `json:"program_name,omitempty"`
	Value        *float64   `json:"value,omitempty"`
	Notes        string     `json:"notes,omitempty"`
	AssignedTo   *string    `json:"assigned_to,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
	// NextTaskType/NextTaskAt preview the soonest open scheduled activity.
	NextTaskType string     `json:"next_task_type,omitempty"`
	NextTaskAt   *time.Time `json:"next_task_at,omitempty"`
	// LastTouchType/LastTouchAt describe the most recent completed touchpoint.
	LastTouchType string     `json:"last_touch_type,omitempty"`
	LastTouchAt   *time.Time `json:"last_touch_at,omitempty"`
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
	LostReason *string  `json:"lost_reason,omitempty"`
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
	OutcomeID   string     `json:"outcome_id,omitempty"`
	OutcomeName string     `json:"outcome_name,omitempty"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	RemindAt    *time.Time `json:"remind_at,omitempty"`
	RespondedAt *time.Time `json:"responded_at,omitempty"`
	// OccurredAt is when the activity actually happened (stamped on completion).
	OccurredAt  *time.Time `json:"occurred_at,omitempty"`
	IsDone      bool       `json:"is_done"`
	IsCancelled bool       `json:"is_cancelled"`
	IsReminded  bool       `json:"is_reminded"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type CreateActivityRequest struct {
	Type        string     `json:"type"`
	Description string     `json:"description"`
	OutcomeID   *string    `json:"outcome_id,omitempty"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	RemindAt    *time.Time `json:"remind_at,omitempty"`
	// RescheduleAt, when set with an outcome, logs the completed attempt and
	// auto-creates the next occurrence of the same type at this time (the
	// create-form equivalent of updateActivity's reschedule flow).
	RescheduleAt *time.Time `json:"reschedule_at,omitempty"`
	// IsDone, when true, creates the activity already completed (e.g. a
	// close_lost outcome logged from the create form). occurred_at is stamped.
	IsDone *bool `json:"is_done,omitempty"`
}

type UpdateActivityRequest struct {
	OutcomeID   *string    `json:"outcome_id,omitempty"`
	IsDone      *bool      `json:"is_done,omitempty"`
	Type        *string    `json:"type,omitempty"`
	Description *string    `json:"description,omitempty"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	RemindAt    *time.Time `json:"remind_at,omitempty"`
	OccurredAt  *time.Time `json:"occurred_at,omitempty"`
	IsCancelled *bool      `json:"is_cancelled,omitempty"`
	// RescheduleAt, when set with is_done=true, logs the completed attempt and
	// auto-creates the next occurrence of the same type at this time.
	RescheduleAt *time.Time `json:"reschedule_at,omitempty"`
}

type SnoozeRequest struct {
	// RemindAt is the new reminder time.
	RemindAt time.Time `json:"remind_at"`
}

type StageHistory struct {
	ID            string    `json:"id"`
	LeadID        string    `json:"lead_id"`
	FromStageID   string    `json:"from_stage_id,omitempty"`
	ToStageID     string    `json:"to_stage_id,omitempty"`
	FromStageName string    `json:"from_stage_name,omitempty"`
	ToStageName   string    `json:"to_stage_name,omitempty"`
	UserID        *string   `json:"user_id,omitempty"`
	MovedAt       time.Time `json:"moved_at"`
}
