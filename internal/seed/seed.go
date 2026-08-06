package seed

import (
	"crm/internal/auth"
	"crm/internal/config"
	"database/sql"
	"fmt"
	"log/slog"
)

func Seed(db *sql.DB, authCfg config.Auth, superadmin config.Superadmin) error {
	if err := seedSuperadmin(db, authCfg, superadmin); err != nil {
		return fmt.Errorf("seed superadmin: %w", err)
	}
	if err := seedPermissions(db); err != nil {
		return fmt.Errorf("seed permissions: %w", err)
	}
	if err := seedSuperadminRole(db, superadmin); err != nil {
		return fmt.Errorf("seed superadmin role: %w", err)
	}
	if err := seedDefaultPipeline(db); err != nil {
		return fmt.Errorf("seed default pipeline: %w", err)
	}
	if err := seedTagsAndStatuses(db); err != nil {
		return fmt.Errorf("seed tags and statuses: %w", err)
	}
	return nil
}

func seedSuperadmin(db *sql.DB, cfg config.Auth, superadmin config.Superadmin) error {
	email := superadmin.Email
	password := superadmin.Password

	var exists bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists); err != nil {
		return fmt.Errorf("check existing superadmin: %w", err)
	}
	if exists {
		slog.Info("superadmin already exists, skipping")
		return nil
	}
	hash, err := auth.HashPassword(password, cfg.BcryptCost)
	if err != nil {
		return fmt.Errorf("hash superadmin password: %w", err)
	}
	_, err = db.Exec(
		`INSERT INTO users (name, email, password_hash, must_change_password) VALUES ('Super Admin', $1, $2, true)`,
		email, hash,
	)
	if err != nil {
		return fmt.Errorf("insert superadmin: %w", err)
	}
	slog.Info("superadmin user created")
	return nil
}

func seedPermissions(db *sql.DB) error {
	perms := []struct {
		Name string
		Desc string
	}{
		{Name: "contact:read", Desc: "View contacts"},
		{Name: "contact:write", Desc: "Create and update contacts"},
		{Name: "contact:delete", Desc: "Delete contacts"},
		{Name: "lead:read", Desc: "View leads"},
		{Name: "lead:write", Desc: "Create and update leads"},
		{Name: "lead:delete", Desc: "Delete leads"},
		{Name: "lead:move_stage", Desc: "Move leads between pipeline stages"},
		{Name: "pipeline:manage", Desc: "Create/edit pipelines and stages"},
		{Name: "program:manage", Desc: "Create/edit programs and catalog prices"},
		{Name: "rbac:manage", Desc: "Manage users, roles, permissions"},
		{Name: "activity:read", Desc: "View audit log"},
	}
	for _, p := range perms {
		_, err := db.Exec(
			`INSERT INTO permissions (name, description) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING`,
			p.Name, p.Desc,
		)
		if err != nil {
			return fmt.Errorf("seed permission %s: %w", p.Name, err)
		}
	}
	slog.Info("default permissions seeded")
	return nil
}

func seedSuperadminRole(db *sql.DB, superadmin config.Superadmin) error {
	email := superadmin.Email

	var exists bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM roles WHERE name = 'superadmin')`).Scan(&exists); err != nil {
		return fmt.Errorf("check existing superadmin role: %w", err)
	}
	if !exists {
		if _, err := db.Exec(`INSERT INTO roles (name, description) VALUES ('superadmin', 'Full system access')`); err != nil {
			return fmt.Errorf("insert superadmin role: %w", err)
		}
	}
	_, err := db.Exec(
		`INSERT INTO permissions (name, description) VALUES ('*', 'Wildcard - all permissions')
		ON CONFLICT (name) DO NOTHING`,
	)
	if err != nil {
		return fmt.Errorf("seed wildcard permission: %w", err)
	}
	_, err = db.Exec(
		`INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r, permissions p
		WHERE r.name = 'superadmin' AND p.name = '*'
		ON CONFLICT DO NOTHING`,
	)
	if err != nil {
		return fmt.Errorf("link wildcard permission: %w", err)
	}
	_, err = db.Exec(
		`UPDATE users SET role_id = r.id, updated_at = now()
		FROM roles r WHERE r.name = 'superadmin' AND users.email = $1`,
		email,
	)
	if err != nil {
		return fmt.Errorf("assign superadmin role: %w", err)
	}
	slog.Info("superadmin role seeded")
	return nil
}

func seedDefaultPipeline(db *sql.DB) error {
	var exists bool
	if err := db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM pipelines WHERE name = 'Default Pipeline')`,
	).Scan(&exists); err != nil {
		return fmt.Errorf("check default pipeline: %w", err)
	}
	if exists {
		return nil
	}
	var pipelineID string
	err := db.QueryRow(
		`INSERT INTO pipelines (name, description) VALUES ('Default Pipeline', 'Default sales pipeline') RETURNING id`,
	).Scan(&pipelineID)
	if err != nil {
		return fmt.Errorf("insert default pipeline: %w", err)
	}
	stages := []struct {
		name      string
		isClosing bool
	}{
		{"New Customer", false},
		{"Contacted", false},
		{"Follow-up", false},
		{"Closed Lost", true},
		{"Converted", true},
	}
	for i, st := range stages {
		_, err := db.Exec(
			`INSERT INTO lead_stages (pipeline_id, name, "order", is_closing) VALUES ($1, $2, $3, $4)`,
			pipelineID, st.name, i, st.isClosing,
		)
		if err != nil {
			return fmt.Errorf("seed stage %s: %w", st.name, err)
		}
	}
	slog.Info("default pipeline seeded with 5 stages")
	return nil
}

func seedTagsAndStatuses(db *sql.DB) error {
	tags := []string{"Hot Lead", "VIP", "Student", "Influencer"}
	for _, name := range tags {
		_, err := db.Exec(`INSERT INTO tags (name, type) VALUES ($1, 'tag') ON CONFLICT (name, type) DO NOTHING`, name)
		if err != nil {
			return fmt.Errorf("seed tag %s: %w", name, err)
		}
	}

	statuses := []string{"New", "Active", "Cold", "Archived"}
	for _, name := range statuses {
		_, err := db.Exec(`INSERT INTO tags (name, type) VALUES ($1, 'status') ON CONFLICT (name, type) DO NOTHING`, name)
		if err != nil {
			return fmt.Errorf("seed status %s: %w", name, err)
		}
	}

	// Activity outcomes (the "Stage Status" of a logged activity).
	activityStatuses := []string{
		"Share Details WA", "No Reply", "Not intrested", "Pending Response",
		"Reminder Call", "Reminder Msg", "Rescheduled", "Fake",
	}
	for _, name := range activityStatuses {
		_, err := db.Exec(`INSERT INTO tags (name, type) VALUES ($1, 'status') ON CONFLICT (name, type) DO NOTHING`, name)
		if err != nil {
			return fmt.Errorf("seed activity status %s: %w", name, err)
		}
	}

	// Activity types (the "Stage Activity" labels; presets, not enforced).
	activityTypes := []string{"Call 1", "Call 2", "WA chat", "WA Auto"}
	for _, name := range activityTypes {
		_, err := db.Exec(`INSERT INTO tags (name, type) VALUES ($1, 'activity_type') ON CONFLICT (name, type) DO NOTHING`, name)
		if err != nil {
			return fmt.Errorf("seed activity type %s: %w", name, err)
		}
	}

	// Loss-reason presets for the "Closed Lost" stage (free text plus presets).
	lossReasons := []string{"Not intrested", "Fake"}
	for _, name := range lossReasons {
		_, err := db.Exec(`INSERT INTO tags (name, type) VALUES ($1, 'loss_reason') ON CONFLICT (name, type) DO NOTHING`, name)
		if err != nil {
			return fmt.Errorf("seed loss reason %s: %w", name, err)
		}
	}

	slog.Info("default tags, statuses, activity types and loss reasons seeded")
	return nil
}
