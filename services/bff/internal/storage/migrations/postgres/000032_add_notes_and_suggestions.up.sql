-- Add notes/rating to matches, deck notes, and improvement suggestions (PostgreSQL)
ALTER TABLE matches ADD COLUMN IF NOT EXISTS notes TEXT DEFAULT '';
ALTER TABLE matches ADD COLUMN IF NOT EXISTS rating INTEGER DEFAULT 0;

CREATE TABLE IF NOT EXISTS deck_notes (
    id BIGSERIAL PRIMARY KEY,
    deck_id TEXT NOT NULL,
    content TEXT NOT NULL,
    category TEXT DEFAULT 'general',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (deck_id) REFERENCES decks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_deck_notes_deck_id ON deck_notes(deck_id);
CREATE INDEX IF NOT EXISTS idx_deck_notes_category ON deck_notes(category);

CREATE TABLE IF NOT EXISTS improvement_suggestions (
    id BIGSERIAL PRIMARY KEY,
    deck_id TEXT NOT NULL,
    suggestion_type TEXT NOT NULL,
    priority TEXT DEFAULT 'medium',
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    evidence TEXT,
    card_references TEXT,
    is_dismissed BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (deck_id) REFERENCES decks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_suggestions_deck_id ON improvement_suggestions(deck_id);
CREATE INDEX IF NOT EXISTS idx_suggestions_type ON improvement_suggestions(suggestion_type);
CREATE INDEX IF NOT EXISTS idx_suggestions_dismissed ON improvement_suggestions(is_dismissed);
