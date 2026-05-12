-- Migration 000076: create user_settings table for per-account preferences.
-- Phase 2 PR #12 (SPA route migration) — replaces the desktop-era global
-- settings(key,value) table with an account-scoped key/value store.
--
-- value is JSONB so callers can store strings, numbers, booleans, or
-- structured objects under any key without schema churn.

CREATE TABLE IF NOT EXISTS user_settings (
    account_id  BIGINT      NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    key         TEXT        NOT NULL,
    value       JSONB       NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, key)
);

CREATE INDEX IF NOT EXISTS idx_user_settings_account ON user_settings(account_id);
