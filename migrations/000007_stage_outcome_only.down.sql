-- Restore the retired is_closing flag from the authoritative outcome: a stage
-- closes a lead exactly when its outcome is not 'open'.
ALTER TABLE lead_stages ADD COLUMN is_closing BOOLEAN NOT NULL DEFAULT false;
UPDATE lead_stages SET is_closing = (outcome <> 'open');
ALTER TABLE lead_stages ADD CONSTRAINT lead_stages_closing_outcome_check
    CHECK (NOT is_closing OR outcome IN ('won', 'lost'));
