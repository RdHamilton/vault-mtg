-- Add mastery pass tracking to accounts table (PostgreSQL)
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS mastery_level INTEGER DEFAULT 0;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS mastery_pass TEXT DEFAULT 'Basic';
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS mastery_max INTEGER DEFAULT 80;
