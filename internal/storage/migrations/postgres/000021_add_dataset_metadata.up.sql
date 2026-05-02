-- Create dataset_metadata table and add data_source columns (PostgreSQL)
CREATE TABLE IF NOT EXISTS dataset_metadata (
    id BIGSERIAL PRIMARY KEY,
    set_code TEXT NOT NULL,
    draft_format TEXT NOT NULL,
    data_source TEXT NOT NULL,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    total_cards INTEGER,
    total_games INTEGER,
    dataset_version TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(set_code, draft_format)
);

CREATE INDEX idx_dataset_metadata_set ON dataset_metadata(set_code, draft_format);
CREATE INDEX idx_dataset_metadata_updated ON dataset_metadata(last_updated_at DESC);
CREATE INDEX idx_dataset_metadata_source ON dataset_metadata(data_source);

ALTER TABLE draft_card_ratings ADD COLUMN IF NOT EXISTS data_source TEXT DEFAULT 'api';
ALTER TABLE draft_color_ratings ADD COLUMN IF NOT EXISTS data_source TEXT DEFAULT 'api';
