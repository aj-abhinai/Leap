-- Down: restore the plain SET NULL FKs and drop the hardening additions.
ALTER TABLE contacts DROP CONSTRAINT contacts_status_id_type_fk;
ALTER TABLE contacts DROP COLUMN status_type;
ALTER TABLE contacts ADD CONSTRAINT contacts_status_id_fkey
    FOREIGN KEY (status_id) REFERENCES tags(id) ON DELETE SET NULL;

ALTER TABLE lead_activities DROP CONSTRAINT lead_activities_quick_reply_id_type_fk;
ALTER TABLE lead_activities DROP COLUMN quick_reply_type;
ALTER TABLE lead_activities ADD CONSTRAINT lead_activities_quick_reply_id_fkey
    FOREIGN KEY (quick_reply_id) REFERENCES tags(id) ON DELETE SET NULL;

DROP INDEX idx_tags_id_type;

ALTER TABLE contacts DROP COLUMN date_of_birth;

DROP INDEX idx_leads_contact_id;
DROP INDEX idx_refresh_tokens_user_id;