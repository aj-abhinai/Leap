-- Reconcile an existing "Default Pipeline" from 6 stages to 4
-- (Lead -> Qualified -> Proposal -> Won). Fresh DBs hit the seed guard and no-op.
DO $$
DECLARE
  pid UUID;
  new_id UUID; qual_id UUID; prop_id UUID; won_id UUID;
  contacted_id UUID; lost_id UUID; old_new_id UUID;
BEGIN
  SELECT id INTO pid FROM pipelines WHERE name = 'Default Pipeline';
  IF pid IS NULL THEN RETURN; END IF;

  SELECT id INTO old_new_id     FROM lead_stages WHERE pipeline_id = pid AND name = 'New';
  SELECT id INTO contacted_id   FROM lead_stages WHERE pipeline_id = pid AND name = 'Contacted';
  SELECT id INTO qual_id        FROM lead_stages WHERE pipeline_id = pid AND name = 'Qualified';
  SELECT id INTO prop_id        FROM lead_stages WHERE pipeline_id = pid AND name = 'Proposal';
  SELECT id INTO won_id         FROM lead_stages WHERE pipeline_id = pid AND name = 'Won';
  SELECT id INTO lost_id        FROM lead_stages WHERE pipeline_id = pid AND name = 'Lost';

  -- Re-stage any leads on the to-be-removed stages onto the renamed "New" stage.
  IF old_new_id IS NOT NULL THEN
    UPDATE leads SET stage_id = old_new_id
      WHERE pipeline_id = pid AND stage_id IN (COALESCE(contacted_id,'00000000-0000-0000-0000-000000000000'::uuid), COALESCE(lost_id,'00000000-0000-0000-0000-000000000000'::uuid));
  END IF;

  -- Drop the extra terminal/intermediate stages (now empty of leads).
  DELETE FROM lead_stages WHERE pipeline_id = pid AND name IN ('Contacted','Lost');

  -- Rename New -> Lead.
  UPDATE lead_stages SET name = 'Lead' WHERE pipeline_id = pid AND name = 'New';

  -- Renumber to the canonical order.
  UPDATE lead_stages SET "order" = CASE name
    WHEN 'Lead' THEN 0
    WHEN 'Qualified' THEN 1
    WHEN 'Proposal' THEN 2
    WHEN 'Won' THEN 3
  END WHERE pipeline_id = pid;
END $$;
