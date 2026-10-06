package auth

import (
	"database/sql"
	"fmt"
)

// Execer is the write surface session revocation needs; *sql.DB and *sql.Tx
// both satisfy it.
type Execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// RevokeUserSessions revokes every unrevoked refresh token of userID. It runs
// on q so a caller can revoke inside the transaction that also writes the
// change that prompted the revocation.
func RevokeUserSessions(q Execer, userID string) error {
	if _, err := q.Exec(
		`UPDATE refresh_tokens SET revoked = true WHERE user_id = $1 AND NOT revoked`,
		userID,
	); err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}
	return nil
}
