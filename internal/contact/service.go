package contact

import (
	"crm/internal/audit"
	"crm/internal/util"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
)

type Service struct {
	db *sql.DB
}

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
			" AND (name ILIKE $%d OR nickname ILIKE $%d OR email ILIKE $%d OR phone ILIKE $%d)",
			argIdx, argIdx+1, argIdx+2, argIdx+3,
		)
		searchTerm := "%" + search + "%"
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
	offset := (page - 1) * perPage

	selectQuery := fmt.Sprintf(
		`SELECT id, name, COALESCE(nickname, ''), COALESCE(email, ''), COALESCE(phone, ''),
			COALESCE(location, ''), age, created_at, updated_at
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
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Nickname, &c.Email, &c.Phone, &c.Location, &c.Age, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		contacts = append(contacts, c)
		contactIDs = append(contactIDs, c.ID)
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

	for i := range contacts {
		contacts[i].Tags = tagsByContact[contacts[i].ID]
		contacts[i].Status = statusByContact[contacts[i].ID]
	}
	return contacts, nil
}

// resolveByPhone returns the contacts whose stored phone matches the given
// number after normalization (digits only). Used by lead entry to ask the
// user whether to link or create (ADR 012) — phone is the duplicate signal.
func (s *Service) resolveByPhone(phone string) ([]ResolveMatch, error) {
	key := util.NormalizePhone(phone)
	if key == "" {
		return []ResolveMatch{}, nil
	}
	rows, err := s.db.Query(
		`SELECT DISTINCT ON (c.id) c.id, c.name,
			COALESCE(c.email, ''), COALESCE(pcp.value, '')
		FROM contacts c
		JOIN contact_phones cp ON cp.contact_id = c.id
		LEFT JOIN LATERAL (
			SELECT value FROM contact_phones WHERE contact_id = c.id AND is_primary LIMIT 1
		) pcp ON true
		WHERE c.deleted_at IS NULL
		  AND regexp_replace(cp.value, '\D', '', 'g') = $1
		ORDER BY c.id, c.updated_at DESC`,
		key,
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
	return matches, rows.Err()
}

func (s *Service) get(id string) (*Contact, error) {
	var c Contact
	err := s.db.QueryRow(
		`SELECT id, name, COALESCE(nickname, ''), COALESCE(email, ''), COALESCE(phone, ''),
			COALESCE(location, ''), age, created_at, updated_at
		FROM contacts
		WHERE id = $1 AND deleted_at IS NULL`,
		id,
	).Scan(
		&c.ID, &c.Name, &c.Nickname, &c.Email, &c.Phone, &c.Location, &c.Age, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get contact: %w", err)
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
	keys, err := s.loadContactKeys()
	if err != nil {
		return nil, fmt.Errorf("create contact: load duplicate keys: %w", err)
	}
	return s.createWithKeys(req, keys)
}

// createWithKeys inserts a contact while reusing already-loaded duplicate
// lookup keys. Bulk import calls this so the full-table key scan runs once
// per import instead of once per row.
func (s *Service) createWithKeys(req CreateRequest, keys contactKeys) (*Contact, error) {
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
	if req.Phone == "" && req.Email == "" && len(req.Phones) == 0 && len(req.Emails) == 0 {
		return nil, ErrNoContactDetail
	}

	err = tx.QueryRow(
		`INSERT INTO contacts (name, nickname, email, phone, location, age, status_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, name, COALESCE(nickname, ''), COALESCE(email, ''), COALESCE(phone, ''), COALESCE(location, ''), age, created_at, updated_at`,
		req.Name, util.NullStr(req.Nickname), util.NullStr(req.Email), util.NullStr(req.Phone),
		util.NullStr(req.Location), req.Age, statusID,
	).Scan(
		&c.ID, &c.Name, &c.Nickname, &c.Email, &c.Phone, &c.Location, &c.Age, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create contact: %w", err)
	}

	// sync phones/emails child rows (also writes the primary flag)
	if err := syncPhonesEmailsTx(tx, c.ID, req.Phones, req.Emails, req.Phone, req.Email); err != nil {
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
	if reason := keys.duplicateReason(req.Phone, req.Email); reason != "" {
		populated[0].Warnings = append(populated[0].Warnings, reason)
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
// is marked, the first value becomes primary; legacy scalar phone/email are
// merged in as the primary when no child rows are provided.
func syncPhonesEmailsTx(q queryer, contactID string, phones []PhoneValue, emails []EmailValue, scalarPhone, scalarEmail string) error {
	// Merge legacy scalar values in as primary when no child rows exist.
	if len(phones) == 0 && scalarPhone != "" {
		phones = []PhoneValue{{Value: scalarPhone, IsPrimary: true}}
	}
	if len(emails) == 0 && scalarEmail != "" {
		emails = []EmailValue{{Value: scalarEmail, IsPrimary: true}}
	}

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
	// ensure exactly one primary
	hasPrimary := false
	for i := range phones {
		if phones[i].IsPrimary {
			hasPrimary = true
			break
		}
	}
	if !hasPrimary && len(phones) > 0 {
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
	hasPrimary := false
	for i := range emails {
		if emails[i].IsPrimary {
			hasPrimary = true
			break
		}
	}
	if !hasPrimary && len(emails) > 0 {
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

	var c Contact
	err = tx.QueryRow(
		`UPDATE contacts SET
			name = COALESCE(NULLIF($2, ''), name),
			nickname = COALESCE($3, nickname),
			email = COALESCE($4, email),
			phone = COALESCE($5, phone),
			location = COALESCE($6, location),
			age = COALESCE($7, age),
			status_id = CASE WHEN $8::text IS NOT NULL THEN NULLIF($8::text, '')::uuid ELSE status_id END,
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, name, COALESCE(nickname, ''), COALESCE(email, ''), COALESCE(phone, ''), COALESCE(location, ''), age, created_at, updated_at`,
		id,
		req.Name,
		req.Nickname,
		req.Email,
		req.Phone,
		req.Location,
		req.Age,
		req.StatusID,
	).Scan(
		&c.ID, &c.Name, &c.Nickname, &c.Email, &c.Phone, &c.Location, &c.Age, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update contact: %w", err)
	}

	// Sync the phone/email child rows when the client sends a list. Each type is
	// replaced only when sent, so a partial update never wipes the other.
	if req.Phones != nil || req.Emails != nil {
		phones := []PhoneValue{}
		if req.Phones != nil {
			phones = *req.Phones
		}
		emails := []EmailValue{}
		if req.Emails != nil {
			emails = *req.Emails
		}
		if len(phones) == 0 && len(emails) == 0 {
			return nil, ErrNoContactDetail
		}
		if err := syncPhonesEmailsTx(tx, id, phones, emails, "", ""); err != nil {
			return nil, fmt.Errorf("update contact: sync phones and emails: %w", err)
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
		s.logActivity(id, "contact", "update", changes, userID)
	}
	return &c, nil
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
	s.logActivity(id, "contact", "delete", `{"action":"deleted"}`, userID)
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

func (s *Service) logActivity(resourceID, resourceType, action, changes, userID string) {
	audit.Log(s.db, resourceID, resourceType, action, changes, userID)
}

func diffContact(old, new *Contact) string {
	diff := map[string]any{}
	if old.Name != new.Name {
		diff["name"] = map[string]string{"old": old.Name, "new": new.Name}
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
