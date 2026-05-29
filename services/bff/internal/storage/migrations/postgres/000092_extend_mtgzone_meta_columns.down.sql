-- Revoke mtga_sync grants added in 000092 up migration.
REVOKE USAGE, SELECT ON SEQUENCE mtgzone_archetype_cards_id_seq FROM mtga_sync;
REVOKE USAGE, SELECT ON SEQUENCE mtgzone_archetypes_id_seq      FROM mtga_sync;
REVOKE SELECT, INSERT, UPDATE ON mtgzone_archetype_cards FROM mtga_sync;
REVOKE SELECT, INSERT, UPDATE ON mtgzone_archetypes      FROM mtga_sync;

-- Drop the metric columns added in 000092 up migration.
ALTER TABLE mtgzone_archetypes
    DROP COLUMN IF EXISTS trend_direction,
    DROP COLUMN IF EXISTS confidence_score,
    DROP COLUMN IF EXISTS tournament_wins,
    DROP COLUMN IF EXISTS tournament_top8s,
    DROP COLUMN IF EXISTS meta_share;
