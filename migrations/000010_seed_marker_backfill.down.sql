-- The marker is runtime state; removing it returns the database to the
-- pre-marker behavior (seed the starter data on every boot), which matches the
-- older code a downgrade would run.

DELETE FROM settings WHERE key = 'seed_completed';
