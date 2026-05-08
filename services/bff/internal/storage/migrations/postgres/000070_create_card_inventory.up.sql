-- Migration 000070: create card_inventory table.
--
-- Stores the player's current card counts as projected from collection.updated
-- daemon events. Each row represents one (account_id, card_id) pair.
--
-- snapshot_hash is a SHA-256 hex digest of the full Cards array from the
-- CollectionUpdatedPayload. It is used for idempotency: re-applying the same
-- payload is a no-op because the ON CONFLICT clause matches on
-- (account_id, card_id, snapshot_hash).
--
-- Acceptance: ticket #1511

CREATE TABLE IF NOT EXISTS card_inventory (
    id            BIGSERIAL       PRIMARY KEY,
    account_id    BIGINT          NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    card_id       INT             NOT NULL,
    count         INT             NOT NULL CHECK (count >= 0),
    snapshot_hash TEXT            NOT NULL,
    updated_at    TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

-- Fast lookup of a single card for a given account.
CREATE UNIQUE INDEX IF NOT EXISTS idx_card_inventory_account_card
    ON card_inventory (account_id, card_id);

-- Idempotency index: (account_id, card_id, snapshot_hash) must be unique so
-- that replaying the same delta is a no-op.
CREATE UNIQUE INDEX IF NOT EXISTS idx_card_inventory_idem
    ON card_inventory (account_id, card_id, snapshot_hash);
