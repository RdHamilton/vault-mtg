-- Rollback migration 000095: restore matches.result CHECK to ('win', 'loss').
--
-- WARNING: any rows with result='unknown' will block this rollback.
-- Remove or update them before rolling back:
--   DELETE FROM matches WHERE result = 'unknown';

ALTER TABLE matches
    DROP CONSTRAINT matches_result_check,
    ADD CONSTRAINT matches_result_check
        CHECK (result IN ('win', 'loss'));
