-- Migration 000096: replace the 3-column quests unique constraint with a 2-column one.
--
-- Problem (issue #204): the upsert conflict key is (account_id, quest_id, assigned_at).
-- The BFF sets assigned_at from the daemon's seen_at timestamp, which changes on every
-- sync event.  As a result, ON CONFLICT never fires and each sync cycle inserts a fresh
-- row — producing duplicate quest entries in the Quest History view.
--
-- Fix: drop the 3-col constraint (added in migration 000078) and replace it with
--      UNIQUE(account_id, quest_id).  The ON CONFLICT clause in quest_repo.go is
--      updated in the same PR to target the new constraint.
--
-- Dedup step: before adding the new constraint, remove the duplicate rows that
-- accumulated due to the bug.  We keep the row with the latest last_seen_at
-- (breaking ties by largest id) as the canonical record per (account_id, quest_id).

DO $$
DECLARE
    dup_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO dup_count
    FROM (
        SELECT account_id, quest_id, COUNT(*) AS cnt
        FROM quests
        GROUP BY account_id, quest_id
        HAVING COUNT(*) > 1
    ) t;

    RAISE NOTICE 'quests: % duplicate (account_id, quest_id) pairs found before dedup', dup_count;
END;
$$;

-- Delete duplicate rows, keeping the survivor with the most recent last_seen_at
-- (or, when last_seen_at is NULL, largest id).
DELETE FROM quests
WHERE id IN (
    SELECT id
    FROM (
        SELECT
            id,
            ROW_NUMBER() OVER (
                PARTITION BY account_id, quest_id
                ORDER BY last_seen_at DESC NULLS LAST, id DESC
            ) AS rn
        FROM quests
    ) ranked
    WHERE rn > 1
);

-- Drop the 3-column constraint from migration 000078.
ALTER TABLE quests
    DROP CONSTRAINT IF EXISTS quests_account_id_quest_id_assigned_at_key;

-- Add the new 2-column constraint.
ALTER TABLE quests
    ADD CONSTRAINT quests_account_id_quest_id_key
    UNIQUE (account_id, quest_id);
