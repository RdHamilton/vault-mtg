-- Migration 000072: add unique constraint on inventory.account_id.
--
-- The projection worker upserts one inventory snapshot per MTGA account using
-- ON CONFLICT (account_id).  Without a unique constraint the ON CONFLICT
-- clause cannot target account_id.
--
-- The original inventory table (000023) seeded a single row with account_id=''.
-- We remove that sentinel row before adding the constraint to avoid a conflict
-- when real account rows are inserted.
--
-- Acceptance: ticket #1510

DELETE FROM inventory WHERE account_id = '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_inventory_account_id_unique
    ON inventory (account_id);
