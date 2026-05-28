-- Rollback migration 000088: remove seventeenlands_code column and disable AED again.
--
-- Rolling back this migration restores the state from migration 000087:
-- AED is disabled (is_draft_active = FALSE) and the seventeenlands_code column
-- is dropped. The sync Lambda will no longer use a separate expansion code
-- for any set.
--
-- NOTE: Do not run this rollback independently without also rolling back any
-- in-flight syncs that may have used the DFT code. The sync_hashes table may
-- contain rows written under the AED Scryfall key — those are unaffected.

UPDATE sets
SET    is_draft_active = FALSE
WHERE  code = 'AED';

ALTER TABLE sets
    DROP COLUMN IF EXISTS seventeenlands_code;
