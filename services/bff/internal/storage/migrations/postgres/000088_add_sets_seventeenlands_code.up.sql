-- Migration 000088: add seventeenlands_code column to sets + re-enable AED with DFT.
-- Ticket: vault-mtg-tickets#47
--
-- The sync Lambda queries 17Lands using the Scryfall set code verbatim.
-- For Aetherdrift, 17Lands uses the expansion code 'DFT', not Scryfall's 'AED'.
-- This migration adds a nullable seventeenlands_code column:
--   - NULL  -> use the Scryfall code as-is (correct for the majority of sets)
--   - 'DFT' -> use 'DFT' for 17Lands API requests while keying all DB rows on 'AED'
--
-- Ray verified live: GET /card_ratings/data?expansion=DFT returns 171,116 bytes.
-- AED and ADRIFT both return empty arrays.
--
-- Migration 000087 disabled AED and cleared the skip-guard counter.
-- This migration re-enables AED now that the correct expansion code is set.

ALTER TABLE sets
    ADD COLUMN IF NOT EXISTS seventeenlands_code TEXT;

UPDATE sets
SET    seventeenlands_code = 'DFT'
WHERE  code = 'AED';

-- Re-enable AED in the draft-active sync loop now that seventeenlands_code = 'DFT'
-- will be used for 17Lands API requests instead of the Scryfall code 'AED'.
UPDATE sets
SET    is_draft_active = TRUE
WHERE  code = 'AED';
