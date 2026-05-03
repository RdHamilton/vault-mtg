-- Ensure all current standard-legal sets exist and are marked correctly.
INSERT INTO sets (code, name, released_at, set_type, is_standard_legal, rotation_date)
VALUES
  ('WOE', 'Wilds of Eldraine',         '2023-09-08', 'expansion', TRUE, '2027-01-23'),
  ('LCI', 'The Lost Caverns of Ixalan','2023-11-17', 'expansion', TRUE, '2027-01-23'),
  ('MKM', 'Murders at Karlov Manor',   '2024-02-09', 'expansion', TRUE, '2027-01-23'),
  ('OTJ', 'Outlaws of Thunder Junction','2024-04-19','expansion', TRUE, '2027-01-23'),
  ('BIG', 'The Big Score',             '2024-04-19', 'expansion', TRUE, '2027-01-23'),
  ('BLB', 'Bloomburrow',               '2024-08-02', 'expansion', TRUE, '2028-01-01'),
  ('DSK', 'Duskmourn: House of Horror','2024-09-27', 'expansion', TRUE, '2028-01-01'),
  ('AED', 'Aetherdrift',               '2025-02-14', 'expansion', TRUE, '2028-01-01'),
  ('FDN', 'Foundations',               '2024-11-15', 'expansion', TRUE, '2029-01-01'),
  ('TDM', 'Tarkir: Dragonstorm',       '2025-04-11', 'expansion', TRUE, '2028-01-01'),
  ('ECL', 'Ecolight',                  '2025-07-25', 'expansion', TRUE, '2029-01-01')
ON CONFLICT (code) DO UPDATE SET
  is_standard_legal = EXCLUDED.is_standard_legal,
  rotation_date     = EXCLUDED.rotation_date;
