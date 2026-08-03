package respond

import (
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// IsNotFound reports errors that mean the requested resource cannot exist:
// a missing row or an unparseable identifier (invalid UUID or text).
func IsNotFound(err error) bool {
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}

// IsDuplicate reports unique-violation errors so handlers can answer 409.
func IsDuplicate(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// IsForeignKeyViolation reports FK errors so handlers can answer 404/409.
func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
