-- Expression indexes so the legacy scalar-column fallback checks in
-- contact/service.go (duplicateReasonPoint) and lead/service.go can use an
-- index instead of scanning the contacts table. The child-table lookups
-- (contact_phones/contact_emails) already use the normalized indexes from
-- migration 000011.
CREATE INDEX idx_contacts_phone_normalized_active ON contacts (regexp_replace(COALESCE(phone, ''), '\D', '', 'g')) WHERE deleted_at IS NULL;
CREATE INDEX idx_contacts_email_normalized_active ON contacts (lower(trim(COALESCE(email, '')))) WHERE deleted_at IS NULL;
