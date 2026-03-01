DROP INDEX IF EXISTS idx_quests_session_id;
ALTER TABLE quests DROP COLUMN completion_source;
ALTER TABLE quests DROP COLUMN session_id;
