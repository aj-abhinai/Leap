DROP INDEX IF EXISTS idx_lead_activities_next_task;

ALTER TABLE lead_activities DROP COLUMN IF EXISTS is_cancelled;
ALTER TABLE lead_activities DROP COLUMN IF EXISTS occurred_at;
