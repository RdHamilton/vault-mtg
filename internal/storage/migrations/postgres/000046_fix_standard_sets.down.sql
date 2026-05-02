-- Revert Standard set fixes (partial rollback)
-- Inserted sets (ECL, BIG, AED) are NOT deleted to avoid data loss.
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2028-01-01' WHERE code = 'TLA';
UPDATE sets SET is_standard_legal = FALSE WHERE code = 'TDM';
UPDATE sets SET is_standard_legal = FALSE WHERE code = 'BIG';
