-- Migration: Add deck performance tracking tables for ML training (PostgreSQL)

CREATE TABLE deck_performance_history (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL,
    deck_id TEXT NOT NULL,
    match_id TEXT NOT NULL,
    archetype TEXT,
    secondary_archetype TEXT,
    archetype_confidence REAL,
    color_identity TEXT NOT NULL,
    card_count INTEGER NOT NULL,
    result TEXT NOT NULL CHECK(result IN ('win', 'loss')),
    games_won INTEGER NOT NULL,
    games_lost INTEGER NOT NULL,
    duration_seconds INTEGER,
    format TEXT NOT NULL,
    event_type TEXT,
    opponent_archetype TEXT,
    rank_tier TEXT,
    match_timestamp TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
    FOREIGN KEY (deck_id) REFERENCES decks(id) ON DELETE CASCADE,
    FOREIGN KEY (match_id) REFERENCES matches(id) ON DELETE CASCADE
);

CREATE INDEX idx_deck_perf_history_account ON deck_performance_history(account_id);
CREATE INDEX idx_deck_perf_history_deck ON deck_performance_history(deck_id);
CREATE INDEX idx_deck_perf_history_archetype ON deck_performance_history(archetype);
CREATE INDEX idx_deck_perf_history_format ON deck_performance_history(format);
CREATE INDEX idx_deck_perf_history_timestamp ON deck_performance_history(match_timestamp);
CREATE INDEX idx_deck_perf_history_result ON deck_performance_history(result);
CREATE INDEX idx_deck_perf_history_archetype_result
    ON deck_performance_history(archetype, result, format);

CREATE TABLE deck_archetypes (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    set_code TEXT,
    format TEXT NOT NULL,
    color_identity TEXT NOT NULL,
    signature_cards TEXT,
    synergy_patterns TEXT,
    total_matches INTEGER NOT NULL DEFAULT 0,
    total_wins INTEGER NOT NULL DEFAULT 0,
    avg_win_rate REAL,
    source TEXT NOT NULL DEFAULT 'system' CHECK(source IN ('system', '17lands', 'user', 'ml')),
    external_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(name, set_code, format)
);

CREATE INDEX idx_deck_archetypes_set ON deck_archetypes(set_code);
CREATE INDEX idx_deck_archetypes_format ON deck_archetypes(format);
CREATE INDEX idx_deck_archetypes_colors ON deck_archetypes(color_identity);

CREATE TABLE recommendation_feedback (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL,
    recommendation_type TEXT NOT NULL CHECK(recommendation_type IN ('card_pick', 'deck_card', 'archetype', 'sideboard')),
    recommendation_id TEXT NOT NULL,
    recommended_card_id INTEGER,
    recommended_archetype TEXT,
    context_data TEXT NOT NULL,
    action TEXT NOT NULL CHECK(action IN ('accepted', 'rejected', 'ignored', 'alternate')),
    alternate_choice_id INTEGER,
    outcome_match_id TEXT,
    outcome_result TEXT CHECK(outcome_result IN ('win', 'loss')),
    recommendation_score REAL,
    recommendation_rank INTEGER,
    recommended_at TIMESTAMPTZ NOT NULL,
    responded_at TIMESTAMPTZ,
    outcome_recorded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

CREATE INDEX idx_rec_feedback_account ON recommendation_feedback(account_id);
CREATE INDEX idx_rec_feedback_type ON recommendation_feedback(recommendation_type);
CREATE INDEX idx_rec_feedback_action ON recommendation_feedback(action);
CREATE INDEX idx_rec_feedback_timestamp ON recommendation_feedback(recommended_at);
CREATE INDEX idx_rec_feedback_card ON recommendation_feedback(recommended_card_id);
CREATE INDEX idx_rec_feedback_type_action
    ON recommendation_feedback(recommendation_type, action, recommended_at);

CREATE TABLE archetype_card_weights (
    id BIGSERIAL PRIMARY KEY,
    archetype_id BIGINT NOT NULL,
    card_id INTEGER NOT NULL,
    weight REAL NOT NULL DEFAULT 1.0,
    is_signature INTEGER NOT NULL DEFAULT 0,
    source TEXT NOT NULL DEFAULT 'system' CHECK(source IN ('system', '17lands', 'user', 'ml')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (archetype_id) REFERENCES deck_archetypes(id) ON DELETE CASCADE,
    UNIQUE(archetype_id, card_id)
);

CREATE INDEX idx_archetype_card_weights_archetype ON archetype_card_weights(archetype_id);
CREATE INDEX idx_archetype_card_weights_card ON archetype_card_weights(card_id);
