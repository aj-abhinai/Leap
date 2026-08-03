ALTER TABLE leads ALTER COLUMN contact_id DROP NOT NULL;
ALTER TABLE leads ADD COLUMN name TEXT NOT NULL DEFAULT '';
ALTER TABLE leads ADD COLUMN email TEXT;
ALTER TABLE leads ADD COLUMN phone TEXT;

ALTER TABLE contacts DROP COLUMN nickname;

DROP INDEX IF EXISTS idx_contact_emails_value;
DROP INDEX IF EXISTS idx_contact_phones_value;
DROP INDEX IF EXISTS idx_contact_emails_contact;
DROP INDEX IF EXISTS idx_contact_phones_contact;
DROP TABLE IF EXISTS contact_emails;
DROP TABLE IF EXISTS contact_phones;
