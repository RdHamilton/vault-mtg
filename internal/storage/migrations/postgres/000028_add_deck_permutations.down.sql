-- Rollback: Remove deck permutation tracking

DROP INDEX IF EXISTS idx_decks_current_permutation;
ALTER TABLE decks DROP COLUMN IF EXISTS current_permutation_id;
DROP TABLE IF EXISTS deck_permutations;
