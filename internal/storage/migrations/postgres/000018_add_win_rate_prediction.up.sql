-- Add win rate prediction fields to draft_sessions table (PostgreSQL)
ALTER TABLE draft_sessions ADD COLUMN IF NOT EXISTS predicted_win_rate REAL;
ALTER TABLE draft_sessions ADD COLUMN IF NOT EXISTS predicted_win_rate_min REAL;
ALTER TABLE draft_sessions ADD COLUMN IF NOT EXISTS predicted_win_rate_max REAL;
ALTER TABLE draft_sessions ADD COLUMN IF NOT EXISTS prediction_factors TEXT;
ALTER TABLE draft_sessions ADD COLUMN IF NOT EXISTS predicted_at TIMESTAMPTZ;

CREATE INDEX idx_draft_sessions_predicted_win_rate ON draft_sessions(predicted_win_rate) WHERE predicted_win_rate IS NOT NULL;
