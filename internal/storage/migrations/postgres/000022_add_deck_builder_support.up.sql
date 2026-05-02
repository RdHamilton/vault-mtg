-- Add v1.3 Deck Builder support to decks table (PostgreSQL)

ALTER TABLE decks ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'constructed'
    CHECK(source IN ('draft', 'constructed', 'imported'));
ALTER TABLE decks ADD COLUMN IF NOT EXISTS draft_event_id TEXT
    REFERENCES draft_events(id) ON DELETE SET NULL;
ALTER TABLE decks ADD COLUMN IF NOT EXISTS matches_played INTEGER NOT NULL DEFAULT 0;
ALTER TABLE decks ADD COLUMN IF NOT EXISTS matches_won INTEGER NOT NULL DEFAULT 0;
ALTER TABLE decks ADD COLUMN IF NOT EXISTS games_played INTEGER NOT NULL DEFAULT 0;
ALTER TABLE decks ADD COLUMN IF NOT EXISTS games_won INTEGER NOT NULL DEFAULT 0;

ALTER TABLE deck_cards ADD COLUMN IF NOT EXISTS from_draft_pick INTEGER NOT NULL DEFAULT 0
    CHECK(from_draft_pick IN (0, 1));

CREATE INDEX IF NOT EXISTS idx_decks_source ON decks(source);
CREATE INDEX IF NOT EXISTS idx_decks_draft_event_id ON decks(draft_event_id);

CREATE TABLE IF NOT EXISTS deck_tags (
    id BIGSERIAL PRIMARY KEY,
    deck_id TEXT NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(deck_id, tag)
);

CREATE INDEX IF NOT EXISTS idx_deck_tags_deck_id ON deck_tags(deck_id);
CREATE INDEX IF NOT EXISTS idx_deck_tags_tag ON deck_tags(tag);
