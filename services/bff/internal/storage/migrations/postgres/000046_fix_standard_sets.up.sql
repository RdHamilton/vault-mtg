-- Migration: Fix missing and incorrect Standard set codes (PostgreSQL)

INSERT INTO sets (code, name, released_at, set_type, is_standard_legal, rotation_date)
VALUES ('ECL', 'Lorwyn Eclipsed', '2026-01-23', 'expansion', TRUE, '2028-01-01')
ON CONFLICT (code) DO NOTHING;

INSERT INTO sets (code, name, released_at, set_type, is_standard_legal, rotation_date)
VALUES ('BIG', 'The Big Score', '2024-04-19', 'expansion', TRUE, '2027-01-23')
ON CONFLICT (code) DO NOTHING;

INSERT INTO sets (code, name, released_at, set_type, is_standard_legal, rotation_date)
VALUES ('AED', 'Aetherdrift', '2025-02-14', 'expansion', TRUE, '2028-01-01')
ON CONFLICT (code) DO NOTHING;

UPDATE sets SET is_standard_legal = FALSE WHERE code = 'TLA';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2028-01-01' WHERE code = 'TDM';

UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2029-01-01' WHERE code = 'FDN';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2027-01-23' WHERE code = 'WOE';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2027-01-23' WHERE code = 'LCI';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2027-01-23' WHERE code = 'MKM';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2027-01-23' WHERE code = 'OTJ';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2027-01-23' WHERE code = 'BIG';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2028-01-01' WHERE code = 'BLB';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2028-01-01' WHERE code = 'DSK';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2028-01-01' WHERE code = 'AED';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2028-01-01' WHERE code = 'TDM';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2028-01-01' WHERE code = 'ECL';
