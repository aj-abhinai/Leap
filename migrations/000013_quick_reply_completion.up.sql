-- A saved quick reply is the record of what happened, so it completes the
-- task. Rows created before that rule existed can carry a quick reply while
-- still reading as open; make the flag match the record.
UPDATE lead_activities SET is_done = true
WHERE quick_reply_id IS NOT NULL AND NOT is_done AND NOT is_cancelled;

-- The mirror case: event stamps without a quick reply come from legacy
-- un-completes. The row is open, so the stamps are not true; clear them so the
-- open set and the timeline agree.
UPDATE lead_activities SET occurred_at = NULL, responded_at = NULL
WHERE quick_reply_id IS NULL AND NOT is_done AND NOT is_cancelled
  AND (occurred_at IS NOT NULL OR responded_at IS NOT NULL);
