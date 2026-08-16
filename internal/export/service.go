package export

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// formulaLeaders are the characters spreadsheet applications treat as formula
// triggers (CWE-1236). Values starting with one of them are neutralized on
// export so stored data renders as text, never executes.
var formulaLeaders = []string{"=", "+", "-", "@", "\t", "\r"}

// sanitizeCell neutralizes spreadsheet formula triggers in exported cells by
// prefixing a single quote. It is applied to every user-controlled field at
// the output boundary; system-generated cells (ids, timestamps) are exempt.
func sanitizeCell(v string) string {
	for _, p := range formulaLeaders {
		if strings.HasPrefix(v, p) {
			return "'" + v
		}
	}
	return v
}

// Service streams CRM data out as CSV for backup and spreadsheet work.
type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// ExportContactsCSV writes all non-deleted contacts as a CSV stream.
func (s *Service) ExportContactsCSV(w io.Writer) error {
	cw := csv.NewWriter(w)

	if err := cw.Write([]string{
		"id", "name", "nickname", "primary_phone", "all_phones", "primary_email", "all_emails",
		"status", "tags", "location", "age", "created_at", "updated_at",
	}); err != nil {
		return fmt.Errorf("write contacts header: %w", err)
	}

	rows, err := s.db.Query(`
		SELECT c.id, COALESCE(c.name, ''), COALESCE(c.nickname, ''), COALESCE(c.location, ''), c.age,
			COALESCE(t.name, ''), c.created_at, c.updated_at
		FROM contacts c
		LEFT JOIN tags t ON t.id = c.status_id
		WHERE c.deleted_at IS NULL
		ORDER BY c.created_at DESC`)
	if err != nil {
		return fmt.Errorf("query contacts: %w", err)
	}
	defer rows.Close()

	type row struct {
		id, name, nickname, location string
		age                          *int
		status                       string
		createdAt, updatedAt         time.Time
	}
	var contacts []row
	var contactIDs []string
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.name, &r.nickname, &r.location, &r.age, &r.status, &r.createdAt, &r.updatedAt); err != nil {
			return fmt.Errorf("scan contact: %w", err)
		}
		contacts = append(contacts, r)
		contactIDs = append(contactIDs, r.id)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate contacts: %w", err)
	}

	phones, err := s.valuesByContact(contactIDs, contactPhones)
	if err != nil {
		return err
	}
	emails, err := s.valuesByContact(contactIDs, contactEmails)
	if err != nil {
		return err
	}
	tags, err := s.tagsByContact(contactIDs)
	if err != nil {
		return err
	}

	for _, c := range contacts {
		age := ""
		if c.age != nil {
			age = strconv.Itoa(*c.age)
		}
		if err := cw.Write([]string{
			c.id,
			sanitizeCell(c.name),
			sanitizeCell(c.nickname),
			sanitizeCell(phones[c.id].primary),
			sanitizeCell(phones[c.id].all),
			sanitizeCell(emails[c.id].primary),
			sanitizeCell(emails[c.id].all),
			sanitizeCell(c.status),
			sanitizeCell(tags[c.id]),
			sanitizeCell(c.location),
			age,
			c.createdAt.UTC().Format(time.RFC3339), c.updatedAt.UTC().Format(time.RFC3339),
		}); err != nil {
			return fmt.Errorf("write contact row: %w", err)
		}
	}
	cw.Flush()
	return cw.Error()
}

type valueSet struct {
	primary string
	all     string
}

// childTable is a contact child table (phones or emails) read by
// valuesByContact. Its SQL name is interpolated into a query, so only the
// package constants may be passed.
type childTable struct {
	sqlName string
	label   string
}

var (
	contactPhones = childTable{sqlName: "contact_phones", label: "phones"}
	contactEmails = childTable{sqlName: "contact_emails", label: "emails"}
)

// valuesByContact loads every value per contact from the given child table,
// primary first, and joins them with "; " for the all-values cell.
func (s *Service) valuesByContact(contactIDs []string, table childTable) (map[string]valueSet, error) {
	result := make(map[string]valueSet, len(contactIDs))
	if len(contactIDs) == 0 {
		return result, nil
	}
	rows, err := s.db.Query(`
		SELECT contact_id, value, is_primary FROM `+table.sqlName+`
		WHERE contact_id = ANY($1)
		ORDER BY contact_id, is_primary DESC, created_at`, contactIDs)
	if err != nil {
		return nil, fmt.Errorf("query contact %s: %w", table.label, err)
	}
	defer rows.Close()
	for rows.Next() {
		var contactID, value string
		var isPrimary bool
		if err := rows.Scan(&contactID, &value, &isPrimary); err != nil {
			return nil, fmt.Errorf("scan contact %s: %w", table.label, err)
		}
		vs := result[contactID]
		if isPrimary && vs.primary == "" {
			vs.primary = value
		}
		if vs.all != "" {
			vs.all += "; "
		}
		vs.all += value
		result[contactID] = vs
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contact %s: %w", table.label, err)
	}
	return result, nil
}

// tagsByContact loads the tag names per contact, comma-joined.
func (s *Service) tagsByContact(contactIDs []string) (map[string]string, error) {
	result := make(map[string]string, len(contactIDs))
	if len(contactIDs) == 0 {
		return result, nil
	}
	rows, err := s.db.Query(`
		SELECT ct.contact_id, t.name
		FROM contact_tags ct
		JOIN tags t ON t.id = ct.tag_id
		WHERE ct.contact_id = ANY($1)
		ORDER BY t.name`, contactIDs)
	if err != nil {
		return nil, fmt.Errorf("query contact tags: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var contactID, name string
		if err := rows.Scan(&contactID, &name); err != nil {
			return nil, fmt.Errorf("scan contact tag: %w", err)
		}
		if result[contactID] != "" {
			result[contactID] += ", "
		}
		result[contactID] += name
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contact tags: %w", err)
	}
	return result, nil
}

// ExportLeadsCSV writes all non-deleted leads as a CSV stream.
func (s *Service) ExportLeadsCSV(w io.Writer) error {
	cw := csv.NewWriter(w)

	if err := cw.Write([]string{
		"id", "nickname", "contact_name", "contact_phone", "contact_email", "pipeline", "stage",
		"outcome", "lost_reason", "program", "value", "assigned_to", "notes", "created_at", "updated_at",
	}); err != nil {
		return fmt.Errorf("write leads header: %w", err)
	}

	rows, err := s.db.Query(`
		SELECT l.id, COALESCE(l.nickname, ''), COALESCE(c.name, ''),
			COALESCE(pcp.value, ''), COALESCE(pce.value, ''),
			COALESCE(pl.name, ''), COALESCE(ls.name, ''), COALESCE(l.outcome, ''),
			COALESCE(l.lost_reason, ''), COALESCE(pr.name, ''), l.value,
			COALESCE(u.name, ''), COALESCE(l.notes, ''), l.created_at, l.updated_at
		FROM leads l
		LEFT JOIN lead_stages ls ON l.stage_id = ls.id
		LEFT JOIN pipelines pl ON l.pipeline_id = pl.id
		LEFT JOIN contacts c ON l.contact_id = c.id
		LEFT JOIN programs pr ON l.program_id = pr.id
		LEFT JOIN users u ON l.assigned_to = u.id
		LEFT JOIN LATERAL (
			SELECT value FROM contact_phones WHERE contact_id = l.contact_id AND is_primary LIMIT 1
		) pcp ON true
		LEFT JOIN LATERAL (
			SELECT value FROM contact_emails WHERE contact_id = l.contact_id AND is_primary LIMIT 1
		) pce ON true
		WHERE l.deleted_at IS NULL
		ORDER BY l.created_at DESC`)
	if err != nil {
		return fmt.Errorf("query leads: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id, nickname, contactName, contactPhone, contactEmail string
			pipeline, stage, outcome, lostReason, program         string
			value                                                 *float64
			assignedTo, notes                                     string
			createdAt, updatedAt                                  time.Time
		)
		if err := rows.Scan(
			&id, &nickname, &contactName, &contactPhone, &contactEmail,
			&pipeline, &stage, &outcome, &lostReason, &program, &value,
			&assignedTo, &notes, &createdAt, &updatedAt,
		); err != nil {
			return fmt.Errorf("scan lead: %w", err)
		}
		val := ""
		if value != nil {
			val = strconv.FormatFloat(*value, 'f', 2, 64)
		}
		if err := cw.Write([]string{
			id,
			sanitizeCell(nickname),
			sanitizeCell(contactName),
			sanitizeCell(contactPhone),
			sanitizeCell(contactEmail),
			sanitizeCell(pipeline),
			sanitizeCell(stage),
			sanitizeCell(outcome),
			sanitizeCell(lostReason),
			sanitizeCell(program),
			val,
			sanitizeCell(assignedTo),
			sanitizeCell(notes),
			createdAt.UTC().Format(time.RFC3339), updatedAt.UTC().Format(time.RFC3339),
		}); err != nil {
			return fmt.Errorf("write lead row: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate leads: %w", err)
	}
	cw.Flush()
	return cw.Error()
}
