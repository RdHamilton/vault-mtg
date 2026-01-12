-- Migration: Add life change tracking to game_plays
-- Enables tracking life total changes (damage, life gain) during matches

ALTER TABLE game_plays ADD COLUMN life_from INTEGER;
ALTER TABLE game_plays ADD COLUMN life_to INTEGER;
