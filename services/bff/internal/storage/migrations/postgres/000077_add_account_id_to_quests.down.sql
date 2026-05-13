-- Rollback: remove account_id from quests table.
DROP INDEX IF EXISTS idx_quests_account_id;
ALTER TABLE quests DROP COLUMN IF EXISTS account_id;
