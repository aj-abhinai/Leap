package lead

import (
	"testing"
	"time"
)

// activityReactivationRequested is the closed-lead guard: only a change away
// from the stored working state counts as reactivation. Resubmitting the
// stored flag value (the edit form sends full bodies) must stay a no-op.
func TestActivityReactivationRequested(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	later := now.Add(time.Hour)
	done := true
	notDone := false
	cancelled := true
	notCancelled := false

	tests := []struct {
		name string
		req  UpdateActivityRequest
		cur  Activity
		want bool
	}{
		{"empty request is not reactivation", UpdateActivityRequest{}, Activity{}, false},
		{"completing an open row is not reactivation", UpdateActivityRequest{IsDone: &done}, Activity{}, false},
		{"reopening a done row is reactivation", UpdateActivityRequest{IsDone: &notDone}, Activity{IsDone: true}, true},
		{"resubmitting stored done=false is not reactivation", UpdateActivityRequest{IsDone: &notDone}, Activity{IsDone: false}, false},
		{"cancelling an open row is not reactivation", UpdateActivityRequest{IsCancelled: &cancelled}, Activity{}, false},
		{"un-cancelling a cancelled row is reactivation", UpdateActivityRequest{IsCancelled: &notCancelled}, Activity{IsCancelled: true}, true},
		{"resubmitting stored cancelled=false is not reactivation", UpdateActivityRequest{IsCancelled: &notCancelled}, Activity{IsCancelled: false}, false},
		{"new schedule is reactivation", UpdateActivityRequest{ScheduledAt: optTime(&later)}, Activity{}, true},
		{"new end is reactivation", UpdateActivityRequest{ScheduledEndAt: optTime(&later)}, Activity{}, true},
		{"new reminder is reactivation", UpdateActivityRequest{RemindAt: optTime(&later)}, Activity{}, true},
		{"resubmitted schedule is not reactivation", UpdateActivityRequest{ScheduledAt: optTime(&now)}, Activity{ScheduledAt: &now}, false},
		{"resubmitted reminder is not reactivation", UpdateActivityRequest{RemindAt: optTime(&now)}, Activity{RemindAt: &now}, false},
		{"cleared schedule is not reactivation", UpdateActivityRequest{ScheduledAt: optTime(nil)}, Activity{ScheduledAt: &now}, false},
		{"cleared reminder is not reactivation", UpdateActivityRequest{RemindAt: optTime(nil)}, Activity{RemindAt: &now}, false},
		{"follow-up is reactivation", UpdateActivityRequest{FollowUp: &FollowUpRequest{ScheduledAt: later}}, Activity{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := activityReactivationRequested(tc.req, tc.cur); got != tc.want {
				t.Errorf("activityReactivationRequested() = %v, want %v", got, tc.want)
			}
		})
	}
}
