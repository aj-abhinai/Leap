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

	// The five catalogs must appear exactly once each: four simple (name+type
	// only) plus the quick-reply catalog. Enquiry rides the activity-type catalog as the
	// repeat-enquiry touchpoint preset.
	want := map[string]int{
		"tag":           3,
		"status":        4,
		"activity_type": 5,
		"loss_reason":   3,
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

func TestSeedNeverOverwritesQuickReplyEdits(t *testing.T) {
	db := testdb.New(t)
	superadmin := config.Superadmin{Email: "admin@admin.com", Password: "admin"}

	if err := Seed(db, testAuthCfg, superadmin); err != nil {
		t.Fatalf("initial Seed: %v", err)
	}
	if _, err := db.Exec(
		`UPDATE tags SET group_name = 'Custom', sort_order = 99, behavior = 'log'
		WHERE name = 'No Reply' AND type = 'quick_reply'`,
	); err != nil {
		t.Fatalf("edit quick reply: %v", err)
	}
	if err := Seed(db, testAuthCfg, superadmin); err != nil {
		t.Fatalf("second Seed: %v", err)
	}

	var group, behavior string
	var order int
	if err := db.QueryRow(
		`SELECT group_name, sort_order, behavior FROM tags WHERE name = 'No Reply' AND type = 'quick_reply'`,
	).Scan(&group, &order, &behavior); err != nil {
		t.Fatalf("load quick reply: %v", err)
	}
	if group != "Custom" || order != 99 || behavior != "log" {
		t.Fatalf("quick reply after second Seed = (%q, %d, %q), want admin edit", group, order, behavior)
	}
}

func TestSeedSeedsSystemRoles(t *testing.T) {
	db := testdb.New(t)
	superadmin := config.Superadmin{Email: "admin@admin.com", Password: "admin"}

	if err := Seed(db, testAuthCfg, superadmin); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	// The seeded trio is permanent (is_system): superadmin from the bootstrap
	// path, Sales and Viewer from the system-role catalog.
	want := map[string]struct {
		permissions map[string]bool
	}{
		"superadmin": {permissions: map[string]bool{"*": true}},
		"Sales":      {permissions: map[string]bool{"contact:read": true, "contact:write": true, "lead:read": true, "lead:write": true}},
		"Viewer":     {permissions: map[string]bool{"contact:read": true, "lead:read": true}},
	}
	for name, spec := range want {
		var isSystem bool
		if err := db.QueryRow(`SELECT is_system FROM roles WHERE name = $1`, name).Scan(&isSystem); err != nil {
			t.Fatalf("load role %s: %v", name, err)
		}
		if !isSystem {
			t.Errorf("role %s is_system = false, want true", name)
		}
		rows, err := db.Query(
			`SELECT p.name FROM role_permissions rp
			JOIN roles r ON r.id = rp.role_id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE r.name = $1`,
			name,
		)
		if err != nil {
			t.Fatalf("load role permissions %s: %v", name, err)
		}
		got := map[string]bool{}
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err != nil {
				t.Fatalf("scan role permission %s: %v", name, err)
			}
			got[p] = true
		}
		rows.Close()
		for perm := range spec.permissions {
			if !got[perm] {
				t.Errorf("role %s missing permission %s", name, perm)
			}
		}
		for perm := range got {
			if !spec.permissions[perm] {
				t.Errorf("role %s carries unexpected permission %s", name, perm)
			}
		}
	}

	// The retired permission must not exist after seeding.
	var retired int
	if err := db.QueryRow(`SELECT COUNT(*) FROM permissions WHERE name = 'activity:read'`).Scan(&retired); err != nil {
		t.Fatalf("count activity:read: %v", err)
	}
	if retired != 0 {
		t.Errorf("activity:read count = %d, want 0 (retired)", retired)
	}

	// A second boot must not duplicate roles or add stray permissions.
	if err := Seed(db, testAuthCfg, superadmin); err != nil {
		t.Fatalf("second Seed: %v", err)
	}
	var roles int
	if err := db.QueryRow(`SELECT COUNT(*) FROM roles WHERE name IN ('Sales', 'Viewer')`).Scan(&roles); err != nil {
		t.Fatalf("count seeded roles: %v", err)
	}
	if roles != 2 {
		t.Errorf("Sales/Viewer rows = %d, want 2 (idempotent)", roles)
	}
}

// The system-role catalog seeds the canonical permission set only on the
// fresh insert; a permission edit by an admin must survive every later boot.
func TestSeedNeverReassertsEditedSystemRolePermissions(t *testing.T) {
	db := testdb.New(t)
	superadmin := config.Superadmin{Email: "admin@admin.com", Password: "admin"}

	if err := Seed(db, testAuthCfg, superadmin); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if _, err := db.Exec(
		`DELETE FROM role_permissions rp
		USING roles r, permissions p
		WHERE rp.role_id = r.id AND rp.permission_id = p.id
		  AND r.name = 'Sales' AND p.name = 'lead:write'`,
	); err != nil {
		t.Fatalf("remove lead:write from Sales: %v", err)
	}
	if err := Seed(db, testAuthCfg, superadmin); err != nil {
		t.Fatalf("second Seed: %v", err)
	}

	var linked bool
	if err := db.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM role_permissions rp
			JOIN roles r ON r.id = rp.role_id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE r.name = 'Sales' AND p.name = 'lead:write'
		)`,
	).Scan(&linked); err != nil {
		t.Fatalf("check lead:write link: %v", err)
	}
	if linked {
		t.Error("lead:write was re-asserted on Sales after the admin removed it")
	}
}

// clearSeedFailureTrigger removes the forced-failure trigger a previous run
// may have left in the persistent per-package test database (testdb truncates
// tables but not functions/triggers), so the tests are self-healing after a
// hard-killed run.
func clearSeedFailureTrigger(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS seed_fail_contacted ON lead_stages`); err != nil {
		t.Fatalf("drop failure trigger: %v", err)
	}
	if _, err := db.Exec(`DROP FUNCTION IF EXISTS seed_fail_contacted() CASCADE`); err != nil {
		t.Fatalf("drop failure trigger function: %v", err)
	}
}

func TestSeedDefaultPipelineIntegration(t *testing.T) {
	db := testdb.New(t)
	clearSeedFailureTrigger(t, db)

	if err := seedDefaultPipeline(db); err != nil {
		t.Fatalf("seedDefaultPipeline: %v", err)
	}
	var pipelineID string
	if err := db.QueryRow(`SELECT id FROM pipelines WHERE name = 'Default Pipeline'`).Scan(&pipelineID); err != nil {
		t.Fatalf("load default pipeline: %v", err)
	}
	var stages int
	if err := db.QueryRow(`SELECT COUNT(*) FROM lead_stages WHERE pipeline_id = $1`, pipelineID).Scan(&stages); err != nil {
		t.Fatalf("count stages: %v", err)
	}
	if stages != 5 {
		t.Fatalf("stages = %d, want 5", stages)
	}

	// A second boot must not duplicate the pipeline or its stages.
	if err := seedDefaultPipeline(db); err != nil {
		t.Fatalf("reseed default pipeline: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM lead_stages WHERE pipeline_id = $1`, pipelineID).Scan(&stages); err != nil {
		t.Fatalf("recount stages: %v", err)
	}
	if stages != 5 {
		t.Fatalf("stages after reseed = %d, want 5", stages)
	}
}

func TestSeedDefaultPipelineRollsBackOnStageFailureIntegration(t *testing.T) {
	db := testdb.New(t)
	clearSeedFailureTrigger(t, db)

	if _, err := db.Exec(`
		CREATE OR REPLACE FUNCTION seed_fail_contacted() RETURNS trigger AS $$
		BEGIN
			IF NEW.name = 'Contacted' THEN
				RAISE EXCEPTION 'forced stage failure';
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql`); err != nil {
		t.Fatalf("create failure trigger function: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DROP FUNCTION IF EXISTS seed_fail_contacted() CASCADE`)
	})
	if _, err := db.Exec(`
		CREATE TRIGGER seed_fail_contacted BEFORE INSERT ON lead_stages
		FOR EACH ROW EXECUTE FUNCTION seed_fail_contacted()`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	if err := seedDefaultPipeline(db); err == nil {
		t.Fatal("seedDefaultPipeline succeeded despite the forced stage failure")
	}
	var pipelines, stages int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pipelines`).Scan(&pipelines); err != nil {
		t.Fatalf("count pipelines: %v", err)
	}
	if pipelines != 0 {
		t.Fatalf("pipelines after failed seed = %d, want 0 (the seed must roll back)", pipelines)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM lead_stages`).Scan(&stages); err != nil {
		t.Fatalf("count stages: %v", err)
	}
	if stages != 0 {
		t.Fatalf("stages after failed seed = %d, want 0 (the seed must roll back)", stages)
	}
}

func TestSeedDoesNotResurrectDeletedStarterDataIntegration(t *testing.T) {
	db := testdb.New(t)
	superadmin := config.Superadmin{Email: "admin@admin.com", Password: "admin"}

	if err := Seed(db, testAuthCfg, superadmin); err != nil {
		t.Fatalf("initial Seed: %v", err)
	}

	// Remove a seeded tag and the default pipeline the way an operator would.
	if _, err := db.Exec(`DELETE FROM tags WHERE name = 'Student' AND type = 'tag'`); err != nil {
		t.Fatalf("delete seeded tag: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM pipelines WHERE name = 'Default Pipeline'`); err != nil {
		t.Fatalf("delete default pipeline: %v", err)
	}

	if err := Seed(db, testAuthCfg, superadmin); err != nil {
		t.Fatalf("second Seed: %v", err)
	}

	var tags int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tags WHERE name = 'Student' AND type = 'tag'`).Scan(&tags); err != nil {
		t.Fatalf("count tags: %v", err)
	}
	if tags != 0 {
		t.Errorf("deleted tag count = %d, want 0 after a restart", tags)
	}
	var pipelines int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pipelines WHERE name = 'Default Pipeline'`).Scan(&pipelines); err != nil {
		t.Fatalf("count pipelines: %v", err)
	}
	if pipelines != 0 {
		t.Errorf("deleted default pipeline count = %d, want 0 after a restart", pipelines)
	}
}

func TestSeedDoesNotDuplicateRenamedPipelineIntegration(t *testing.T) {
	db := testdb.New(t)
	superadmin := config.Superadmin{Email: "admin@admin.com", Password: "admin"}

	if err := Seed(db, testAuthCfg, superadmin); err != nil {
		t.Fatalf("initial Seed: %v", err)
	}
	if _, err := db.Exec(`UPDATE pipelines SET name = 'Sales Pipeline' WHERE name = 'Default Pipeline'`); err != nil {
		t.Fatalf("rename default pipeline: %v", err)
	}
	if err := Seed(db, testAuthCfg, superadmin); err != nil {
		t.Fatalf("second Seed: %v", err)
	}

	var pipelines int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pipelines`).Scan(&pipelines); err != nil {
		t.Fatalf("count pipelines: %v", err)
	}
	if pipelines != 1 {
		t.Errorf("pipelines after rename + reseed = %d, want 1 (no duplicate)", pipelines)
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
