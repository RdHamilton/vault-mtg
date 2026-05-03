-- Migration: Add opponent deck analysis tables (PostgreSQL)

ALTER TABLE deck_performance_history ADD COLUMN IF NOT EXISTS opponent_color_identity TEXT;
ALTER TABLE deck_performance_history ADD COLUMN IF NOT EXISTS opponent_confidence REAL DEFAULT 0;
ALTER TABLE deck_performance_history ADD COLUMN IF NOT EXISTS opponent_cards_seen INTEGER DEFAULT 0;

CREATE TABLE IF NOT EXISTS opponent_deck_profiles (
    id BIGSERIAL PRIMARY KEY,
    match_id TEXT NOT NULL UNIQUE,
    detected_archetype TEXT,
    archetype_confidence REAL DEFAULT 0,
    color_identity TEXT NOT NULL,
    deck_style TEXT,
    cards_observed INTEGER NOT NULL DEFAULT 0,
    estimated_deck_size INTEGER DEFAULT 60,
    observed_card_ids TEXT,
    inferred_card_ids TEXT,
    signature_cards TEXT,
    format TEXT,
    meta_archetype_id INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (match_id) REFERENCES matches(id) ON DELETE CASCADE
);

CREATE INDEX idx_opponent_profiles_match_id ON opponent_deck_profiles(match_id);
CREATE INDEX idx_opponent_profiles_archetype ON opponent_deck_profiles(detected_archetype);
CREATE INDEX idx_opponent_profiles_format ON opponent_deck_profiles(format);

CREATE TABLE IF NOT EXISTS matchup_statistics (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL,
    player_archetype TEXT NOT NULL,
    opponent_archetype TEXT NOT NULL,
    format TEXT NOT NULL,
    total_matches INTEGER DEFAULT 0,
    wins INTEGER DEFAULT 0,
    losses INTEGER DEFAULT 0,
    avg_game_duration INTEGER,
    last_match_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
    UNIQUE(account_id, player_archetype, opponent_archetype, format)
);

CREATE INDEX idx_matchup_stats_account ON matchup_statistics(account_id);
CREATE INDEX idx_matchup_stats_player_archetype ON matchup_statistics(player_archetype);
CREATE INDEX idx_matchup_stats_opponent_archetype ON matchup_statistics(opponent_archetype);
CREATE INDEX idx_matchup_stats_format ON matchup_statistics(format);

CREATE TABLE IF NOT EXISTS archetype_expected_cards (
    id BIGSERIAL PRIMARY KEY,
    archetype_name TEXT NOT NULL,
    format TEXT NOT NULL,
    card_id INTEGER NOT NULL,
    card_name TEXT NOT NULL,
    inclusion_rate REAL DEFAULT 0,
    avg_copies REAL DEFAULT 1,
    is_signature BOOLEAN DEFAULT FALSE,
    category TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(archetype_name, format, card_id)
);

CREATE INDEX idx_expected_cards_archetype ON archetype_expected_cards(archetype_name);
CREATE INDEX idx_expected_cards_format ON archetype_expected_cards(format);
