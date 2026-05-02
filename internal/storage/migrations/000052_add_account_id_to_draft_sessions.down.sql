DROP INDEX IF EXISTS idx_draft_sessions_account_id;

ALTER TABLE draft_sessions DROP COLUMN account_id;
