-- Rollback: Remove session_id and completion_source columns
DROP INDEX IF EXISTS idx_quests_session_id;
ALTER TABLE quests DROP COLUMN IF EXISTS completion_source;
ALTER TABLE quests DROP COLUMN IF EXISTS session_id;
