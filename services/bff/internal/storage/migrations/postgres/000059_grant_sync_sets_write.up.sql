-- Grant INSERT and UPDATE on sets to mtga_sync so the Scryfall set sync
-- (UpsertSets) can keep sets.is_standard_legal current automatically.
-- Migration 000057 only granted SELECT on sets; this corrects the omission.
GRANT INSERT, UPDATE ON sets TO mtga_sync;
