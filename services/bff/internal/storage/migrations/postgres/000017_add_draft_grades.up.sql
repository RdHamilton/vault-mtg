-- Add draft grade fields to draft_sessions table (PostgreSQL)
ALTER TABLE draft_sessions ADD COLUMN IF NOT EXISTS overall_grade TEXT;
ALTER TABLE draft_sessions ADD COLUMN IF NOT EXISTS overall_score INTEGER;
ALTER TABLE draft_sessions ADD COLUMN IF NOT EXISTS pick_quality_score REAL;
ALTER TABLE draft_sessions ADD COLUMN IF NOT EXISTS color_discipline_score REAL;
ALTER TABLE draft_sessions ADD COLUMN IF NOT EXISTS deck_composition_score REAL;
ALTER TABLE draft_sessions ADD COLUMN IF NOT EXISTS strategic_score REAL;

CREATE INDEX idx_draft_sessions_grade ON draft_sessions(overall_grade);
CREATE INDEX idx_draft_sessions_score ON draft_sessions(overall_score DESC);
