-- Migration 000071: create quest_session_tracking table.
--
-- Records completed quests as projected from quest.completed daemon events.
-- Each row represents a single quest completion for one account, identified
-- by (account_id, quest_id, occurred_at) so that re-projecting the same event
-- is idempotent.
--
-- account_id is TEXT (the raw MTGA client_id string) consistent with the
-- daemon_events.account_id column and migration 000068.
--
-- Acceptance: ticket #1510

CREATE TABLE IF NOT EXISTS quest_session_tracking (
    id               BIGSERIAL    PRIMARY KEY,
    account_id       TEXT         NOT NULL,
    quest_id         TEXT         NOT NULL,
    quest_name       TEXT         NOT NULL,
    progress         INT          NOT NULL DEFAULT 0,
    goal             INT          NOT NULL DEFAULT 0,
    xp_reward        INT          NOT NULL DEFAULT 0,
    completion_source TEXT,
    occurred_at      TIMESTAMPTZ  NOT NULL,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_quest_session_tracking_account_quest_time
        UNIQUE (account_id, quest_id, occurred_at)
);

CREATE INDEX IF NOT EXISTS idx_qst_account_id  ON quest_session_tracking (account_id);
CREATE INDEX IF NOT EXISTS idx_qst_quest_id    ON quest_session_tracking (quest_id);
CREATE INDEX IF NOT EXISTS idx_qst_occurred_at ON quest_session_tracking (occurred_at DESC);
