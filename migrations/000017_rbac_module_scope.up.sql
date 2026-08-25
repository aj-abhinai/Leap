-- Remap the drifted field-level permissions onto the module-scoped set.
-- Folds: lead:move_stage, lead:delete -> lead:write;
--        contact:delete -> contact:write;
--        pipeline:manage, program:manage, rbac:manage -> settings:manage.
--
-- The seed (seedPermissions) runs after migrations, so this migration is
-- self-contained: it inserts the seven module-scoped permissions first, then
-- remaps existing role_permissions rows onto them and deletes the six
-- superseded permission rows.

INSERT INTO permissions (name, description) VALUES
    ('contact:read',   'View contacts'),
    ('contact:write',  'Create, update and delete contacts'),
    ('lead:read',      'View leads and pipelines'),
    ('lead:write',     'Create, update, move and delete leads'),
    ('settings:manage','Manage settings: pipelines, programs, tags, users, roles, permissions'),
    ('activity:read',  'View audit log'),
    ('data:export',    'Export contacts and leads to CSV')
ON CONFLICT (name) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT rp.role_id, target.id
FROM role_permissions rp
JOIN permissions old ON old.id = rp.permission_id
JOIN permissions target ON target.name = CASE old.name
    WHEN 'lead:move_stage' THEN 'lead:write'
    WHEN 'lead:delete'     THEN 'lead:write'
    WHEN 'contact:delete'  THEN 'contact:write'
    WHEN 'pipeline:manage' THEN 'settings:manage'
    WHEN 'program:manage'  THEN 'settings:manage'
    WHEN 'rbac:manage'     THEN 'settings:manage'
END
ON CONFLICT (role_id, permission_id) DO NOTHING;

DELETE FROM role_permissions rp
USING permissions p
WHERE p.id = rp.permission_id
  AND p.name IN ('lead:move_stage', 'lead:delete', 'contact:delete',
                 'pipeline:manage', 'program:manage', 'rbac:manage');

DELETE FROM permissions
WHERE name IN ('lead:move_stage', 'lead:delete', 'contact:delete',
               'pipeline:manage', 'program:manage', 'rbac:manage');
