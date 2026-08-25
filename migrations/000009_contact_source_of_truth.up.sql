-- Contacts are the single source of truth for leads:
-- * contacts gain nickname and multi-valued phones/emails (child tables, is_primary)
-- * leads drop their own name/email/phone; contact_id becomes required

CREATE TABLE contact_phones (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contact_id UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    value TEXT NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE contact_emails (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contact_id UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    value TEXT NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_contact_phones_contact ON contact_phones(contact_id);
CREATE INDEX idx_contact_emails_contact ON contact_emails(contact_id);
-- Plain-value indexes; the resolve-or-create lookup normalizes the column
-- with regexp_replace/lower, so these do not serve that query.
CREATE INDEX idx_contact_phones_value ON contact_phones(value);
CREATE INDEX idx_contact_emails_value ON contact_emails(value);

ALTER TABLE contacts ADD COLUMN nickname TEXT;

-- leads: drop identity columns, make contact_id required
ALTER TABLE leads DROP COLUMN name;
ALTER TABLE leads DROP COLUMN email;
ALTER TABLE leads DROP COLUMN phone;
ALTER TABLE leads ALTER COLUMN contact_id SET NOT NULL;
