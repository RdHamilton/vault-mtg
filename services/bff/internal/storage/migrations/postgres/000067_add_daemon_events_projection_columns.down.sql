-- Down migration for 000067: remove projection cursor and idempotency key
-- from daemon_events. Fully reverses 000067_add_daemon_events_projection_columns.up.sql.

DROP INDEX IF EXISTS idx_daemon_events_user_event_id;
DROP INDEX IF EXISTS idx_daemon_events_pending;

ALTER TABLE daemon_events
    DROP COLUMN IF EXISTS projected_at,
    DROP COLUMN IF EXISTS event_id;
