-- Lead journey modeling: activity outcomes, auto-logged response time,
-- closing stages with win/loss, and stage-move history.

ALTER TABLE lead_activities ADD COLUMN outcome_id UUID REFERENCES tags(id) ON DELETE SET NULL;
ALTER TABLE lead_activities ADD COLUMN responded_at TIMESTAMPTZ;

ALTER TABLE lead_stages ADD COLUMN is_closing BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE leads ADD COLUMN outcome TEXT CHECK (outcome IN ('won','lost'));
ALTER TABLE leads ADD COLUMN lost_reason TEXT;

CREATE TABLE lead_stage_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    lead_id UUID NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    from_stage_id UUID REFERENCES lead_stages(id) ON DELETE SET NULL,
    to_stage_id UUID REFERENCES lead_stages(id) ON DELETE SET NULL,
    from_stage_name TEXT NOT NULL DEFAULT '',
    to_stage_name TEXT NOT NULL DEFAULT '',
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    moved_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_lead_stage_history_lead ON lead_stage_history(lead_id);

-- Reconcile an existing "Default Pipeline" from the 4-stage shape
-- (Lead -> Qualified -> Proposal -> Won) to the journey shape
-- (New Customer -> Contacted -> Follow-up -> Closed Lost -> Converted),
-- where Closed Lost and Converted are closing stages.
--
-- Order matters (mirrors 000013): create the new stages FIRST, re-stage any
-- leads off the old stages onto the closest surviving stage, then drop the
-- old stages. Fresh DBs hit the seed guard and no-op.
DO $$
DECLARE
  pid UUID;
  lead_id UUID; qual_id UUID; prop_id UUID; won_id UUID;
  new_customer_id UUID; contacted_id UUID; followup_id UUID;
  closed_lost_id UUID; converted_id UUID;
BEGIN
  SELECT id INTO pid FROM pipelines WHERE name = 'Default Pipeline';
  IF pid IS NULL THEN RETURN; END IF;

  -- Snapshot the old 4-stage ids.
  SELECT id INTO lead_id FROM lead_stages WHERE pipeline_id = pid AND name = 'Lead';
  SELECT id INTO qual_id FROM lead_stages WHERE pipeline_id = pid AND name = 'Qualified';
  SELECT id INTO prop_id FROM lead_stages WHERE pipeline_id = pid AND name = 'Proposal';
  SELECT id INTO won_id  FROM lead_stages WHERE pipeline_id = pid AND name = 'Won';

  -- Create the journey stages if missing (before re-staging).
  IF NOT EXISTS (SELECT 1 FROM lead_stages WHERE pipeline_id = pid AND name = 'New Customer') THEN
    INSERT INTO lead_stages (pipeline_id, name, "order", is_closing) VALUES (pid, 'New Customer', 0, false);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM lead_stages WHERE pipeline_id = pid AND name = 'Contacted') THEN
    INSERT INTO lead_stages (pipeline_id, name, "order", is_closing) VALUES (pid, 'Contacted', 1, false);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM lead_stages WHERE pipeline_id = pid AND name = 'Follow-up') THEN
    INSERT INTO lead_stages (pipeline_id, name, "order", is_closing) VALUES (pid, 'Follow-up', 2, false);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM lead_stages WHERE pipeline_id = pid AND name = 'Closed Lost') THEN
    INSERT INTO lead_stages (pipeline_id, name, "order", is_closing) VALUES (pid, 'Closed Lost', 3, true);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM lead_stages WHERE pipeline_id = pid AND name = 'Converted') THEN
    INSERT INTO lead_stages (pipeline_id, name, "order", is_closing) VALUES (pid, 'Converted', 4, true);
  END IF;

  -- Re-read the new stage ids.
  SELECT id INTO new_customer_id FROM lead_stages WHERE pipeline_id = pid AND name = 'New Customer';
  SELECT id INTO contacted_id    FROM lead_stages WHERE pipeline_id = pid AND name = 'Contacted';
  SELECT id INTO followup_id     FROM lead_stages WHERE pipeline_id = pid AND name = 'Follow-up';
  SELECT id INTO closed_lost_id  FROM lead_stages WHERE pipeline_id = pid AND name = 'Closed Lost';
  SELECT id INTO converted_id    FROM lead_stages WHERE pipeline_id = pid AND name = 'Converted';

  -- Re-stage leads off the old stages onto the closest surviving stage:
  -- Won -> Converted, Lead -> New Customer, Qualified -> Contacted,
  -- Proposal -> Follow-up.
  IF won_id IS NOT NULL AND converted_id IS NOT NULL THEN
    UPDATE leads SET stage_id = converted_id WHERE pipeline_id = pid AND stage_id = won_id;
  END IF;
  IF lead_id IS NOT NULL AND new_customer_id IS NOT NULL THEN
    UPDATE leads SET stage_id = new_customer_id WHERE pipeline_id = pid AND stage_id = lead_id;
  END IF;
  IF qual_id IS NOT NULL AND contacted_id IS NOT NULL THEN
    UPDATE leads SET stage_id = contacted_id WHERE pipeline_id = pid AND stage_id = qual_id;
  END IF;
  IF prop_id IS NOT NULL AND followup_id IS NOT NULL THEN
    UPDATE leads SET stage_id = followup_id WHERE pipeline_id = pid AND stage_id = prop_id;
  END IF;

  -- Drop the old stages now that they are empty of leads.
  DELETE FROM lead_stages WHERE pipeline_id = pid AND name IN ('Lead','Qualified','Proposal','Won');

  -- Create the journey stages if missing.
  IF NOT EXISTS (SELECT 1 FROM lead_stages WHERE pipeline_id = pid AND name = 'New Customer') THEN
    INSERT INTO lead_stages (pipeline_id, name, "order", is_closing) VALUES (pid, 'New Customer', 0, false);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM lead_stages WHERE pipeline_id = pid AND name = 'Contacted') THEN
    INSERT INTO lead_stages (pipeline_id, name, "order", is_closing) VALUES (pid, 'Contacted', 1, false);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM lead_stages WHERE pipeline_id = pid AND name = 'Follow-up') THEN
    INSERT INTO lead_stages (pipeline_id, name, "order", is_closing) VALUES (pid, 'Follow-up', 2, false);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM lead_stages WHERE pipeline_id = pid AND name = 'Closed Lost') THEN
    INSERT INTO lead_stages (pipeline_id, name, "order", is_closing) VALUES (pid, 'Closed Lost', 3, true);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM lead_stages WHERE pipeline_id = pid AND name = 'Converted') THEN
    INSERT INTO lead_stages (pipeline_id, name, "order", is_closing) VALUES (pid, 'Converted', 4, true);
  END IF;

  -- Renumber to the canonical order and set closing flags.
  UPDATE lead_stages SET "order" = CASE name
    WHEN 'New Customer' THEN 0
    WHEN 'Contacted' THEN 1
    WHEN 'Follow-up' THEN 2
    WHEN 'Closed Lost' THEN 3
    WHEN 'Converted' THEN 4
  END, is_closing = CASE
    WHEN name IN ('Closed Lost','Converted') THEN true
    ELSE false
  END WHERE pipeline_id = pid;
END $$;
