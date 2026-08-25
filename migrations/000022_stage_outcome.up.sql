-- Stage outcome metadata: every stage declares what reaching it means for a
-- lead — open (in play), won, or lost. Closing stages must be won or lost; a
-- lead's outcome is read from its stage's metadata instead of matching stage
-- names by text.

ALTER TABLE lead_stages ADD COLUMN outcome TEXT NOT NULL DEFAULT 'open'
    CHECK (outcome IN ('open', 'won', 'lost'));

-- One-time backfill so existing data is correct without operator action:
-- closing stages become 'won' when the name signals a win (won/converted),
-- otherwise 'lost'. Non-closing stages stay 'open'. After this, the metadata
-- is authoritative and the name heuristics are retired.
UPDATE lead_stages SET outcome = CASE
    WHEN is_closing AND (name ILIKE '%won%' OR name ILIKE '%convert%') THEN 'won'
    WHEN is_closing THEN 'lost'
    ELSE 'open'
END;