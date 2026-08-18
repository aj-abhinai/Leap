package seed

import (
	"crm/internal/config"
	"crm/internal/testdb"
	"database/sql"
	"testing"
)

var testAuthCfg = config.Auth{BcryptCost: 4}

func TestSeedBootstrapsAdminOnEmptyDatabase(t *testing.T) {
	db := testdb.New(t)
	superadmin := config.Superadmin{Email: "admin@admin.com", Password: "admin"}

	if err := Seed(db, testAuthCfg, superadmin); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	assertBootstrapAdmin(t, db, superadmin.Email, true)

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("users after first seed = %d, want 1", count)
	}
}

func TestSeedNeverReconcilesOnLaterBoots(t *testing.T) {
	db := testdb.New(t)
	superadmin := config.Superadmin{Email: "admin@admin.com", Password: "admin"}

	if err := Seed(db, testAuthCfg, superadmin); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if err := Seed(db, testAuthCfg, superadmin); err != nil {
		t.Fatalf("second Seed: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("users after second seed = %d, want 1 (bootstrap must happen once)", count)
	}
}

func TestSeedSkipsBootstrapWhenUsersExist(t *testing.T) {
	db := testdb.New(t)

	if _, err := db.Exec(
		`INSERT INTO users (name, email, password_hash) VALUES ('Existing', 'existing@example.com', 'hash')`,
	); err != nil {
		t.Fatalf("insert existing user: %v", err)
	}

	superadmin := config.Superadmin{Email: "admin@admin.com", Password: "admin"}
	if err := Seed(db, testAuthCfg, superadmin); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	var exists bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, superadmin.Email).Scan(&exists); err != nil {
		t.Fatalf("check bootstrap admin: %v", err)
	}
	if exists {
		t.Fatal("bootstrap admin must not be created once users already exist")
	}
}

func TestSeedSuperadminRoleNeverReassigns(t *testing.T) {
	db := testdb.New(t)

	if _, err := db.Exec(
		`INSERT INTO users (name, email, password_hash) VALUES ('Existing', 'super@example.com', 'hash')`,
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	// The role + wildcard are ensured every boot, but the user is never
	// assigned the role (no reconciliation on later boots).
	if err := seedSuperadminRole(db, config.Superadmin{Email: "super@example.com"}); err != nil {
		t.Fatalf("seedSuperadminRole: %v", err)
	}
	assertWildcardLinked(t, db)
	assertRoleAssigned(t, db, "super@example.com", false)
}

func TestSeedSuperadminBootstrapsAdminWithRole(t *testing.T) {
	db := testdb.New(t)
	superadmin := config.Superadmin{Email: "admin@admin.com", Password: "admin"}

	created, err := seedSuperadmin(db, testAuthCfg, superadmin)
	if err != nil {
		t.Fatalf("seedSuperadmin: %v", err)
	}
	if !created {
		t.Fatal("seedSuperadmin should report creation on an empty database")
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("users after bootstrap = %d, want 1", count)
	}
	assertRoleAssigned(t, db, superadmin.Email, true)
}

func TestSeedSuperadminSkipsAndRollsBackOnExistingUsers(t *testing.T) {
	db := testdb.New(t)

	if _, err := db.Exec(
		`INSERT INTO users (name, email, password_hash) VALUES ('Existing', 'existing@example.com', 'hash')`,
	); err != nil {
		t.Fatalf("insert existing user: %v", err)
	}

	superadmin := config.Superadmin{Email: "admin@admin.com", Password: "admin"}
	created, err := seedSuperadmin(db, testAuthCfg, superadmin)
	if err != nil {
		t.Fatalf("seedSuperadmin: %v", err)
	}
	if created {
		t.Fatal("seedSuperadmin must not bootstrap when users already exist")
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("users after skipped bootstrap = %d, want 1 (no insert, no role assignment)", count)
	}
}

func TestSeedNormalizesSuperadminEmail(t *testing.T) {
	db := testdb.New(t)
	superadmin := config.Superadmin{Email: "  Admin@Example.com ", Password: "admin"}

	if err := Seed(db, testAuthCfg, superadmin); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	// Login normalizes email with util.NormalizeEmail, so the stored address
	// must be the normalized form or a mixed-case config cannot log in.
	var email string
	if err := db.QueryRow(`SELECT email FROM users`).Scan(&email); err != nil {
		t.Fatalf("load seeded email: %v", err)
	}
	if email != "admin@example.com" {
		t.Errorf("seeded email = %q, want admin@example.com", email)
	}
}

func TestSeedSeedsTagCatalog(t *testing.T) {
	db := testdb.New(t)
	superadmin := config.Superadmin{Email: "admin@admin.com", Password: "admin"}

	if err := Seed(db, testAuthCfg, superadmin); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	// The four simple catalogs (name+type only) must appear exactly once each,
	// and the quick-reply catalog (with its group/sort/behavior reconciliation)
	// must be present with its seven entries.
	want := map[string]int{
		"tag":           4,
		"status":        4,
		"activity_type": 4,
		"loss_reason":   2,
		"quick_reply":   7,
	}
	for typ, count := range want {
		var got int
		if err := db.QueryRow(`SELECT COUNT(*) FROM tags WHERE type = $1`, typ).Scan(&got); err != nil {
			t.Fatalf("count tags type %q: %v", typ, err)
		}
		if got != count {
			t.Errorf("tags of type %q = %d, want %d", typ, got, count)
		}
	}
}

func assertBootstrapAdmin(t *testing.T, db *sql.DB, email string, wantChangePassword bool) {
	t.Helper()

	var mustChange bool
	var roleAssigned bool
	err := db.QueryRow(`
		SELECT u.must_change_password, r.name IS NOT NULL
		FROM users u
		LEFT JOIN roles r ON r.id = u.role_id
		WHERE u.email = $1`,
		email,
	).Scan(&mustChange, &roleAssigned)
	if err != nil {
		t.Fatalf("load bootstrap admin: %v", err)
	}
	if !roleAssigned {
		t.Error("bootstrap admin should hold the superadmin role")
	}
	if mustChange != wantChangePassword {
		t.Errorf("bootstrap admin must_change_password = %v, want %v", mustChange, wantChangePassword)
	}
}

func assertWildcardLinked(t *testing.T, db *sql.DB) {
	t.Helper()

	var linked bool
	if err := db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM role_permissions rp
			JOIN roles r ON r.id = rp.role_id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE r.name = 'superadmin' AND p.name = '*'
		)`,
	).Scan(&linked); err != nil {
		t.Fatalf("check wildcard permission: %v", err)
	}
	if !linked {
		t.Error("superadmin role should carry the wildcard permission")
	}
}

func assertRoleAssigned(t *testing.T, db *sql.DB, email string, want bool) {
	t.Helper()

	var assigned bool
	if err := db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM users u
			JOIN roles r ON r.id = u.role_id
			WHERE u.email = $1 AND r.name = 'superadmin'
		)`,
		email,
	).Scan(&assigned); err != nil {
		t.Fatalf("check superadmin assignment: %v", err)
	}
	if assigned != want {
		t.Errorf("superadmin role assigned for %q = %v, want %v", email, assigned, want)
	}
}
