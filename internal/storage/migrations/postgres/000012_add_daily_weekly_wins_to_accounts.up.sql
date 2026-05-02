-- Add daily and weekly wins tracking to accounts table (PostgreSQL)
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS daily_wins INTEGER DEFAULT 0;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS weekly_wins INTEGER DEFAULT 0;
