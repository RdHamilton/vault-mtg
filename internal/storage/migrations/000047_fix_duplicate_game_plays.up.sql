-- Migration: Fix duplicate game plays
-- Adds unique constraint to prevent duplicate plays from being inserted

-- First, delete duplicate rows keeping only the first occurrence (by id)
DELETE FROM game_plays
WHERE id NOT IN (
    SELECT MIN(id)
    FROM game_plays
    GROUP BY game_id, sequence_number
);

-- Now add unique constraint to prevent future duplicates
CREATE UNIQUE INDEX IF NOT EXISTS idx_game_plays_unique ON game_plays(game_id, sequence_number);
