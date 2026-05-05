DROP INDEX IF EXISTS idx_sets_draft_active;
ALTER TABLE sets DROP COLUMN IF EXISTS is_draft_active;
