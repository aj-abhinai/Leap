-- Best-effort reverse: 4 stages -> 6 stages (Lead -> New, add Contacted and Lost).
DO $$
DECLARE
  pid UUID;
  lead_id UUID; qual_id UUID; prop_id UUID; won_id UUID;
BEGIN
  SELECT id INTO pid FROM pipelines WHERE name = 'Default Pipeline';
  IF pid IS NULL THEN RETURN; END IF;

  SELECT id INTO lead_id FROM lead_stages WHERE pipeline_id = pid AND name = 'Lead';
  SELECT id INTO qual_id FROM lead_stages WHERE pipeline_id = pid AND name = 'Qualified';
  SELECT id INTO prop_id FROM lead_stages WHERE pipeline_id = pid AND name = 'Proposal';
  SELECT id INTO won_id  FROM lead_stages WHERE pipeline_id = pid AND name = 'Won';

  -- Re-stage leads sitting on the new terminal stage onto Won before inserting extras.
  IF won_id IS NOT NULL THEN
    UPDATE leads SET stage_id = won_id WHERE pipeline_id = pid AND stage_id = lead_id;
  END IF;

  -- Insert Contacted (order 1) and Lost (order 5) if missing.
  IF lead_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM lead_stages WHERE pipeline_id = pid AND name = 'Contacted') THEN
    INSERT INTO lead_stages (pipeline_id, name, "order") VALUES (pid, 'Contacted', 1);
  END IF;
  IF won_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM lead_stages WHERE pipeline_id = pid AND name = 'Lost') THEN
    INSERT INTO lead_stages (pipeline_id, name, "order") VALUES (pid, 'Lost', 5);
  END IF;

  -- Rename Lead -> New and renumber to the 6-stage order.
  UPDATE lead_stages SET name = 'New' WHERE pipeline_id = pid AND name = 'Lead';
  UPDATE lead_stages SET "order" = CASE name
    WHEN 'New' THEN 0
    WHEN 'Contacted' THEN 1
    WHEN 'Qualified' THEN 2
    WHEN 'Proposal' THEN 3
    WHEN 'Won' THEN 4
    WHEN 'Lost' THEN 5
  END WHERE pipeline_id = pid;
END $$;
