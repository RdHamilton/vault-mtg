-- Rollback: Remove opponent deck analysis tables
DROP TABLE IF EXISTS archetype_expected_cards;
DROP TABLE IF EXISTS matchup_statistics;
DROP TABLE IF EXISTS opponent_deck_profiles;

ALTER TABLE deck_performance_history DROP COLUMN IF EXISTS opponent_cards_seen;
ALTER TABLE deck_performance_history DROP COLUMN IF EXISTS opponent_confidence;
ALTER TABLE deck_performance_history DROP COLUMN IF EXISTS opponent_color_identity;
