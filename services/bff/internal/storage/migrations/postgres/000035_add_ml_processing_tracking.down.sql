-- Remove ML processing tracking from matches table
DROP INDEX IF EXISTS idx_matches_processed_for_ml;
DROP INDEX IF EXISTS idx_card_individual_stats_format;
DROP TABLE IF EXISTS card_individual_stats;
ALTER TABLE matches DROP COLUMN IF EXISTS processed_for_ml;
