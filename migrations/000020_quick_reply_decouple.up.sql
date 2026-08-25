-- Decouple activity quick replies from contact statuses.
--
-- Status tags were doing double duty: contact status identifiers (contacts.
-- status_id) AND the "What happened?" quick replies in the activity form.
-- Quick replies now become their own tag type ('quick_reply'), keyed by having
-- an outcome group/behavior configured; plain statuses keep type 'status'.

-- 1. Re-type seeded quick replies (those with group/behavior config) to the
--    new catalog type.
UPDATE tags SET type = 'quick_reply'
WHERE type = 'status' AND (group_name <> '' OR behavior <> 'log');

-- 1b. A contact status must reference a status tag (validated server-side as
--     type='status'). If a contact was previously assigned one of the re-typed
--     quick-reply tags as its status, clear it so validation cannot break and
--     the contact no longer points at an activity-scoped tag.
UPDATE contacts c
SET status_id = NULL
WHERE c.status_id IS NOT NULL
  AND c.status_id IN (SELECT id FROM tags WHERE type = 'quick_reply');

-- 2. Rename the activity quick-reply column to match the concept. The FK target
--    is the same tags table; only the name changes.
ALTER TABLE lead_activities RENAME COLUMN outcome_id TO quick_reply_id;

-- 3. Index for the global activities list (status + due date scans).
CREATE INDEX idx_lead_activities_status
    ON lead_activities (is_done, is_cancelled);