package contact

import (
	"crm/internal/audit"
	"crm/internal/lead"
	"crm/internal/settings"
	"crm/internal/util"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrNotFound marks mutations targeting a contact that does not exist or
	// has been deleted.
	ErrNotFound = errors.New("contact not found")
	// ErrInvalidStatus marks status_id values that do not reference a
	// type='status' tag.
	ErrInvalidStatus = errors.New("status_id must reference a status tag")
	// ErrNoContactDetail marks contacts created without a phone or an email.
	ErrNoContactDetail = errors.New("contact must have at least one phone or one email")
	// ErrInvalidDateOfBirth marks a date of birth that is not a real calendar
	// date in YYYY-MM-DD form or lies in the future.
	ErrInvalidDateOfBirth = errors.New("date of birth must be a real date (YYYY-MM-DD) in the past")
	// ErrCollectionLimit marks requests whose phones, emails, or tag lists
	// exceed the per-contact caps or whose value lengths exceed the maximum.
	ErrCollectionLimit = errors.New("contact collection limit exceeded")
	// ErrInvalidEmail marks an email value that is not a plain address, so free
	// text cannot land in the email columns.
	ErrInvalidEmail = errors.New("invalid email address")
	// ErrDuplicate marks a create whose primary phone or email collides with
	// a live contact and the request did not confirm the duplicate. It wraps
	// the matched contact(s) so the handler can return them in a 409.
	ErrDuplicate = errors.New("duplicate contact")
)

// DuplicateError carries the live-contact matches for a rejected duplicate
// create, so the handler can surface which contact already exists.
type DuplicateError struct {
	Matches []DuplicateMatch
}

func (e *DuplicateError) Error() string { return ErrDuplicate.Error() }
func (e *DuplicateError) Unwrap() error { return ErrDuplicate }

// Per-contact collection and value caps bound the work a single create/update
// request can trigger: each element is inserted with its own Exec inside the
// mutation transaction, so unbounded arrays can exhaust the connection pool.
const (
	maxContactPhones = 10
	maxContactEmails = 10
	maxContactTags   = 50
	maxValueLength   = 255
)

// Service provides database-backed contact operations: list/create/update/
// delete, notes, bulk import, and phone resolution for lead entry.
type Service struct {
	db *sql.DB
}

// NewService creates a contact Service backed by db.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) list(page, perPage int, search string) ([]Contact, int, error) {
	var total int
	baseWhere := "WHERE deleted_at IS NULL"
	args := []any{}
	argIdx := 1

	if search != "" {
		baseWhere += fmt.Sprintf(
			` AND (name ILIKE $%d ESCAPE '\' OR nickname ILIKE $%d ESCAPE '\'
				OR EXISTS (SELECT 1 FROM contact_phones cp WHERE cp.contact_id = contacts.id AND cp.value ILIKE $%d ESCAPE '\')
				OR EXISTS (SELECT 1 FROM contact_emails ce WHERE ce.contact_id = contacts.id AND ce.value ILIKE $%d ESCAPE '\'))`,
			argIdx, argIdx+1, argIdx+2, argIdx+3,
		)
		searchTerm := util.LikePattern(search)
		args = append(args, searchTerm, searchTerm, searchTerm, searchTerm)
		argIdx += 4
	}

	countQuery := "SELECT COUNT(*) FROM contacts " + baseWhere
	err := s.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count contacts: %w", err)
	}

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	offset := util.Offset(page, perPage)

	selectQuery := fmt.Sprintf(
		`SELECT id, name, COALESCE(nickname, ''), COALESCE(location, ''), age, COALESCE(to_char(date_of_birth, 'YYYY-MM-DD'), ''), created_at, updated_at
		FROM contacts %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`,
		baseWhere, argIdx, argIdx+1,
	)
	args = append(args, perPage, offset)
	rows, err := s.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list contacts: %w", err)
	}
	defer rows.Close()

	contacts := []Contact{}
	contactIDs := []string{}
	for rows.Next() {
		var c Contact
		var dob string
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Nickname, &c.Location, &c.Age, &dob, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		if dob != "" {
			c.DateOfBirth = &dob
		}
		contacts = append(contacts, c)
		contactIDs = append(contactIDs, c.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list contacts: iterate: %w", err)
	}

	if len(contacts) > 0 {
		contacts, err = s.populateTagsAndStatus(contacts, contactIDs)
		if err != nil {
			return nil, 0, err
		}
		contacts, err = s.populatePhonesEmails(contacts)
		if err != nil {
			return nil, 0, err
		}
	}

	return contacts, total, nil
}

// populatePhonesEmails attaches the primary phone/email (lists/compact views) to
// each contact in one batched query, avoiding a join per value.
func (s *Service) populatePhonesEmails(contacts []Contact) ([]Contact, error) {
	if len(contacts) == 0 {
		return contacts, nil
	}
	ids := make([]string, len(contacts))
	for i, c := range contacts {
		ids[i] = c.ID
	}

	// primary phone per contact
	phoneRows, err := s.db.Query(
		`SELECT DISTINCT ON (contact_id) contact_id, id, value
		FROM contact_phones
		WHERE contact_id = ANY($1) AND is_primary
		ORDER BY contact_id, created_at`,
		ids,
	)
	if err != nil {
		return contacts, fmt.Errorf("load primary phones: %w", err)
	}
	defer phoneRows.Close()
	phoneByContact := map[string]PhoneValue{}
	for phoneRows.Next() {
		var cid, id, value string
		if err := phoneRows.Scan(&cid, &id, &value); err != nil {
			return contacts, fmt.Errorf("scan primary phone: %w", err)
		}
		phoneByContact[cid] = PhoneValue{ID: id, Value: value, IsPrimary: true}
	}
	if err := phoneRows.Err(); err != nil {
		return contacts, fmt.Errorf("load primary phones: iterate: %w", err)
	}

	// primary email per contact
	emailRows, err := s.db.Query(
		`SELECT DISTINCT ON (contact_id) contact_id, id, value
		FROM contact_emails
		WHERE contact_id = ANY($1) AND is_primary
		ORDER BY contact_id, created_at`,
		ids,
	)
	if err != nil {
		return contacts, fmt.Errorf("load primary emails: %w", err)
	}
	defer emailRows.Close()
	emailByContact := map[string]EmailValue{}
	for emailRows.Next() {
		var cid, id, value string
		if err := emailRows.Scan(&cid, &id, &value); err != nil {
			return contacts, fmt.Errorf("scan primary email: %w", err)
		}
		emailByContact[cid] = EmailValue{ID: id, Value: value, IsPrimary: true}
	}
	if err := emailRows.Err(); err != nil {
		return contacts, fmt.Errorf("load primary emails: iterate: %w", err)
	}

	for i := range contacts {
		if p, ok := phoneByContact[contacts[i].ID]; ok {
			contacts[i].Phone = p.Value
			contacts[i].Phones = []PhoneValue{p}
		}
		if e, ok := emailByContact[contacts[i].ID]; ok {
			contacts[i].Email = e.Value
			contacts[i].Emails = []EmailValue{e}
		}
	}
	return contacts, nil
}

// loadAllPhonesEmails attaches every phone and email (detail view) for a single
// contact, ordered primary-first.
func (s *Service) loadAllPhonesEmails(c *Contact) error {
	phoneRows, err := s.db.Query(
		`SELECT id, value, is_primary FROM contact_phones WHERE contact_id = $1 ORDER BY is_primary DESC, created_at`,
		c.ID,
	)
	if err != nil {
		return fmt.Errorf("load contact phones: %w", err)
	}
	defer phoneRows.Close()
	c.Phones = []PhoneValue{}
	for phoneRows.Next() {
		var p PhoneValue
		if err := phoneRows.Scan(&p.ID, &p.Value, &p.IsPrimary); err != nil {
			return fmt.Errorf("scan contact phone: %w", err)
		}
		c.Phones = append(c.Phones, p)
	}
	if len(c.Phones) > 0 {
		c.Phone = c.Phones[0].Value
	}

	emailRows, err := s.db.Query(
		`SELECT id, value, is_primary FROM contact_emails WHERE contact_id = $1 ORDER BY is_primary DESC, created_at`,
		c.ID,
	)
	if err != nil {
		return fmt.Errorf("load contact emails: %w", err)
	}
	defer emailRows.Close()
	c.Emails = []EmailValue{}
	for emailRows.Next() {
		var e EmailValue
		if err := emailRows.Scan(&e.ID, &e.Value, &e.IsPrimary); err != nil {
			return fmt.Errorf("scan contact email: %w", err)
		}
		c.Emails = append(c.Emails, e)
	}
	if len(c.Emails) > 0 {
		c.Email = c.Emails[0].Value
	}
	return nil
}

func (s *Service) populateTagsAndStatus(contacts []Contact, contactIDs []string) ([]Contact, error) {
	tagsByContact := make(map[string][]TagRef, len(contactIDs))
	statusByContact := make(map[string]*TagRef)

	for _, id := range contactIDs {
		tagsByContact[id] = []TagRef{}
	}

	tagRows, err := s.db.Query(
		`SELECT ct.contact_id, t.id, t.name, COALESCE(t.color, '')
		FROM contact_tags ct
		JOIN tags t ON t.id = ct.tag_id
		WHERE ct.contact_id = ANY($1)
		ORDER BY t.name`,
		contactIDs,
	)
	if err != nil {
		return contacts, fmt.Errorf("load contact tags: %w", err)
	}
	defer tagRows.Close()
	for tagRows.Next() {
		var contactID string
		var ref TagRef
		if err := tagRows.Scan(&contactID, &ref.ID, &ref.Name, &ref.Color); err != nil {
			return contacts, fmt.Errorf("scan contact tag: %w", err)
		}
		tagsByContact[contactID] = append(tagsByContact[contactID], ref)
	}
	if err := tagRows.Err(); err != nil {
		return contacts, fmt.Errorf("load contact tags: iterate: %w", err)
	}

	statusRows, err := s.db.Query(
		`SELECT c.id, COALESCE(t.id::text, ''), COALESCE(t.name, ''), COALESCE(t.color, '')
		FROM contacts c
		LEFT JOIN tags t ON t.id = c.status_id
		WHERE c.id = ANY($1)`,
		contactIDs,
	)
	if err != nil {
		return contacts, fmt.Errorf("load contact statuses: %w", err)
	}
	defer statusRows.Close()
	for statusRows.Next() {
		var contactID string
		var ref TagRef
		if err := statusRows.Scan(&contactID, &ref.ID, &ref.Name, &ref.Color); err != nil {
			return contacts, fmt.Errorf("scan contact status: %w", err)
		}
		if ref.ID != "" {
			statusByContact[contactID] = &ref
		}
	}
	if err := statusRows.Err(); err != nil {
		return contacts, fmt.Errorf("load contact statuses: iterate: %w", err)
	}

	for i := range contacts {
		contacts[i].Tags = tagsByContact[contacts[i].ID]
		contacts[i].Status = statusByContact[contacts[i].ID]
	}
	return contacts, nil
}

// resolveByPhone returns the contacts whose stored phone matches the given
// number after normalization. Used by lead entry to ask the user whether to
// link or create — phone is the duplicate signal. Both storage generations
// match: canonical rows store country-coded digits, legacy rows the national
// form.
func (s *Service) resolveByPhone(phone string) ([]ResolveMatch, error) {
	defaultCC, err := settings.DefaultCountryCode(s.db)
	if err != nil {
		return nil, err
	}
	key, codedKey := util.PhoneLookupKeys(phone, defaultCC)
	if key == "" {
		return []ResolveMatch{}, nil
	}
	rows, err := s.db.Query(
		`SELECT DISTINCT ON (c.id) c.id, c.name,
			COALESCE(ece.value, ''), COALESCE(pcp.value, '')
		FROM contacts c
		JOIN contact_phones cp ON cp.contact_id = c.id
		LEFT JOIN LATERAL (
			SELECT value FROM contact_phones WHERE contact_id = c.id AND is_primary LIMIT 1
		) pcp ON true
		LEFT JOIN LATERAL (
			SELECT value FROM contact_emails WHERE contact_id = c.id AND is_primary LIMIT 1
		) ece ON true
		WHERE c.deleted_at IS NULL
		  AND `+util.PhoneMatchCond("cp.value", "$1", "$2")+`
		ORDER BY c.id, c.updated_at DESC`,
		key, codedKey,
	)
	if err != nil {
		return nil, fmt.Errorf("resolve contact by phone: %w", err)
	}
	defer rows.Close()
	matches := []ResolveMatch{}
	for rows.Next() {
		var m ResolveMatch
		if err := rows.Scan(&m.ID, &m.Name, &m.Email, &m.Phone); err != nil {
			return nil, fmt.Errorf("resolve contact by phone: scan: %w", err)
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("resolve contact by phone: iterate: %w", err)
	}
	if len(matches) > 0 {
		if err := s.attachOpenLeads(matches); err != nil {
			return nil, err
		}
	}
	return matches, nil
}

// attachOpenLeads fills each match's OpenLeads with the live leads whose
// linked stage declares outcome 'open' — the same stage-metadata signal the
// rest of the product reads. Matches without an open lead keep an empty list.
func (s *Service) attachOpenLeads(matches []ResolveMatch) error {
	ids := make([]string, 0, len(matches))
	for _, m := range matches {
		ids = append(ids, m.ID)
	}
	rows, err := s.db.Query(
		`SELECT l.contact_id, l.id, COALESCE(l.nickname, c.name, ''), COALESCE(ls.name, ''),
			COALESCE(p.name, ''), l.program_id, l.pipeline_id, COALESCE(pl.name, '')
		FROM leads l
		JOIN contacts c ON c.id = l.contact_id
		JOIN lead_stages ls ON ls.id = l.stage_id AND ls.outcome = 'open'
		LEFT JOIN programs p ON p.id = l.program_id
		LEFT JOIN pipelines pl ON pl.id = l.pipeline_id
		WHERE l.contact_id = ANY($1) AND l.deleted_at IS NULL
		ORDER BY l.created_at ASC`,
		ids,
	)
	if err != nil {
		return fmt.Errorf("resolve open leads: %w", err)
	}
	defer rows.Close()
	openByContact := map[string][]lead.OpenLeadRef{}
	for rows.Next() {
		var contactID string
		var ref lead.OpenLeadRef
		var programIDNull sql.NullString
		if err := rows.Scan(&contactID, &ref.ID, &ref.DisplayName, &ref.StageName,
			&ref.ProgramName, &programIDNull, &ref.PipelineID, &ref.PipelineName); err != nil {
			return fmt.Errorf("resolve open leads: scan: %w", err)
		}
		if programIDNull.Valid {
			ref.ProgramID = &programIDNull.String
		}
		openByContact[contactID] = append(openByContact[contactID], ref)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("resolve open leads: iterate: %w", err)
	}
	for i := range matches {
		open := openByContact[matches[i].ID]
		if open == nil {
			open = []lead.OpenLeadRef{}
		}
		matches[i].OpenLeads = open
	}
	return nil
}

func (s *Service) get(id string) (*Contact, error) {
	var c Contact
	var dob string
	err := s.db.QueryRow(
		`SELECT id, name, COALESCE(nickname, ''), COALESCE(location, ''), age, COALESCE(to_char(date_of_birth, 'YYYY-MM-DD'), ''), created_at, updated_at
		FROM contacts
		WHERE id = $1 AND deleted_at IS NULL`,
		id,
	).Scan(
		&c.ID, &c.Name, &c.Nickname, &c.Location, &c.Age, &dob, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get contact: %w", err)
	}
	if dob != "" {
		c.DateOfBirth = &dob
	}
	populated, err := s.populateTagsAndStatus([]Contact{c}, []string{c.ID})
	if err != nil {
		return nil, fmt.Errorf("get contact: populate tags and status: %w", err)
	}
	c = populated[0]
	if err := s.loadAllPhonesEmails(&c); err != nil {
		return nil, fmt.Errorf("get contact: load phones and emails: %w", err)
	}
	return &c, nil
}

func (s *Service) create(req CreateRequest) (*Contact, error) {
	// Phones are canonicalized to one stored form ('+' + digits) at every
	// entry point, so duplicates and lookups compare canonical values.
	defaultCC, err := settings.DefaultCountryCode(s.db)
	if err != nil {
		return nil, err
	}
	if req.Phone != "" {
		req.Phone = util.CanonicalPhone(req.Phone, defaultCC)
	}
	for i := range req.Phones {
		req.Phones[i].Value = util.CanonicalPhone(req.Phones[i].Value, defaultCC)
	}
	// Fold the scalar phone/email form fields into the child-row lists so the
	// rest of the create path deals with lists only.
	if req.Phone != "" && len(req.Phones) == 0 {
		req.Phones = []PhoneValue{{Value: req.Phone, IsPrimary: true}}
	}
	if req.Email != "" && len(req.Emails) == 0 {
		req.Emails = []EmailValue{{Value: req.Email, IsPrimary: true}}
	}
	if err := validateCollectionLimits(req.Phones, req.Emails, req.TagIDs); err != nil {
		return nil, err
	}
	// Duplicate guard: a create whose primary phone or email collides with a
	// live contact is rejected with a 409 unless the caller confirms the
	// duplicate. The match lookup runs before the insert so the new contact
	// itself is not flagged.
	// The effective primary is the first marked-primary list value, else the
	// first value, so list-only creates (no scalar) are guarded too.
	phone := req.Phone
	if phone == "" && len(req.Phones) > 0 {
		phone = primaryValue(req.Phones)
	}
	email := req.Email
	if email == "" && len(req.Emails) > 0 {
		email = primaryValue(req.Emails)
	}
	matches, err := s.duplicateMatches(s.db, phone, email, "")
	if err != nil {
		return nil, err
	}
	if len(matches) > 0 && !req.ConfirmDuplicates {
		return nil, &DuplicateError{Matches: matches}
	}
	created, err := s.insertContact(req)
	if err != nil {
		return nil, err
	}
	if len(matches) > 0 {
		created.Warnings = append(created.Warnings, "phone or email matches an existing contact")
	}
	return created, nil
}

// valueEntry is satisfied by PhoneValue and EmailValue so primaryValue can
// serve both phone and email lists.
type valueEntry interface {
	Primary() bool
	Val() string
}

func (p PhoneValue) Primary() bool { return p.IsPrimary }
func (p PhoneValue) Val() string   { return p.Value }
func (e EmailValue) Primary() bool { return e.IsPrimary }
func (e EmailValue) Val() string   { return e.Value }

// primaryValue returns the first marked-primary entry's value, else the first
// entry's value, mirroring the insert's primary promotion so the duplicate
// check looks at the value that will actually be primary.
func primaryValue[T valueEntry](entries []T) string {
	for _, e := range entries {
		if e.Primary() {
			return e.Val()
		}
	}
	return entries[0].Val()
}

// duplicateMatches returns the live contacts whose primary phone or email
// collides with the given phone/email after normalization, using targeted
// indexed lookups rather than scanning the whole contact table. excludeID
// skips one contact (the row being edited); an empty id excludes nothing.
func (s *Service) duplicateMatches(q interface {
	Query(query string, args ...any) (*sql.Rows, error)
}, phone, email, excludeID string) ([]DuplicateMatch, error) {
	e := util.NormalizeEmail(email)
	// The phone is matched in both storage generations: canonical rows store
	// country-coded digits, legacy rows store the national form.
	phoneKey, codedKey := "", ""
	if phone != "" {
		defaultCC, err := settings.DefaultCountryCode(s.db)
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

// validateCollectionLimits enforces the per-contact caps on phones, emails,
// and tags so a single request cannot flood the child tables or hold a pooled
// connection for thousands of sequential inserts.
func validateCollectionLimits(phones []PhoneValue, emails []EmailValue, tagIDs []string) error {
	if len(phones) > maxContactPhones {
		return fmt.Errorf("%w: at most %d phones per contact", ErrCollectionLimit, maxContactPhones)
	}
	if len(emails) > maxContactEmails {
		return fmt.Errorf("%w: at most %d emails per contact", ErrCollectionLimit, maxContactEmails)
	}
	if len(tagIDs) > maxContactTags {
		return fmt.Errorf("%w: at most %d tags per contact", ErrCollectionLimit, maxContactTags)
	}
	for _, p := range phones {
		if len(p.Value) > maxValueLength {
			return fmt.Errorf("%w: phone value is too long", ErrCollectionLimit)
		}
	}
	for _, e := range emails {
		if len(e.Value) > maxValueLength {
			return fmt.Errorf("%w: email value is too long", ErrCollectionLimit)
		}
		if !util.IsEmail(e.Value) {
			return fmt.Errorf("%w: %q", ErrInvalidEmail, e.Value)
		}
	}
	return nil
}

// validateDateOfBirth accepts an absent value, an empty string (which clears
// the field on update), or a real calendar date in YYYY-MM-DD form that is not
// in the future. Years below 100 are rejected: no real birth year is that low,
// and JavaScript's Date remaps two-digit years (0095 → 1995), so the client
// cannot render them. The birth date is the truth for computed age; the
// approximate integer age stays as the fallback.
func validateDateOfBirth(value *string) error {
	if value == nil || *value == "" {
		return nil
	}
	d, err := time.ParseInLocation(time.DateOnly, *value, time.Local)
	if err != nil || d.Year() < 100 || d.After(time.Now()) {
		return ErrInvalidDateOfBirth
	}
	return nil
}

// insertContact inserts a contact and its child rows (phones, emails, tags)
// in one transaction, then returns the fully populated contact.
func (s *Service) insertContact(req CreateRequest) (*Contact, error) {
	var c Contact
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("create contact: %w", err)
	}
	defer tx.Rollback()

	statusID := util.NullPtr(req.StatusID)
	if statusID != nil {
		valid, err := statusTagExists(tx, *statusID)
		if err != nil {
			return nil, fmt.Errorf("create contact: validate status: %w", err)
		}
		if !valid {
			return nil, ErrInvalidStatus
		}
	}

	// validate the "at least one phone or one email" invariant
	if len(req.Phones) == 0 && len(req.Emails) == 0 {
		return nil, ErrNoContactDetail
	}
	if err := validateDateOfBirth(req.DateOfBirth); err != nil {
		return nil, err
	}
	var dob any
	if req.DateOfBirth != nil && *req.DateOfBirth != "" {
		dob = *req.DateOfBirth
	}

	var dobOut string
	err = tx.QueryRow(
		`INSERT INTO contacts (name, nickname, location, age, date_of_birth, status_id)
		VALUES ($1, $2, $3, $4, $5::date, $6)
		RETURNING id, name, COALESCE(nickname, ''), COALESCE(location, ''), age, COALESCE(to_char(date_of_birth, 'YYYY-MM-DD'), ''), created_at, updated_at`,
		req.Name, util.NullStr(req.Nickname), util.NullStr(req.Location), req.Age, dob, statusID,
	).Scan(
		&c.ID, &c.Name, &c.Nickname, &c.Location, &c.Age, &dobOut, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create contact: %w", err)
	}
	if dobOut != "" {
		c.DateOfBirth = &dobOut
	}

	if err := syncPhonesEmailsTx(tx, c.ID, req.Phones, req.Emails); err != nil {
		return nil, fmt.Errorf("create contact: %w", err)
	}

	unknownTags, err := syncTags(tx, c.ID, req.TagIDs)
	if err != nil {
		return nil, fmt.Errorf("create contact: sync tags: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit contact: %w", err)
	}

	populated, err := s.populateTagsAndStatus([]Contact{c}, []string{c.ID})
	if err != nil {
		return nil, fmt.Errorf("create contact: populate tags and status: %w", err)
	}
	if err := s.loadAllPhonesEmails(&populated[0]); err != nil {
		return nil, fmt.Errorf("create contact: load phones and emails: %w", err)
	}
	if len(unknownTags) > 0 {
		populated[0].Warnings = append(populated[0].Warnings, "ignored unknown tag id(s)")
	}
	return &populated[0], nil
}

// syncPhonesEmailsTx replaces a contact's phone/email child rows with the given
// values, maintaining exactly one primary per type. Each type is cleared and
// rewritten only when its slice is non-nil, so a caller that sends just the
// phones (or just the emails) does not wipe the other. When no explicit primary
// is marked, the first value becomes primary.
func syncPhonesEmailsTx(q queryer, contactID string, phones []PhoneValue, emails []EmailValue) error {
	if phones != nil {
		if _, err := q.Exec(`DELETE FROM contact_phones WHERE contact_id = $1`, contactID); err != nil {
			return fmt.Errorf("clear contact phones: %w", err)
		}
		if err := insertPhoneRows(q, contactID, phones); err != nil {
			return fmt.Errorf("sync contact phones: %w", err)
		}
	}
	if emails != nil {
		if _, err := q.Exec(`DELETE FROM contact_emails WHERE contact_id = $1`, contactID); err != nil {
			return fmt.Errorf("clear contact emails: %w", err)
		}
		if err := insertEmailRows(q, contactID, emails); err != nil {
			return fmt.Errorf("sync contact emails: %w", err)
		}
	}
	return nil
}

func insertPhoneRows(q queryer, contactID string, phones []PhoneValue) error {
	// Ensure exactly one primary: the first marked entry wins, later ones are
	// demoted so the invariant holds even for client-supplied duplicates.
	primarySeen := false
	for i := range phones {
		if phones[i].IsPrimary {
			if primarySeen {
				phones[i].IsPrimary = false
				continue
			}
			primarySeen = true
		}
	}
	if !primarySeen && len(phones) > 0 {
		phones[0].IsPrimary = true
	}
	for _, p := range phones {
		if _, err := q.Exec(
			`INSERT INTO contact_phones (contact_id, value, is_primary) VALUES ($1, $2, $3)`,
			contactID, p.Value, p.IsPrimary,
		); err != nil {
			return fmt.Errorf("insert contact phone: %w", err)
		}
	}
	return nil
}

func insertEmailRows(q queryer, contactID string, emails []EmailValue) error {
	// Ensure exactly one primary: the first marked entry wins, later ones are
	// demoted so the invariant holds even for client-supplied duplicates.
	primarySeen := false
	for i := range emails {
		if emails[i].IsPrimary {
			if primarySeen {
				emails[i].IsPrimary = false
				continue
			}
			primarySeen = true
		}
	}
	if !primarySeen && len(emails) > 0 {
		emails[0].IsPrimary = true
	}
	for _, e := range emails {
		if _, err := q.Exec(
			`INSERT INTO contact_emails (contact_id, value, is_primary) VALUES ($1, $2, $3)`,
			contactID, e.Value, e.IsPrimary,
		); err != nil {
			return fmt.Errorf("insert contact email: %w", err)
		}
	}
	return nil
}

func (s *Service) update(id string, req UpdateRequest, userID string) (*Contact, error) {
	if req.Phones != nil || req.Emails != nil || req.TagIDs != nil || req.Phone != nil || req.Email != nil {
		// Phones are canonicalized to one stored form ('+' + digits) at every
		// entry point, so duplicates and lookups compare canonical values.
		defaultCC, err := settings.DefaultCountryCode(s.db)
		if err != nil {
			return nil, err
		}
		if req.Phone != nil {
			canonical := util.CanonicalPhone(*req.Phone, defaultCC)
			req.Phone = &canonical
		}
		if req.Phones != nil {
			for i := range *req.Phones {
				(*req.Phones)[i].Value = util.CanonicalPhone((*req.Phones)[i].Value, defaultCC)
			}
		}
		var phones []PhoneValue
		if req.Phones != nil {
			phones = *req.Phones
		}
		var emails []EmailValue
		if req.Emails != nil {
			emails = *req.Emails
		}
		if err := validateCollectionLimits(phones, emails, req.TagIDs); err != nil {
			return nil, err
		}
		// A scalar phone/email lands in the child tables too, so bound its length.
		if req.Phone != nil && len(*req.Phone) > maxValueLength {
			return nil, fmt.Errorf("%w: phone value is too long", ErrCollectionLimit)
		}
		if req.Email != nil && len(*req.Email) > maxValueLength {
			return nil, fmt.Errorf("%w: email value is too long", ErrCollectionLimit)
		}
		if req.Email != nil && *req.Email != "" && !util.IsEmail(*req.Email) {
			return nil, fmt.Errorf("%w: %q", ErrInvalidEmail, *req.Email)
		}
	}

	old, err := s.get(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("update contact: %w", err)
	}
	defer tx.Rollback()

	if req.StatusID != nil && *req.StatusID != "" {
		valid, err := statusTagExists(tx, *req.StatusID)
		if err != nil {
			return nil, fmt.Errorf("update contact: validate status: %w", err)
		}
		if !valid {
			return nil, ErrInvalidStatus
		}
	}

	if err := validateDateOfBirth(req.DateOfBirth); err != nil {
		return nil, err
	}

	var c Contact
	var dobOut string
	err = tx.QueryRow(
		`UPDATE contacts SET
			name = COALESCE(NULLIF($2, ''), name),
			nickname = COALESCE($3, nickname),
			location = COALESCE($4, location),
			age = COALESCE($5, age),
			date_of_birth = CASE WHEN $7::text IS NOT NULL THEN NULLIF($7::text, '')::date ELSE date_of_birth END,
			status_id = CASE WHEN $6::text IS NOT NULL THEN NULLIF($6::text, '')::uuid ELSE status_id END,
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, name, COALESCE(nickname, ''), COALESCE(location, ''), age, COALESCE(to_char(date_of_birth, 'YYYY-MM-DD'), ''), created_at, updated_at`,
		id,
		req.Name,
		req.Nickname,
		req.Location,
		req.Age,
		req.StatusID,
		req.DateOfBirth,
	).Scan(
		&c.ID, &c.Name, &c.Nickname, &c.Location, &c.Age, &dobOut, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update contact: %w", err)
	}
	if dobOut != "" {
		c.DateOfBirth = &dobOut
	}

	// Sync the phone/email child rows when the client sends a list. Each type is
	// replaced only when sent, so a partial update never wipes the other. The
	// final-state check below is the authority: clearing a type is valid while
	// the other still holds a value.
	if req.Phones != nil || req.Emails != nil {
		var phones []PhoneValue
		if req.Phones != nil {
			phones = *req.Phones
		}
		var emails []EmailValue
		if req.Emails != nil {
			emails = *req.Emails
		}
		if err := syncPhonesEmailsTx(tx, id, phones, emails); err != nil {
			return nil, fmt.Errorf("update contact: sync phones and emails: %w", err)
		}
	} else if req.Phone != nil || req.Email != nil {
		// Scalar-only update: the child tables are the source of truth for
		// reads, so the scalar is mirrored into that type's child rows — the
		// whole list for the sent type is replaced (scalar semantics; list
		// clients send phones/emails and take the branch above). An empty
		// scalar clears the type; a type the client did not send is untouched.
		var phones []PhoneValue
		var emails []EmailValue
		if req.Phone != nil {
			phones = []PhoneValue{}
			if *req.Phone != "" {
				phones = append(phones, PhoneValue{Value: *req.Phone, IsPrimary: true})
			}
		}
		if req.Email != nil {
			emails = []EmailValue{}
			if *req.Email != "" {
				emails = append(emails, EmailValue{Value: *req.Email, IsPrimary: true})
			}
		}
		if err := syncPhonesEmailsTx(tx, id, phones, emails); err != nil {
			return nil, fmt.Errorf("update contact: sync scalar phone and email: %w", err)
		}
	}

	// Warn + confirm on edits: an update that sets a primary phone/email
	// already on another live contact is refused with the matches unless the
	// client confirms, the same posture as manual create.
	if req.Phones != nil || req.Emails != nil || req.Phone != nil || req.Email != nil {
		var primaryPhone, primaryEmail string
		if err := tx.QueryRow(`
			SELECT
				COALESCE((SELECT value FROM contact_phones WHERE contact_id = $1 ORDER BY is_primary DESC, created_at ASC LIMIT 1), ''),
				COALESCE((SELECT value FROM contact_emails WHERE contact_id = $1 ORDER BY is_primary DESC, created_at ASC LIMIT 1), '')`,
			id,
		).Scan(&primaryPhone, &primaryEmail); err != nil {
			return nil, fmt.Errorf("update contact: load primary detail: %w", err)
		}
		matches := []DuplicateMatch{}
		// Only a change to the effective primary is a duplicate risk; the edit
		// form resubmits unchanged details, which must not nag.
		if primaryPhone != old.Phone || primaryEmail != old.Email {
			matches, err = s.duplicateMatches(tx, primaryPhone, primaryEmail, id)
			if err != nil {
				return nil, err
			}
		}
		if len(matches) > 0 && !req.ConfirmDuplicates {
			return nil, &DuplicateError{Matches: matches}
		}
	}

	unknownTags := []string{}
	if req.TagIDs != nil {
		if _, err := tx.Exec(`DELETE FROM contact_tags WHERE contact_id = $1`, id); err != nil {
			return nil, fmt.Errorf("clear contact tags: %w", err)
		}
		unknownTags, err = syncTags(tx, id, req.TagIDs)
		if err != nil {
			return nil, fmt.Errorf("update contact: sync tags: %w", err)
		}
	}
	// A live contact always keeps at least one phone or one email, whatever
	// combination of scalar and list fields the update sent; an update that
	// would strip the last detail rolls back with ErrNoContactDetail.
	var hasDetail bool
	if err := tx.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM contact_phones WHERE contact_id = $1)
			OR EXISTS(SELECT 1 FROM contact_emails WHERE contact_id = $1)`,
		id,
	).Scan(&hasDetail); err != nil {
		return nil, fmt.Errorf("update contact: check contact detail: %w", err)
	}
	if !hasDetail {
		return nil, ErrNoContactDetail
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit contact update: %w", err)
	}

	populated, err := s.populateTagsAndStatus([]Contact{c}, []string{c.ID})
	if err != nil {
		return nil, fmt.Errorf("update contact: populate tags and status: %w", err)
	}
	if err := s.loadAllPhonesEmails(&populated[0]); err != nil {
		return nil, fmt.Errorf("update contact: load phones and emails: %w", err)
	}
	if len(unknownTags) > 0 {
		populated[0].Warnings = append(populated[0].Warnings, "ignored unknown tag id(s)")
	}
	c = populated[0]

	changes := diffContact(old, &c)
	if changes != "" {
		s.auditLogDesc(
			fmt.Sprintf("Updated contact %q (%s)", c.Name, strings.Join(diffFieldLabels(changes), ", ")),
			"contact", id, "update", userID,
		)
	}
	return &c, nil
}

// diffFieldLabels renders the changed-field keys of a diffContact payload as
// human labels for the audit description.
func diffFieldLabels(changes string) []string {
	var diff map[string]any
	if err := json.Unmarshal([]byte(changes), &diff); err != nil {
		return []string{"details"}
	}
	labels := map[string]string{
		"name": "name", "email": "email", "phone": "phone",
		"location": "location", "age": "age", "date_of_birth": "date of birth", "tags": "tags",
		"status": "status", "phones": "phones", "emails": "emails",
	}
	out := []string{}
	for _, key := range []string{"name", "nickname", "email", "phone", "phones", "emails", "location", "age", "date_of_birth", "tags", "status"} {
		if _, ok := diff[key]; ok {
			if label, ok := labels[key]; ok {
				out = append(out, label)
			} else {
				out = append(out, key)
			}
		}
	}
	if len(out) == 0 {
		return []string{"details"}
	}
	return out
}

// queryer is satisfied by both *sql.DB and *sql.Tx so tag syncing can run
// inside the mutation transaction.
type queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

// syncTags validates that every tag id exists and is a contact tag, inserts
// the valid ones, and reports the unknown ids so callers can surface them as
// warnings instead of silently dropping tags.
func syncTags(q queryer, contactID string, tagIDs []string) ([]string, error) {
	if len(tagIDs) == 0 {
		return nil, nil
	}
	valid := map[string]bool{}
	rows, err := q.Query(`SELECT id::text FROM tags WHERE type = 'tag' AND id::text = ANY($1)`, tagIDs)
	if err != nil {
		return nil, fmt.Errorf("validate contact tags: %w", err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("validate contact tags: scan: %w", err)
		}
		valid[id] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("validate contact tags: iterate: %w", err)
	}
	unknown := []string{}
	for _, id := range tagIDs {
		if !valid[id] {
			unknown = append(unknown, id)
			continue
		}
		if _, err := q.Exec(
			`INSERT INTO contact_tags (contact_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			contactID, id,
		); err != nil {
			return nil, fmt.Errorf("attach contact tag: %w", err)
		}
	}
	return unknown, nil
}

func (s *Service) delete(id string, userID string) error {
	// Capture the name before the soft delete so the audit row names the
	// contact; the row hides from every read path afterwards.
	var name string
	if err := s.db.QueryRow(`SELECT name FROM contacts WHERE id = $1 AND deleted_at IS NULL`, id).Scan(&name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("delete contact: load name: %w", err)
	}
	res, err := s.db.Exec(`UPDATE contacts SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete contact: rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	s.auditLogDesc(fmt.Sprintf("Deleted contact %q", name), "contact", id, "delete", userID)
	return nil
}

// statusTagExists reports whether the id references a tag of type 'status'.
func statusTagExists(q queryer, statusID string) (bool, error) {
	var exists bool
	err := q.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM tags WHERE id = $1 AND type = 'status')`,
		statusID,
	).Scan(&exists)
	return exists, err
}

// auditLogDesc writes a best-effort audit entry whose description is a human
// sentence naming the entity — the description is the record an admin reads.
func (s *Service) auditLogDesc(desc, resourceType, resourceID, action, userID string) {
	audit.LogCustom(s.db, desc, resourceType, resourceID, action, "", userID)
}

func diffContact(old, new *Contact) string {
	diff := map[string]any{}
	if old.Name != new.Name {
		diff["name"] = map[string]string{"old": old.Name, "new": new.Name}
	}
	if old.Nickname != new.Nickname {
		diff["nickname"] = map[string]string{"old": old.Nickname, "new": new.Nickname}
	}
	if old.Email != new.Email {
		diff["email"] = map[string]string{"old": old.Email, "new": new.Email}
	}
	if old.Phone != new.Phone {
		diff["phone"] = map[string]string{"old": old.Phone, "new": new.Phone}
	}
	if old.Location != new.Location {
		diff["location"] = map[string]string{"old": old.Location, "new": new.Location}
	}
	if (old.Age == nil) != (new.Age == nil) || (old.Age != nil && *old.Age != *new.Age) {
		diff["age"] = map[string]any{"old": old.Age, "new": new.Age}
	}

	oldDob, newDob := "", ""
	if old.DateOfBirth != nil {
		oldDob = *old.DateOfBirth
	}
	if new.DateOfBirth != nil {
		newDob = *new.DateOfBirth
	}
	if oldDob != newDob {
		diff["date_of_birth"] = map[string]string{"old": oldDob, "new": newDob}
	}

	oldTags := tagsToSet(old.Tags)
	newTags := tagsToSet(new.Tags)
	if tagsChanged(oldTags, newTags) {
		diff["tags"] = map[string]any{"old": old.Tags, "new": new.Tags}
	}

	oldStatus := ""
	if old.Status != nil {
		oldStatus = old.Status.Name
	}
	newStatus := ""
	if new.Status != nil {
		newStatus = new.Status.Name
	}
	if oldStatus != newStatus {
		diff["status"] = map[string]string{"old": oldStatus, "new": newStatus}
	}

	// Phones/emails live in child tables (contact_phones/contact_emails); the
	// scalar fields mirror the primary value, so compare the full lists.
	if !phoneValuesEqual(old.Phones, new.Phones) {
		diff["phones"] = map[string]any{"old": old.Phones, "new": new.Phones}
	}
	if !emailValuesEqual(old.Emails, new.Emails) {
		diff["emails"] = map[string]any{"old": old.Emails, "new": new.Emails}
	}

	if len(diff) == 0 {
		return ""
	}
	b, _ := json.Marshal(diff)
	return string(b)
}

// phoneValuesEqual reports whether two phone lists are identical (same values
// in the same order, ignoring the primary flag which is derived).
func phoneValuesEqual(a, b []PhoneValue) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Value != b[i].Value {
			return false
		}
	}
	return true
}

// emailValuesEqual reports whether two email lists are identical.
func emailValuesEqual(a, b []EmailValue) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Value != b[i].Value {
			return false
		}
	}
	return true
}

func tagsToSet(tags []TagRef) map[string]bool {
	s := make(map[string]bool)
	for _, t := range tags {
		s[t.ID] = true
	}
	return s
}

func tagsChanged(old, new map[string]bool) bool {
	if len(old) != len(new) {
		return true
	}
	for k := range old {
		if !new[k] {
			return true
		}
	}
	return false
}
