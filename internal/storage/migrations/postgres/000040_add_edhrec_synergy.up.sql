-- EDHREC synergy data tables (PostgreSQL)

CREATE TABLE IF NOT EXISTS edhrec_synergy (
    id BIGSERIAL PRIMARY KEY,
    card_name TEXT NOT NULL,
    synergy_card_name TEXT NOT NULL,
    synergy_score REAL NOT NULL,
    inclusion_count INTEGER DEFAULT 0,
    num_decks INTEGER DEFAULT 0,
    lift REAL DEFAULT 0.0,
    last_updated TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(card_name, synergy_card_name)
);

CREATE INDEX IF NOT EXISTS idx_edhrec_synergy_card ON edhrec_synergy(card_name);
CREATE INDEX IF NOT EXISTS idx_edhrec_synergy_score ON edhrec_synergy(synergy_score DESC);

CREATE TABLE IF NOT EXISTS edhrec_card_metadata (
    id BIGSERIAL PRIMARY KEY,
    card_name TEXT NOT NULL UNIQUE,
    sanitized_name TEXT NOT NULL,
    num_decks INTEGER DEFAULT 0,
    salt_score REAL DEFAULT 0.0,
    color_identity TEXT,
    last_updated TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_edhrec_metadata_name ON edhrec_card_metadata(card_name);

CREATE TABLE IF NOT EXISTS edhrec_theme_cards (
    id BIGSERIAL PRIMARY KEY,
    theme_name TEXT NOT NULL,
    card_name TEXT NOT NULL,
    synergy_score REAL DEFAULT 0.0,
    is_top_card BOOLEAN DEFAULT FALSE,
    is_high_synergy BOOLEAN DEFAULT FALSE,
    last_updated TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(theme_name, card_name)
);

CREATE INDEX IF NOT EXISTS idx_edhrec_theme_cards_theme ON edhrec_theme_cards(theme_name);
CREATE INDEX IF NOT EXISTS idx_edhrec_theme_cards_card ON edhrec_theme_cards(card_name);
