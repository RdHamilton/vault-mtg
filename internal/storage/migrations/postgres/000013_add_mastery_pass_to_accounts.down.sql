-- Remove mastery pass columns from accounts table
ALTER TABLE accounts DROP COLUMN IF EXISTS mastery_level;
ALTER TABLE accounts DROP COLUMN IF EXISTS mastery_pass;
ALTER TABLE accounts DROP COLUMN IF EXISTS mastery_max;
