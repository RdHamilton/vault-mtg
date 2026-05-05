-- Add is_draft_active to sets to distinguish "currently draftable on Arena"
-- from "standard-legal".  A set may be draftable (masters, alchemy, etc.) without
-- being standard-legal, and a rotated set may still appear in chaos/special drafts.
ALTER TABLE sets ADD COLUMN IF NOT EXISTS is_draft_active BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_sets_draft_active ON sets(is_draft_active);

-- Backfill: any set that is currently standard-legal is also draft-active.
UPDATE sets SET is_draft_active = TRUE WHERE is_standard_legal = TRUE;


-- Grant the sync role write access to the new column (column-level GRANTs require
-- table-level privilege in PostgreSQL, which was already granted in migration 000059).
-- No additional GRANT is needed; existing INSERT/UPDATE on sets covers is_draft_active.
