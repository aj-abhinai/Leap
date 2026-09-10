package pipeline

import "testing"

func TestStageOutcome(t *testing.T) {
	tests := []struct {
		name    string
		outcome string
		want    string
		wantErr bool
	}{
		{"unspecified defaults open", "", OutcomeOpen, false},
		{"open kept", "open", OutcomeOpen, false},
		{"won kept", "won", OutcomeWon, false},
		{"lost kept", "lost", OutcomeLost, false},
		{"garbage rejected", "whatever", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := stageOutcome(tc.outcome)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("stageOutcome(%q) = %q, want error", tc.outcome, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("stageOutcome(%q) unexpected error: %v", tc.outcome, err)
			}
			if got != tc.want {
				t.Errorf("stageOutcome(%q) = %q, want %q", tc.outcome, got, tc.want)
			}
		})
	}
}
