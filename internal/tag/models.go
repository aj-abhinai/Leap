package tag

import "time"

type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Color     string    `json:"color,omitempty"`
	GroupName string    `json:"group_name,omitempty"`
	SortOrder int       `json:"sort_order"`
	Behavior  string    `json:"behavior"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateRequest struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Color     string `json:"color,omitempty"`
	GroupName string `json:"group_name,omitempty"`
	SortOrder int    `json:"sort_order"`
	Behavior  string `json:"behavior"`
}

type UpdateRequest struct {
	Name      *string `json:"name,omitempty"`
	Color     *string `json:"color,omitempty"`
	GroupName *string `json:"group_name,omitempty"`
	SortOrder *int    `json:"sort_order,omitempty"`
	Behavior  *string `json:"behavior,omitempty"`
}
