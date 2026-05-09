-- Add partial flag to game_plays.
-- Rows with partial=true were emitted before the game was confirmed complete
-- (GRE buffer threshold flush or stale-sweep eviction).
ALTER TABLE game_plays
    ADD COLUMN IF NOT EXISTS partial BOOLEAN NOT NULL DEFAULT FALSE;
