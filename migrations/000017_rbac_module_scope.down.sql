-- Best-effort reverse of 000017_rbac_module_scope: recreate the six deleted
-- ADR 004 permissions. Remapping role_permissions rows back to the old caps
-- is not guaranteed (the app is pre-alpha; down is for schema rollback, not
-- data round-trip).

INSERT INTO permissions (name, description) VALUES
    ('contact:delete',  'Delete contacts'),
    ('lead:delete',     'Delete leads'),
    ('lead:move_stage', 'Move leads between pipeline stages'),
    ('pipeline:manage', 'Create/edit pipelines and stages'),
    ('program:manage',  'Create/edit programs and catalog prices'),
    ('rbac:manage',     'Manage users, roles, permissions')
ON CONFLICT (name) DO NOTHING;
