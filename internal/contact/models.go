package contact

import "time"

// PhoneValue is a single phone on a contact. Exactly one per contact is primary.
type PhoneValue struct {
	ID        string `json:"id"`
	Value     string `json:"value"`
	IsPrimary bool   `json:"is_primary"`
}

// EmailValue is a single email on a contact. Exactly one per contact is primary.
type EmailValue struct {
	ID        string `json:"id"`
	Value     string `json:"value"`
	IsPrimary bool   `json:"is_primary"`
}

// Contact is the identity source of truth for leads. It carries the primary
// phone/email scalars for compact views plus the full multi-valued lists.
type Contact struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Nickname  string       `json:"nickname,omitempty"`
	Email     string       `json:"email,omitempty"`
	Phone     string       `json:"phone,omitempty"`
	Phones    []PhoneValue `json:"phones,omitempty"`
	Emails    []EmailValue `json:"emails,omitempty"`
	Location  string       `json:"location,omitempty"`
	Age       *int         `json:"age,omitempty"`
	Tags      []TagRef     `json:"tags,omitempty"`
	Status    *TagRef      `json:"status,omitempty"`
	Warnings  []string     `json:"warnings,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
	DeletedAt *time.Time   `json:"deleted_at,omitempty"`
}

// TagRef is a compact tag reference (id, name, color) used on contacts.
type TagRef struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

// CreateRequest is the contact creation payload. A contact must carry at
// least one phone or one email. ConfirmDuplicates acknowledges that the
// created phone/email may collide with an existing live contact.
type CreateRequest struct {
	Name              string       `json:"name"`
	Nickname          string       `json:"nickname,omitempty"`
	Email             string       `json:"email,omitempty"`
	Phone             string       `json:"phone,omitempty"`
	Phones            []PhoneValue `json:"phones,omitempty"`
	Emails            []EmailValue `json:"emails,omitempty"`
	Location          string       `json:"location,omitempty"`
	Age               *int         `json:"age,omitempty"`
	TagIDs            []string     `json:"tag_ids,omitempty"`
	StatusID          *string      `json:"status_id,omitempty"`
	ConfirmDuplicates bool         `json:"confirm_duplicates,omitempty"`
}

// DuplicateMatch is a live contact whose primary phone or email collides with
// the phone/email being created. It is returned in a 409 so the UI can show
// which contact already exists before the user confirms the duplicate.
type DuplicateMatch struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone,omitempty"`
	Email string `json:"email,omitempty"`
}

// UpdateRequest is the partial-update payload. Scalar phone/email fields
// mirror into the child tables; the list fields replace their whole type and
// are only applied when sent.
type UpdateRequest struct {
	Name     *string       `json:"name,omitempty"`
	Nickname *string       `json:"nickname,omitempty"`
	Email    *string       `json:"email,omitempty"`
	Phone    *string       `json:"phone,omitempty"`
	Phones   *[]PhoneValue `json:"phones,omitempty"`
	Emails   *[]EmailValue `json:"emails,omitempty"`
	Location *string       `json:"location,omitempty"`
	Age      *int          `json:"age,omitempty"`
	TagIDs   []string      `json:"tag_ids,omitempty"`
	StatusID *string       `json:"status_id,omitempty"`
}

type ListResponse struct {
	Contacts []Contact `json:"contacts"`
	Total    int       `json:"total"`
}

// Note is a free-text note on a contact, attributed to its author.
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

// ResolveMatch is a compact contact result for the lead-entry phone resolve
// endpoint: id, name, and the primary phone/email for display.
type ResolveMatch struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone,omitempty"`
	Email string `json:"email,omitempty"`
}
