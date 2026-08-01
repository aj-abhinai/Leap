package rbac

import (
	"crm/internal/testdb"
	"database/sql"
	"fmt"
	"testing"
)

func TestDeleteUserRevokesSessionsTransactionally(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var userID string
	err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash) VALUES ('Test User', 'alice@example.com', 'hash') RETURNING id`,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := db.Exec(
			`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '7 days')`,
			userID, fmt.Sprintf("token-hash-%d", i),
		); err != nil {
			t.Fatalf("seed refresh token: %v", err)
		}
	}

	if err := svc.deleteUser(userID); err != nil {
		t.Fatalf("deleteUser: %v", err)
	}

	var active int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1 AND NOT revoked`,
		userID,
	).Scan(&active); err != nil {
		t.Fatalf("count active refresh tokens: %v", err)
	}
	if active != 0 {
		t.Errorf("active refresh tokens = %d, want 0 after deactivation", active)
	}

	var deleted sql.NullTime
	if err := db.QueryRow(`SELECT deleted_at FROM users WHERE id = $1`, userID).Scan(&deleted); err != nil {
		t.Fatalf("load deleted_at: %v", err)
	}
	if !deleted.Valid {
		t.Error("user should be soft-deleted after deactivation")
	}
}
