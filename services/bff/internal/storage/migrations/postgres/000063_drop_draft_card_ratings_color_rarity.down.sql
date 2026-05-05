-- Restore the color and rarity columns to draft_card_ratings.
-- These columns were dropped in the up migration because they were never
-- written by the sync Lambda. They are restored here for rollback purposes;
-- previously-stored values are lost and cannot be recovered from this migration.
ALTER TABLE draft_card_ratings
    ADD COLUMN IF NOT EXISTS color  TEXT,
    ADD COLUMN IF NOT EXISTS rarity TEXT;
