-- Rollback 000077: restore quests.account_id to the TEXT form from 000068.
DROP INDEX IF EXISTS idx_quests_account_id;
ALTER TABLE quests DROP COLUMN IF EXISTS account_id;
ALTER TABLE quests ADD COLUMN account_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_quests_account_id ON quests(account_id);
