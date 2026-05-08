-- game_plays stores one row per completed game within a match.
-- (account_id, match_id, game_number) is the natural unique key.
-- sequence carries the per-session monotonic counter from the DaemonEvent
-- envelope and is used by the projector to enforce causal ordering.
CREATE TABLE IF NOT EXISTS game_plays (
    id             BIGSERIAL    PRIMARY KEY,
    account_id     BIGINT       NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    match_id       TEXT         NOT NULL,
    game_number    INT          NOT NULL,
    winning_team_id INT         NOT NULL DEFAULT 0,
    turn_count     INT          NOT NULL DEFAULT 0,
    duration_secs  INT          NOT NULL DEFAULT 0,
    sequence       BIGINT       NOT NULL DEFAULT 0,
    occurred_at    TIMESTAMPTZ  NOT NULL,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_game_plays_account_match_game
        UNIQUE (account_id, match_id, game_number)
);

CREATE INDEX IF NOT EXISTS idx_game_plays_account_match
    ON game_plays (account_id, match_id);

-- life_change_tracking stores each life-total mutation observed during a game.
-- Rows are immutable once inserted; there is no upsert path.
CREATE TABLE IF NOT EXISTS life_change_tracking (
    id          BIGSERIAL    PRIMARY KEY,
    account_id  BIGINT       NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    game_play_id BIGINT      NOT NULL REFERENCES game_plays(id) ON DELETE CASCADE,
    team_id     INT          NOT NULL,
    life_total  INT          NOT NULL,
    delta       INT          NOT NULL,
    turn_number INT          NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_life_change_tracking_game_play
    ON life_change_tracking (game_play_id);
