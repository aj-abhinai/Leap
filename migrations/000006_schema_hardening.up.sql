-- Schema hardening: vocabulary links become type-guarded and refuse the
-- silent erasure of history, the contact journey gets its index, deactivation
-- gets its revoke-all index, and contacts gain a real date of birth.

-- tags(id, type) must be unique so a composite foreign key can pin a link to
-- exactly one tag type (id alone is already unique; this index exists for the
-- FK below).
CREATE UNIQUE INDEX idx_tags_id_type ON tags(id, type);

-- Contacts: date of birth is the truth for computed age display; age remains
-- as the approximate fallback.
ALTER TABLE contacts ADD COLUMN date_of_birth DATE;

-- A status link may only point at a status tag. The composite FK pins the
-- type on the contacts side with a stored generated column, and the RESTRICT
-- rule keeps history intact: deleting a referenced status fails in the
-- database even if the app guard is bypassed. Links that pointed at a
-- non-status tag under the old schema are cleared (the old FK was
-- ON DELETE SET NULL; clearing is the same outcome for an invalid link).
UPDATE contacts c SET status_id = NULL
WHERE status_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM tags t WHERE t.id = c.status_id AND t.type = 'status');

ALTER TABLE contacts DROP CONSTRAINT contacts_status_id_fkey;
ALTER TABLE contacts ADD COLUMN status_type TEXT GENERATED ALWAYS AS ('status') STORED;
ALTER TABLE contacts ADD CONSTRAINT contacts_status_id_type_fk
    FOREIGN KEY (status_id, status_type) REFERENCES tags(id, type) ON DELETE RESTRICT;

-- The same type guard and RESTRICT rule for quick-reply links on activities.
UPDATE lead_activities a SET quick_reply_id = NULL
WHERE quick_reply_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM tags t WHERE t.id = a.quick_reply_id AND t.type = 'quick_reply');

ALTER TABLE lead_activities DROP CONSTRAINT lead_activities_quick_reply_id_fkey;
ALTER TABLE lead_activities ADD COLUMN quick_reply_type TEXT GENERATED ALWAYS AS ('quick_reply') STORED;
ALTER TABLE lead_activities ADD CONSTRAINT lead_activities_quick_reply_id_type_fk
    FOREIGN KEY (quick_reply_id, quick_reply_type) REFERENCES tags(id, type) ON DELETE RESTRICT;

-- The contact journey read (open leads per contact, resolve matches) filters
-- by contact_id; the resolve-or-create phone lock path reads it per request.
CREATE INDEX idx_leads_contact_id ON leads(contact_id);

-- Deactivation revokes every refresh token of a user; the update is a
-- user_id scan without this index.
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);