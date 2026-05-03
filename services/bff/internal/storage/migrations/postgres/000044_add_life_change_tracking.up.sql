-- Migration: Add life change tracking to game_plays (PostgreSQL)
ALTER TABLE game_plays ADD COLUMN IF NOT EXISTS life_from INTEGER;
ALTER TABLE game_plays ADD COLUMN IF NOT EXISTS life_to INTEGER;
