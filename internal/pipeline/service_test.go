package pipeline

import "testing"

func TestStageOutcome(t *testing.T) {
	tests := []struct {
		name      string
		isClosing bool
		outcome   string
		want      string
		wantErr   bool
	}{
		{"non-closing is always open", false, "won", OutcomeOpen, false},
		{"non-closing ignores supplied value", false, "lost", OutcomeOpen, false},
		{"closing unspecified defaults lost", true, "", OutcomeLost, false},
		{"closing lost kept", true, "lost", OutcomeLost, false},
		{"closing won kept", true, "won", OutcomeWon, false},
		{"closing open rejected", true, "open", "", true},
		{"closing garbage rejected", true, "whatever", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := stageOutcome(tc.isClosing, tc.outcome)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("stageOutcome(%v, %q) = %q, want error", tc.isClosing, tc.outcome, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("stageOutcome(%v, %q) unexpected error: %v", tc.isClosing, tc.outcome, err)
			}
			if got != tc.want {
				t.Errorf("stageOutcome(%v, %q) = %q, want %q", tc.isClosing, tc.outcome, got, tc.want)
			}
		})
	}
}
