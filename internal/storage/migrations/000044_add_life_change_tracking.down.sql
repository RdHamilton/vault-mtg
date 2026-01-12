-- Rollback migration: Remove life change tracking from game_plays
-- Note: SQLite doesn't support DROP COLUMN directly, need to recreate table

-- Create temporary table without the new columns
CREATE TABLE game_plays_backup AS
SELECT id, game_id, match_id, turn_number, phase, step, player_type, action_type,
       card_id, card_name, zone_from, zone_to, timestamp, sequence_number, created_at
FROM game_plays;

-- Drop original table
DROP TABLE game_plays;

-- Recreate table without new columns
CREATE TABLE game_plays (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id INTEGER NOT NULL,
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
    timestamp TIMESTAMP NOT NULL,
    sequence_number INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE
);

-- Copy data back
INSERT INTO game_plays SELECT * FROM game_plays_backup;

-- Drop backup table
DROP TABLE game_plays_backup;

-- Recreate indexes
CREATE INDEX idx_game_plays_game_id ON game_plays(game_id);
CREATE INDEX idx_game_plays_match_id ON game_plays(match_id);
CREATE INDEX idx_game_plays_turn ON game_plays(game_id, turn_number);
