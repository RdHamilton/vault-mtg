-- Migration 000097: rename quests.assigned_at to quests.first_seen_at.
--
-- The column was originally named assigned_at to suggest the time MTGA
-- assigned the quest to the player.  In practice the BFF writes the
-- daemon's seen_at timestamp — the moment our log parser first observed
-- the quest — which is semantically "first seen", not "assigned by the
-- game server".  The rename makes the intent unambiguous.
--
-- The old idx_quests_assigned_at index is dropped and recreated under the
-- new name so pg_catalog stays tidy.

ALTER TABLE quests RENAME COLUMN assigned_at TO first_seen_at;

ALTER INDEX IF EXISTS idx_quests_assigned_at RENAME TO idx_quests_first_seen_at;
