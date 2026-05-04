-- Reverse: drop all composite account_id indexes added in 000053 up migration.
-- Note: CONCURRENTLY omitted — cannot run inside a migration transaction block.
-- Note: idx_currency_history_account_id_timestamp_desc is not dropped here —
-- it was never created by this migration (currency_history does not exist at this point).

DROP INDEX IF EXISTS idx_matches_account_id_timestamp;
DROP INDEX IF EXISTS idx_matches_account_id_format;
DROP INDEX IF EXISTS idx_matches_account_id_format_timestamp;

DROP INDEX IF EXISTS idx_draft_sessions_account_id_created_at;
DROP INDEX IF EXISTS idx_draft_sessions_account_id_set_code;

DROP INDEX IF EXISTS idx_player_stats_account_id_date;
DROP INDEX IF EXISTS idx_player_stats_account_id_format_date;

DROP INDEX IF EXISTS idx_decks_account_id_modified_at;
DROP INDEX IF EXISTS idx_decks_account_id_format;

DROP INDEX IF EXISTS idx_matchup_stats_account_id_format;
DROP INDEX IF EXISTS idx_matchup_stats_account_id_format_archetype;

DROP INDEX IF EXISTS idx_deck_perf_history_account_id_timestamp;
DROP INDEX IF EXISTS idx_deck_perf_history_account_id_format;
