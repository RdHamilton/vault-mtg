-- Rollback: Remove last_seen_at column and index
DROP INDEX IF EXISTS idx_quests_last_seen_at;
ALTER TABLE quests DROP COLUMN IF EXISTS last_seen_at;
