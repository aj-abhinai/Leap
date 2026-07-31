package contact

import "time"

type Contact struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Email     string     `json:"email,omitempty"`
	Phone     string     `json:"phone,omitempty"`
	Location  string     `json:"location,omitempty"`
	Age       *int       `json:"age,omitempty"`
	Tags      []TagRef   `json:"tags,omitempty"`
	Status    *TagRef    `json:"status,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type TagRef struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

type CreateRequest struct {
	Name     string   `json:"name"`
	Email    string   `json:"email,omitempty"`
	Phone    string   `json:"phone,omitempty"`
	Location string   `json:"location,omitempty"`
	Age      *int     `json:"age,omitempty"`
	TagIDs   []string `json:"tag_ids,omitempty"`
	StatusID *string  `json:"status_id,omitempty"`
}

type UpdateRequest struct {
	Name     *string  `json:"name,omitempty"`
	Email    *string  `json:"email,omitempty"`
	Phone    *string  `json:"phone,omitempty"`
	Location *string  `json:"location,omitempty"`
	Age      *int     `json:"age,omitempty"`
	TagIDs   []string `json:"tag_ids,omitempty"`
	StatusID *string  `json:"status_id,omitempty"`
}

type ListResponse struct {
	Contacts []Contact `json:"contacts"`
	Total    int       `json:"total"`
}

type Note struct {
	ID        string    `json:"id"`
	ContactID string    `json:"contact_id"`
	UserID    *string   `json:"user_id,omitempty"`
	UserName  string    `json:"user_name,omitempty"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateNoteRequest struct {
	Note string `json:"note"`
}
