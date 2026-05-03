-- Drop old draft ratings tables from migration 000007 and recreate (PostgreSQL)
DROP TABLE IF EXISTS draft_card_ratings;
DROP TABLE IF EXISTS draft_color_ratings;

CREATE TABLE IF NOT EXISTS draft_card_ratings (
    id BIGSERIAL PRIMARY KEY,
    set_code TEXT NOT NULL,
    draft_format TEXT NOT NULL,
    arena_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    color TEXT,
    rarity TEXT,
    gihwr REAL,
    ohwr REAL,
    alsa REAL,
    ata REAL,
    gih_count INTEGER,
    cached_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(set_code, draft_format, arena_id)
);

CREATE INDEX idx_draft_card_ratings_set ON draft_card_ratings(set_code, draft_format);
CREATE INDEX idx_draft_card_ratings_arena_id ON draft_card_ratings(arena_id);
CREATE INDEX idx_draft_card_ratings_gihwr ON draft_card_ratings(gihwr DESC);

CREATE TABLE IF NOT EXISTS draft_color_ratings (
    id BIGSERIAL PRIMARY KEY,
    set_code TEXT NOT NULL,
    draft_format TEXT NOT NULL,
    color_combination TEXT NOT NULL,
    win_rate REAL,
    games_played INTEGER,
    cached_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(set_code, draft_format, color_combination)
);

CREATE INDEX idx_draft_color_ratings_set ON draft_color_ratings(set_code, draft_format);
CREATE INDEX idx_draft_color_ratings_win_rate ON draft_color_ratings(win_rate DESC);
