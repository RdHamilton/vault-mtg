-- Track decks created within the app for ML training (PostgreSQL)
ALTER TABLE decks ADD COLUMN IF NOT EXISTS is_app_created BOOLEAN DEFAULT FALSE;
ALTER TABLE decks ADD COLUMN IF NOT EXISTS created_method TEXT DEFAULT 'imported';
ALTER TABLE decks ADD COLUMN IF NOT EXISTS seed_card_id INTEGER;

CREATE INDEX IF NOT EXISTS idx_decks_app_created ON decks(is_app_created) WHERE is_app_created = TRUE;
CREATE INDEX IF NOT EXISTS idx_decks_created_method ON decks(created_method);
