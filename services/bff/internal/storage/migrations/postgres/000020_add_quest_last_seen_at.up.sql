-- Add last_seen_at column to quests table (PostgreSQL)
ALTER TABLE quests ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;

UPDATE quests SET last_seen_at = created_at WHERE last_seen_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_quests_last_seen_at ON quests(last_seen_at);
