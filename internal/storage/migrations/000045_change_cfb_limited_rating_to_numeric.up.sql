-- Change CFB limited_rating from TEXT (letter grade) to REAL (0-5 numerical score)
-- SQLite doesn't support ALTER COLUMN, so we recreate the table

-- Create new table with REAL type for limited_rating
CREATE TABLE cfb_ratings_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    card_name TEXT NOT NULL,
    set_code TEXT NOT NULL,
    arena_id INTEGER,

    -- CFB Limited Rating (0.0-5.0 numerical scale, matching TCGPlayer/MTG Arena Zone)
    limited_rating REAL DEFAULT 0.0,
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

-- Copy existing data (convert old letter grades to numeric if any exist)
-- Old grades: A+=5.0, A=4.5, A-=4.0, B+=3.5, B=3.0, B-=2.5, C+=2.0, C=1.5, C-=1.0, D=0.5, F=0.0
INSERT INTO cfb_ratings_new (
    id, card_name, set_code, arena_id,
    limited_rating, limited_score,
    constructed_rating, constructed_score,
    archetype_fit, commentary, source_url, author,
    imported_at, updated_at
)
SELECT
    id, card_name, set_code, arena_id,
    CASE limited_rating
        WHEN 'A+' THEN 5.0
        WHEN 'A' THEN 4.5
        WHEN 'A-' THEN 4.0
        WHEN 'B+' THEN 3.5
        WHEN 'B' THEN 3.0
        WHEN 'B-' THEN 2.5
        WHEN 'C+' THEN 2.0
        WHEN 'C' THEN 1.5
        WHEN 'C-' THEN 1.0
        WHEN 'D' THEN 0.5
        WHEN 'F' THEN 0.0
        ELSE CAST(limited_rating AS REAL)  -- Handle if already numeric
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
