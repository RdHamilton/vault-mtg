-- Rollback: Remove Standard legality tracking

DROP TABLE IF EXISTS standard_config CASCADE;
DROP INDEX IF EXISTS idx_sets_standard;
DROP INDEX IF EXISTS idx_set_cards_legalities;
ALTER TABLE set_cards DROP COLUMN IF EXISTS legalities;
ALTER TABLE sets DROP COLUMN IF EXISTS rotation_date;
ALTER TABLE sets DROP COLUMN IF EXISTS is_standard_legal;
