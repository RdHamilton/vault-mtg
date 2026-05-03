-- Track which log files have been processed (PostgreSQL)
CREATE TABLE IF NOT EXISTS processed_log_files (
    filename TEXT PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL,
    entry_count INTEGER DEFAULT 0,
    matches_found INTEGER DEFAULT 0,
    file_size_bytes INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_processed_log_files_processed_at ON processed_log_files(processed_at);
