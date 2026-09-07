-- Down: drop the is_system column and restore the retired permission row.
-- The permission remap is one-way: original holder rows cannot be restored.
ALTER TABLE roles DROP COLUMN is_system;

INSERT INTO permissions (name, description)
VALUES ('activity:read', 'View audit log')
ON CONFLICT (name) DO NOTHING;