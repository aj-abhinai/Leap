-- Stage closure lives in one field: a stage closes a lead when its outcome is
-- 'won' or 'lost'. The duplicate is_closing flag is retired so the two can
-- never disagree. Dropping the column also drops the one-directional
-- lead_stages_closing_outcome_check constraint that referenced it.
ALTER TABLE lead_stages DROP COLUMN is_closing;
