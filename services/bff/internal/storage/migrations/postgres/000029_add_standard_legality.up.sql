-- Migration: Add Standard format legality tracking (PostgreSQL)

ALTER TABLE sets ADD COLUMN IF NOT EXISTS is_standard_legal BOOLEAN DEFAULT FALSE;
ALTER TABLE sets ADD COLUMN IF NOT EXISTS rotation_date TEXT;

CREATE TABLE IF NOT EXISTS standard_config (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    next_rotation_date TEXT NOT NULL,
    rotation_enabled BOOLEAN DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO standard_config (id, next_rotation_date, rotation_enabled)
VALUES (1, '2027-01-23', TRUE)
ON CONFLICT (id) DO NOTHING;

ALTER TABLE set_cards ADD COLUMN IF NOT EXISTS legalities TEXT;

CREATE INDEX idx_sets_standard ON sets(is_standard_legal);
CREATE INDEX idx_set_cards_legalities ON set_cards(legalities) WHERE legalities IS NOT NULL;

UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2029-01-01' WHERE code = 'FDN';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2027-01-23' WHERE code = 'WOE';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2027-01-23' WHERE code = 'LCI';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2027-01-23' WHERE code = 'MKM';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2027-01-23' WHERE code = 'OTJ';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2028-01-01' WHERE code = 'BLB';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2028-01-01' WHERE code = 'DSK';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2028-01-01' WHERE code = 'AED';
UPDATE sets SET is_standard_legal = TRUE, rotation_date = '2028-01-01' WHERE code = 'TLA';
