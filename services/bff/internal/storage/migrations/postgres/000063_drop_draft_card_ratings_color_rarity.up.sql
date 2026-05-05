-- Drop the color and rarity columns from draft_card_ratings.
-- These columns were never written by the sync Lambda; color and rarity are
-- resolved at BFF read time via LEFT JOIN against the cards table (Option C,
-- issue #1133). Removing the dead columns avoids confusion and keeps the
-- schema consistent with actual write patterns.
ALTER TABLE draft_card_ratings
    DROP COLUMN IF EXISTS color,
    DROP COLUMN IF EXISTS rarity;
