-- Drop unused tables that have been replaced (PostgreSQL)
-- cards: Replaced by set_cards (migration 000014)
-- currency_history: Dropped (from migration 000004) then replaced with new schema later
-- draft_events: Replaced by draft_sessions (migration 000014)

DROP INDEX IF EXISTS idx_cards_arena_id;
DROP INDEX IF EXISTS idx_cards_name;
DROP INDEX IF EXISTS idx_cards_set;
DROP INDEX IF EXISTS idx_cards_last_updated;
DROP INDEX IF EXISTS idx_draft_events_start_time;
DROP INDEX IF EXISTS idx_draft_events_status;

DROP TABLE IF EXISTS cards;
DROP TABLE IF EXISTS currency_history;
DROP TABLE IF EXISTS draft_events CASCADE;
