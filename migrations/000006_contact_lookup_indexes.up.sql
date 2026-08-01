CREATE INDEX idx_contacts_phone_active ON contacts (phone) WHERE deleted_at IS NULL AND phone IS NOT NULL;
CREATE INDEX idx_contacts_email_active ON contacts (email) WHERE deleted_at IS NULL AND email IS NOT NULL;
