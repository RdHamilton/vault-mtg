-- Rollback for 000081: drop the composite (arena_id, id) index on set_cards.

DROP INDEX IF EXISTS idx_set_cards_arena_id_id;
