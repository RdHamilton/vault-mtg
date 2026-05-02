-- Remove draft grade indexes and columns
DROP INDEX IF EXISTS idx_draft_sessions_score;
DROP INDEX IF EXISTS idx_draft_sessions_grade;
ALTER TABLE draft_sessions DROP COLUMN IF EXISTS overall_grade;
ALTER TABLE draft_sessions DROP COLUMN IF EXISTS overall_score;
ALTER TABLE draft_sessions DROP COLUMN IF EXISTS pick_quality_score;
ALTER TABLE draft_sessions DROP COLUMN IF EXISTS color_discipline_score;
ALTER TABLE draft_sessions DROP COLUMN IF EXISTS deck_composition_score;
ALTER TABLE draft_sessions DROP COLUMN IF EXISTS strategic_score;
