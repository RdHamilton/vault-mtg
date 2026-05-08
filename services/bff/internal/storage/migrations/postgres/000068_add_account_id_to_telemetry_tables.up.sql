-- Migration: Add account_id to telemetry tables for multi-tenant isolation
--
-- Tables quests, inventory, inventory_history, game_plays already exist —
-- account_id is added via ALTER TABLE.
--
-- Tables quest_session_tracking and life_change_tracking do not exist yet —
-- they are created here with account_id from the start so no backfill is needed.

-- quests
ALTER TABLE quests ADD COLUMN IF NOT EXISTS account_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_quests_account_id ON quests (account_id);

-- inventory
ALTER TABLE inventory ADD COLUMN IF NOT EXISTS account_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_inventory_account_id ON inventory (account_id);

-- inventory_history
ALTER TABLE inventory_history ADD COLUMN IF NOT EXISTS account_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_inventory_history_account_id ON inventory_history (account_id);

-- game_plays
ALTER TABLE game_plays ADD COLUMN IF NOT EXISTS account_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_game_plays_account_id ON game_plays (account_id);

-- quest_session_tracking (new table — account_id included from creation)
CREATE TABLE IF NOT EXISTS quest_session_tracking (
    id BIGSERIAL PRIMARY KEY,
    account_id TEXT NOT NULL DEFAULT '',
    session_id TEXT NOT NULL,
    quest_id TEXT NOT NULL,
    progress_delta INTEGER NOT NULL DEFAULT 0,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_quest_session_tracking_account_id ON quest_session_tracking (account_id);
CREATE INDEX IF NOT EXISTS idx_quest_session_tracking_session_id ON quest_session_tracking (account_id, session_id);

-- life_change_tracking (new table — account_id included from creation)
CREATE TABLE IF NOT EXISTS life_change_tracking (
    id BIGSERIAL PRIMARY KEY,
    account_id TEXT NOT NULL DEFAULT '',
    match_id TEXT NOT NULL,
    game_number INTEGER NOT NULL DEFAULT 1,
    player_type TEXT NOT NULL,
    life_total INTEGER NOT NULL,
    delta INTEGER NOT NULL DEFAULT 0,
    turn_number INTEGER,
    recorded_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_life_change_tracking_account_id ON life_change_tracking (account_id);
CREATE INDEX IF NOT EXISTS idx_life_change_tracking_match_id ON life_change_tracking (account_id, match_id);
