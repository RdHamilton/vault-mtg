-- Rollback: Remove life change tracking from game_plays
ALTER TABLE game_plays DROP COLUMN IF EXISTS life_to;
ALTER TABLE game_plays DROP COLUMN IF EXISTS life_from;
