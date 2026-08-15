-- Activity lifecycle: activities are tasks with a real "happened at" time,
-- cancellable state, and an index for next-task lookups on lead cards.

ALTER TABLE lead_activities ADD COLUMN occurred_at TIMESTAMPTZ;
ALTER TABLE lead_activities ADD COLUMN is_cancelled BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX idx_lead_activities_next_task
    ON lead_activities(lead_id, is_done, scheduled_at)
    WHERE scheduled_at IS NOT NULL AND NOT is_done AND NOT is_cancelled;
