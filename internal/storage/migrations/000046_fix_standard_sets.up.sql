-- Migration: Fix missing and incorrect Standard set codes
-- Updates Standard set configuration with correct MTGA set codes
-- Also inserts missing sets that aren't yet in the sets table

-- Insert missing sets (won't conflict if they already exist)
INSERT OR IGNORE INTO sets (code, name, released_at, set_type, is_standard_legal, rotation_date)
VALUES ('ECL', 'Lorwyn Eclipsed', '2026-01-23', 'expansion', TRUE, '2028-01-01');

INSERT OR IGNORE INTO sets (code, name, released_at, set_type, is_standard_legal, rotation_date)
VALUES ('BIG', 'The Big Score', '2024-04-19', 'expansion', TRUE, '2027-01-23');

INSERT OR IGNORE INTO sets (code, name, released_at, set_type, is_standard_legal, rotation_date)
VALUES ('AED', 'Aetherdrift', '2025-02-14', 'expansion', TRUE, '2028-01-01');

-- Fix TLA -> TDM (Tarkir: Dragonstorm uses TDM in MTGA, TLA is Avatar: The Last Airbender)
UPDATE sets SET is_standard_legal = FALSE WHERE code = 'TLA';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2028-01-01' WHERE code = 'TDM';

-- Ensure all core Standard sets are properly marked
-- These are the sets legal in Standard as of January 2026:

-- Foundations (FDN) - Legal until 2029 (special extended legality)
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2029-01-01' WHERE code = 'FDN';

-- 2027 rotation cohort
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2027-01-23' WHERE code = 'WOE';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2027-01-23' WHERE code = 'LCI';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2027-01-23' WHERE code = 'MKM';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2027-01-23' WHERE code = 'OTJ';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2027-01-23' WHERE code = 'BIG';

-- 2028 rotation cohort
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2028-01-01' WHERE code = 'BLB';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2028-01-01' WHERE code = 'DSK';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2028-01-01' WHERE code = 'AED';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2028-01-01' WHERE code = 'TDM';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2028-01-01' WHERE code = 'ECL';
