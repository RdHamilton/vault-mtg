-- Add currency tracking history table (PostgreSQL)

CREATE TABLE IF NOT EXISTS currency_history (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    gems INTEGER NOT NULL,
    gold INTEGER NOT NULL,
    gems_delta INTEGER NOT NULL DEFAULT 0,
    gold_delta INTEGER NOT NULL DEFAULT 0,
    source TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_currency_history_account_id ON currency_history(account_id);
CREATE INDEX IF NOT EXISTS idx_currency_history_timestamp ON currency_history(timestamp);
CREATE INDEX IF NOT EXISTS idx_currency_history_account_timestamp ON currency_history(account_id, timestamp);
