package testdb

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// The schema-hardening migration must be safe on databases that already hold
// data (invalid legacy links get cleared, valid links survive, and the down
// path restores the old schema). testdb.New always migrates a fresh database,
// so these tests drive golang-migrate directly against a disposable scratch
// database.

// scratchDB creates a brand-new disposable database (dropped and recreated
// each run) without applying any migrations, and returns the connection and
// its DSN.
func scratchDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping integration test")
	}
	const dbName = "crm_itest_upgrade"

	adminDSN := setPath(t, dsn, "/postgres")
	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatalf("scratchDB: open admin connection: %v", err)
	}
	defer admin.Close()
	admin.SetMaxOpenConns(2)
	if _, err := admin.Exec(`DROP DATABASE IF EXISTS "` + dbName + `"`); err != nil {
		t.Fatalf("scratchDB: drop database: %v", err)
	}
	if _, err := admin.Exec(`CREATE DATABASE "` + dbName + `"`); err != nil {
		t.Fatalf("scratchDB: create database: %v", err)
	}

	testDSN := setPath(t, dsn, "/"+dbName)
	db, err := sql.Open("pgx", testDSN)
	if err != nil {
		t.Fatalf("scratchDB: open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(4)
	if err := db.Ping(); err != nil {
		t.Fatalf("scratchDB: ping test database: %v", err)
	}
	return db, testDSN
}

// migrateScratch moves the scratch database to the given migration version
// (forward or backward).
func migrateScratch(t *testing.T, dsn string, version uint) {
	t.Helper()
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("migrateScratch: cannot resolve calling package")
	}
	src, err := iofs.New(os.DirFS(migrationsDir(t, filepath.Dir(file))), ".")
	if err != nil {
		t.Fatalf("migrateScratch: migration source: %v", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		t.Fatalf("migrateScratch: migrate init: %v", err)
	}
	defer m.Close()
	if err := m.Migrate(version); err != nil {
		t.Fatalf("migrateScratch: migrate to %d: %v", version, err)
	}
}

func TestUpgradeCleansInvalidLegacyLinks(t *testing.T) {
	db, dsn := scratchDB(t)
	migrateScratch(t, dsn, 5)

	var statusID, plainTagID, quickReplyID string
	for _, row := range []struct {
		id   *string
		name string
		typ  string
	}{
		{&statusID, "New", "status"},
		{&plainTagID, "Hot", "tag"},
		{&quickReplyID, "No Reply", "quick_reply"},
	} {
		if err := db.QueryRow(`INSERT INTO tags (name, type) VALUES ($1, $2) RETURNING id`, row.name, row.typ).Scan(row.id); err != nil {
			t.Fatalf("seed tag %s: %v", row.name, err)
		}
	}
	var validContact, invalidContact string
	if err := db.QueryRow(
		`INSERT INTO contacts (name, status_id) VALUES ('Valid', $1) RETURNING id`, statusID,
	).Scan(&validContact); err != nil {
		t.Fatalf("seed valid contact: %v", err)
	}
	if err := db.QueryRow(
		`INSERT INTO contacts (name, status_id) VALUES ('Invalid', $1) RETURNING id`, plainTagID,
	).Scan(&invalidContact); err != nil {
		t.Fatalf("seed invalid contact: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO contacts (name) VALUES ('Null')`); err != nil {
		t.Fatalf("seed null-status contact: %v", err)
	}

	var pipelineID, stageID, leadID string
	if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ('P') RETURNING id`).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order") VALUES ($1, 'Open', 0) RETURNING id`, pipelineID).Scan(&stageID); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	if err := db.QueryRow(
		`INSERT INTO leads (contact_id, pipeline_id, stage_id) VALUES ($1, $2, $3) RETURNING id`,
		validContact, pipelineID, stageID,
	).Scan(&leadID); err != nil {
		t.Fatalf("seed lead: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO lead_activities (lead_id, stage_id, quick_reply_id) VALUES ($1, $2, $3)`,
		leadID, stageID, quickReplyID,
	); err != nil {
		t.Fatalf("seed valid activity: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO lead_activities (lead_id, stage_id, quick_reply_id) VALUES ($1, $2, $3)`,
		leadID, stageID, statusID,
	); err != nil {
		t.Fatalf("seed invalid activity: %v", err)
	}

	migrateScratch(t, dsn, 6)

	// The invalid links were cleared; the valid ones survived.
	var status sql.NullString
	if err := db.QueryRow(`SELECT status_id FROM contacts WHERE id = $1`, invalidContact).Scan(&status); err != nil {
		t.Fatalf("load invalid contact: %v", err)
	}
	if status.Valid {
		t.Error("invalid status link survived the upgrade; want it cleared")
	}
	if err := db.QueryRow(`SELECT status_id FROM contacts WHERE id = $1`, validContact).Scan(&status); err != nil {
		t.Fatalf("load valid contact: %v", err)
	}
	if !status.Valid || status.String != statusID {
		t.Errorf("valid status link = %v, want kept", status)
	}

	var invalidActivities int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM lead_activities WHERE quick_reply_id = $1`,
		statusID,
	).Scan(&invalidActivities); err != nil {
		t.Fatalf("count invalid quick replies: %v", err)
	}
	if invalidActivities != 0 {
		t.Errorf("invalid quick-reply links = %d, want 0", invalidActivities)
	}

	// The type guard is live after the upgrade.
	if _, err := db.Exec(`INSERT INTO contacts (name, status_id) VALUES ('X', $1)`, plainTagID); err == nil {
		t.Error("plain-tag status link inserted after upgrade; type guard must reject it")
	}
	if _, err := db.Exec(`DELETE FROM tags WHERE id = $1`, statusID); err == nil {
		t.Error("referenced status deleted after upgrade; RESTRICT must refuse it")
	}
}

func TestDownMigrationRestoresSetNullSemantics(t *testing.T) {
	db, dsn := scratchDB(t)
	migrateScratch(t, dsn, 6)

	var statusID, contactID string
	if err := db.QueryRow(`INSERT INTO tags (name, type) VALUES ('New', 'status') RETURNING id`).Scan(&statusID); err != nil {
		t.Fatalf("seed status tag: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO contacts (name, status_id) VALUES ('Alice', $1) RETURNING id`, statusID).Scan(&contactID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}

	migrateScratch(t, dsn, 5)

	// The old SET NULL FK is back: deleting the referenced status succeeds
	// and clears the link instead of failing.
	if _, err := db.Exec(`DELETE FROM tags WHERE id = $1`, statusID); err != nil {
		t.Fatalf("delete status after downgrade: %v", err)
	}
	var status sql.NullString
	if err := db.QueryRow(`SELECT status_id FROM contacts WHERE id = $1`, contactID).Scan(&status); err != nil {
		t.Fatalf("load contact: %v", err)
	}
	if status.Valid {
		t.Error("status link survived the downgrade; SET NULL must have cleared it")
	}

	// The hardening objects are gone.
	for _, idx := range []string{"idx_tags_id_type", "idx_leads_contact_id", "idx_refresh_tokens_user_id"} {
		var exists bool
		if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE indexname = $1)`, idx).Scan(&exists); err != nil {
			t.Fatalf("check index %s: %v", idx, err)
		}
		if exists {
			t.Errorf("index %s survived the downgrade", idx)
		}
	}
	var hasDOB bool
	if err := db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = 'contacts' AND column_name = 'date_of_birth')`,
	).Scan(&hasDOB); err != nil {
		t.Fatalf("check date_of_birth column: %v", err)
	}
	if hasDOB {
		t.Error("date_of_birth survived the downgrade")
	}
	var hasStatusType bool
	if err := db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = 'contacts' AND column_name = 'status_type')`,
	).Scan(&hasStatusType); err != nil {
		t.Fatalf("check status_type column: %v", err)
	}
	if hasStatusType {
		t.Error("status_type generated column survived the downgrade")
	}
}