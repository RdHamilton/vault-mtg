-- Add multi-account support (PostgreSQL)
-- Creates accounts table and adds account_id to all relevant tables

-- Accounts table: stores account information
CREATE TABLE IF NOT EXISTS accounts (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    screen_name TEXT,
    client_id TEXT,
    is_default INTEGER NOT NULL DEFAULT 0 CHECK(is_default IN (0, 1)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_accounts_is_default ON accounts(is_default);
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_default ON accounts(is_default) WHERE is_default = 1;

-- Add account_id to matches table
ALTER TABLE matches ADD COLUMN IF NOT EXISTS account_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_matches_account_id ON matches(account_id);
-- Insert default account if none exists
INSERT INTO accounts (name, is_default) VALUES ('Default Account', 1) ON CONFLICT DO NOTHING;
UPDATE matches SET account_id = (SELECT id FROM accounts WHERE is_default = 1 LIMIT 1) WHERE account_id IS NULL;

-- Add account_id to player_stats table
ALTER TABLE player_stats ADD COLUMN IF NOT EXISTS account_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_player_stats_account_id ON player_stats(account_id);
UPDATE player_stats SET account_id = (SELECT id FROM accounts WHERE is_default = 1 LIMIT 1) WHERE account_id IS NULL;
-- Update unique constraint to include account_id
DROP INDEX IF EXISTS idx_player_stats_date_format;
CREATE UNIQUE INDEX IF NOT EXISTS idx_player_stats_date_format_account ON player_stats(date, format, account_id);

-- Add account_id to decks table
ALTER TABLE decks ADD COLUMN IF NOT EXISTS account_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_decks_account_id ON decks(account_id);
UPDATE decks SET account_id = (SELECT id FROM accounts WHERE is_default = 1 LIMIT 1) WHERE account_id IS NULL;

-- Recreate collection table with account_id composite PK
-- First ensure default account exists
INSERT INTO accounts (name, is_default) VALUES ('Default Account', 1) ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS collection_new (
    account_id BIGINT NOT NULL,
    card_id INTEGER NOT NULL,
    quantity INTEGER NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, card_id),
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

INSERT INTO collection_new (account_id, card_id, quantity, updated_at)
SELECT (SELECT id FROM accounts WHERE is_default = 1 LIMIT 1), card_id, quantity, updated_at FROM collection;

DROP TABLE collection;
ALTER TABLE collection_new RENAME TO collection;
CREATE INDEX IF NOT EXISTS idx_collection_account_id ON collection(account_id);

-- Add account_id to collection_history table
ALTER TABLE collection_history ADD COLUMN IF NOT EXISTS account_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_collection_history_account_id ON collection_history(account_id);
UPDATE collection_history SET account_id = (SELECT id FROM accounts WHERE is_default = 1 LIMIT 1) WHERE account_id IS NULL;

-- Add account_id to rank_history table
ALTER TABLE rank_history ADD COLUMN IF NOT EXISTS account_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_rank_history_account_id ON rank_history(account_id);
UPDATE rank_history SET account_id = (SELECT id FROM accounts WHERE is_default = 1 LIMIT 1) WHERE account_id IS NULL;

-- Add account_id to draft_events table
ALTER TABLE draft_events ADD COLUMN IF NOT EXISTS account_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_draft_events_account_id ON draft_events(account_id);
UPDATE draft_events SET account_id = (SELECT id FROM accounts WHERE is_default = 1 LIMIT 1) WHERE account_id IS NULL;
