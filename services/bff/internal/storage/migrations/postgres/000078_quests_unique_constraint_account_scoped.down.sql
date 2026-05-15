-- Rollback migration 000078: restore the original non-account-scoped UNIQUE constraint.
ALTER TABLE quests DROP CONSTRAINT IF EXISTS quests_account_id_quest_id_assigned_at_key;
ALTER TABLE quests
    ADD CONSTRAINT quests_quest_id_assigned_at_key
    UNIQUE (quest_id, assigned_at);
