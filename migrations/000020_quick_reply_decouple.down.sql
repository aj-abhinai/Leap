DROP INDEX IF EXISTS idx_lead_activities_status;

ALTER TABLE lead_activities RENAME COLUMN quick_reply_id TO outcome_id;

UPDATE tags SET type = 'status'
WHERE type = 'quick_reply' AND (group_name <> '' OR behavior <> 'log');