package seed

import (
	"crm/internal/auth"
	"crm/internal/config"
	"database/sql"
	"fmt"
	"log/slog"
)

// querier is satisfied by both *sql.DB and *sql.Tx so the bootstrap can run
// inside a single transaction.
type querier interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

func Seed(db *sql.DB, authCfg config.Auth, superadmin config.Superadmin) error {
	if _, err := seedSuperadmin(db, authCfg, superadmin); err != nil {
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

// seedSuperadmin bootstraps the superadmin user only when the users table is
// empty (first boot). It never reconciles or re-creates users on later boots.
// The user insert and its superadmin role assignment are committed in one
// transaction, so an interrupted first boot cannot leave a role-less admin:
// either both commit or neither does, and the next boot sees an empty table
// and bootstraps cleanly. It reports whether the bootstrap admin was created.
func seedSuperadmin(db *sql.DB, cfg config.Auth, superadmin config.Superadmin) (bool, error) {
	email := superadmin.Email
	password := superadmin.Password

	tx, err := db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin bootstrap transaction: %w", err)
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM users)`).Scan(&exists); err != nil {
		return false, fmt.Errorf("check existing users: %w", err)
	}
	if exists {
		slog.Info("users already exist, skipping bootstrap superadmin")
		return false, nil
	}
	hash, err := auth.HashPassword(password, cfg.BcryptCost)
	if err != nil {
		return false, fmt.Errorf("hash superadmin password: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO users (name, email, password_hash, must_change_password) VALUES ('Super Admin', $1, $2, true)`,
		email, hash,
	); err != nil {
		return false, fmt.Errorf("insert superadmin: %w", err)
	}
	if err := ensureSuperadminRole(tx, email, true); err != nil {
		return false, fmt.Errorf("assign bootstrap role: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit bootstrap transaction: %w", err)
	}
	slog.Info("superadmin user and role created")
	return true, nil
}

func seedPermissions(db *sql.DB) error {
	perms := []struct {
		Name string
		Desc string
	}{
		{Name: "contact:read", Desc: "View contacts"},
		{Name: "contact:write", Desc: "Create, update and delete contacts"},
		{Name: "lead:read", Desc: "View leads and pipelines"},
		{Name: "lead:write", Desc: "Create, update, move and delete leads"},
		{Name: "settings:manage", Desc: "Manage settings: pipelines, programs, tags, users, roles, permissions"},
		{Name: "activity:read", Desc: "View audit log"},
		{Name: "data:export", Desc: "Export contacts and leads to CSV"},
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

// seedSuperadminRole ensures the superadmin role and its wildcard permission
// exist (idempotent every boot). It never assigns the role to a user — that
// happens atomically with the bootstrap admin inside seedSuperadmin.
func seedSuperadminRole(db *sql.DB, superadmin config.Superadmin) error {
	if err := ensureSuperadminRole(db, superadmin.Email, false); err != nil {
		return err
	}
	slog.Info("superadmin role seeded")
	return nil
}

// ensureSuperadminRole seeds the superadmin role and its wildcard permission
// (idempotent) and, when assign is true (the fresh-bootstrap path), assigns
// the role to the given superadmin. On later boots assign is false: the app
// never re-reconciles user roles.
func ensureSuperadminRole(q querier, email string, assign bool) error {
	var exists bool
	if err := q.QueryRow(`SELECT EXISTS(SELECT 1 FROM roles WHERE name = 'superadmin')`).Scan(&exists); err != nil {
		return fmt.Errorf("check existing superadmin role: %w", err)
	}
	if !exists {
		if _, err := q.Exec(`INSERT INTO roles (name, description) VALUES ('superadmin', 'Full system access')`); err != nil {
			return fmt.Errorf("insert superadmin role: %w", err)
		}
	}
	_, err := q.Exec(
		`INSERT INTO permissions (name, description) VALUES ('*', 'Wildcard - all permissions')
		ON CONFLICT (name) DO NOTHING`,
	)
	if err != nil {
		return fmt.Errorf("seed wildcard permission: %w", err)
	}
	_, err = q.Exec(
		`INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r, permissions p
		WHERE r.name = 'superadmin' AND p.name = '*'
		ON CONFLICT DO NOTHING`,
	)
	if err != nil {
		return fmt.Errorf("link wildcard permission: %w", err)
	}
	if assign {
		_, err = q.Exec(
			`UPDATE users SET role_id = r.id, updated_at = now()
			FROM roles r WHERE r.name = 'superadmin' AND users.email = $1`,
			email,
		)
		if err != nil {
			return fmt.Errorf("assign superadmin role: %w", err)
		}
	}
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
