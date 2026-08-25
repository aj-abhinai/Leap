package lead

import (
	"encoding/json"
	"time"
)

// Lead is an opportunity moving through a pipeline. Identity comes from its
// linked contact; the display name prefers the lead nickname.
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

// CreateRequest is the lead creation payload. It takes either an existing
// contact_id or a new_contact to resolve-or-create; stage_id must belong to
// pipeline_id and cannot be a closing stage.
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

// UpdateRequest is the lead partial-update payload. Stage moves resolve the
// outcome from the target stage's metadata and record stage history.
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

// ListFilters drives the lead list (GET /api/leads). Search matches the lead
// nickname, contact name, primary phone, primary email, or program name;
// Outcome filters on the linked stage's outcome ('open' | 'won' | 'lost');
// AssignedTo filters on the assignee (a user id or "none" for unassigned).
type ListFilters struct {
	PipelineID string
	StageID    string
	ContactID  string
	Search     string
	Outcome    string
	AssignedTo string
}

// Activity is a task on a lead with a lifecycle: scheduled, done, cancelled,
// or responded via a quick reply.
type Activity struct {
	ID             string     `json:"id"`
	LeadID         string     `json:"lead_id"`
	StageID        string     `json:"stage_id"`
	StageName      string     `json:"stage_name,omitempty"`
	UserID         *string    `json:"user_id,omitempty"`
	UserName       string     `json:"user_name,omitempty"`
	Type           string     `json:"type"`
	Description    string     `json:"description"`
	QuickReplyID   string     `json:"quick_reply_id,omitempty"`
	QuickReplyName string     `json:"quick_reply_name,omitempty"`
	ScheduledAt    *time.Time `json:"scheduled_at,omitempty"`
	RemindAt       *time.Time `json:"remind_at,omitempty"`
	RespondedAt    *time.Time `json:"responded_at,omitempty"`
	// OccurredAt is when the activity actually happened (stamped on completion).
	OccurredAt  *time.Time `json:"occurred_at,omitempty"`
	IsDone      bool       `json:"is_done"`
	IsCancelled bool       `json:"is_cancelled"`
	IsReminded  bool       `json:"is_reminded"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type CreateActivityRequest struct {
	Type         string     `json:"type"`
	Description  string     `json:"description"`
	QuickReplyID *string    `json:"quick_reply_id,omitempty"`
	ScheduledAt  *time.Time `json:"scheduled_at,omitempty"`
	RemindAt     *time.Time `json:"remind_at,omitempty"`
	// RescheduleAt, when set with a quick reply, logs the completed attempt and
	// auto-creates the next occurrence of the same type at this time (the
	// create-form equivalent of updateActivity's reschedule flow).
	RescheduleAt *time.Time `json:"reschedule_at,omitempty"`
	// IsDone, when true, creates the activity already completed (e.g. a
	// close_lost quick reply logged from the create form). occurred_at is
	// stamped.
	IsDone *bool `json:"is_done,omitempty"`
}

type UpdateActivityRequest struct {
	QuickReplyID *string `json:"quick_reply_id,omitempty"`
	IsDone       *bool   `json:"is_done,omitempty"`
	Type         *string `json:"type,omitempty"`
	Description  *string `json:"description,omitempty"`
	// ScheduledAt and RemindAt use optionalTime so an update can distinguish
	// "not sent" (keep the current value) from an explicit null (clear it).
	// A plain *time.Time cannot tell the two apart — both decode to nil — which
	// made schedules impossible to clear via the edit form.
	ScheduledAt  optionalTime `json:"scheduled_at,omitempty"`
	RemindAt     optionalTime `json:"remind_at,omitempty"`
	OccurredAt   *time.Time   `json:"occurred_at,omitempty"`
	IsCancelled  *bool        `json:"is_cancelled,omitempty"`
	// RescheduleAt, when set with is_done=true, logs the completed attempt and
	// auto-creates the next occurrence of the same type at this time.
	RescheduleAt *time.Time `json:"reschedule_at,omitempty"`
}

// optionalTime decodes a nullable timestamp while recording whether the field
// was present at all. Set is true when the key appears in the JSON body —
// either with a value (Value non-nil) or as null (Value nil). An absent key
// leaves Set false and Value nil.
type optionalTime struct {
	Set   bool
	Value *time.Time
}

func (o *optionalTime) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		return nil
	}
	var t time.Time
	if err := json.Unmarshal(b, &t); err != nil {
		return err
	}
	o.Value = &t
	return nil
}

// optTime builds an optionalTime with an explicit value (helper for tests and
// in-process callers that construct request structs directly).
func optTime(t *time.Time) optionalTime {
	if t == nil {
		return optionalTime{Set: true}
	}
	return optionalTime{Set: true, Value: t}
}

type SnoozeRequest struct {
	// RemindAt is the new reminder time.
	RemindAt time.Time `json:"remind_at"`
}

// ActivityListFilters drives the global activities list (GET /api/activities).
type ActivityListFilters struct {
	Status   string
	Overdue  bool
	Mine     bool
	UserID   string
	Type     string
	Search   string
	From     *time.Time
	To       *time.Time
	Sort     string
	Order    string
	Page     int
	PerPage  int
}

// ActivityListItem is an Activity plus the lead context needed by the global
// activities list: the lead display name and the linked contact id.
type ActivityListItem struct {
	Activity
	LeadDisplayName string `json:"lead_display_name"`
	ContactID       string `json:"contact_id"`
}

// StageHistory records a lead's stage move with the from/to stage names
// snapshotted at move time and the actor who performed it.
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
