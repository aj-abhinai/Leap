package contact

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

type BulkCreateRequest struct {
	Contacts []BulkContact `json:"contacts"`
}

type BulkContact struct {
	Name     string   `json:"name"`
	Email    string   `json:"email,omitempty"`
	Phone    string   `json:"phone,omitempty"`
	Location string   `json:"location,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

type BulkCreateResponse struct {
	Imported int            `json:"imported"`
	Failed   int            `json:"failed"`
	Errors   []BulkRowError `json:"errors,omitempty"`
}

type BulkRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

var nonDigit = regexp.MustCompile(`\D`)

// normalizePhone keeps only ASCII digits so identical numbers with different
// formatting collapse to one lookup key.
func normalizePhone(phone string) string {
	return nonDigit.ReplaceAllString(phone, "")
}

// normalizeEmail trims and lowercases so case/whitespace differences collapse
// to one lookup key.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

type contactKeys struct {
	phones map[string]bool
	emails map[string]bool
}

func (s *Service) loadContactKeys() (contactKeys, error) {
	keys := contactKeys{phones: map[string]bool{}, emails: map[string]bool{}}
	rows, err := s.db.Query(
		`SELECT COALESCE(phone, ''), COALESCE(email, '') FROM contacts WHERE deleted_at IS NULL`,
	)
	if err != nil {
		return keys, fmt.Errorf("load contact keys: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var phone, email string
		if err := rows.Scan(&phone, &email); err != nil {
			return keys, fmt.Errorf("scan contact key: %w", err)
		}
		if p := normalizePhone(phone); p != "" {
			keys.phones[p] = true
		}
		if e := normalizeEmail(email); e != "" {
			keys.emails[e] = true
		}
	}
	return keys, rows.Err()
}

// recordMatch registers a successfully imported row so later rows in the same
// file are caught as same-file duplicates.
func (k *contactKeys) recordMatch(phone, email string) {
	if p := normalizePhone(phone); p != "" {
		k.phones[p] = true
	}
	if e := normalizeEmail(email); e != "" {
		k.emails[e] = true
	}
}

func (k contactKeys) duplicateReason(phone, email string) string {
	phoneMatch := normalizePhone(phone) != "" && k.phones[normalizePhone(phone)]
	emailMatch := normalizeEmail(email) != "" && k.emails[normalizeEmail(email)]
	switch {
	case phoneMatch && emailMatch:
		return "phone and email match an existing contact"
	case phoneMatch:
		return "phone matches an existing contact"
	case emailMatch:
		return "email matches an existing contact"
	default:
		return ""
	}
}

func (s *Service) bulkCreate(req BulkCreateRequest) (*BulkCreateResponse, error) {
	resp := &BulkCreateResponse{}

	tagIDs, err := s.loadTagIDs(req.Contacts)
	if err != nil {
		return nil, err
	}

	keys, err := s.loadContactKeys()
	if err != nil {
		return nil, err
	}

	for i, c := range req.Contacts {
		if c.Name == "" {
			resp.Failed++
			resp.Errors = append(resp.Errors, BulkRowError{Row: i + 1, Message: "name is required"})
			continue
		}
		if reason := keys.duplicateReason(c.Phone, c.Email); reason != "" {
			resp.Failed++
			resp.Errors = append(resp.Errors, BulkRowError{Row: i + 1, Message: reason})
			continue
		}
		tagIDsForContact := []string{}
		for _, tagName := range c.Tags {
			tagIDsForContact = append(tagIDsForContact, tagIDs[strings.TrimSpace(tagName)]...)
		}
		_, err := s.create(CreateRequest{
			Name:     c.Name,
			Email:    c.Email,
			Phone:    c.Phone,
			Location: c.Location,
			TagIDs:   tagIDsForContact,
		})
		if err != nil {
			resp.Failed++
			slog.Error("bulk import row failed", "row", i+1, "error", err)
			resp.Errors = append(resp.Errors, BulkRowError{Row: i + 1, Message: "failed to create contact"})
			continue
		}
		keys.recordMatch(c.Phone, c.Email)
		resp.Imported++
	}
	return resp, nil
}

// loadTagIDs resolves every tag name in the import with one query instead of a
// per-row lookup.
func (s *Service) loadTagIDs(contacts []BulkContact) (map[string][]string, error) {
	seen := map[string]bool{}
	names := []string{}
	for _, c := range contacts {
		for _, tag := range c.Tags {
			name := strings.TrimSpace(tag)
			if name != "" && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	result := map[string][]string{}
	if len(names) == 0 {
		return result, nil
	}
	rows, err := s.db.Query(
		`SELECT name, id FROM tags WHERE type = 'tag' AND name = ANY($1)`,
		names,
	)
	if err != nil {
		return nil, fmt.Errorf("load tags: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, id string
		if err := rows.Scan(&name, &id); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		result[name] = append(result[name], id)
	}
	return result, rows.Err()
}
