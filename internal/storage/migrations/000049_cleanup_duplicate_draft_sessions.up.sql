-- Migration: Clean up duplicate draft sessions created by log re-processing bug.
-- Duplicate sessions have a UnixNano timestamp suffix appended to the base session ID
-- (e.g., "QuickDraft_ECL_20260223_1772406070596784000" is a duplicate of "QuickDraft_ECL_20260223").
-- ON DELETE CASCADE on draft_picks and draft_packs foreign keys ensures associated rows
-- are automatically cleaned up when the duplicate session is deleted.

DELETE FROM draft_sessions
WHERE id IN (
    SELECT dup.id
    FROM draft_sessions dup
    INNER JOIN draft_sessions base
        ON dup.id LIKE base.id || '_%'
        AND LENGTH(dup.id) > LENGTH(base.id) + 1
        -- Ensure the suffix (after base_id + '_') is purely numeric (a UnixNano timestamp)
        AND TRIM(SUBSTR(dup.id, LENGTH(base.id) + 2), '0123456789') = ''
);
