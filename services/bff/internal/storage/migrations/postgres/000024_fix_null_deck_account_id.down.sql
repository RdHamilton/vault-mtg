-- Revert arena source and NOT NULL constraint on account_id (PostgreSQL)
ALTER TABLE decks DROP CONSTRAINT IF EXISTS fk_decks_account_id;
ALTER TABLE decks DROP CONSTRAINT IF EXISTS decks_source_check;
ALTER TABLE decks ALTER COLUMN account_id DROP NOT NULL;
ALTER TABLE decks ALTER COLUMN account_id DROP DEFAULT;
ALTER TABLE decks ADD CONSTRAINT decks_source_check
    CHECK(source IN ('draft', 'constructed', 'imported'));
