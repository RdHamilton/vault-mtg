-- Rollback migration 000087: re-enable AED in draft-active sync.
--
-- NOTE: This rollback restores is_draft_active = TRUE for AED. However, the
-- sync Lambda will immediately query 17Lands with code 'AED', which returns
-- an empty array. The circuit-breaker will trip again after 3 consecutive
-- invocations unless migration 000088 (seventeenlands_code = 'DFT') is also
-- rolled back or the skip guard is manually cleared.
--
-- In practice, do not run this rollback in isolation without also coordinating
-- the PR2 migration (000088). The intended rollback path is: roll back 000088
-- first, then roll back this migration only if needed.
--
-- The sync_hashes row is NOT restored here — a zero/absent skip count is the
-- correct starting state regardless.

UPDATE sets
SET    is_draft_active = TRUE
WHERE  code = 'AED';
