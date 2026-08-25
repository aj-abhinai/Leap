// Package audit records best-effort entries in the shared audit log. Logging
// must never fail the mutation it describes, so failures are logged
// and swallowed.
package audit

import (
	"database/sql"
	"log/slog"
)

// Log writes a best-effort entry to the audit log with a description derived
// from the action, resource type, and resource id. The actor's name is
// resolved from the users table so the trail stays readable after a rename.
// resourceID must be a well-formed UUID or empty, and changes must be valid
// JSON or empty.
func Log(db *sql.DB, resourceID, resourceType, action, changes, userID string) {
	LogCustom(db, action+" "+resourceType+" "+resourceID, resourceType, resourceID, action, changes, userID)
}

// LogCustom writes a best-effort audit entry with an explicit description.
// Logging must never fail the mutation it describes, so failures are logged
// and swallowed; the actor's name is resolved from the users table, and
// resourceID/changes follow the same well-formedness rules as Log.
func LogCustom(db *sql.DB, description, resourceType, resourceID, action, changes, userID string) {
	userName := ""
	if userID != "" {
		if err := db.QueryRow(`SELECT name FROM users WHERE id = $1`, userID).Scan(&userName); err != nil {
			slog.Error("resolve audit actor name", "error", err, "user_id", userID)
		}
	}
	// resource_id is UUID and changes is JSONB; an empty string fails the
	// insert for both, so an empty value must be written as NULL.
	var resourceIDArg, changesArg any
	if resourceID != "" {
		resourceIDArg = resourceID
	}
	if changes != "" {
		changesArg = changes
	}
	var userIDArg, userNameArg any
	if userID != "" {
		userIDArg = userID
	}
	if userName != "" {
		userNameArg = userName
	}
	if _, err := db.Exec(
		`INSERT INTO audit_logs (description, resource_type, resource_id, action, changes, user_id, user_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		description, resourceType, resourceIDArg, action, changesArg, userIDArg, userNameArg,
	); err != nil {
		slog.Error("write audit log", "error", err, "resource_type", resourceType, "resource_id", resourceID)
	}
}
