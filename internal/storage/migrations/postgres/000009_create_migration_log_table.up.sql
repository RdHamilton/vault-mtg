-- Create migration_log table to track Scryfall card migrations (PostgreSQL)
CREATE TABLE IF NOT EXISTS migration_log (
    id BIGSERIAL PRIMARY KEY,
    migration_id TEXT NOT NULL UNIQUE,
    old_scryfall_id TEXT NOT NULL,
    new_scryfall_id TEXT,
    strategy TEXT NOT NULL CHECK(strategy IN ('merge', 'delete')),
    note TEXT,
    performed_at TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_migration_log_migration_id ON migration_log(migration_id);
CREATE INDEX IF NOT EXISTS idx_migration_log_old_scryfall_id ON migration_log(old_scryfall_id);
CREATE INDEX IF NOT EXISTS idx_migration_log_processed_at ON migration_log(processed_at);
