-- Add opponent tracking and game-level result reasons (PostgreSQL)

-- Add opponent tracking fields to matches table
ALTER TABLE matches ADD COLUMN IF NOT EXISTS opponent_name TEXT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS opponent_id TEXT;

CREATE INDEX IF NOT EXISTS idx_matches_opponent_id ON matches(opponent_id);
CREATE INDEX IF NOT EXISTS idx_matches_opponent_name ON matches(opponent_name);

-- Add result_reason to games table
ALTER TABLE games ADD COLUMN IF NOT EXISTS result_reason TEXT;

CREATE INDEX IF NOT EXISTS idx_games_result_reason ON games(result_reason);
