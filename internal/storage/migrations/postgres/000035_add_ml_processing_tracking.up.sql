-- Add ML processing tracking to matches table (PostgreSQL)
ALTER TABLE matches ADD COLUMN IF NOT EXISTS processed_for_ml BOOLEAN DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_matches_processed_for_ml ON matches(processed_for_ml) WHERE processed_for_ml = FALSE;

CREATE TABLE IF NOT EXISTS card_individual_stats (
    card_id INTEGER NOT NULL,
    format TEXT NOT NULL,
    total_games INTEGER DEFAULT 0,
    wins INTEGER DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (card_id, format)
);

CREATE INDEX IF NOT EXISTS idx_card_individual_stats_format ON card_individual_stats(format);
