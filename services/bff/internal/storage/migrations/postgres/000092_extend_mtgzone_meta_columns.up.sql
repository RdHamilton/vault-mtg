-- Extend mtgzone_archetypes with scraped metric columns (ADR-039).
-- All columns are nullable so existing rows remain NULL until the first
-- meta-scrape run populates them. No default values — intentional per ADR-039.

ALTER TABLE mtgzone_archetypes
    ADD COLUMN IF NOT EXISTS meta_share       REAL,
    ADD COLUMN IF NOT EXISTS tournament_top8s INTEGER,
    ADD COLUMN IF NOT EXISTS tournament_wins  INTEGER,
    ADD COLUMN IF NOT EXISTS confidence_score REAL,
    ADD COLUMN IF NOT EXISTS trend_direction  TEXT;

-- Grant mtga_sync write access to the mtgzone tables so the meta-scrape
-- Lambda can INSERT/UPDATE archetypes and their card lists.
-- migration 000057 granted USAGE ON SCHEMA public to mtga_sync already.
GRANT SELECT, INSERT, UPDATE ON mtgzone_archetypes      TO mtga_sync;
GRANT SELECT, INSERT, UPDATE ON mtgzone_archetype_cards TO mtga_sync;

-- Sequence grants so mtga_sync can use the BIGSERIAL default values.
-- Without USAGE + SELECT on the sequences, nextval() fails with a permission
-- error before the INSERT even reaches the ON CONFLICT path.
GRANT USAGE, SELECT ON SEQUENCE mtgzone_archetypes_id_seq      TO mtga_sync;
GRANT USAGE, SELECT ON SEQUENCE mtgzone_archetype_cards_id_seq TO mtga_sync;
