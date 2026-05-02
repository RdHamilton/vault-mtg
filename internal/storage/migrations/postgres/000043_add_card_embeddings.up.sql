-- Card embeddings for semantic similarity (PostgreSQL)
CREATE TABLE IF NOT EXISTS card_embeddings (
    id BIGSERIAL PRIMARY KEY,
    arena_id INTEGER NOT NULL UNIQUE,
    card_name TEXT NOT NULL,
    embedding TEXT NOT NULL,
    embedding_version INTEGER NOT NULL DEFAULT 1,
    source TEXT DEFAULT 'characteristics',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_card_embeddings_arena_id ON card_embeddings(arena_id);
CREATE INDEX IF NOT EXISTS idx_card_embeddings_name ON card_embeddings(card_name);
CREATE INDEX IF NOT EXISTS idx_card_embeddings_version ON card_embeddings(embedding_version);

CREATE TABLE IF NOT EXISTS card_similarity_cache (
    id BIGSERIAL PRIMARY KEY,
    card_arena_id INTEGER NOT NULL,
    similar_arena_id INTEGER NOT NULL,
    similarity_score REAL NOT NULL,
    rank INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(card_arena_id, similar_arena_id)
);

CREATE INDEX IF NOT EXISTS idx_similarity_cache_card ON card_similarity_cache(card_arena_id);
CREATE INDEX IF NOT EXISTS idx_similarity_cache_score ON card_similarity_cache(similarity_score DESC);
