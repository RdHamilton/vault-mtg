-- Drop improvement suggestions table
DROP INDEX IF EXISTS idx_suggestions_dismissed;
DROP INDEX IF EXISTS idx_suggestions_type;
DROP INDEX IF EXISTS idx_suggestions_deck_id;
DROP TABLE IF EXISTS improvement_suggestions;

-- Drop deck notes table
DROP INDEX IF EXISTS idx_deck_notes_category;
DROP INDEX IF EXISTS idx_deck_notes_deck_id;
DROP TABLE IF EXISTS deck_notes;

-- Remove notes and rating columns from matches table
ALTER TABLE matches DROP COLUMN IF EXISTS rating;
ALTER TABLE matches DROP COLUMN IF EXISTS notes;
