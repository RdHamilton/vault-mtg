-- Reverse of 000054_initial_schema.up.sql
-- Drops all tables created by the initial schema migration.
-- Order respects FK dependencies (children first).

-- Account-scoped / tenant tables (reverse creation order)
DROP TABLE IF EXISTS matchup_statistics CASCADE;
DROP TABLE IF EXISTS opponent_deck_profiles CASCADE;
DROP TABLE IF EXISTS draft_pattern_analysis CASCADE;
DROP TABLE IF EXISTS draft_temporal_trends CASCADE;
DROP TABLE IF EXISTS draft_community_comparison CASCADE;
DROP TABLE IF EXISTS draft_archetype_stats CASCADE;
DROP TABLE IF EXISTS draft_match_results CASCADE;
DROP TABLE IF EXISTS draft_packs CASCADE;
DROP TABLE IF EXISTS draft_picks CASCADE;
DROP TABLE IF EXISTS draft_sessions CASCADE;
DROP TABLE IF EXISTS user_play_patterns CASCADE;
DROP TABLE IF EXISTS recommendation_feedback CASCADE;
DROP TABLE IF EXISTS ml_suggestions CASCADE;
DROP TABLE IF EXISTS improvement_suggestions CASCADE;
DROP TABLE IF EXISTS deck_performance_history CASCADE;
DROP TABLE IF EXISTS deck_notes CASCADE;
DROP TABLE IF EXISTS deck_tags CASCADE;
DROP TABLE IF EXISTS deck_cards CASCADE;
DROP TABLE IF EXISTS deck_permutations CASCADE;
DROP TABLE IF EXISTS decks CASCADE;
DROP TABLE IF EXISTS inventory_history CASCADE;
DROP TABLE IF EXISTS inventory CASCADE;
DROP TABLE IF EXISTS currency_history CASCADE;
DROP TABLE IF EXISTS collection_history CASCADE;
DROP TABLE IF EXISTS collection CASCADE;
DROP TABLE IF EXISTS rank_history CASCADE;
DROP TABLE IF EXISTS player_stats CASCADE;
DROP TABLE IF EXISTS opponent_cards_observed CASCADE;
DROP TABLE IF EXISTS game_state_snapshots CASCADE;
DROP TABLE IF EXISTS game_plays CASCADE;
DROP TABLE IF EXISTS games CASCADE;
DROP TABLE IF EXISTS matches CASCADE;
DROP TABLE IF EXISTS quests CASCADE;
DROP TABLE IF EXISTS accounts CASCADE;

-- Global / reference tables
DROP TABLE IF EXISTS ml_model_metadata CASCADE;
DROP TABLE IF EXISTS archetype_card_weights CASCADE;
DROP TABLE IF EXISTS archetype_expected_cards CASCADE;
DROP TABLE IF EXISTS deck_archetypes CASCADE;
DROP TABLE IF EXISTS card_individual_stats CASCADE;
DROP TABLE IF EXISTS card_affinity CASCADE;
DROP TABLE IF EXISTS card_combination_stats CASCADE;
DROP TABLE IF EXISTS card_similarity_cache CASCADE;
DROP TABLE IF EXISTS card_embeddings CASCADE;
DROP TABLE IF EXISTS card_frequency CASCADE;
DROP TABLE IF EXISTS cooccurrence_sources CASCADE;
DROP TABLE IF EXISTS card_cooccurrence CASCADE;
DROP TABLE IF EXISTS mtgzone_synergies CASCADE;
DROP TABLE IF EXISTS mtgzone_archetype_cards CASCADE;
DROP TABLE IF EXISTS mtgzone_archetypes CASCADE;
DROP TABLE IF EXISTS mtgzone_articles CASCADE;
DROP TABLE IF EXISTS edhrec_theme_cards CASCADE;
DROP TABLE IF EXISTS edhrec_card_metadata CASCADE;
DROP TABLE IF EXISTS edhrec_synergy CASCADE;
DROP TABLE IF EXISTS cfb_ratings CASCADE;
DROP TABLE IF EXISTS dataset_metadata CASCADE;
DROP TABLE IF EXISTS draft_color_ratings CASCADE;
DROP TABLE IF EXISTS draft_card_ratings CASCADE;
DROP TABLE IF EXISTS set_cards CASCADE;
DROP TABLE IF EXISTS sets CASCADE;
DROP TABLE IF EXISTS standard_config CASCADE;
DROP TABLE IF EXISTS processed_log_files CASCADE;
DROP TABLE IF EXISTS settings CASCADE;
DROP TABLE IF EXISTS metadata CASCADE;
DROP TABLE IF EXISTS migration_log CASCADE;
