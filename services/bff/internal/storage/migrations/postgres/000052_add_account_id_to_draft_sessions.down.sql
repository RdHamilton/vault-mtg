DROP INDEX IF EXISTS idx_draft_sessions_account_id;

ALTER TABLE draft_sessions DROP COLUMN IF EXISTS account_id;
