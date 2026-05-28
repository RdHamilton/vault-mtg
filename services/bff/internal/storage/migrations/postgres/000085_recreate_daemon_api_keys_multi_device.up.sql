-- Migration 000085: drop+recreate daemon_api_keys with multi-device schema.
-- Per ADR-031 + RdHamilton/vault-mtg#2573 + #2630.
--
-- Pre-beta cohort means essentially zero real pairings exist in production
-- (ADR-031 §"Constraints in force"). Any currently-paired daemon receives 401
-- on next dispatch (per ADR-031 §3 + middleware's WHERE revoked_at IS NULL
-- filter) and must re-pair through the SPA. No row-level backfill.
--
-- Schema changes vs. migration 000075:
--   * Drop global UNIQUE(device_id) — UUIDv4 entropy + composite unique is
--     sufficient; ADR-028 makes device_id server-issued.
--   * Replace UNIQUE(account_id) with UNIQUE(account_id, device_id) — the
--     multi-device authentication principal per ADR-031 §1.
--   * Add paired_at TIMESTAMPTZ NOT NULL DEFAULT now() — load-bearing for
--     ADR-031 §4's GET /v1/daemons response contract.
--   * Rename last_used → last_used_at — column-naming consistency with
--     ADR-031 §4's projection.
--   * Add updated_at TIMESTAMPTZ NOT NULL DEFAULT now() — audit-trail
--     symmetry; UPDATE statements must keep it current.
--   * Add partial index on key_prefix WHERE revoked_at IS NULL — pre-stages
--     ADR-031 Open Questions §6 (N x bcrypt scaling cliff fix at the v0.4.0+
--     indexed-lookup rewrite).

DROP TABLE IF EXISTS daemon_api_keys;

CREATE TABLE daemon_api_keys (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id    TEXT        NOT NULL,
    key_hash      TEXT        NOT NULL,
    key_prefix    TEXT        NOT NULL,
    device_id     UUID        NOT NULL,
    platform      TEXT        NOT NULL,
    daemon_ver    TEXT        NOT NULL,
    paired_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at  TIMESTAMPTZ,
    revoked_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT daemon_api_keys_account_device_unique UNIQUE (account_id, device_id)
);

CREATE INDEX daemon_api_keys_account_id_idx
    ON daemon_api_keys (account_id);

CREATE INDEX daemon_api_keys_account_active_idx
    ON daemon_api_keys (account_id) WHERE revoked_at IS NULL;

CREATE INDEX daemon_api_keys_key_prefix_active_idx
    ON daemon_api_keys (key_prefix) WHERE revoked_at IS NULL;
