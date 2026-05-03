-- Remove data_source columns from existing tables
ALTER TABLE draft_color_ratings DROP COLUMN IF EXISTS data_source;
ALTER TABLE draft_card_ratings DROP COLUMN IF EXISTS data_source;

-- Drop dataset_metadata table
DROP TABLE IF EXISTS dataset_metadata;
