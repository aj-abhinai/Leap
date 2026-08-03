package contact

import (
	"crm/internal/util"
	"fmt"
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
	// phones and emails come from the child tables (contact_phones/contact_emails);
	// legacy scalar columns are also scanned so dedupe still works for any rows
	// created before the child tables were populated.
	rows, err := s.db.Query(`
		SELECT COALESCE(cp.value, ''), COALESCE(ce.value, '')
		FROM contacts c
		LEFT JOIN LATERAL (
			SELECT value FROM contact_phones WHERE contact_id = c.id AND is_primary LIMIT 1
		) cp ON true
		LEFT JOIN LATERAL (
			SELECT value FROM contact_emails WHERE contact_id = c.id AND is_primary LIMIT 1
		) ce ON true
		WHERE c.deleted_at IS NULL
		UNION
		SELECT COALESCE(phone, ''), COALESCE(email, '') FROM contacts WHERE deleted_at IS NULL
	`)
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

type pendingContact struct {
	contact BulkContact
	tagIDs  []string
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

	pending := []pendingContact{}

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
		unknown := []string{}
		for _, tagName := range c.Tags {
			name := strings.TrimSpace(tagName)
			ids := tagIDs[name]
			if len(ids) == 0 {
				unknown = append(unknown, name)
			}
			tagIDsForContact = append(tagIDsForContact, ids...)
		}
		if len(unknown) > 0 {
			resp.Failed++
			resp.Errors = append(resp.Errors, BulkRowError{
				Row:     i + 1,
				Message: "unknown tag(s): " + strings.Join(unknown, ", "),
			})
			continue
		}
		pending = append(pending, pendingContact{contact: c, tagIDs: tagIDsForContact})
		keys.recordMatch(c.Phone, c.Email)
	}

	if len(pending) == 0 {
		return resp, nil
	}

	if err := s.insertContactsBulk(pending); err != nil {
		return nil, err
	}
	resp.Imported = len(pending)
	return resp, nil
}

// insertContactsBulk inserts every pre-validated row with one multi-row
// INSERT and attaches all contact tags with one batch statement, so a 500-row
// import costs a handful of queries instead of thousands.
func (s *Service) insertContactsBulk(pending []pendingContact) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("bulk create: %w", err)
	}
	defer tx.Rollback()

	args := make([]any, 0, len(pending)*6)
	placeholders := make([]string, 0, len(pending))
	argIdx := 1
	for _, p := range pending {
		placeholders = append(placeholders,
			fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d)", argIdx, argIdx+1, argIdx+2, argIdx+3, argIdx+4, argIdx+5))
		args = append(args,
			p.contact.Name,
			util.NullStr(p.contact.Email),
			util.NullStr(p.contact.Phone),
			util.NullStr(p.contact.Location),
			nil,
			nil,
		)
		argIdx += 6
	}

	query := `INSERT INTO contacts (name, email, phone, location, age, status_id) VALUES ` +
		strings.Join(placeholders, ", ") +
		` RETURNING id`
	rows, err := tx.Query(query, args...)
	if err != nil {
		return fmt.Errorf("bulk insert contacts: %w", err)
	}
	createdIDs := make([]string, 0, len(pending))
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		createdIDs = append(createdIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	tagContactIDs := []string{}
	tagIDsForInsert := []string{}
	phoneContactIDs := []string{}
	phoneValues := []string{}
	emailContactIDs := []string{}
	emailValues := []string{}
	for i, p := range pending {
		for _, tid := range p.tagIDs {
			tagContactIDs = append(tagContactIDs, createdIDs[i])
			tagIDsForInsert = append(tagIDsForInsert, tid)
		}
		if p.contact.Phone != "" {
			phoneContactIDs = append(phoneContactIDs, createdIDs[i])
			phoneValues = append(phoneValues, p.contact.Phone)
		}
		if p.contact.Email != "" {
			emailContactIDs = append(emailContactIDs, createdIDs[i])
			emailValues = append(emailValues, p.contact.Email)
		}
	}
	if len(tagContactIDs) > 0 {
		if _, err := tx.Exec(
			`INSERT INTO contact_tags (contact_id, tag_id)
			SELECT cid, tid FROM unnest($1::text[], $2::text[]) AS x(cid, tid)
			ON CONFLICT DO NOTHING`,
			tagContactIDs, tagIDsForInsert,
		); err != nil {
			return fmt.Errorf("bulk attach contact tags: %w", err)
		}
	}
	if len(phoneContactIDs) > 0 {
		if _, err := tx.Exec(
			`INSERT INTO contact_phones (contact_id, value, is_primary)
			SELECT cid, val, true FROM unnest($1::text[], $2::text[]) AS x(cid, val)`,
			phoneContactIDs, phoneValues,
		); err != nil {
			return fmt.Errorf("bulk insert contact phones: %w", err)
		}
	}
	if len(emailContactIDs) > 0 {
		if _, err := tx.Exec(
			`INSERT INTO contact_emails (contact_id, value, is_primary)
			SELECT cid, val, true FROM unnest($1::text[], $2::text[]) AS x(cid, val)`,
			emailContactIDs, emailValues,
		); err != nil {
			return fmt.Errorf("bulk insert contact emails: %w", err)
		}
	}

	return tx.Commit()
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
