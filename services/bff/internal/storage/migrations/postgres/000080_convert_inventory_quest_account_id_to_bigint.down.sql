-- Migration 000080 rollback: revert inventory.account_id and
-- quest_session_tracking.account_id from BIGINT FK back to TEXT client_id.
--
-- This is a best-effort rollback; data written after the up migration that
-- cannot be mapped back to a TEXT client_id (e.g. accounts with no client_id)
-- will be deleted.

-- ── inventory ────────────────────────────────────────────────────────────────

ALTER TABLE inventory DROP CONSTRAINT IF EXISTS fk_inventory_account_id;
DROP INDEX IF EXISTS idx_inventory_account_id_unique;

ALTER TABLE inventory ADD COLUMN IF NOT EXISTS account_id_old TEXT;

UPDATE inventory i
SET    account_id_old = a.client_id
FROM   accounts a
WHERE  a.id = i.account_id;

DELETE FROM inventory WHERE account_id_old IS NULL;

ALTER TABLE inventory DROP COLUMN IF EXISTS account_id;
ALTER TABLE inventory RENAME COLUMN account_id_old TO account_id;
ALTER TABLE inventory ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE inventory ALTER COLUMN account_id SET DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_inventory_account_id
    ON inventory (account_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_inventory_account_id_unique
    ON inventory (account_id);

-- ── quest_session_tracking ───────────────────────────────────────────────────

ALTER TABLE quest_session_tracking DROP CONSTRAINT IF EXISTS fk_qst_account_id;
ALTER TABLE quest_session_tracking DROP CONSTRAINT IF EXISTS uq_quest_session_tracking_account_quest_time;
DROP INDEX IF EXISTS idx_qst_account_id;

ALTER TABLE quest_session_tracking ADD COLUMN IF NOT EXISTS account_id_old TEXT;

UPDATE quest_session_tracking qst
SET    account_id_old = a.client_id
FROM   accounts a
WHERE  a.id = qst.account_id;

DELETE FROM quest_session_tracking WHERE account_id_old IS NULL;

ALTER TABLE quest_session_tracking DROP COLUMN IF EXISTS account_id;
ALTER TABLE quest_session_tracking RENAME COLUMN account_id_old TO account_id;
ALTER TABLE quest_session_tracking ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE quest_session_tracking ALTER COLUMN account_id SET DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_qst_account_id
    ON quest_session_tracking (account_id);

ALTER TABLE quest_session_tracking
    ADD CONSTRAINT uq_quest_session_tracking_account_quest_time
    UNIQUE (account_id, quest_id, occurred_at);
