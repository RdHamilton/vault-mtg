-- Rollback migration 000096: restore the 3-column unique constraint.
--
-- NOTE: the dedup DELETE performed in the .up.sql is NOT reversible — rows
-- that were deleted as duplicates cannot be restored by this rollback.
-- This rollback only restores the constraint shape, not any lost data.

ALTER TABLE quests
    DROP CONSTRAINT IF EXISTS quests_account_id_quest_id_key;

ALTER TABLE quests
    ADD CONSTRAINT quests_account_id_quest_id_assigned_at_key
    UNIQUE (account_id, quest_id, assigned_at);
