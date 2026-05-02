-- Create draft_card_ratings and draft_color_ratings tables (PostgreSQL)
CREATE TABLE IF NOT EXISTS draft_card_ratings (
    id BIGSERIAL PRIMARY KEY,
    arena_id INTEGER NOT NULL,
    expansion TEXT NOT NULL,
    format TEXT NOT NULL,
    colors TEXT,
    gihwr REAL,
    ohwr REAL,
    gpwr REAL,
    gdwr REAL,
    ihdwr REAL,
    gihwr_delta REAL,
    ohwr_delta REAL,
    gdwr_delta REAL,
    ihdwr_delta REAL,
    alsa REAL,
    ata REAL,
    gih INTEGER,
    oh INTEGER,
    gp INTEGER,
    gd INTEGER,
    ihd INTEGER,
    games_played INTEGER,
    num_decks INTEGER,
    start_date TEXT,
    end_date TEXT,
    cached_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_updated TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(arena_id, expansion, format, colors, start_date, end_date)
);

CREATE INDEX IF NOT EXISTS idx_draft_ratings_arena_id ON draft_card_ratings(arena_id);
CREATE INDEX IF NOT EXISTS idx_draft_ratings_expansion ON draft_card_ratings(expansion);
CREATE INDEX IF NOT EXISTS idx_draft_ratings_format ON draft_card_ratings(expansion, format);
CREATE INDEX IF NOT EXISTS idx_draft_ratings_staleness ON draft_card_ratings(last_updated);

CREATE TABLE IF NOT EXISTS draft_color_ratings (
    id BIGSERIAL PRIMARY KEY,
    expansion TEXT NOT NULL,
    event_type TEXT NOT NULL,
    color_combination TEXT NOT NULL,
    win_rate REAL,
    games_played INTEGER,
    num_decks INTEGER,
    start_date TEXT,
    end_date TEXT,
    cached_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_updated TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(expansion, event_type, color_combination, start_date, end_date)
);

CREATE INDEX IF NOT EXISTS idx_draft_colors_expansion ON draft_color_ratings(expansion);
CREATE INDEX IF NOT EXISTS idx_draft_colors_event ON draft_color_ratings(expansion, event_type);
CREATE INDEX IF NOT EXISTS idx_draft_colors_staleness ON draft_color_ratings(last_updated);
