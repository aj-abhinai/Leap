DROP INDEX IF EXISTS idx_leads_program_id;

ALTER TABLE leads DROP COLUMN IF EXISTS program_id;

DROP TABLE IF EXISTS programs;
