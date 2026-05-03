-- MTGZone archetype tables (PostgreSQL)

CREATE TABLE IF NOT EXISTS mtgzone_archetypes (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    format TEXT NOT NULL,
    tier TEXT,
    description TEXT,
    play_style TEXT,
    source_url TEXT,
    last_updated TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(name, format)
);

CREATE INDEX IF NOT EXISTS idx_mtgzone_archetypes_format ON mtgzone_archetypes(format);
CREATE INDEX IF NOT EXISTS idx_mtgzone_archetypes_tier ON mtgzone_archetypes(tier);

CREATE TABLE IF NOT EXISTS mtgzone_archetype_cards (
    id BIGSERIAL PRIMARY KEY,
    archetype_id BIGINT NOT NULL,
    card_name TEXT NOT NULL,
    role TEXT NOT NULL,
    copies INTEGER DEFAULT 4,
    importance TEXT,
    notes TEXT,
    last_updated TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (archetype_id) REFERENCES mtgzone_archetypes(id) ON DELETE CASCADE,
    UNIQUE(archetype_id, card_name)
);

CREATE INDEX IF NOT EXISTS idx_mtgzone_archetype_cards_archetype ON mtgzone_archetype_cards(archetype_id);
CREATE INDEX IF NOT EXISTS idx_mtgzone_archetype_cards_card ON mtgzone_archetype_cards(card_name);
CREATE INDEX IF NOT EXISTS idx_mtgzone_archetype_cards_role ON mtgzone_archetype_cards(role);

CREATE TABLE IF NOT EXISTS mtgzone_synergies (
    id BIGSERIAL PRIMARY KEY,
    card_a TEXT NOT NULL,
    card_b TEXT NOT NULL,
    reason TEXT NOT NULL,
    source_url TEXT,
    archetype_context TEXT,
    confidence REAL DEFAULT 0.5,
    last_updated TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(card_a, card_b, archetype_context)
);

CREATE INDEX IF NOT EXISTS idx_mtgzone_synergies_card_a ON mtgzone_synergies(card_a);
CREATE INDEX IF NOT EXISTS idx_mtgzone_synergies_card_b ON mtgzone_synergies(card_b);

CREATE TABLE IF NOT EXISTS mtgzone_articles (
    id BIGSERIAL PRIMARY KEY,
    url TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    article_type TEXT,
    format TEXT,
    archetype TEXT,
    published_at TIMESTAMPTZ,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cards_mentioned TEXT
);

CREATE INDEX IF NOT EXISTS idx_mtgzone_articles_type ON mtgzone_articles(article_type);
CREATE INDEX IF NOT EXISTS idx_mtgzone_articles_format ON mtgzone_articles(format);
