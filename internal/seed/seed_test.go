package seed

import (
	"crm/internal/config"
	"crm/internal/testdb"
	"testing"
)

func TestSeedSuperadminRoleRepairsExistingRole(t *testing.T) {
	db := testdb.New(t)

	var roleID string
	if err := db.QueryRow(
		`INSERT INTO roles (name, description) VALUES ('superadmin', 'Full system access') RETURNING id`,
	).Scan(&roleID); err != nil {
		t.Fatalf("insert superadmin role: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO users (name, email, password_hash) VALUES ('Super Admin', 'super@example.com', 'hash')`,
	); err != nil {
		t.Fatalf("insert superadmin user: %v", err)
	}

	if err := seedSuperadminRole(db, config.Superadmin{Email: "super@example.com"}); err != nil {
		t.Fatalf("seedSuperadminRole: %v", err)
	}

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
		t.Error("existing superadmin role should receive wildcard permission")
	}

	var assigned bool
	if err := db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM users u
			JOIN roles r ON r.id = u.role_id
			WHERE u.email = 'super@example.com' AND r.name = 'superadmin'
		)`,
	).Scan(&assigned); err != nil {
		t.Fatalf("check superadmin assignment: %v", err)
	}
	if !assigned {
		t.Error("existing superadmin user should receive the superadmin role")
	}

	var gotRoleID string
	if err := db.QueryRow(`SELECT id FROM roles WHERE name = 'superadmin'`).Scan(&gotRoleID); err != nil {
		t.Fatalf("load superadmin role: %v", err)
	}
	if gotRoleID != roleID {
		t.Errorf("role id changed from %q to %q", roleID, gotRoleID)
	}
}
