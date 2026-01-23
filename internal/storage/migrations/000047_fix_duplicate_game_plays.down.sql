-- Drop the unique constraint (duplicates may re-occur)
DROP INDEX IF EXISTS idx_game_plays_unique;
