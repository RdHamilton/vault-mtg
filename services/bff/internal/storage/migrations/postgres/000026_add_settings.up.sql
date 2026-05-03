-- Settings table for persisting user preferences (PostgreSQL)
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO settings (key, value) VALUES
    ('autoRefresh', 'false'),
    ('refreshInterval', '30'),
    ('showNotifications', 'true'),
    ('theme', '"dark"'),
    ('daemonPort', '9999'),
    ('daemonMode', '"standalone"')
ON CONFLICT (key) DO NOTHING;
