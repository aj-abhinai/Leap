package seed

import (
	"crm/internal/auth"
	"crm/internal/config"
	"crm/internal/util"
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

// Seed bootstraps a fresh database with the superadmin user, the permission
// set, the default pipeline, and the tag/status/quick-reply catalog. It runs
// on every boot and is idempotent: the superadmin is created only on an empty
// users table, and every other seed is upsert-style.
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
	// Normalize the configured email exactly like the login lookup
	// (util.NormalizeEmail), so a mixed-case address in config can still log in.
	email := util.NormalizeEmail(superadmin.Email)
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
		outcome   string
	}{
		{"New Customer", false, "open"},
		{"Contacted", false, "open"},
		{"Follow-up", false, "open"},
		{"Closed Lost", true, "lost"},
		{"Converted", true, "won"},
	}
	for i, st := range stages {
		_, err := db.Exec(
			`INSERT INTO lead_stages (pipeline_id, name, "order", is_closing, outcome) VALUES ($1, $2, $3, $4, $5)`,
			pipelineID, st.name, i, st.isClosing, st.outcome,
		)
		if err != nil {
			return fmt.Errorf("seed stage %s: %w", st.name, err)
		}
	}
	slog.Info("default pipeline seeded with 5 stages")
	return nil
}

func seedTagsAndStatuses(db *sql.DB) error {
	catalog := []struct {
		name  string
		typ   string
		label string
	}{
		// Tags shown on the contact/lead forms.
		{"Hot Lead", "tag", "tag"},
		{"VIP", "tag", "tag"},
		{"Student", "tag", "tag"},
		{"Influencer", "tag", "tag"},
		// Contact statuses (a separate tag type).
		{"New", "status", "status"},
		{"Active", "status", "status"},
		{"Cold", "status", "status"},
		{"Archived", "status", "status"},
		// Activity types (the "Stage Activity" labels; presets, not enforced).
		{"Call", "activity_type", "activity type"},
		{"WhatsApp", "activity_type", "activity type"},
		{"Email", "activity_type", "activity type"},
		{"Meeting", "activity_type", "activity type"},
		// Loss-reason presets for the "Closed Lost" stage (free text plus presets).
		{"Not interested", "loss_reason", "loss reason"},
		{"Fake", "loss_reason", "loss reason"},
	}
	for _, t := range catalog {
		_, err := db.Exec(
			`INSERT INTO tags (name, type) VALUES ($1, $2) ON CONFLICT (name, type) DO NOTHING`,
			t.name, t.typ,
		)
		if err != nil {
			return fmt.Errorf("seed %s %s: %w", t.label, t.name, err)
		}
	}

	// Activity quick replies (the "what happened" chips in the activity form).
	// These are a separate catalog from contact statuses: each carries
	// a group (ordered palette section) and a behavior that drives the follow-up
	// when the quick reply is picked:
	//   log        — record the reply only (e.g. Interested)
	//   next       — prompt for the next time and auto-create the next task
	//                of the same type (e.g. Not Connected, Rescheduled — repeat)
	//   close_lost — record the reply and move the lead to Closed Lost (ends)
	quickReplies := []struct {
		name     string
		group    string
		order    int
		behavior string
	}{
		{"Share Details", "Connected", 0, "log"},
		{"Interested", "Connected", 1, "log"},
		{"Rescheduled", "Connected", 2, "next"},
		{"No Reply", "Not Connected", 0, "next"},
		{"Busy", "Not Connected", 1, "next"},
		{"Not Interested", "Heard Details", 0, "log"},
		{"Closed Lost", "Heard Details", 1, "close_lost"},
	}
	for _, st := range quickReplies {
		// On conflict, reconcile group/sort/behavior so an existing DB picks up
		// the quick-reply config instead of keeping the migration default
		// (empty group, behavior 'log'). Name/type/color are never touched.
		_, err := db.Exec(
			`INSERT INTO tags (name, type, group_name, sort_order, behavior)
			VALUES ($1, 'quick_reply', $2, $3, $4)
			ON CONFLICT (name, type) DO UPDATE SET
				group_name = EXCLUDED.group_name,
				sort_order = EXCLUDED.sort_order,
				behavior = EXCLUDED.behavior`,
			st.name, st.group, st.order, st.behavior,
		)
		if err != nil {
			return fmt.Errorf("seed quick reply %s: %w", st.name, err)
		}
	}

	slog.Info("default tags, statuses, activity types and loss reasons seeded")
	return nil
}
