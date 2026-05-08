-- Reverse: remove account_id from telemetry tables

-- game_plays
DROP INDEX IF EXISTS idx_game_plays_account_id;
ALTER TABLE game_plays DROP COLUMN IF EXISTS account_id;

-- inventory_history
DROP INDEX IF EXISTS idx_inventory_history_account_id;
ALTER TABLE inventory_history DROP COLUMN IF EXISTS account_id;

-- inventory
DROP INDEX IF EXISTS idx_inventory_account_id;
ALTER TABLE inventory DROP COLUMN IF EXISTS account_id;

-- quests
DROP INDEX IF EXISTS idx_quests_account_id;
ALTER TABLE quests DROP COLUMN IF EXISTS account_id;
