-- Migration 000077: add account_id to quests table.
--
-- The quests table was created in 000010 without account_id.
-- Projection worker and read queries both need a BIGINT FK to accounts(id)
-- so that quest rows are properly scoped per-account.
--
-- The column is nullable to avoid breaking existing rows; the projection
-- worker will populate it for all new quest writes going forward.

ALTER TABLE quests
    ADD COLUMN IF NOT EXISTS account_id BIGINT REFERENCES accounts(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_quests_account_id ON quests(account_id);
