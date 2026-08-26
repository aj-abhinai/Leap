-- Contacts are the identity source of truth. Phones and emails live only in
-- the child tables (exactly one primary per type, enforced by the app);
-- there are deliberately no scalar phone/email columns on contacts.

CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'tag',
    color TEXT,
    -- group_name/sort_order/behavior configure quick replies (type='quick_reply'):
    -- group orders chips in the activity form, behavior decides the follow-up
    -- when the reply is picked (log only / schedule next / close lost).
    group_name TEXT NOT NULL DEFAULT '',
    sort_order INT NOT NULL DEFAULT 0,
    behavior TEXT NOT NULL DEFAULT 'log',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT tags_behavior_check CHECK (behavior IN ('log', 'next', 'close_lost'))
);

CREATE UNIQUE INDEX idx_tags_name_type ON tags(name, type);

CREATE TABLE contacts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    nickname TEXT,
    location TEXT,
    age INTEGER,
    status_id UUID REFERENCES tags(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE contact_tags (
    contact_id UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (contact_id, tag_id)
);

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
-- Expression indexes matching how lookups normalize values (digits-only phone,
-- lower/trim email) so resolve-or-create and duplicate checks use an index
-- instead of scanning every row.
CREATE INDEX idx_contact_phones_normalized ON contact_phones (regexp_replace(value, '\D', '', 'g'));
CREATE INDEX idx_contact_emails_normalized ON contact_emails (lower(trim(value)));

CREATE TABLE contact_notes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contact_id UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    note TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_contact_notes_contact ON contact_notes(contact_id);
