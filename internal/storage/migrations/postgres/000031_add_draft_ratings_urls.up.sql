-- Add URL columns to draft_card_ratings (PostgreSQL)
ALTER TABLE draft_card_ratings ADD COLUMN IF NOT EXISTS url TEXT;
ALTER TABLE draft_card_ratings ADD COLUMN IF NOT EXISTS url_back TEXT;
