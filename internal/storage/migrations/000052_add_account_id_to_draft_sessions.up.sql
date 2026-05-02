-- Add account_id to draft_sessions to scope sessions per MTGA account.
-- Nullable initially; existing rows backfilled to the default account.
ALTER TABLE draft_sessions ADD COLUMN account_id INTEGER REFERENCES accounts(id) ON DELETE CASCADE;

UPDATE draft_sessions
SET account_id = (SELECT id FROM accounts WHERE is_default = 1 LIMIT 1)
WHERE account_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_draft_sessions_account_id ON draft_sessions(account_id);
