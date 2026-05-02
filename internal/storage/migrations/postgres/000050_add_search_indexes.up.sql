-- Add index on set_cards.name for ORDER BY performance (PostgreSQL)
CREATE INDEX IF NOT EXISTS idx_set_cards_name ON set_cards(name);
