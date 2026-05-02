-- Remove daily and weekly wins columns from accounts table
ALTER TABLE accounts DROP COLUMN IF EXISTS daily_wins;
ALTER TABLE accounts DROP COLUMN IF EXISTS weekly_wins;
