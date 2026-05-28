-- Minimal schema for sync service integration tests.
-- Contains only the tables touched by postgres_store_integration_test.go.
-- Keep in sync with the canonical BFF migrations:
--   000054_initial_schema (sets, draft_card_ratings, draft_color_ratings)
--   000062_add_is_draft_active (is_draft_active column on sets)
--   000065_add_sync_hashes (sync_hashes)
--   000088_add_sets_seventeenlands_code (seventeenlands_code column on sets)

-- Sets: card set metadata from Scryfall
CREATE TABLE IF NOT EXISTS sets (
    code                TEXT PRIMARY KEY,
    name                TEXT NOT NULL,
    released_at         TEXT,
    card_count          INTEGER,
    set_type            TEXT,
    icon_svg_uri        TEXT,
    is_standard_legal   BOOLEAN NOT NULL DEFAULT FALSE,
    is_draft_active     BOOLEAN NOT NULL DEFAULT FALSE,
    seventeenlands_code TEXT,
    rotation_date       TEXT,
    cached_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_updated        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sets_released_at   ON sets(released_at);
CREATE INDEX IF NOT EXISTS idx_sets_standard       ON sets(is_standard_legal);
CREATE INDEX IF NOT EXISTS idx_sets_draft_active   ON sets(is_draft_active);

-- Draft card ratings: 17Lands card performance data
CREATE TABLE IF NOT EXISTS draft_card_ratings (
    id           BIGSERIAL PRIMARY KEY,
    set_code     TEXT NOT NULL,
    draft_format TEXT NOT NULL,
    arena_id     INTEGER NOT NULL,
    name         TEXT NOT NULL,
    color        TEXT,
    rarity       TEXT,
    gihwr        DOUBLE PRECISION,
    ohwr         DOUBLE PRECISION,
    alsa         DOUBLE PRECISION,
    ata          DOUBLE PRECISION,
    gih_count    INTEGER,
    data_source  TEXT NOT NULL DEFAULT 'api',
    url          TEXT,
    url_back     TEXT,
    cached_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(set_code, draft_format, arena_id)
);

CREATE INDEX IF NOT EXISTS idx_draft_card_ratings_set      ON draft_card_ratings(set_code, draft_format);
CREATE INDEX IF NOT EXISTS idx_draft_card_ratings_arena_id ON draft_card_ratings(arena_id);

-- Draft color ratings: 17Lands color combination performance
CREATE TABLE IF NOT EXISTS draft_color_ratings (
    id                BIGSERIAL PRIMARY KEY,
    set_code          TEXT NOT NULL,
    draft_format      TEXT NOT NULL,
    color_combination TEXT NOT NULL,
    win_rate          DOUBLE PRECISION,
    games_played      INTEGER,
    data_source       TEXT NOT NULL DEFAULT 'api',
    cached_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(set_code, draft_format, color_combination)
);

-- Sync hashes: content-hash dedup so Lambda skips unchanged payloads
CREATE TABLE IF NOT EXISTS sync_hashes (
    key        TEXT PRIMARY KEY,
    hash       TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
