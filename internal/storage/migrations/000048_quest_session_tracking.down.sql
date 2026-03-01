-- Rollback: Remove session_id and completion_source columns

DROP INDEX IF EXISTS idx_quests_session_id;

-- SQLite doesn't support ALTER TABLE DROP COLUMN directly in older versions
-- We need to recreate the table without the columns

-- Create temporary table with schema before migration 000048
CREATE TABLE quests_backup (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    quest_id TEXT NOT NULL,
    quest_type TEXT,
    goal INTEGER NOT NULL,
    starting_progress INTEGER NOT NULL DEFAULT 0,
    ending_progress INTEGER NOT NULL DEFAULT 0,
    completed BOOLEAN NOT NULL DEFAULT 0,
    can_swap BOOLEAN NOT NULL DEFAULT 1,
    rewards TEXT,
    assigned_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP,
    rerolled BOOLEAN NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_seen_at TIMESTAMP,
    UNIQUE(quest_id, assigned_at)
);

-- Copy data from current table
INSERT INTO quests_backup
SELECT id, quest_id, quest_type, goal, starting_progress, ending_progress,
       completed, can_swap, rewards, assigned_at, completed_at, rerolled, created_at,
       last_seen_at
FROM quests;

-- Drop current table
DROP TABLE quests;

-- Rename backup to original name
ALTER TABLE quests_backup RENAME TO quests;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_quests_completed ON quests(completed);
CREATE INDEX IF NOT EXISTS idx_quests_assigned_at ON quests(assigned_at);
CREATE INDEX IF NOT EXISTS idx_quests_completed_at ON quests(completed_at);
CREATE INDEX IF NOT EXISTS idx_quests_last_seen_at ON quests(last_seen_at);
