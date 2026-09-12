package util

import (
	"strings"
	"testing"
)

// TestPhoneMatchCond pins the generated predicate: callers embed it inside
// larger AND/OR expressions, so the outer parentheses and both match arms must
// survive.
func TestPhoneMatchCond(t *testing.T) {
	got := PhoneMatchCond("cp.value", "$1", "$2")
	if !strings.HasPrefix(got, "(") || !strings.HasSuffix(got, ")") {
		t.Errorf("PhoneMatchCond = %q, want an outer-parenthesized condition", got)
	}
	for _, want := range []string{
		`regexp_replace(cp.value, '\D', '', 'g')`,
		`IN ($1, $2)`,
		`ltrim(regexp_replace(cp.value, '\D', '', 'g'), '0') = $1`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("PhoneMatchCond = %q, want it to contain %q", got, want)
		}
	}
}

func TestCanonicalPhone(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		defaultCC string
		want      string
	}{
		{name: "bare national number", input: "98765 43210", defaultCC: "+91", want: "+919876543210"},
		{name: "leading zero stripped", input: "09876543210", defaultCC: "+91", want: "+919876543210"},
		{name: "plus keeps its code", input: "+91 98765 43210", defaultCC: "+91", want: "+919876543210"},
		{name: "plus keeps foreign code", input: "+971501234567", defaultCC: "+91", want: "+971501234567"},
		{name: "double zero then country code", input: "0091 98765 43210", defaultCC: "+91", want: "+919876543210"},
		{name: "already carrying the default code", input: "919876543210", defaultCC: "+91", want: "+919876543210"},
		{name: "foreign code without plus", input: "971501234567", defaultCC: "+971", want: "+971501234567"},
		{name: "spaces and dashes", input: "987 65-43210", defaultCC: "+91", want: "+919876543210"},
		{name: "empty", input: "", defaultCC: "+91", want: ""},
		{name: "no digits at all", input: "call me", defaultCC: "+91", want: ""},
		{name: "already canonical", input: "+919876543210", defaultCC: "+91", want: "+919876543210"},
		{name: "plus with no digits", input: "+", defaultCC: "+91", want: ""},
		{name: "plus with spaces only", input: "+  ", defaultCC: "+91", want: ""},
		// A '+' marks the number as explicitly international, so its code is
		// kept as typed after the leading zeros go.
		{name: "plus then zeros", input: "+0 98765 43210", defaultCC: "+91", want: "+9876543210"},
		{name: "plus double zero then code", input: "+0091 98765 43210", defaultCC: "+91", want: "+919876543210"},
		{name: "all zeros", input: "000", defaultCC: "+91", want: ""},
		{name: "short number starting with code", input: "91123", defaultCC: "+91", want: "+91123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanonicalPhone(tt.input, tt.defaultCC); got != tt.want {
				t.Errorf("CanonicalPhone(%q, %q) = %q, want %q", tt.input, tt.defaultCC, got, tt.want)
			}
		})
	}
}

func TestPhoneLookupKeys(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		defaultCC     string
		wantNational  string
		wantCoded     string
	}{
		{name: "bare national", input: "9876543210", defaultCC: "+91", wantNational: "9876543210", wantCoded: "919876543210"},
		{name: "leading zero", input: "09876543210", defaultCC: "+91", wantNational: "9876543210", wantCoded: "919876543210"},
		{name: "plus code", input: "+91 98765 43210", defaultCC: "+91", wantNational: "9876543210", wantCoded: "919876543210"},
		{name: "double zero then code", input: "00919876543210", defaultCC: "+91", wantNational: "9876543210", wantCoded: "919876543210"},
		{name: "foreign code", input: "+971501234567", defaultCC: "+91", wantNational: "971501234567", wantCoded: "91971501234567"},
		{name: "no digits", input: "abc", defaultCC: "+91", wantNational: "", wantCoded: "91"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			national, coded := PhoneLookupKeys(tt.input, tt.defaultCC)
			if national != tt.wantNational || coded != tt.wantCoded {
				t.Errorf("PhoneLookupKeys(%q, %q) = (%q, %q), want (%q, %q)",
					tt.input, tt.defaultCC, national, coded, tt.wantNational, tt.wantCoded)
			}
		})
	}
}

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "digits only", input: "9876543210", want: "9876543210"},
		{name: "formatted", input: "+91 (98765) 43210", want: "919876543210"},
		{name: "canonical", input: "+919876543210", want: "919876543210"},
		{name: "empty", input: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizePhone(tt.input); got != tt.want {
				t.Errorf("NormalizePhone(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsEmail(t *testing.T) {
	valid := []string{"a@example.com", "a.b+tag@sub.example.co", " A@Example.com "}
	for _, s := range valid {
		if !IsEmail(s) {
			t.Errorf("IsEmail(%q) = false, want true", s)
		}
	}
	invalid := []string{"", "not-an-email", "Name <a@b.com>", "two@add@resses", "a b@c.com"}
	for _, s := range invalid {
		if IsEmail(s) {
			t.Errorf("IsEmail(%q) = true, want false", s)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "lowercases", input: "Alice@Example.COM", want: "alice@example.com"},
		{name: "trims whitespace", input: "  alice@example.com  ", want: "alice@example.com"},
		{name: "lowercases and trims", input: "\tBob@Example.com\n", want: "bob@example.com"},
		{name: "already normalized", input: "alice@example.com", want: "alice@example.com"},
		{name: "empty", input: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeEmail(tt.input); got != tt.want {
				t.Errorf("NormalizeEmail(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
