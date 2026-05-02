-- Migration: Fix duplicate game plays (PostgreSQL)

DELETE FROM game_plays
WHERE id NOT IN (
    SELECT MIN(id)
    FROM game_plays
    GROUP BY game_id, sequence_number
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_game_plays_unique ON game_plays(game_id, sequence_number);
