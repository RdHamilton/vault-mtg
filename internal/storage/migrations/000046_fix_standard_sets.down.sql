-- Revert Standard set fixes (partial rollback)
--
-- This down migration intentionally does NOT fully revert all changes from the up migration:
-- - Inserted sets (ECL, BIG, AED) are NOT deleted to avoid data loss
-- - Rotation date changes for WOE, LCI, MKM, OTJ, BLB, DSK, FDN are NOT reverted
--   as these are corrections, not reversible state changes
--
-- Only the TLA/TDM/BIG standard legality changes are reverted here.

UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2028-01-01' WHERE code = 'TLA';
UPDATE sets SET is_standard_legal = FALSE WHERE code = 'TDM';
UPDATE sets SET is_standard_legal = FALSE WHERE code = 'BIG';
