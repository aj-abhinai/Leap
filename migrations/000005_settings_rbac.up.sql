-- RBAC hardening: the audit log is strictly an admin surface, so the
-- activity:read permission retires in favor of settings:manage. Roles gain
-- an is_system flag so the seeded roles (superadmin, Sales, Viewer) are
-- editable but never deletable.

-- The remap below needs the settings:manage row present; on a database where
-- the app never booted past the permission seed it does not exist yet.
INSERT INTO permissions (name, description)
VALUES ('settings:manage', 'Manage settings: pipelines, programs, tags, users, roles, permissions')
ON CONFLICT (name) DO NOTHING;

-- Holders of activity:read move to settings:manage (the same admin scope).
-- Roles that already carry settings:manage keep a single row.
UPDATE role_permissions
SET permission_id = settings_manage.id
FROM permissions AS settings_manage
WHERE role_permissions.permission_id = (SELECT id FROM permissions WHERE name = 'activity:read')
  AND settings_manage.name = 'settings:manage'
  AND NOT EXISTS (
      SELECT 1 FROM role_permissions rp2
      WHERE rp2.role_id = role_permissions.role_id
        AND rp2.permission_id = settings_manage.id
  );

-- The seed only inserts missing rows, so this migration owns the retirement.
DELETE FROM permissions WHERE name = 'activity:read';

ALTER TABLE roles ADD COLUMN is_system BOOLEAN NOT NULL DEFAULT false;

-- The system roles predate the flag on every upgraded database; the flag
-- must agree with their permanent status so the delete guard (and the boot
-- seed) treats them uniformly from day one.
UPDATE roles SET is_system = true WHERE name IN ('superadmin', 'Sales', 'Viewer');