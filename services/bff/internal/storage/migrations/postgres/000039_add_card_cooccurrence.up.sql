-- Card co-occurrence table (PostgreSQL)

CREATE TABLE IF NOT EXISTS card_cooccurrence (
    id BIGSERIAL PRIMARY KEY,
    card_a_arena_id INTEGER NOT NULL,
    card_b_arena_id INTEGER NOT NULL,
    format TEXT NOT NULL DEFAULT 'all',
    count INTEGER NOT NULL DEFAULT 0,
    pmi_score REAL DEFAULT 0.0,
    last_updated TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(card_a_arena_id, card_b_arena_id, format)
);

CREATE INDEX IF NOT EXISTS idx_cooccurrence_card_a ON card_cooccurrence(card_a_arena_id, format);
CREATE INDEX IF NOT EXISTS idx_cooccurrence_card_b ON card_cooccurrence(card_b_arena_id, format);
CREATE INDEX IF NOT EXISTS idx_cooccurrence_format ON card_cooccurrence(format);
CREATE INDEX IF NOT EXISTS idx_cooccurrence_pmi ON card_cooccurrence(pmi_score DESC);

CREATE TABLE IF NOT EXISTS cooccurrence_sources (
    id BIGSERIAL PRIMARY KEY,
    source_type TEXT NOT NULL,
    source_id TEXT NOT NULL,
    format TEXT NOT NULL DEFAULT 'all',
    deck_count INTEGER NOT NULL DEFAULT 0,
    card_count INTEGER NOT NULL DEFAULT 0,
    last_synced TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(source_type, source_id, format)
);

-- Card frequency table for PMI calculation
CREATE TABLE IF NOT EXISTS card_frequency (
    id BIGSERIAL PRIMARY KEY,
    card_arena_id INTEGER NOT NULL,
    format TEXT NOT NULL DEFAULT 'all',
    deck_count INTEGER NOT NULL DEFAULT 0,
    total_decks INTEGER NOT NULL DEFAULT 0,
    frequency REAL GENERATED ALWAYS AS (
        CAST(deck_count AS REAL) / NULLIF(total_decks, 0)
    ) STORED,
    last_updated TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(card_arena_id, format)
);

CREATE INDEX IF NOT EXISTS idx_frequency_card ON card_frequency(card_arena_id, format);
CREATE INDEX IF NOT EXISTS idx_frequency_format ON card_frequency(format);
