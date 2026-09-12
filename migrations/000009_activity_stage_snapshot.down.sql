-- Restore the RESTRICT link and drop the snapshot. Rows whose stage was
-- deleted while the snapshot schema was active have a NULL stage_id and no
-- target to restore, so the down migration fails loudly instead of silently
-- leaving a half-restored schema.

ALTER TABLE lead_activities DROP CONSTRAINT lead_activities_stage_id_fkey;
ALTER TABLE lead_activities ADD CONSTRAINT lead_activities_stage_id_fkey
    FOREIGN KEY (stage_id) REFERENCES lead_stages(id) ON DELETE RESTRICT;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM lead_activities WHERE stage_id IS NULL) THEN
        RAISE EXCEPTION 'cannot restore NOT NULL: lead_activities rows lost their stage';
    END IF;
END $$;

ALTER TABLE lead_activities ALTER COLUMN stage_id SET NOT NULL;
ALTER TABLE lead_activities DROP COLUMN stage_name;
