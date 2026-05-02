-- Remove URL columns from draft_card_ratings
ALTER TABLE draft_card_ratings DROP COLUMN IF EXISTS url_back;
ALTER TABLE draft_card_ratings DROP COLUMN IF EXISTS url;
