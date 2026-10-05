package contact

import (
	"crm/internal/settings"
	"crm/internal/util"
	"database/sql"
	"errors"
	"fmt"
)

var (
	// ErrDuplicate marks a create or update whose primary phone or email
	// collides with a live contact and the request did not confirm the
	// duplicate. It wraps the matched contact(s) for a 409 response.
	ErrDuplicate = errors.New("duplicate contact")
)

// DuplicateError carries the live-contact matches for a rejected duplicate
// write, so the handler can surface which contact already exists.
type DuplicateError struct {
	Matches []DuplicateMatch
}

func (e *DuplicateError) Error() string { return ErrDuplicate.Error() }
func (e *DuplicateError) Unwrap() error { return ErrDuplicate }

// User-facing duplicate strings; every one lives here so the write paths
// cannot drift apart in wording.
const (
	dupReasonPhone   = "phone matches an existing contact"
	dupReasonEmail   = "email matches an existing contact"
	dupReasonBoth    = "phone and email match an existing contact"
	dupConfirmedWarn = "phone or email matches an existing contact"
)

// duplicates owns what counts as a duplicate: one key derivation that
// matches both phone storage generations, and one email normalization. The
// three write paths keep their own policies (reject-and-confirm on create
// and update, skip-and-report on bulk import) but all ask this one type.
type duplicates struct {
	db *sql.DB
}

// newDuplicates creates the duplicate rule backed by db.
func newDuplicates(db *sql.DB) *duplicates {
	return &duplicates{db: db}
}

// check returns the live contacts whose primary or secondary phone/email
// collides with the given phone and email after normalization. excludeID
// skips one contact (the row being edited); an empty id excludes nothing.
// It accepts *sql.DB and *sql.Tx so the update path can run inside its
// transaction.
func (d *duplicates) check(q interface {
	Query(query string, args ...any) (*sql.Rows, error)
}, phone, email, excludeID string) ([]DuplicateMatch, error) {
	e := util.NormalizeEmail(email)
	// The phone is matched in both storage generations: canonical rows store
	// country-coded digits, legacy rows store the national form.
	phoneKey, codedKey := "", ""
	if phone != "" {
		defaultCC, err := settings.DefaultCountryCode(d.db)
		if err != nil {
			return nil, err
		}
		phoneKey, codedKey = util.PhoneLookupKeys(phone, defaultCC)
	}
	if phoneKey == "" && e == "" {
		return nil, nil
	}
	// Match on the primary phone/email of existing live contacts, plus any
	// phone/email value so a secondary value also surfaces as a duplicate.
	rows, err := q.Query(
		`SELECT DISTINCT c.id, c.name,
			COALESCE(pcp.value, ''), COALESCE(ece.value, '')
		FROM contacts c
		LEFT JOIN LATERAL (
			SELECT value FROM contact_phones WHERE contact_id = c.id AND is_primary LIMIT 1
		) pcp ON true
		LEFT JOIN LATERAL (
			SELECT value FROM contact_emails WHERE contact_id = c.id AND is_primary LIMIT 1
		) ece ON true
		WHERE c.deleted_at IS NULL
		  AND ($4 = '' OR c.id <> NULLIF($4, '')::uuid)
		  AND (
			($1 <> '' AND EXISTS (
				SELECT 1 FROM contact_phones cp
				WHERE cp.contact_id = c.id
				  AND `+util.PhoneMatchCond("cp.value", "$1", "$2")+`
			))
			OR
			($3 <> '' AND EXISTS (
				SELECT 1 FROM contact_emails ce
				WHERE ce.contact_id = c.id
				  AND lower(trim(ce.value)) = $3
			))
		  )`,
		phoneKey, codedKey, e,
		excludeID,
	)
	if err != nil {
		return nil, fmt.Errorf("find duplicate contacts: %w", err)
	}
	defer rows.Close()
	matches := []DuplicateMatch{}
	for rows.Next() {
		var m DuplicateMatch
		if err := rows.Scan(&m.ID, &m.Name, &m.Phone, &m.Email); err != nil {
			return nil, fmt.Errorf("find duplicate contacts: scan: %w", err)
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find duplicate contacts: iterate: %w", err)
	}
	return matches, nil
}

// dupSnapshot holds the normalized duplicate keys of every live contact,
// loaded once per bulk import. Recording accepted rows adds their keys so
// later rows in the same file are caught too.
type dupSnapshot struct {
	// cc is the workspace default country code, applied to every key so both
	// storage generations (canonical country-coded, legacy national) collapse
	// to one national-form key.
	cc     string
	phones map[string]bool
	emails map[string]bool
}

// snapshot loads the normalized keys of every live contact once, for
// row-by-row checks during bulk import.
func (d *duplicates) snapshot() (*dupSnapshot, error) {
	cc, err := settings.DefaultCountryCode(d.db)
	if err != nil {
		return nil, err
	}
	s := &dupSnapshot{cc: cc, phones: map[string]bool{}, emails: map[string]bool{}}
	rows, err := d.db.Query(`
		SELECT cp.value FROM contact_phones cp
		JOIN contacts c ON c.id = cp.contact_id AND c.deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("load contact phones: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var phone string
		if err := rows.Scan(&phone); err != nil {
			return nil, fmt.Errorf("scan contact phone: %w", err)
		}
		if p := util.NormalizePhoneKey(phone, cc); p != "" {
			s.phones[p] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load contact phones: iterate: %w", err)
	}
	rows.Close()

	rows, err = d.db.Query(`
		SELECT ce.value FROM contact_emails ce
		JOIN contacts c ON c.id = ce.contact_id AND c.deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("load contact emails: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, fmt.Errorf("scan contact email: %w", err)
		}
		if e := util.NormalizeEmail(email); e != "" {
			s.emails[e] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load contact emails: iterate: %w", err)
	}
	rows.Close()
	return s, nil
}

// reason returns "" when the row is clean, else a human-readable skip reason
// naming which value matched.
func (s *dupSnapshot) reason(phone, email string) string {
	p := util.NormalizePhoneKey(phone, s.cc)
	e := util.NormalizeEmail(email)
	phoneMatch := p != "" && s.phones[p]
	emailMatch := e != "" && s.emails[e]
	switch {
	case phoneMatch && emailMatch:
		return dupReasonBoth
	case phoneMatch:
		return dupReasonPhone
	case emailMatch:
		return dupReasonEmail
	default:
		return ""
	}
}

// record registers a successfully imported row so later rows in the same
// file are caught as same-file duplicates.
func (s *dupSnapshot) record(phone, email string) {
	if p := util.NormalizePhoneKey(phone, s.cc); p != "" {
		s.phones[p] = true
	}
	if e := util.NormalizeEmail(email); e != "" {
		s.emails[e] = true
	}
}
