-- Add inventory tracking tables (PostgreSQL)

CREATE TABLE IF NOT EXISTS inventory (
    id BIGSERIAL PRIMARY KEY,
    gold INTEGER NOT NULL DEFAULT 0,
    gems INTEGER NOT NULL DEFAULT 0,
    wc_common INTEGER NOT NULL DEFAULT 0,
    wc_uncommon INTEGER NOT NULL DEFAULT 0,
    wc_rare INTEGER NOT NULL DEFAULT 0,
    wc_mythic INTEGER NOT NULL DEFAULT 0,
    vault_progress REAL NOT NULL DEFAULT 0,
    draft_tokens INTEGER NOT NULL DEFAULT 0,
    sealed_tokens INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS inventory_history (
    id BIGSERIAL PRIMARY KEY,
    field TEXT NOT NULL,
    previous_value INTEGER NOT NULL,
    new_value INTEGER NOT NULL,
    delta INTEGER NOT NULL,
    source TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_inventory_history_field ON inventory_history(field);
CREATE INDEX IF NOT EXISTS idx_inventory_history_created_at ON inventory_history(created_at);

INSERT INTO inventory (gold, gems, wc_common, wc_uncommon, wc_rare, wc_mythic, vault_progress, draft_tokens, sealed_tokens)
VALUES (0, 0, 0, 0, 0, 0, 0, 0, 0);
