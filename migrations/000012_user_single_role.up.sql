-- Add a single role per user (one-to-many), replacing the user_roles join table.
ALTER TABLE users ADD COLUMN role_id UUID REFERENCES roles(id) ON DELETE SET NULL;

-- Backfill from user_roles: prefer superadmin, else alphabetically-first role.
UPDATE users u
SET role_id = sub.role_id
FROM (
  SELECT DISTINCT ON (ur.user_id) ur.user_id, ur.role_id
  FROM user_roles ur
  JOIN roles r ON r.id = ur.role_id
  ORDER BY ur.user_id, (r.name = 'superadmin') DESC, r.name ASC
) sub
WHERE sub.user_id = u.id;

-- Make the assignment exclusive and drop the now-redundant join table.
CREATE INDEX IF NOT EXISTS idx_users_role_id ON users(role_id);
DROP TABLE user_roles;
