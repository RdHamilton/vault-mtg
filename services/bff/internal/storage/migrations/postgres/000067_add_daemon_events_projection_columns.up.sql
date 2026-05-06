-- Migration 000067: add projection cursor and idempotency key to daemon_events.
--
-- projected_at is set by the projection worker after a row has been written
-- to its destination table (matches / draft_sessions). NULL means "pending".
--
-- event_id is the daemon-issued unique identifier for the event used for
-- ON CONFLICT idempotency. The daemon already populates this in payload.event_id;
-- promote it to a top-level column so we can index it.
--
-- Acceptance: ticket #1401

ALTER TABLE daemon_events
    ADD COLUMN IF NOT EXISTS event_id     TEXT,
    ADD COLUMN IF NOT EXISTS projected_at TIMESTAMPTZ;

-- Backfill event_id for any pre-existing rows from payload->>'event_id'.
-- Rows without a payload event_id are left NULL; they will be skipped by the
-- projection worker (logged + projected_at set so they do not re-scan).
UPDATE daemon_events
SET event_id = payload->>'event_id'
WHERE event_id IS NULL
  AND payload->>'event_id' IS NOT NULL;

-- Cursor index: the worker scans WHERE projected_at IS NULL ORDER BY received_at
-- LIMIT 100. Partial index keeps the index small because most rows are projected.
CREATE INDEX IF NOT EXISTS idx_daemon_events_pending
    ON daemon_events (received_at)
    WHERE projected_at IS NULL;

-- Idempotency: (user_id, event_id) is unique per daemon. If the daemon retries
-- the same event, the projection ON CONFLICT clauses use this to dedupe.
-- Partial index allows legacy rows with NULL event_id to coexist.
CREATE UNIQUE INDEX IF NOT EXISTS idx_daemon_events_user_event_id
    ON daemon_events (user_id, event_id)
    WHERE event_id IS NOT NULL;
