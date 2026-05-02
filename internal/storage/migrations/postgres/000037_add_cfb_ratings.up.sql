-- ChannelFireball card ratings table (PostgreSQL)
CREATE TABLE IF NOT EXISTS cfb_ratings (
    id BIGSERIAL PRIMARY KEY,
    card_name TEXT NOT NULL,
    set_code TEXT NOT NULL,
    arena_id INTEGER,
    limited_rating TEXT,
    limited_score REAL DEFAULT 0.0,
    constructed_rating TEXT,
    constructed_score REAL DEFAULT 0.0,
    archetype_fit TEXT,
    commentary TEXT,
    source_url TEXT,
    author TEXT,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(card_name, set_code)
);

CREATE INDEX IF NOT EXISTS idx_cfb_ratings_set_code ON cfb_ratings(set_code);
CREATE INDEX IF NOT EXISTS idx_cfb_ratings_arena_id ON cfb_ratings(arena_id);
CREATE INDEX IF NOT EXISTS idx_cfb_ratings_card_name ON cfb_ratings(card_name);
