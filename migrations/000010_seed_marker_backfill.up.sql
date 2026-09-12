-- Existing installations predate the seed-once marker. Mark any database that
-- already has users as already seeded, so the first boot after the upgrade
-- does not resurrect starter entries the operator deleted or renamed before
-- the upgrade. Fresh databases have no users when migrations run, so they are
-- not marked and still seed on first boot.

INSERT INTO settings (key, value)
SELECT 'seed_completed', 'true'
WHERE EXISTS (SELECT 1 FROM users)
ON CONFLICT (key) DO NOTHING;
