-- Fix decks with NULL account_id and add 'arena' as valid source (PostgreSQL)

UPDATE decks SET account_id = 1 WHERE account_id IS NULL;

-- Add 'arena' to the source CHECK constraint by dropping and re-adding it
ALTER TABLE decks DROP CONSTRAINT IF EXISTS decks_source_check;
ALTER TABLE decks ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE decks ALTER COLUMN account_id SET DEFAULT 1;
ALTER TABLE decks ADD CONSTRAINT decks_source_check
    CHECK(source IN ('draft', 'constructed', 'imported', 'arena'));

-- Re-add FK constraint for account_id
ALTER TABLE decks ADD CONSTRAINT fk_decks_account_id
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE;
