package util

import "testing"

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
