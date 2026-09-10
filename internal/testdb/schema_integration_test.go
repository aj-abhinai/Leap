package testdb

import (
	"strings"
	"testing"
)

// The schema-hardening migration flips vocabulary links to type-guarded
// RESTRICT FKs; these tests pin the database-level contract so the app-level
// guards (which the services own) always have a backstop.

func TestStatusLinkRejectsNonStatusTag(t *testing.T) {
	db := New(t)
	var tagID string
	if err := db.QueryRow(`INSERT INTO tags (name, type) VALUES ('Hot', 'tag') RETURNING id`).Scan(&tagID); err != nil {
		t.Fatalf("insert plain tag: %v", err)
	}

	// A plain tag is not a status: linking it must fail in the database with
	// the composite-FK violation, not a generic error.
	_, err := db.Exec(`INSERT INTO contacts (name, status_id) VALUES ('Alice', $1)`, tagID)
	if err == nil {
		t.Fatal("contact with a plain-tag status inserted; type guard must reject it")
	}
	if !strings.Contains(err.Error(), "violates foreign key constraint") {
		t.Errorf("insert error = %v, want a foreign key violation", err)
	}
}

func TestStatusLinkAcceptsStatusTag(t *testing.T) {
	db := New(t)
	var tagID string
	if err := db.QueryRow(`INSERT INTO tags (name, type) VALUES ('New', 'status') RETURNING id`).Scan(&tagID); err != nil {
		t.Fatalf("insert status tag: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO contacts (name, status_id) VALUES ('Alice', $1)`, tagID); err != nil {
		t.Fatalf("contact with a status tag: %v", err)
	}
}

func TestDeletingReferencedStatusFails(t *testing.T) {
	db := New(t)
	var tagID string
	if err := db.QueryRow(`INSERT INTO tags (name, type) VALUES ('New', 'status') RETURNING id`).Scan(&tagID); err != nil {
		t.Fatalf("insert status tag: %v", err)
	}
	var contactID string
	if err := db.QueryRow(`INSERT INTO contacts (name, status_id) VALUES ('Alice', $1) RETURNING id`, tagID).Scan(&contactID); err != nil {
		t.Fatalf("insert contact: %v", err)
	}

	if _, err := db.Exec(`DELETE FROM tags WHERE id = $1`, tagID); err == nil {
		t.Fatal("deleting a referenced status succeeded; RESTRICT must refuse it")
	}

	// History stays untouched until the referencing row goes away.
	if _, err := db.Exec(`DELETE FROM contacts WHERE id = $1`, contactID); err != nil {
		t.Fatalf("delete contact: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM tags WHERE id = $1`, tagID); err != nil {
		t.Fatalf("delete status after contact removal: %v", err)
	}
}

func TestQuickReplyLinkRejectsNonQuickReplyTag(t *testing.T) {
	db := New(t)
	var pipelineID string
	if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ('P') RETURNING id`).Scan(&pipelineID); err != nil {
		t.Fatalf("insert pipeline: %v", err)
	}
	var stageID string
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order") VALUES ($1, 'Open', 0) RETURNING id`, pipelineID).Scan(&stageID); err != nil {
		t.Fatalf("insert stage: %v", err)
	}
	var contactID string
	if err := db.QueryRow(`INSERT INTO contacts (name) VALUES ('Alice') RETURNING id`).Scan(&contactID); err != nil {
		t.Fatalf("insert contact: %v", err)
	}
	var leadID string
	if err := db.QueryRow(
		`INSERT INTO leads (contact_id, pipeline_id, stage_id) VALUES ($1, $2, $3) RETURNING id`,
		contactID, pipelineID, stageID,
	).Scan(&leadID); err != nil {
		t.Fatalf("insert lead: %v", err)
	}
	var statusTagID string
	if err := db.QueryRow(`INSERT INTO tags (name, type) VALUES ('New', 'status') RETURNING id`).Scan(&statusTagID); err != nil {
		t.Fatalf("insert status tag: %v", err)
	}

	// A status tag is not a quick reply: the link must fail in the database.
	if _, err := db.Exec(
		`INSERT INTO lead_activities (lead_id, stage_id, quick_reply_id) VALUES ($1, $2, $3)`,
		leadID, stageID, statusTagID,
	); err == nil {
		t.Fatal("activity with a non-quick-reply link inserted; type guard must reject it")
	}

	var quickReplyID string
	if err := db.QueryRow(`INSERT INTO tags (name, type) VALUES ('No Reply', 'quick_reply') RETURNING id`).Scan(&quickReplyID); err != nil {
		t.Fatalf("insert quick reply: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO lead_activities (lead_id, stage_id, quick_reply_id) VALUES ($1, $2, $3)`,
		leadID, stageID, quickReplyID,
	); err != nil {
		t.Fatalf("activity with a quick reply: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM tags WHERE id = $1`, quickReplyID); err == nil {
		t.Fatal("deleting a referenced quick reply succeeded; RESTRICT must refuse it")
	}

	// Removing the referencing activity frees the tag for deletion.
	if _, err := db.Exec(`DELETE FROM lead_activities WHERE lead_id = $1`, leadID); err != nil {
		t.Fatalf("delete activities: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM tags WHERE id = $1`, quickReplyID); err != nil {
		t.Fatalf("delete quick reply after activity removal: %v", err)
	}
}

func TestJourneyAndTokenIndexesExist(t *testing.T) {
	db := New(t)
	for _, idx := range []string{"idx_leads_contact_id", "idx_refresh_tokens_user_id", "idx_tags_id_type"} {
		var exists bool
		if err := db.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE indexname = $1)`,
			idx,
		).Scan(&exists); err != nil {
			t.Fatalf("check index %s: %v", idx, err)
		}
		if !exists {
			t.Errorf("index %s is missing", idx)
		}
	}
}

func TestContactDateOfBirthColumn(t *testing.T) {
	db := New(t)
	if _, err := db.Exec(`INSERT INTO contacts (name) VALUES ('Alice')`); err != nil {
		t.Fatalf("contact without date of birth: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO contacts (name, date_of_birth) VALUES ('Bob', '2000-01-15')`); err != nil {
		t.Fatalf("contact with date of birth: %v", err)
	}
	var dob string
	if err := db.QueryRow(
		`SELECT date_of_birth::text FROM contacts WHERE name = 'Bob'`,
	).Scan(&dob); err != nil {
		t.Fatalf("load date of birth: %v", err)
	}
	if dob != "2000-01-15" {
		t.Errorf("date_of_birth = %q, want 2000-01-15", dob)
	}
}
