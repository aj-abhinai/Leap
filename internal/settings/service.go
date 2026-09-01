// Package settings serves org-wide key-value settings. The nudge lead time
// ("how many minutes before the start time a reminder fires") is the first
// consumer; the table is generic so future org settings land the same way.
package settings

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
)

// DefaultNudgeLeadMinutes is the nudge lead time when no setting is stored
// (5 minutes before the start time).
const DefaultNudgeLeadMinutes = 5

// NudgeLeadMinutesKey is the settings row key for the nudge lead time.
const NudgeLeadMinutesKey = "nudge_lead_minutes"

// Service provides database-backed org settings.
type Service struct {
	db *sql.DB
}

// NewService creates a settings Service backed by db.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Queryer is satisfied by *sql.DB and *sql.Tx so settings reads can run
// inside a caller's transaction.
type Queryer interface {
	QueryRow(query string, args ...any) *sql.Row
}

// NudgeLeadMinutes reads the org-wide nudge lead time in minutes, defaulting
// to DefaultNudgeLeadMinutes when the setting is absent or malformed. It runs
// on any Queryer so the lead transaction paths share the settings package's
// parse/default rule instead of re-implementing it.
func NudgeLeadMinutes(q Queryer) (int, error) {
	var raw string
	err := q.QueryRow(`SELECT value FROM settings WHERE key = $1`, NudgeLeadMinutesKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultNudgeLeadMinutes, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get nudge lead minutes: %w", err)
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return DefaultNudgeLeadMinutes, nil
	}
	return n, nil
}

// GetNudgeLeadMinutes returns the org-wide nudge lead time in minutes,
// defaulting to DefaultNudgeLeadMinutes when unset or malformed.
func (s *Service) GetNudgeLeadMinutes() (int, error) {
	return NudgeLeadMinutes(s.db)
}

// SetNudgeLeadMinutes stores the org-wide nudge lead time in minutes. Negative
// values are rejected so a misbehaving client cannot make reminders fire
// after the start time.
func (s *Service) SetNudgeLeadMinutes(minutes int) error {
	if minutes < 0 {
		return errors.New("nudge lead minutes must be non-negative")
	}
	if _, err := s.db.Exec(
		`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		NudgeLeadMinutesKey, strconv.Itoa(minutes),
	); err != nil {
		return fmt.Errorf("set nudge lead minutes: %w", err)
	}
	return nil
}
