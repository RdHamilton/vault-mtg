-- ML Suggestion Engine Schema (PostgreSQL)

CREATE TABLE IF NOT EXISTS card_combination_stats (
    id BIGSERIAL PRIMARY KEY,
    card_id_1 INTEGER NOT NULL,
    card_id_2 INTEGER NOT NULL,
    deck_id TEXT,
    format TEXT DEFAULT 'Standard',
    games_together INTEGER DEFAULT 0,
    games_card1_only INTEGER DEFAULT 0,
    games_card2_only INTEGER DEFAULT 0,
    wins_together INTEGER DEFAULT 0,
    wins_card1_only INTEGER DEFAULT 0,
    wins_card2_only INTEGER DEFAULT 0,
    synergy_score REAL DEFAULT 0.0,
    confidence_score REAL DEFAULT 0.0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(card_id_1, card_id_2, deck_id, format),
    CHECK(card_id_1 < card_id_2)
);

CREATE INDEX IF NOT EXISTS idx_combo_stats_card1 ON card_combination_stats(card_id_1);
CREATE INDEX IF NOT EXISTS idx_combo_stats_card2 ON card_combination_stats(card_id_2);
CREATE INDEX IF NOT EXISTS idx_combo_stats_deck ON card_combination_stats(deck_id);
CREATE INDEX IF NOT EXISTS idx_combo_stats_format ON card_combination_stats(format);
CREATE INDEX IF NOT EXISTS idx_combo_stats_synergy ON card_combination_stats(synergy_score DESC);

CREATE TABLE IF NOT EXISTS ml_suggestions (
    id BIGSERIAL PRIMARY KEY,
    deck_id TEXT NOT NULL,
    suggestion_type TEXT NOT NULL,
    card_id INTEGER,
    card_name TEXT,
    swap_for_card_id INTEGER,
    swap_for_card_name TEXT,
    confidence REAL DEFAULT 0.0,
    expected_win_rate_change REAL DEFAULT 0.0,
    title TEXT NOT NULL,
    description TEXT,
    reasoning TEXT,
    evidence TEXT,
    is_dismissed BOOLEAN DEFAULT FALSE,
    was_applied BOOLEAN DEFAULT FALSE,
    outcome_win_rate_change REAL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    applied_at TIMESTAMPTZ,
    outcome_recorded_at TIMESTAMPTZ,
    FOREIGN KEY (deck_id) REFERENCES decks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_ml_suggestions_deck ON ml_suggestions(deck_id);
CREATE INDEX IF NOT EXISTS idx_ml_suggestions_type ON ml_suggestions(suggestion_type);
CREATE INDEX IF NOT EXISTS idx_ml_suggestions_confidence ON ml_suggestions(confidence DESC);
CREATE INDEX IF NOT EXISTS idx_ml_suggestions_active ON ml_suggestions(deck_id, is_dismissed);

CREATE TABLE IF NOT EXISTS card_affinity (
    id BIGSERIAL PRIMARY KEY,
    card_id_1 INTEGER NOT NULL,
    card_id_2 INTEGER NOT NULL,
    format TEXT DEFAULT 'Standard',
    affinity_score REAL DEFAULT 0.0,
    sample_size INTEGER DEFAULT 0,
    confidence REAL DEFAULT 0.0,
    source TEXT DEFAULT 'historical',
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(card_id_1, card_id_2, format),
    CHECK(card_id_1 < card_id_2)
);

CREATE INDEX IF NOT EXISTS idx_affinity_card1 ON card_affinity(card_id_1);
CREATE INDEX IF NOT EXISTS idx_affinity_card2 ON card_affinity(card_id_2);
CREATE INDEX IF NOT EXISTS idx_affinity_score ON card_affinity(affinity_score DESC);

CREATE TABLE IF NOT EXISTS user_play_patterns (
    id BIGSERIAL PRIMARY KEY,
    account_id TEXT NOT NULL,
    preferred_archetype TEXT,
    aggro_affinity REAL DEFAULT 0.0,
    midrange_affinity REAL DEFAULT 0.0,
    control_affinity REAL DEFAULT 0.0,
    combo_affinity REAL DEFAULT 0.0,
    color_preferences TEXT,
    avg_game_length REAL DEFAULT 0.0,
    aggression_score REAL DEFAULT 0.0,
    interaction_score REAL DEFAULT 0.0,
    total_matches INTEGER DEFAULT 0,
    total_decks INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(account_id)
);

CREATE INDEX IF NOT EXISTS idx_play_patterns_account ON user_play_patterns(account_id);

CREATE TABLE IF NOT EXISTS ml_model_metadata (
    id BIGSERIAL PRIMARY KEY,
    model_name TEXT NOT NULL,
    model_version TEXT NOT NULL,
    training_samples INTEGER DEFAULT 0,
    training_date TIMESTAMPTZ,
    accuracy REAL,
    precision_score REAL,
    recall REAL,
    f1_score REAL,
    is_active BOOLEAN DEFAULT FALSE,
    model_data BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(model_name, model_version)
);
