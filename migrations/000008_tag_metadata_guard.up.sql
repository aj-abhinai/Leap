-- group_name and behavior configure quick replies only. Non-quick-reply
-- catalog entries may hold invisible leftovers from before that rule;
-- normalize them to the neutral defaults, then constrain the combination so
-- they can never come back.
UPDATE tags SET group_name = '', behavior = 'log'
WHERE type <> 'quick_reply' AND (group_name <> '' OR behavior <> 'log');

ALTER TABLE tags ADD CONSTRAINT tags_quick_reply_metadata_check
    CHECK (type = 'quick_reply' OR (group_name = '' AND behavior = 'log'));
