-- Stage deletion must not erase task history or pin configuration forever.
-- Tasks snapshot the stage name at write time (like lead_stage_history) and
-- their FK becomes SET NULL: deleting a stage clears only the link, and reads
-- fall back to the snapshot.

ALTER TABLE lead_activities ADD COLUMN stage_name TEXT NOT NULL DEFAULT '';

-- Backfill existing rows from the live stage so reads keep working after the
-- stage is deleted.
UPDATE lead_activities la
SET stage_name = ls.name
FROM lead_stages ls
WHERE la.stage_id = ls.id;

ALTER TABLE lead_activities ALTER COLUMN stage_id DROP NOT NULL;
ALTER TABLE lead_activities DROP CONSTRAINT lead_activities_stage_id_fkey;
ALTER TABLE lead_activities ADD CONSTRAINT lead_activities_stage_id_fkey
    FOREIGN KEY (stage_id) REFERENCES lead_stages(id) ON DELETE SET NULL;
