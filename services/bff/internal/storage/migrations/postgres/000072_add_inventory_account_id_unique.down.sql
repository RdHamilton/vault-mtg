-- Rollback migration 000072: drop unique index on inventory.account_id.
DROP INDEX IF EXISTS idx_inventory_account_id_unique;
