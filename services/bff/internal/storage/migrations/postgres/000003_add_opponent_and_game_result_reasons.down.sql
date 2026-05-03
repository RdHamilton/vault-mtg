-- Rollback opponent tracking and game-level result reasons

DROP INDEX IF EXISTS idx_games_result_reason;
ALTER TABLE games DROP COLUMN IF EXISTS result_reason;

DROP INDEX IF EXISTS idx_matches_opponent_id;
DROP INDEX IF EXISTS idx_matches_opponent_name;
ALTER TABLE matches DROP COLUMN IF EXISTS opponent_id;
ALTER TABLE matches DROP COLUMN IF EXISTS opponent_name;
