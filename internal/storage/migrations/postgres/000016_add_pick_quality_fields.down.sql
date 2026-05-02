-- Remove pick quality analysis fields from draft_picks table
DROP INDEX IF EXISTS idx_draft_picks_quality_grade;
ALTER TABLE draft_picks DROP COLUMN IF EXISTS pick_quality_grade;
ALTER TABLE draft_picks DROP COLUMN IF EXISTS pick_quality_rank;
ALTER TABLE draft_picks DROP COLUMN IF EXISTS pack_best_gihwr;
ALTER TABLE draft_picks DROP COLUMN IF EXISTS picked_card_gihwr;
ALTER TABLE draft_picks DROP COLUMN IF EXISTS alternatives_json;
