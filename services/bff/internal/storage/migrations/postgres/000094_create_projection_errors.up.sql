-- Migration 000094: create projection_errors dead-letter table (DLQ).
--
-- Rows that fail permanently during projection are written here so they are
-- not lost, and so the queue continues to drain rather than stalling.
--
-- raw_payload is TEXT, NOT JSONB.  This is a deliberate deviation from the
-- project's default of JSONB for event payloads (see ADR-039).  The reason is
-- that a structurally invalid payload (non-JSON bytes, truncated JSON, wrong
-- charset) would be rejected at the Postgres level on INSERT if the column type
-- were JSONB.  Keeping it TEXT allows the DLQ to store any byte string the
-- daemon delivered, which is the correct safety-net behaviour.
--
-- Acceptance: ticket #1633

CREATE TABLE IF NOT EXISTS projection_errors (
    id              BIGSERIAL PRIMARY KEY,
    daemon_event_id BIGINT      NOT NULL,
    account_id      TEXT        NOT NULL,
    event_type      TEXT        NOT NULL,
    raw_payload     TEXT        NOT NULL,
    error_message   TEXT        NOT NULL,
    failed_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Lookup by account (all errors for a given account, newest-first).
CREATE INDEX IF NOT EXISTS idx_projection_errors_account_id
    ON projection_errors (account_id, failed_at DESC);

-- Lookup by source event id (idempotency + audit trail).
CREATE INDEX IF NOT EXISTS idx_projection_errors_daemon_event_id
    ON projection_errors (daemon_event_id);
