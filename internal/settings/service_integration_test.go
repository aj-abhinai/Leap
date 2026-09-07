package settings

import (
	"crm/internal/testdb"
	"testing"
)

func TestDefaultCountryCodeDefaultsWhenAbsent(t *testing.T) {
	db := testdb.New(t)

	cc, err := DefaultCountryCode(db)
	if err != nil {
		t.Fatalf("DefaultCountryCode: %v", err)
	}
	if cc != DefaultDefaultCountryCode {
		t.Errorf("code = %q, want %q", cc, DefaultDefaultCountryCode)
	}
}

func TestDefaultCountryCodeReadsStoredValue(t *testing.T) {
	db := testdb.New(t)
	if _, err := db.Exec(
		`INSERT INTO settings (key, value) VALUES ($1, '+971')`,
		DefaultCountryCodeKey,
	); err != nil {
		t.Fatalf("insert setting: %v", err)
	}

	cc, err := DefaultCountryCode(db)
	if err != nil {
		t.Fatalf("DefaultCountryCode: %v", err)
	}
	if cc != "+971" {
		t.Errorf("code = %q, want +971", cc)
	}
}

// Malformed stored values fall back to the default so a bad row can never
// stamp a broken country code onto stored numbers.
func TestDefaultCountryCodeFallsBackOnMalformed(t *testing.T) {
	for _, raw := range []string{"91", "+", "+abc", "++91", "+91 ", ""} {
		db := testdb.New(t)
		if _, err := db.Exec(
			`INSERT INTO settings (key, value) VALUES ($1, $2)`,
			DefaultCountryCodeKey, raw,
		); err != nil {
			t.Fatalf("insert setting %q: %v", raw, err)
		}
		cc, err := DefaultCountryCode(db)
		if err != nil {
			t.Fatalf("DefaultCountryCode(%q): %v", raw, err)
		}
		if cc != DefaultDefaultCountryCode {
			t.Errorf("code for %q = %q, want default %q", raw, cc, DefaultDefaultCountryCode)
		}
	}
}