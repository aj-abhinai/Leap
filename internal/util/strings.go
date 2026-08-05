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
// formatting collapse to one lookup key.
func NormalizePhone(phone string) string {
	return nonDigit.ReplaceAllString(phone, "")
}

// NormalizeEmail trims and lowercases so case/whitespace differences collapse
// to one lookup key.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
