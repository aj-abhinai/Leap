package pipeline

import (
	"errors"
	"testing"
)

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

// A malformed path id is client input, not a database error: every mutation
// must reject it as not-found before it reaches a uuid column cast.
func TestMalformedIDsAreNotFound(t *testing.T) {
	svc := NewService(nil)
	const bad = "not-a-uuid"

	if _, err := svc.updatePipeline(bad, UpdatePipelineRequest{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("updatePipeline(malformed) = %v, want ErrNotFound", err)
	}
	if _, err := svc.deletePipeline(bad); !errors.Is(err, ErrNotFound) {
		t.Errorf("deletePipeline(malformed) = %v, want ErrNotFound", err)
	}
	if _, err := svc.createStage(bad, CreateStageRequest{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("createStage(malformed) = %v, want ErrNotFound", err)
	}
	if _, err := svc.updateStage(bad, UpdateStageRequest{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("updateStage(malformed) = %v, want ErrNotFound", err)
	}
	if _, err := svc.deleteStage(bad); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleteStage(malformed) = %v, want ErrNotFound", err)
	}
	if err := svc.reorderStages(bad, []string{"00000000-0000-0000-0000-000000000001"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("reorderStages(malformed) = %v, want ErrNotFound", err)
	}
}
