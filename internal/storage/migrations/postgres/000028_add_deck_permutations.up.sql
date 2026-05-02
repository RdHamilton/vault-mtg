-- Migration: Add deck permutation tracking (PostgreSQL)

CREATE TABLE deck_permutations (
    id BIGSERIAL PRIMARY KEY,
    deck_id TEXT NOT NULL,
    parent_permutation_id BIGINT,
    cards TEXT NOT NULL,
    card_hash TEXT NOT NULL,
    version_number INTEGER NOT NULL DEFAULT 1,
    version_name TEXT,
    change_summary TEXT,
    matches_played INTEGER NOT NULL DEFAULT 0,
    matches_won INTEGER NOT NULL DEFAULT 0,
    games_played INTEGER NOT NULL DEFAULT 0,
    games_won INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_played_at TIMESTAMPTZ,
    FOREIGN KEY (deck_id) REFERENCES decks(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_permutation_id) REFERENCES deck_permutations(id) ON DELETE SET NULL
);

CREATE INDEX idx_deck_permutations_deck_id ON deck_permutations(deck_id);
CREATE INDEX idx_deck_permutations_parent ON deck_permutations(parent_permutation_id);
CREATE INDEX idx_deck_permutations_created ON deck_permutations(deck_id, created_at DESC);
CREATE INDEX idx_deck_permutations_win_rate ON deck_permutations(deck_id, matches_won, matches_played);
CREATE UNIQUE INDEX idx_deck_permutations_hash ON deck_permutations(deck_id, card_hash);
CREATE INDEX idx_deck_permutations_version ON deck_permutations(deck_id, version_number);

ALTER TABLE decks ADD COLUMN IF NOT EXISTS current_permutation_id BIGINT REFERENCES deck_permutations(id);
CREATE INDEX idx_decks_current_permutation ON decks(current_permutation_id);

-- Create initial permutations for existing decks
INSERT INTO deck_permutations (deck_id, cards, card_hash, version_number, matches_played, matches_won, games_played, games_won, created_at, last_played_at)
SELECT
    d.id,
    COALESCE(
        (SELECT json_agg(json_build_object('card_id', dc.card_id, 'quantity', dc.quantity, 'board', dc.board) ORDER BY dc.card_id, dc.board)::TEXT
         FROM deck_cards dc WHERE dc.deck_id = d.id),
        '[]'
    ),
    COALESCE(
        (SELECT string_agg(dc.card_id::TEXT || ':' || dc.quantity::TEXT || ':' || dc.board, '|' ORDER BY dc.card_id, dc.board)
         FROM deck_cards dc WHERE dc.deck_id = d.id),
        ''
    ),
    1,
    d.matches_played,
    d.matches_won,
    d.games_played,
    d.games_won,
    d.created_at,
    d.last_played
FROM decks d;

UPDATE decks
SET current_permutation_id = (
    SELECT id FROM deck_permutations
    WHERE deck_permutations.deck_id = decks.id
    ORDER BY created_at ASC
    LIMIT 1
);
