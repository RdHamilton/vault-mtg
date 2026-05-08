-- Reverse migration 000069: remove sequence column and its index.

DROP INDEX IF EXISTS idx_daemon_events_account_sequence;

ALTER TABLE daemon_events
    DROP COLUMN IF EXISTS sequence;
