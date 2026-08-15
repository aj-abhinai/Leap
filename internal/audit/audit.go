// Package audit records best-effort entries in the shared audit log. Logging
// must never fail the mutation it describes (ADR 009), so failures are logged
// and swallowed.
package audit

import (
	"database/sql"
	"log/slog"
)

// Log writes a best-effort entry to the audit log. The actor's name is
// resolved from the users table so the trail stays readable after a rename.
// resourceID must be a well-formed UUID or empty, and changes must be valid
// JSON or empty.
func Log(db *sql.DB, resourceID, resourceType, action, changes, userID string) {
	userName := ""
	if userID != "" {
		if err := db.QueryRow(`SELECT name FROM users WHERE id = $1`, userID).Scan(&userName); err != nil {
			slog.Error("resolve audit actor name", "error", err, "user_id", userID)
		}
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
		action+" "+resourceType+" "+resourceID, resourceType, resourceID, action, changes, userIDArg, userNameArg,
	); err != nil {
		slog.Error("write audit log", "error", err, "resource_type", resourceType, "resource_id", resourceID)
	}
}
