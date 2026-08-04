-- first-login forced password change (self-hosted internal tool, no email):
-- users created by a superadmin and the seeded superadmin must set their own
-- password on first login. Default false keeps existing rows unaffected.
ALTER TABLE users ADD COLUMN must_change_password BOOLEAN NOT NULL DEFAULT false;
