-- Create a Postgres role for the sync service, scoped to card/ratings tables only.
DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'mtga_sync') THEN
    CREATE ROLE mtga_sync LOGIN PASSWORD 'changeme';
  END IF;
END
$$;

GRANT CONNECT ON DATABASE postgres TO mtga_sync;
GRANT USAGE ON SCHEMA public TO mtga_sync;

-- Sync service may read/write card and ratings tables.
GRANT SELECT, INSERT, UPDATE ON draft_card_ratings TO mtga_sync;
GRANT SELECT, INSERT, UPDATE ON draft_color_ratings TO mtga_sync;
GRANT SELECT, INSERT, UPDATE ON dataset_metadata TO mtga_sync;
GRANT SELECT ON set_cards TO mtga_sync;
GRANT SELECT ON sets TO mtga_sync;

-- Explicitly deny write access to user-facing tables.
REVOKE INSERT, UPDATE, DELETE ON matches FROM mtga_sync;
REVOKE INSERT, UPDATE, DELETE ON draft_sessions FROM mtga_sync;
REVOKE INSERT, UPDATE, DELETE ON collection FROM mtga_sync;
