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
// formatting collapse to one lookup key. A leading Indian country code (+91 or
// 91) is dropped from numbers longer than 10 digits, so "+91 98765 43210" and
// "98765 43210" dedupe to the same key without touching 10-digit numbers that
// happen to start with 91.
func NormalizePhone(phone string) string {
	digits := nonDigit.ReplaceAllString(phone, "")
	if len(digits) > 10 && strings.HasPrefix(digits, "91") {
		digits = digits[2:]
	}
	return digits
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
