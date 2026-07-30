package activity

import (
	"encoding/json"
	"time"
)

type Entry struct {
	ID           string           `json:"id"`
	Description  string           `json:"description"`
	UserID       *string          `json:"user_id,omitempty"`
	UserName     *string          `json:"user_name,omitempty"`
	Action       string           `json:"action"`
	ResourceType string           `json:"resource_type"`
	ResourceID   *string          `json:"resource_id,omitempty"`
	Changes      *json.RawMessage `json:"changes,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
}

type ActivityFilters struct {
	UserID       string
	Action       string
	ResourceType string
}
