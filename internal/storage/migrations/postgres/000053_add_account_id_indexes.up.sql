-- Add composite account_id indexes for multi-tenant PostgreSQL query patterns.
-- Every user-data table that has account_id must have it as the leading column
-- in composite indexes covering the common sort/filter dimensions.
--
-- Note: CONCURRENTLY is omitted here because migrations run inside a transaction
-- block and PostgreSQL does not allow CREATE INDEX CONCURRENTLY inside transactions.
--
-- Note: currency_history is excluded — it was dropped in migration 000025 and is
-- not recreated until migration 000054, which creates its own account_id indexes.

-- matches: queries by account + time range, account + format, account + format + time
CREATE INDEX IF NOT EXISTS idx_matches_account_id_timestamp
    ON matches (account_id, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_matches_account_id_format
    ON matches (account_id, format);

CREATE INDEX IF NOT EXISTS idx_matches_account_id_format_timestamp
    ON matches (account_id, format, timestamp DESC);

-- draft_sessions: queries by account sorted by created_at; also by set_code within account
CREATE INDEX IF NOT EXISTS idx_draft_sessions_account_id_created_at
    ON draft_sessions (account_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_draft_sessions_account_id_set_code
    ON draft_sessions (account_id, set_code);

-- player_stats: queries by account + date range, account + format + date
CREATE INDEX IF NOT EXISTS idx_player_stats_account_id_date
    ON player_stats (account_id, date DESC);

CREATE INDEX IF NOT EXISTS idx_player_stats_account_id_format_date
    ON player_stats (account_id, format, date DESC);

-- decks: queries by account + modified_at (list/sort), account + format
CREATE INDEX IF NOT EXISTS idx_decks_account_id_modified_at
    ON decks (account_id, modified_at DESC);

CREATE INDEX IF NOT EXISTS idx_decks_account_id_format
    ON decks (account_id, format);

-- matchup_statistics: queries by account + format + archetype
CREATE INDEX IF NOT EXISTS idx_matchup_stats_account_id_format
    ON matchup_statistics (account_id, format);

CREATE INDEX IF NOT EXISTS idx_matchup_stats_account_id_format_archetype
    ON matchup_statistics (account_id, format, player_archetype);

-- deck_performance_history: queries by account + timestamp, account + format
CREATE INDEX IF NOT EXISTS idx_deck_perf_history_account_id_timestamp
    ON deck_performance_history (account_id, match_timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_deck_perf_history_account_id_format
    ON deck_performance_history (account_id, format);
