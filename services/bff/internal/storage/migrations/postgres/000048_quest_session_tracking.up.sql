-- Add quest session tracking columns (PostgreSQL)
ALTER TABLE quests ADD COLUMN IF NOT EXISTS session_id TEXT;
ALTER TABLE quests ADD COLUMN IF NOT EXISTS completion_source TEXT;
CREATE INDEX IF NOT EXISTS idx_quests_session_id ON quests(session_id);
