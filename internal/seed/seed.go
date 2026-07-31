package seed

import (
	"crm/internal/auth"
	"crm/internal/config"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
)

func Seed(db *sql.DB, cfg config.Auth) error {
	if err := seedSuperadmin(db, cfg); err != nil {
		return fmt.Errorf("seed superadmin: %w", err)
	}
	if err := seedPermissions(db); err != nil {
		return fmt.Errorf("seed permissions: %w", err)
	}
	if err := seedSuperadminRole(db); err != nil {
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

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func seedSuperadmin(db *sql.DB, cfg config.Auth) error {
	email := getEnv("SUPERADMIN_EMAIL", "admin@crm.local")
	password := getEnv("SUPERADMIN_PASSWORD", "admin")

	var exists bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists); err != nil {
		return err
	}
	if exists {
		slog.Info("superadmin already exists, skipping")
		return nil
	}
	hash, err := auth.HashPassword(password, cfg.BcryptCost)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO users (name, email, password_hash) VALUES ('Super Admin', $1, $2)`, email, hash)
	if err != nil {
		return err
	}
	slog.Info("superadmin user created", "email", email)
	return nil
}

func seedPermissions(db *sql.DB) error {
	perms := []struct {
		name, desc string
	}{
		{"contact:read", "View contacts"},
		{"contact:write", "Create and update contacts"},
		{"contact:delete", "Delete contacts"},
		{"lead:read", "View leads"},
		{"lead:write", "Create and update leads"},
		{"lead:delete", "Delete leads"},
		{"lead:move_stage", "Move leads between pipeline stages"},
		{"pipeline:manage", "Create/edit pipelines and stages"},
		{"rbac:manage", "Manage users, roles, permissions"},
		{"activity:read", "View audit log"},
	}
	for _, p := range perms {
		_, err := db.Exec(`INSERT INTO permissions (name, description) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING`, p.name, p.desc)
		if err != nil {
			return fmt.Errorf("seed permission %s: %w", p.name, err)
		}
	}
	slog.Info("default permissions seeded")
	return nil
}

func seedSuperadminRole(db *sql.DB) error {
	email := getEnv("SUPERADMIN_EMAIL", "admin@crm.local")

	var exists bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM roles WHERE name = 'superadmin')`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err := db.Exec(`INSERT INTO roles (name, description) VALUES ('superadmin', 'Full system access')`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO permissions (name, description) VALUES ('*', 'Wildcard - all permissions') ON CONFLICT (name) DO NOTHING`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO role_permissions (role_id, permission_id) SELECT r.id, p.id FROM roles r, permissions p WHERE r.name = 'superadmin' AND p.name = '*'`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO user_roles (user_id, role_id) SELECT u.id, r.id FROM users u, roles r WHERE u.email = $1 AND r.name = 'superadmin' ON CONFLICT DO NOTHING`, email)
	if err != nil {
		return err
	}
	slog.Info("superadmin role seeded")
	return nil
}

func seedDefaultPipeline(db *sql.DB) error {
	var exists bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pipelines WHERE name = 'Default Pipeline')`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	var pipelineID string
	err := db.QueryRow(`INSERT INTO pipelines (name, description) VALUES ('Default Pipeline', 'Default sales pipeline') RETURNING id`).Scan(&pipelineID)
	if err != nil {
		return err
	}
	stages := []string{"New", "Contacted", "Qualified", "Proposal", "Won", "Lost"}
	for i, name := range stages {
		_, err := db.Exec(`INSERT INTO lead_stages (pipeline_id, name, "order") VALUES ($1, $2, $3)`, pipelineID, name, i)
		if err != nil {
			return fmt.Errorf("seed stage %s: %w", name, err)
		}
	}
	slog.Info("default pipeline seeded with 6 stages")
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

	slog.Info("default tags and statuses seeded")
	return nil
}
