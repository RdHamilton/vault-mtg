-- Migration: Add in-game play tracking (PostgreSQL)

CREATE TABLE IF NOT EXISTS game_plays (
    id BIGSERIAL PRIMARY KEY,
    game_id BIGINT NOT NULL,
    match_id TEXT NOT NULL,
    turn_number INTEGER NOT NULL,
    phase TEXT,
    step TEXT,
    player_type TEXT NOT NULL,
    action_type TEXT NOT NULL,
    card_id INTEGER,
    card_name TEXT,
    zone_from TEXT,
    zone_to TEXT,
    timestamp TIMESTAMPTZ NOT NULL,
    sequence_number INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS game_state_snapshots (
    id BIGSERIAL PRIMARY KEY,
    game_id BIGINT NOT NULL,
    match_id TEXT NOT NULL,
    turn_number INTEGER NOT NULL,
    active_player TEXT NOT NULL,
    player_life INTEGER,
    opponent_life INTEGER,
    player_cards_in_hand INTEGER,
    opponent_cards_in_hand INTEGER,
    player_lands_in_play INTEGER,
    opponent_lands_in_play INTEGER,
    board_state_json TEXT,
    timestamp TIMESTAMPTZ NOT NULL,
    FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE,
    UNIQUE(game_id, turn_number)
);

CREATE TABLE IF NOT EXISTS opponent_cards_observed (
    id BIGSERIAL PRIMARY KEY,
    game_id BIGINT NOT NULL,
    match_id TEXT NOT NULL,
    card_id INTEGER NOT NULL,
    card_name TEXT,
    zone_observed TEXT,
    turn_first_seen INTEGER,
    times_seen INTEGER DEFAULT 1,
    FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE,
    UNIQUE(game_id, card_id)
);

CREATE INDEX idx_game_plays_game_id ON game_plays(game_id);
CREATE INDEX idx_game_plays_match_id ON game_plays(match_id);
CREATE INDEX idx_game_plays_turn ON game_plays(game_id, turn_number);
CREATE INDEX idx_game_snapshots_game_id ON game_state_snapshots(game_id);
CREATE INDEX idx_game_snapshots_match_id ON game_state_snapshots(match_id);
CREATE INDEX idx_opponent_cards_game_id ON opponent_cards_observed(game_id);
CREATE INDEX idx_opponent_cards_match_id ON opponent_cards_observed(match_id);
