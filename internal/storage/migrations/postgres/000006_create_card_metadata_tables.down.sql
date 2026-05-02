-- Rollback card metadata tables

DROP INDEX IF EXISTS idx_sets_released_at;
DROP INDEX IF EXISTS idx_cards_last_updated;
DROP INDEX IF EXISTS idx_cards_set;
DROP INDEX IF EXISTS idx_cards_name;
DROP INDEX IF EXISTS idx_cards_arena_id;

DROP TABLE IF EXISTS sets;
DROP TABLE IF EXISTS cards;
