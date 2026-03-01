ALTER TABLE quests ADD COLUMN session_id TEXT;
ALTER TABLE quests ADD COLUMN completion_source TEXT;
CREATE INDEX IF NOT EXISTS idx_quests_session_id ON quests(session_id);
