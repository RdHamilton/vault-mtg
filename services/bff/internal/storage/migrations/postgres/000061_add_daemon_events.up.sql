-- daemon_events persists inbound events received from the daemon service.
-- user_id links to users.id (BIGINT); account_id is the MTGA Arena account
-- string sent by the daemon (TEXT, not a FK into accounts.id).
-- References: issue #1171, sub-task A of #1126.
CREATE TABLE IF NOT EXISTS daemon_events (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL,
    account_id  TEXT NOT NULL,
    event_type  TEXT NOT NULL,
    payload     JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_daemon_events_user_occurred
    ON daemon_events (user_id, occurred_at DESC);
