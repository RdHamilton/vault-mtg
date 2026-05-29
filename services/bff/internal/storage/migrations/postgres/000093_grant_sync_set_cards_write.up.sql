-- Grant mtga_sync INSERT and UPDATE on set_cards.
-- Migration 000057 granted only SELECT on set_cards; UpsertSetCards requires
-- INSERT and UPDATE. Without this grant the sync Lambda fails with a permissions
-- error on its first write to set_cards.
-- cards is retired (dropped in 000025) — no grant is needed or possible.
GRANT INSERT, UPDATE ON set_cards TO mtga_sync;
