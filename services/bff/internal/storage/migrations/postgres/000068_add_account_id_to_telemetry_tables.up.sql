-- Migration: Add account_id to telemetry tables for multi-tenant isolation
--
-- Tables covered: quests, inventory, inventory_history, game_plays
--
-- NOTE: quest_session_tracking and life_change_tracking do not exist in any
-- prior migration and are therefore skipped. They must be created with
-- account_id from the start in a future migration.

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
