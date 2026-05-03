-- Enable pgcrypto for gen_random_bytes (API key generation)
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- users represents the app-level user (the person who logs in and pays).
-- Distinct from `accounts`, which are MTGA Arena accounts owned by a user.
CREATE TABLE users (
    id                  BIGSERIAL PRIMARY KEY,
    email               TEXT NOT NULL UNIQUE,
    api_key             TEXT NOT NULL UNIQUE DEFAULT encode(gen_random_bytes(32), 'hex'),
    subscription_status TEXT NOT NULL DEFAULT 'free'
                            CHECK (subscription_status IN ('free', 'pro')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email   ON users(email);
CREATE INDEX idx_users_api_key ON users(api_key);

-- Link each MTGA account to an app-level user.
-- Nullable during migration; backfill required before enforcing NOT NULL.
ALTER TABLE accounts ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;

CREATE INDEX idx_accounts_user_id ON accounts(user_id);
