-- Add pick quality analysis fields to draft_picks table (PostgreSQL)
ALTER TABLE draft_picks ADD COLUMN IF NOT EXISTS pick_quality_grade TEXT;
ALTER TABLE draft_picks ADD COLUMN IF NOT EXISTS pick_quality_rank INTEGER;
ALTER TABLE draft_picks ADD COLUMN IF NOT EXISTS pack_best_gihwr REAL;
ALTER TABLE draft_picks ADD COLUMN IF NOT EXISTS picked_card_gihwr REAL;
ALTER TABLE draft_picks ADD COLUMN IF NOT EXISTS alternatives_json TEXT;

CREATE INDEX idx_draft_picks_quality_grade ON draft_picks(pick_quality_grade);
