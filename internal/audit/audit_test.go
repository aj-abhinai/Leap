package audit

import (
	"crm/internal/testdb"
	"database/sql"
	"testing"
)

// An invalid changes diff must not lose the audit row: the description is the
// required record and the diff is optional extra data.
func TestLogCustomKeepsRowWhenChangesIsInvalidJSONIntegration(t *testing.T) {
	db := testdb.New(t)

	LogCustom(db, "Broke the diff", "settings", "", "update", "{not-json", "")

	var description string
	var changes sql.NullString
	if err := db.QueryRow(
		`SELECT description, changes FROM audit_logs WHERE resource_type = 'settings' ORDER BY created_at DESC LIMIT 1`,
	).Scan(&description, &changes); err != nil {
		t.Fatalf("load audit row: %v", err)
	}
	if description != "Broke the diff" {
		t.Errorf("description = %q, want the row kept", description)
	}
	if changes.Valid {
		t.Errorf("changes = %q, want NULL for invalid JSON", changes.String)
	}
}

func TestLogCustomStoresValidChangesIntegration(t *testing.T) {
	db := testdb.New(t)

	LogCustom(db, "Changed a thing", "settings", "", "update", `{"a":1}`, "")

	var value string
	if err := db.QueryRow(
		`SELECT changes ->> 'a' FROM audit_logs WHERE resource_type = 'settings' ORDER BY created_at DESC LIMIT 1`,
	).Scan(&value); err != nil {
		t.Fatalf("load audit changes: %v", err)
	}
	if value != "1" {
		t.Errorf("changes->>a = %q, want 1", value)
	}
}
