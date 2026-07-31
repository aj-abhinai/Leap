package tag

import "time"

type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Color     string    `json:"color,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateRequest struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Color string `json:"color,omitempty"`
}
