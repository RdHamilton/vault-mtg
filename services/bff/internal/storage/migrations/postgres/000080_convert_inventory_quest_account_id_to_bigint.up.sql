-- Migration 000080: convert inventory.account_id and quest_session_tracking.account_id
-- from TEXT (raw MTGA client_id) to BIGINT FK referencing accounts.id.
--
-- Background: migration 000068 added account_id as TEXT to inventory and
-- quest_session_tracking.  The projection worker previously wrote the raw MTGA
-- client_id string there, which bypassed tenant isolation.  With the security
-- fix in ticket #2030 the projection worker now resolves accounts.id via
-- GetOrCreateByClientID (which enforces user_id ownership) and passes the
-- BIGINT FK to UpsertInventory / InsertQuestCompleted.  This migration aligns
-- the schema with that behaviour.
--
-- Steps:
--   1. Add a new BIGINT column (account_id_new) to avoid casting in-place.
--   2. Back-fill it by joining on accounts.client_id.
--   3. Drop the old TEXT column and its index/constraint.
--   4. Rename account_id_new to account_id.
--   5. Add NOT NULL constraint, index, and FK.
--
-- Rows whose TEXT account_id has no matching accounts row (orphans from the
-- sentinel empty-string row or test data) are set to 0 and then deleted so the
-- NOT NULL constraint can be applied cleanly.
--
-- Acceptance: ticket #2030

-- ── inventory ────────────────────────────────────────────────────────────────

ALTER TABLE inventory ADD COLUMN IF NOT EXISTS account_id_new BIGINT;

UPDATE inventory i
SET    account_id_new = a.id
FROM   accounts a
WHERE  a.client_id = i.account_id;

-- Remove orphan rows that could not be resolved.
DELETE FROM inventory WHERE account_id_new IS NULL;

-- Drop old TEXT column and its supporting index/constraint.
DROP INDEX IF EXISTS idx_inventory_account_id;
DROP INDEX IF EXISTS idx_inventory_account_id_unique;
ALTER TABLE inventory DROP COLUMN IF EXISTS account_id;

-- Promote the new column.
ALTER TABLE inventory RENAME COLUMN account_id_new TO account_id;
ALTER TABLE inventory ALTER COLUMN account_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_inventory_account_id_unique
    ON inventory (account_id);

ALTER TABLE inventory
    ADD CONSTRAINT fk_inventory_account_id
    FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE CASCADE;

-- ── quest_session_tracking ───────────────────────────────────────────────────

ALTER TABLE quest_session_tracking ADD COLUMN IF NOT EXISTS account_id_new BIGINT;

UPDATE quest_session_tracking qst
SET    account_id_new = a.id
FROM   accounts a
WHERE  a.client_id = qst.account_id;

-- Remove orphan rows.
DELETE FROM quest_session_tracking WHERE account_id_new IS NULL;

-- Drop old TEXT column and its supporting index.
DROP INDEX IF EXISTS idx_qst_account_id;

-- Some Postgres versions require explicit syntax; use IF EXISTS for safety.
ALTER TABLE quest_session_tracking
    DROP CONSTRAINT IF EXISTS uq_quest_session_tracking_account_quest_time;

ALTER TABLE quest_session_tracking DROP COLUMN IF EXISTS account_id;

ALTER TABLE quest_session_tracking RENAME COLUMN account_id_new TO account_id;
ALTER TABLE quest_session_tracking ALTER COLUMN account_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_qst_account_id
    ON quest_session_tracking (account_id);

ALTER TABLE quest_session_tracking
    ADD CONSTRAINT uq_quest_session_tracking_account_quest_time
    UNIQUE (account_id, quest_id, occurred_at);

ALTER TABLE quest_session_tracking
    ADD CONSTRAINT fk_qst_account_id
    FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE CASCADE;
