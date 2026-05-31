-- Migration 000098: add partial unique index on card_combination_stats for
-- NULL deck_id rows (global aggregated stats).
--
-- The existing UNIQUE(card_id_1, card_id_2, deck_id, format) constraint does
-- NOT deduplicate rows where deck_id IS NULL because PostgreSQL treats NULL as
-- distinct from every other value including another NULL. Without this index,
-- the ON CONFLICT upsert in ComputeAndWritePairStats would insert duplicate
-- global rows on every call instead of updating them.
--
-- This partial index targets exactly the (card_id_1, card_id_2, format) tuple
-- when deck_id IS NULL, enabling the ON CONFLICT ... WHERE deck_id IS NULL
-- upsert clause in the writer.
CREATE UNIQUE INDEX IF NOT EXISTS idx_combo_stats_global
    ON card_combination_stats (card_id_1, card_id_2, format)
    WHERE deck_id IS NULL;
