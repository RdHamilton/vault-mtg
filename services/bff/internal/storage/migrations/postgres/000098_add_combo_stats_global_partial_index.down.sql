-- Rollback migration 000098: remove partial unique index for global
-- card_combination_stats rows.
DROP INDEX IF EXISTS idx_combo_stats_global;
