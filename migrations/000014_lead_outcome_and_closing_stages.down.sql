DROP TABLE IF EXISTS lead_stage_history;

ALTER TABLE leads DROP COLUMN IF EXISTS lost_reason;
ALTER TABLE leads DROP COLUMN IF EXISTS outcome;

ALTER TABLE lead_stages DROP COLUMN IF EXISTS is_closing;

ALTER TABLE lead_activities DROP COLUMN IF EXISTS responded_at;
ALTER TABLE lead_activities DROP COLUMN IF EXISTS outcome_id;
