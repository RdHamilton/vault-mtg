-- Migration 000077: convert quests.account_id from TEXT → BIGINT FK.
--
-- Migration 000068 added account_id TEXT NOT NULL DEFAULT '' to the quests
-- table as a placeholder.  The projection worker never populated it correctly
-- (it wrote TEXT client_id strings, which can't be resolved back to accounts),
-- so all existing rows have account_id = ''.  Those rows are orphaned and
-- not recoverable; we drop the column and re-add it as BIGINT NULLABLE with
-- a proper FK to accounts(id).
--
-- Note: DROP COLUMN automatically cascades to any indexes on the column,
-- so we do not issue a separate DROP INDEX (the migration user may not own
-- the index that was created by a superuser in migration 000068).

ALTER TABLE quests DROP COLUMN IF EXISTS account_id;
ALTER TABLE quests
    ADD COLUMN account_id BIGINT REFERENCES accounts(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_quests_account_id ON quests(account_id);
