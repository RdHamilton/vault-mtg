-- Revert CFB limited_rating from REAL back to TEXT (letter grade)
-- SQLite doesn't support ALTER COLUMN, so we recreate the table

-- Create new table with TEXT type for limited_rating
CREATE TABLE cfb_ratings_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    card_name TEXT NOT NULL,
    set_code TEXT NOT NULL,
    arena_id INTEGER,

    -- CFB Limited Rating (A+, A, A-, B+, B, B-, C+, C, C-, D, F)
    limited_rating TEXT,
    limited_score REAL DEFAULT 0.0,

    -- CFB Constructed Rating (Staple, Playable, Fringe, Unplayable)
    constructed_rating TEXT,
    constructed_score REAL DEFAULT 0.0,

    -- Archetype fit notes
    archetype_fit TEXT,

    -- Commentary/notes from CFB review
    commentary TEXT,

    -- Source information
    source_url TEXT,
    author TEXT,

    -- Metadata
    imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(card_name, set_code)
);

-- Copy existing data (convert numeric back to letter grades)
INSERT INTO cfb_ratings_new (
    id, card_name, set_code, arena_id,
    limited_rating, limited_score,
    constructed_rating, constructed_score,
    archetype_fit, commentary, source_url, author,
    imported_at, updated_at
)
SELECT
    id, card_name, set_code, arena_id,
    CASE
        WHEN limited_rating >= 4.75 THEN 'A+'
        WHEN limited_rating >= 4.25 THEN 'A'
        WHEN limited_rating >= 3.75 THEN 'A-'
        WHEN limited_rating >= 3.25 THEN 'B+'
        WHEN limited_rating >= 2.75 THEN 'B'
        WHEN limited_rating >= 2.25 THEN 'B-'
        WHEN limited_rating >= 1.75 THEN 'C+'
        WHEN limited_rating >= 1.25 THEN 'C'
        WHEN limited_rating >= 0.75 THEN 'C-'
        WHEN limited_rating >= 0.25 THEN 'D'
        ELSE 'F'
    END,
    limited_score,
    constructed_rating, constructed_score,
    archetype_fit, commentary, source_url, author,
    imported_at, updated_at
FROM cfb_ratings;

-- Drop old table
DROP TABLE cfb_ratings;

-- Rename new table
ALTER TABLE cfb_ratings_new RENAME TO cfb_ratings;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_cfb_ratings_set_code ON cfb_ratings(set_code);
CREATE INDEX IF NOT EXISTS idx_cfb_ratings_arena_id ON cfb_ratings(arena_id);
CREATE INDEX IF NOT EXISTS idx_cfb_ratings_card_name ON cfb_ratings(card_name);
