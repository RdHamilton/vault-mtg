-- Migration 000069: add sequence column to daemon_events per ADR-013.
--
-- sequence is the monotonically-increasing counter emitted by the daemon for
-- each event within a session.  It enables the GRE projector to order events
-- by (occurred_at, sequence) and detect gaps in delivery.
--
-- Pre-ADR-013 rows have no ordering guarantee; 0 is a safe sentinel value.
--
-- Acceptance: ticket #1521

ALTER TABLE daemon_events
    ADD COLUMN IF NOT EXISTS sequence BIGINT NOT NULL DEFAULT 0;

-- Belt-and-suspenders backfill: for any row where the column landed as NULL
-- due to a race during a concurrent migration, set it to 0.
UPDATE daemon_events SET sequence = 0 WHERE sequence IS NULL;

-- Composite index supporting gap detection queries scoped by account:
--   SELECT ... FROM daemon_events WHERE account_id = $1 ORDER BY sequence
CREATE INDEX IF NOT EXISTS idx_daemon_events_account_sequence
    ON daemon_events (account_id, sequence);
