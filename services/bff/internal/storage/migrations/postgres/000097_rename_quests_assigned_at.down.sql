-- Rollback for migration 000097: restore quests.first_seen_at to assigned_at.

ALTER TABLE quests RENAME COLUMN first_seen_at TO assigned_at;

ALTER INDEX IF EXISTS idx_quests_first_seen_at RENAME TO idx_quests_assigned_at;
