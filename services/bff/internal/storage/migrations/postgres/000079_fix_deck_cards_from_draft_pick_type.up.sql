-- Fix from_draft_pick column type: INTEGER → BOOLEAN (PostgreSQL)
-- The column was originally added as INTEGER CHECK(IN (0, 1)) in migration 000022.
-- The 000054 initial-schema snapshot defined it as BOOLEAN, but IF NOT EXISTS
-- prevented the table from being recreated on incrementally-migrated databases.
-- This migration normalises the type on all existing databases.

-- Drop the INTEGER default before changing type; PostgreSQL cannot auto-cast DEFAULT 0 to boolean.
ALTER TABLE deck_cards ALTER COLUMN from_draft_pick DROP DEFAULT;
ALTER TABLE deck_cards ALTER COLUMN from_draft_pick TYPE BOOLEAN USING (from_draft_pick::boolean);
ALTER TABLE deck_cards ALTER COLUMN from_draft_pick SET DEFAULT false;
