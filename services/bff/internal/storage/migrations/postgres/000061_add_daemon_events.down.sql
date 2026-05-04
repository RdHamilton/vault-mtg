-- Revert migration 000061: drop the daemon_events table and its index.
DROP INDEX IF EXISTS idx_daemon_events_user_occurred;
DROP TABLE IF EXISTS daemon_events CASCADE;
