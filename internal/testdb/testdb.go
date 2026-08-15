package testdb

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// New opens a disposable PostgreSQL database for the calling test package,
// applies the repository migrations, and truncates every table so the test
// starts from a clean schema. It skips the test when TEST_DATABASE_URL is
// unset. Each test package gets its own database on the same server, so
// packages may run in parallel without interfering with each other. The
// per-package database persists between runs; truncation at New keeps it
// fresh. Closing the returned connection is registered with t.Cleanup.
func New(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping integration test")
	}
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("testdb: cannot resolve calling package")
	}
	pkgDir := filepath.Dir(file)
	dbName := dbNameFor(pkgDir)

	adminDSN := setPath(t, dsn, "/postgres")
	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatalf("testdb: open admin connection: %v", err)
	}
	defer admin.Close()
	admin.SetMaxOpenConns(4)
	ensureDatabase(t, admin, dbName)

	testDSN := setPath(t, dsn, "/"+dbName)
	db, err := sql.Open("pgx", testDSN)
	if err != nil {
		t.Fatalf("testdb: open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// Cap the pool: pgx defaults to one connection per CPU, and the full suite
	// runs many packages in parallel, which can exhaust the database server.
	db.SetMaxOpenConns(8)
	if err := db.Ping(); err != nil {
		t.Fatalf("testdb: ping test database: %v", err)
	}
	migrateDB(t, pkgDir, testDSN)
	truncateAll(t, db)
	return db
}

// setPath returns dsn with its database path replaced, failing the test on
// an unparseable URL.
func setPath(t *testing.T, dsn, path string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("testdb: parse TEST_DATABASE_URL: %v", err)
	}
	u.Path = path
	return u.String()
}

func dbNameFor(pkgDir string) string {
	name := strings.ToLower(filepath.Base(pkgDir))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return "crm_itest_" + b.String()
}

func ensureDatabase(t *testing.T, admin *sql.DB, name string) {
	t.Helper()
	var exists bool
	if err := admin.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`,
		name,
	).Scan(&exists); err != nil {
		t.Fatalf("testdb: check database %q: %v", name, err)
	}
	if exists {
		return
	}
	if _, err := admin.Exec(`CREATE DATABASE "` + name + `"`); err != nil {
		t.Fatalf("testdb: create database %q (TEST_DATABASE_URL user needs CREATEDB privilege): %v", name, err)
	}
}

func migrateDB(t *testing.T, pkgDir, dsn string) {
	t.Helper()
	dir := migrationsDir(t, pkgDir)
	src, err := iofs.New(os.DirFS(dir), ".")
	if err != nil {
		t.Fatalf("testdb: migration source %q: %v", dir, err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		t.Fatalf("testdb: migrate init: %v", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("testdb: migrate up: %v", err)
	}
}

func migrationsDir(t *testing.T, pkgDir string) string {
	t.Helper()
	dir := pkgDir
	for {
		candidate := filepath.Join(dir, "migrations")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("testdb: cannot locate migrations directory from %s", pkgDir)
	return ""
}

func truncateAll(t *testing.T, db *sql.DB) {
	t.Helper()
	var tables string
	if err := db.QueryRow(`
		SELECT COALESCE(string_agg(quote_ident(tablename), ', '), '')
		FROM pg_tables
		WHERE schemaname = 'public' AND tablename <> 'schema_migrations'
	`).Scan(&tables); err != nil {
		t.Fatalf("testdb: list tables: %v", err)
	}
	if tables == "" {
		return
	}
	if _, err := db.Exec("TRUNCATE TABLE " + tables + " RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("testdb: truncate tables: %v", err)
	}
}
