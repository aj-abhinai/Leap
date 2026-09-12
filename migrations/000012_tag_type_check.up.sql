-- The tag type vocabulary is fixed: the app validates it, and the database
-- now refuses unknown kinds written around the app. Existing values outside
-- the vocabulary are normalized first so the constraint applies on any
-- database.

UPDATE tags SET type = 'tag'
WHERE type NOT IN ('tag', 'status', 'quick_reply', 'activity_type', 'loss_reason');

ALTER TABLE tags ADD CONSTRAINT tags_type_check
    CHECK (type IN ('tag', 'status', 'quick_reply', 'activity_type', 'loss_reason'));
