-- Expression indexes so the normalized contact lookups in lead/service.go
-- (regexp_replace(value, '\D', '', 'g') for phone, lower(trim(value)) for
-- email) can use an index instead of scanning every row.
CREATE INDEX idx_contact_phones_normalized ON contact_phones (regexp_replace(value, '\D', '', 'g'));
CREATE INDEX idx_contact_emails_normalized ON contact_emails (lower(trim(value)));
