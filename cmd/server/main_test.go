package main

import (
	"crm/internal/assets"
	"database/sql"
	"net/url"
	"os"
	"testing"
	"time"
)

// scratchMigrationDSN creates a disposable database for the migration
// lifecycle test and returns its DSN, so the assertions never touch the
// developer's working database.
func scratchMigrationDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping integration test")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	const dbName = "crm_itest_migration_close"

	admin := *u
	admin.Path = "/postgres"
	adminConn, err := sql.Open("pgx", admin.String())
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	defer adminConn.Close()
	if _, err := adminConn.Exec(`DROP DATABASE IF EXISTS "` + dbName + `"`); err != nil {
		t.Fatalf("drop scratch database: %v", err)
	}
	if _, err := adminConn.Exec(`CREATE DATABASE "` + dbName + `"`); err != nil {
		t.Fatalf("create scratch database: %v", err)
	}

	test := *u
	test.Path = "/" + dbName
	return test.String()
}

// runMigrations must release the migration driver's own connection when it
// returns — on the applied path and on the no-change path; a leak keeps one
// idle connection open per boot for the process lifetime.
func TestRunMigrationsClosesDriverConnection(t *testing.T) {
	dsn := scratchMigrationDSN(t)
	appAssets, err := assets.NewLocal("../..")
	if err != nil {
		t.Skipf("local assets unavailable (build the frontend first): %v", err)
	}

	if err := runMigrations(dsn, appAssets); err != nil {
		t.Fatalf("first runMigrations: %v", err)
	}
	if err := runMigrations(dsn, appAssets); err != nil {
		t.Fatalf("second runMigrations (no change): %v", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open scratch database: %v", err)
	}
	defer db.Close()

	// The server may reap a closed TCP connection a moment after the client
	// closes it, so poll briefly before declaring a leak.
	deadline := time.Now().Add(2 * time.Second)
	var conns int
	for {
		if err := db.QueryRow(
			`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND pid <> pg_backend_pid()`,
		).Scan(&conns); err != nil {
			t.Fatalf("count connections: %v", err)
		}
		if conns == 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if conns != 0 {
		t.Errorf("migration driver left %d connection(s) open after the run", conns)
	}
}
