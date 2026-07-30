package contact

import "time"

type Contact struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Email     string     `json:"email,omitempty"`
	Phone     string     `json:"phone,omitempty"`
	Location  string     `json:"location,omitempty"`
	Age       *int       `json:"age,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type CreateRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Location string `json:"location,omitempty"`
	Age      *int   `json:"age,omitempty"`
}

type UpdateRequest struct {
	Name     *string `json:"name,omitempty"`
	Email    *string `json:"email,omitempty"`
	Phone    *string `json:"phone,omitempty"`
	Location *string `json:"location,omitempty"`
	Age      *int    `json:"age,omitempty"`
}

type ListResponse struct {
	Contacts []Contact `json:"contacts"`
	Total    int       `json:"total"`
}
