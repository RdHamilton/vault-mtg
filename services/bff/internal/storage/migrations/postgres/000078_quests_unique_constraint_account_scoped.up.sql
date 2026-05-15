-- Migration 000078: make the quests UNIQUE constraint account-scoped.
--
-- The original constraint UNIQUE(quest_id, assigned_at) from migration 000010
-- is not scoped to account_id.  When two different accounts are assigned the
-- same quest at the same timestamp the second INSERT silently fails, causing
-- silent data loss (multi-tenancy correctness bug, issue #1924).
--
-- Fix: drop the old constraint and replace it with
--      UNIQUE(account_id, quest_id, assigned_at).
--
-- The ON CONFLICT clause in quest_repo.go is updated in the same PR to target
-- the new constraint columns.

-- Drop the old constraint.  The constraint name matches the PostgreSQL default
-- for UNIQUE(quest_id, assigned_at) defined inline in CREATE TABLE.
ALTER TABLE quests DROP CONSTRAINT IF EXISTS quests_quest_id_assigned_at_key;

-- Add the account-scoped constraint.
ALTER TABLE quests
    ADD CONSTRAINT quests_account_id_quest_id_assigned_at_key
    UNIQUE (account_id, quest_id, assigned_at);
