-- Migration 000081: add composite (arena_id, id) index on set_cards.
--
-- Background: set_cards has UNIQUE(set_code, arena_id) but not UNIQUE(arena_id),
-- so the same arena_id appears across multiple printings.  Two hot query shapes
-- on the read path pay for a sort pass on every invocation:
--
--   1. collection_repo.go ListCollection / CountByRarity / ValueRows use
--      DISTINCT ON (arena_id) ... ORDER BY arena_id, id  — the existing
--      idx_set_cards_arena_id covers the leading predicate but Postgres must
--      sort by id within each arena_id group in a separate pass.
--
--   2. decks_repo.go deckCards() runs a per-row LEFT JOIN LATERAL with
--      ORDER BY id LIMIT 1 inside the subquery.  Again, the existing
--      single-column index satisfies the WHERE but forces a sort for the
--      ORDER BY id.
--
-- Adding (arena_id, id) as a composite index lets both shapes satisfy the full
-- ORDER BY from the index, eliminating the per-row sort step.
--
-- Acceptance: ticket #2032

CREATE INDEX IF NOT EXISTS idx_set_cards_arena_id_id
    ON set_cards (arena_id, id);
