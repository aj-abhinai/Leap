// Package settings serves org-wide key-value settings. The nudge lead time
// ("how many minutes before the start time a reminder fires") is the first
// consumer; the table is generic so future org settings land the same way.
package settings

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// DefaultNudgeLeadMinutes is the nudge lead time when no setting is stored
// (5 minutes before the start time).
const DefaultNudgeLeadMinutes = 5

// NudgeLeadMinutesKey is the settings row key for the nudge lead time.
const NudgeLeadMinutesKey = "nudge_lead_minutes"

// DefaultCountryCodeKey is the settings row key for the org's default
// country code.
const DefaultCountryCodeKey = "default_country_code"

// DefaultDefaultCountryCode is the country code assumed when no setting is
// stored.
const DefaultDefaultCountryCode = "+91"

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

// validCountryCode reports whether a value is a '+' followed by one to three
// digits (ITU calling codes). The same rule guards writes and stored values on
// read, so a malformed code can neither be stored nor stamped onto a number;
// a malformed persisted value falls back to the default.
func validCountryCode(code string) bool {
	if len(code) < 2 || len(code) > 4 || code[0] != '+' {
		return false
	}
	for _, r := range code[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// DefaultCountryCode returns the org's default country code — a '+' followed
// by digits — defaulting to DefaultDefaultCountryCode when the setting is
// absent or malformed. It runs on any Queryer so every phone entry point
// canonicalizes with the same code; a database failure is reported, not
// silently defaulted, so a wrong country code is never stamped on stored
// numbers.
func DefaultCountryCode(q Queryer) (string, error) {
	var raw string
	err := q.QueryRow(`SELECT value FROM settings WHERE key = $1`, DefaultCountryCodeKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultDefaultCountryCode, nil
	}
	if err != nil {
		return "", fmt.Errorf("get default country code: %w", err)
	}
	if !validCountryCode(raw) {
		return DefaultDefaultCountryCode, nil
	}
	return raw, nil
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

// GetDefaultCountryCode returns the org's default country code, defaulting to
// DefaultDefaultCountryCode when unset or malformed.
func (s *Service) GetDefaultCountryCode() (string, error) {
	return DefaultCountryCode(s.db)
}

// SetDefaultCountryCode stores the org's default country code. The value must
// pass the same validity rule as reads — a '+' followed by one to three
// digits — so a malformed code can never be stamped onto stored numbers.
func (s *Service) SetDefaultCountryCode(code string) error {
	code = strings.TrimSpace(code)
	if !validCountryCode(code) {
		return errors.New("country code must be '+' followed by 1 to 3 digits")
	}
	if _, err := s.db.Exec(
		`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		DefaultCountryCodeKey, code,
	); err != nil {
		return fmt.Errorf("set default country code: %w", err)
	}
	return nil
}
