-- Rollback migration 000085: restore the pre-multi-device shape from 000075.
--
-- Symmetric drop+recreate. Per ADR-031's pre-beta cohort constraint, any
-- currently-paired daemon loses its row again and re-pairs on next dispatch.
-- The rollback is strictly equivalent in user impact to the up migration —
-- both force a re-pair.
--
-- IMPORTANT: rollback must roll back dependents in order
--   UI (#2632) -> endpoints (#21) -> handler (#2631) -> migration
-- Rolling back the schema without rolling back the dependents will hard-break
-- the BFF. See PR description "Rollback story" section.

DROP TABLE IF EXISTS daemon_api_keys;

CREATE TABLE IF NOT EXISTS daemon_api_keys (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id  TEXT        NOT NULL,
    key_hash    TEXT        NOT NULL,
    key_prefix  TEXT        NOT NULL,
    device_id   UUID        NOT NULL,
    platform    TEXT        NOT NULL,
    daemon_ver  TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used   TIMESTAMPTZ,
    revoked_at  TIMESTAMPTZ,
    CONSTRAINT daemon_api_keys_account_id_unique UNIQUE (account_id),
    CONSTRAINT daemon_api_keys_device_id_unique  UNIQUE (device_id)
);

CREATE INDEX IF NOT EXISTS daemon_api_keys_account_id_idx ON daemon_api_keys (account_id);
CREATE INDEX IF NOT EXISTS daemon_api_keys_device_id_idx  ON daemon_api_keys (device_id);
