package export

import (
	"crm/internal/testdb"
	"encoding/csv"
	"strings"
	"testing"
)

func TestExportContactsCSVIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	// Contact with phones/emails/tags/status via the child tables.
	var contactID string
	if err := db.QueryRow(
		`INSERT INTO contacts (name, nickname, location) VALUES ('Alice Example', 'Ali', 'Delhi') RETURNING id`,
	).Scan(&contactID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	for _, v := range []string{"+91 98765 43210", "+91 11111 22222"} {
		if _, err := db.Exec(
			`INSERT INTO contact_phones (contact_id, value, is_primary) VALUES ($1, $2, $3)`,
			contactID, v, v == "+91 98765 43210",
		); err != nil {
			t.Fatalf("seed phone: %v", err)
		}
	}
	if _, err := db.Exec(
		`INSERT INTO contact_emails (contact_id, value, is_primary) VALUES ($1, 'alice@example.com', true)`,
		contactID,
	); err != nil {
		t.Fatalf("seed email: %v", err)
	}
	var statusID, tagID string
	if err := db.QueryRow(`INSERT INTO tags (name, type) VALUES ('Active', 'status') RETURNING id`).Scan(&statusID); err != nil {
		t.Fatalf("seed status: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO tags (name, type) VALUES ('VIP', 'tag') RETURNING id`).Scan(&tagID); err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	if _, err := db.Exec(`UPDATE contacts SET status_id = $1 WHERE id = $2`, statusID, contactID); err != nil {
		t.Fatalf("set status: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO contact_tags (contact_id, tag_id) VALUES ($1, $2)`, contactID, tagID); err != nil {
		t.Fatalf("attach tag: %v", err)
	}
	// A deleted contact must not appear.
	var deletedID string
	if err := db.QueryRow(`INSERT INTO contacts (name) VALUES ('Ghost') RETURNING id`).Scan(&deletedID); err != nil {
		t.Fatalf("seed ghost: %v", err)
	}
	if _, err := db.Exec(`UPDATE contacts SET deleted_at = now() WHERE id = $1`, deletedID); err != nil {
		t.Fatalf("soft-delete ghost: %v", err)
	}

	var sb strings.Builder
	if err := svc.ExportContactsCSV(&sb); err != nil {
		t.Fatalf("export contacts: %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(sb.String())).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(records) != 2 { // header + 1 live contact
		t.Fatalf("rows = %d, want 2 (header + Alice)", len(records))
	}
	header := records[0]
	row := records[1]
	if header[0] != "id" || header[1] != "name" {
		t.Errorf("header = %v, want id,name,...", header)
	}
	if row[1] != "Alice Example" {
		t.Errorf("name = %q, want Alice Example", row[1])
	}
	if row[3] != "+91 98765 43210" {
		t.Errorf("primary_phone = %q, want primary", row[3])
	}
	if row[4] != "+91 98765 43210; +91 11111 22222" {
		t.Errorf("all_phones = %q, want primary-first joined", row[4])
	}
	if row[5] != "alice@example.com" {
		t.Errorf("primary_email = %q", row[5])
	}
	if row[7] != "Active" {
		t.Errorf("status = %q, want Active", row[7])
	}
	if row[8] != "VIP" {
		t.Errorf("tags = %q, want VIP", row[8])
	}
}

func TestExportLeadsCSVIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var pipelineID, stageID string
	if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ('Test Pipeline') RETURNING id`).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRow(
		`INSERT INTO lead_stages (pipeline_id, name) VALUES ($1, 'New') RETURNING id`,
		pipelineID,
	).Scan(&stageID); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	var contactID string
	if err := db.QueryRow(`INSERT INTO contacts (name) VALUES ('Alice Example') RETURNING id`).Scan(&contactID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO contact_phones (contact_id, value, is_primary) VALUES ($1, '9876543210', true)`,
		contactID,
	); err != nil {
		t.Fatalf("seed phone: %v", err)
	}
	var programID string
	if err := db.QueryRow(`INSERT INTO programs (name, price) VALUES ('Coaching', 25000) RETURNING id`).Scan(&programID); err != nil {
		t.Fatalf("seed program: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO leads (nickname, contact_id, pipeline_id, stage_id, program_id, value, outcome, lost_reason)
		VALUES ('Lead Nick', $1, $2, $3, $4, 25000, 'lost', 'Not intrested')`,
		contactID, pipelineID, stageID, programID,
	); err != nil {
		t.Fatalf("seed lead: %v", err)
	}

	var sb strings.Builder
	if err := svc.ExportLeadsCSV(&sb); err != nil {
		t.Fatalf("export leads: %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(sb.String())).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("rows = %d, want 2 (header + lead)", len(records))
	}
	row := records[1]
	if row[1] != "Lead Nick" {
		t.Errorf("nickname = %q, want Lead Nick", row[1])
	}
	if row[2] != "Alice Example" {
		t.Errorf("contact_name = %q, want Alice Example", row[2])
	}
	if row[3] != "9876543210" {
		t.Errorf("contact_phone = %q, want 9876543210", row[3])
	}
	if row[5] != "Test Pipeline" {
		t.Errorf("pipeline = %q, want Test Pipeline", row[5])
	}
	if row[6] != "New" {
		t.Errorf("stage = %q, want New", row[6])
	}
	if row[7] != "lost" {
		t.Errorf("outcome = %q, want lost", row[7])
	}
	if row[8] != "Not intrested" {
		t.Errorf("lost_reason = %q, want Not intrested", row[8])
	}
	if row[9] != "Coaching" {
		t.Errorf("program = %q, want Coaching", row[9])
	}
	if row[10] != "25000.00" {
		t.Errorf("value = %q, want 25000.00", row[10])
	}
}
