package util

import (
	"regexp"
	"strings"
)

// NullStr maps an empty string to NULL so create payloads store a clear
// empty value rather than an empty string.
func NullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// NullPtr maps a pointer to an empty string to NULL, so clients can send an
// empty string to mean "clear this optional id field".
func NullPtr(p *string) *string {
	if p == nil || *p == "" {
		return nil
	}
	return p
}

var nonDigit = regexp.MustCompile(`\D`)

// NormalizePhone keeps only ASCII digits so identical numbers with different
// formatting collapse to one lookup key. A leading 91-country-code chop is
// deliberately absent: numbers are stored canonical ('+' + digits), so
// digits-only comparison is enough and never ambiguous.
func NormalizePhone(phone string) string {
	return nonDigit.ReplaceAllString(phone, "")
}

// CanonicalPhone returns the single stored form of a phone number: a leading
// '+' keeps its country code, leading zeros are stripped from the national
// part, and a bare number is prefixed with the default country code. The
// output is always '+' followed by digits, and a value that already carries
// the default code is never coded twice. An empty result means the input
// contained no digits at all.
func CanonicalPhone(value, defaultCC string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "+") {
		digits := nonDigit.ReplaceAllString(value, "")
		digits = strings.TrimLeft(digits, "0")
		if digits == "" {
			return ""
		}
		return "+" + digits
	}
	digits := nonDigit.ReplaceAllString(value, "")
	digits = strings.TrimLeft(digits, "0")
	if digits == "" {
		return ""
	}
	cc := nonDigit.ReplaceAllString(defaultCC, "")
	if cc != "" && strings.HasPrefix(digits, cc) {
		return "+" + digits
	}
	return "+" + cc + digits
}

// NormalizePhoneKey returns the national-form lookup key for a phone: the
// canonical form with the default country code stripped. Canonical rows and
// legacy raw rows collapse to the same key, so every lookup and duplicate
// guard matches both storage generations with one comparison.
func NormalizePhoneKey(value, defaultCC string) string {
	digits := nonDigit.ReplaceAllString(CanonicalPhone(value, defaultCC), "")
	cc := nonDigit.ReplaceAllString(defaultCC, "")
	if cc != "" && strings.HasPrefix(digits, cc) {
		return digits[len(cc):]
	}
	return digits
}

// PhoneLookupKeys returns the national-form key for a phone and its
// country-coded form, so SQL can match legacy raw rows and canonical rows
// with one IN comparison. The coded form is the raw digits a canonical row
// stores; the national form is what a legacy row stores.
func PhoneLookupKeys(value, defaultCC string) (national, coded string) {
	national = NormalizePhoneKey(value, defaultCC)
	cc := nonDigit.ReplaceAllString(defaultCC, "")
	return national, cc + national
}

// NormalizeEmail trims and lowercases so case/whitespace differences collapse
// to one lookup key.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// likeEscaper escapes the ILIKE/LIKE metacharacters in a user search term so
// a literal '%' or '_' is matched literally instead of widening the pattern.
// Patterns built with it must use ESCAPE '\' in the SQL.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// LikePattern wraps a user search term in '%...%' with its wildcards escaped,
// for use in ILIKE/LIKE predicates with ESCAPE '\'.
func LikePattern(s string) string {
	return "%" + likeEscaper.Replace(s) + "%"
}
