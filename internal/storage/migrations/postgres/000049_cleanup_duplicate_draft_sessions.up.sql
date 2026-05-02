-- Migration: Clean up duplicate draft sessions (PostgreSQL)
-- Duplicate sessions have a UnixNano timestamp suffix appended to the base session ID.
-- ON DELETE CASCADE on draft_picks and draft_packs ensures associated rows are cleaned up.

DELETE FROM draft_sessions
WHERE id IN (
    SELECT dup.id
    FROM draft_sessions dup
    INNER JOIN draft_sessions base
        ON dup.id LIKE base.id || '_%'
        AND LENGTH(dup.id) > LENGTH(base.id) + 1
        -- Ensure the suffix is purely numeric (a UnixNano timestamp)
        AND SUBSTRING(dup.id FROM LENGTH(base.id) + 2) ~ '^[0-9]+$'
);
