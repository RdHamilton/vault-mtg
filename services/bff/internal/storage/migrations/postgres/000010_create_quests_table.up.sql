-- Create quests table for tracking daily quest progress and completion (PostgreSQL)
CREATE TABLE quests (
    id BIGSERIAL PRIMARY KEY,
    quest_id TEXT NOT NULL,
    quest_type TEXT,
    goal INTEGER NOT NULL,
    starting_progress INTEGER NOT NULL DEFAULT 0,
    ending_progress INTEGER NOT NULL DEFAULT 0,
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    can_swap BOOLEAN NOT NULL DEFAULT TRUE,
    rewards TEXT,
    assigned_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    rerolled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(quest_id, assigned_at)
);

CREATE INDEX idx_quests_completed ON quests(completed);
CREATE INDEX idx_quests_assigned_at ON quests(assigned_at);
CREATE INDEX idx_quests_completed_at ON quests(completed_at);
