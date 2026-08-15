-- Outcome groups and behaviors on tags: status tags drive a configurable
-- outcome flow. group orders chips in the form, behavior decides the
-- follow-up when an outcome is picked (log only / schedule next / close lost).

ALTER TABLE tags ADD COLUMN group_name TEXT NOT NULL DEFAULT '';
ALTER TABLE tags ADD COLUMN sort_order INT NOT NULL DEFAULT 0;
ALTER TABLE tags ADD COLUMN behavior TEXT NOT NULL DEFAULT 'log';
ALTER TABLE tags ADD CONSTRAINT tags_behavior_check CHECK (behavior IN ('log', 'next', 'close_lost'));
