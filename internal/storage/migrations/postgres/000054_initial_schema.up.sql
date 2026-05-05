-- PostgreSQL initial schema for MTGA Companion
-- Equivalent to SQLite migrations 000001 through 000052 (consolidated).
-- Migrations 000051-000053 (users table, account_id on draft_sessions,
-- composite account_id indexes) are handled by their own postgres/ files.
-- This migration creates the base schema that those files build on.

-- Extensions
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ---------------------------------------------------------------------------
-- GLOBAL / REFERENCE TABLES  (no account_id — shared across all tenants)
-- ---------------------------------------------------------------------------

-- Sets: card set metadata from Scryfall
CREATE TABLE IF NOT EXISTS sets (
    code                TEXT PRIMARY KEY,
    name                TEXT NOT NULL,
    released_at         TEXT,
    card_count          INTEGER,
    set_type            TEXT,
    icon_svg_uri        TEXT,
    is_standard_legal   BOOLEAN NOT NULL DEFAULT FALSE,
    rotation_date       TEXT,
    cached_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_updated        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sets_released_at    ON sets(released_at);
CREATE INDEX IF NOT EXISTS idx_sets_standard        ON sets(is_standard_legal);

-- Set cards: per-set card cache from Scryfall
CREATE TABLE IF NOT EXISTS set_cards (
    id                  BIGSERIAL PRIMARY KEY,
    set_code            TEXT NOT NULL,
    arena_id            TEXT NOT NULL,
    scryfall_id         TEXT NOT NULL,
    name                TEXT NOT NULL,
    mana_cost           TEXT,
    cmc                 INTEGER,
    types               TEXT,
    colors              TEXT,
    rarity              TEXT,
    text                TEXT,
    power               TEXT,
    toughness           TEXT,
    image_url           TEXT,
    image_url_small     TEXT,
    image_url_art       TEXT,
    legalities          TEXT,
    price_usd           DOUBLE PRECISION,
    price_usd_foil      DOUBLE PRECISION,
    price_eur           DOUBLE PRECISION,
    price_eur_foil      DOUBLE PRECISION,
    price_tix           DOUBLE PRECISION,
    prices_updated_at   TIMESTAMPTZ,
    fetched_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(set_code, arena_id)
);

CREATE INDEX IF NOT EXISTS idx_set_cards_arena_id   ON set_cards(arena_id);
CREATE INDEX IF NOT EXISTS idx_set_cards_set_code   ON set_cards(set_code);
CREATE INDEX IF NOT EXISTS idx_set_cards_scryfall_id ON set_cards(scryfall_id);
CREATE INDEX IF NOT EXISTS idx_set_cards_name        ON set_cards(name);
CREATE INDEX IF NOT EXISTS idx_set_cards_prices      ON set_cards(price_usd) WHERE price_usd IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_set_cards_legalities  ON set_cards(legalities) WHERE legalities IS NOT NULL;

-- Draft card ratings: 17Lands card performance data
CREATE TABLE IF NOT EXISTS draft_card_ratings (
    id              BIGSERIAL PRIMARY KEY,
    set_code        TEXT NOT NULL,
    draft_format    TEXT NOT NULL,
    arena_id        INTEGER NOT NULL,
    name            TEXT NOT NULL,
    color           TEXT,
    rarity          TEXT,
    gihwr           DOUBLE PRECISION,
    ohwr            DOUBLE PRECISION,
    alsa            DOUBLE PRECISION,
    ata             DOUBLE PRECISION,
    gih_count       INTEGER,
    data_source     TEXT NOT NULL DEFAULT 'api',
    url             TEXT,
    url_back        TEXT,
    cached_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(set_code, draft_format, arena_id)
);

CREATE INDEX IF NOT EXISTS idx_draft_card_ratings_set      ON draft_card_ratings(set_code, draft_format);
CREATE INDEX IF NOT EXISTS idx_draft_card_ratings_arena_id ON draft_card_ratings(arena_id);
CREATE INDEX IF NOT EXISTS idx_draft_card_ratings_gihwr    ON draft_card_ratings(gihwr DESC);

-- Draft color ratings: 17Lands color combination performance
CREATE TABLE IF NOT EXISTS draft_color_ratings (
    id                  BIGSERIAL PRIMARY KEY,
    set_code            TEXT NOT NULL,
    draft_format        TEXT NOT NULL,
    color_combination   TEXT NOT NULL,
    win_rate            DOUBLE PRECISION,
    games_played        INTEGER,
    data_source         TEXT NOT NULL DEFAULT 'api',
    cached_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(set_code, draft_format, color_combination)
);

CREATE INDEX IF NOT EXISTS idx_draft_color_ratings_set      ON draft_color_ratings(set_code, draft_format);
CREATE INDEX IF NOT EXISTS idx_draft_color_ratings_win_rate ON draft_color_ratings(win_rate DESC);

-- Dataset metadata: tracks 17Lands data freshness
CREATE TABLE IF NOT EXISTS dataset_metadata (
    id                  BIGSERIAL PRIMARY KEY,
    set_code            TEXT NOT NULL,
    draft_format        TEXT NOT NULL,
    data_source         TEXT NOT NULL,
    last_updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    total_cards         INTEGER,
    total_games         INTEGER,
    dataset_version     TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(set_code, draft_format)
);

CREATE INDEX IF NOT EXISTS idx_dataset_metadata_set     ON dataset_metadata(set_code, draft_format);
CREATE INDEX IF NOT EXISTS idx_dataset_metadata_updated ON dataset_metadata(last_updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_dataset_metadata_source  ON dataset_metadata(data_source);

-- CFB ratings: ChannelFireball card ratings
CREATE TABLE IF NOT EXISTS cfb_ratings (
    id                  BIGSERIAL PRIMARY KEY,
    card_name           TEXT NOT NULL,
    set_code            TEXT NOT NULL,
    arena_id            INTEGER,
    limited_rating      DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    limited_score       DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    constructed_rating  TEXT,
    constructed_score   DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    archetype_fit       TEXT,
    commentary          TEXT,
    source_url          TEXT,
    author              TEXT,
    imported_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(card_name, set_code)
);

CREATE INDEX IF NOT EXISTS idx_cfb_ratings_set_code  ON cfb_ratings(set_code);
CREATE INDEX IF NOT EXISTS idx_cfb_ratings_arena_id  ON cfb_ratings(arena_id);
CREATE INDEX IF NOT EXISTS idx_cfb_ratings_card_name ON cfb_ratings(card_name);

-- Card co-occurrence: cards appearing together in decks
CREATE TABLE IF NOT EXISTS card_cooccurrence (
    id              BIGSERIAL PRIMARY KEY,
    card_a_arena_id INTEGER NOT NULL,
    card_b_arena_id INTEGER NOT NULL,
    format          TEXT NOT NULL DEFAULT 'all',
    count           INTEGER NOT NULL DEFAULT 0,
    pmi_score       DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    last_updated    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(card_a_arena_id, card_b_arena_id, format)
);

CREATE INDEX IF NOT EXISTS idx_cooccurrence_card_a ON card_cooccurrence(card_a_arena_id, format);
CREATE INDEX IF NOT EXISTS idx_cooccurrence_card_b ON card_cooccurrence(card_b_arena_id, format);
CREATE INDEX IF NOT EXISTS idx_cooccurrence_format ON card_cooccurrence(format);
CREATE INDEX IF NOT EXISTS idx_cooccurrence_pmi    ON card_cooccurrence(pmi_score DESC);

-- Co-occurrence sources: deck sources for co-occurrence data
CREATE TABLE IF NOT EXISTS cooccurrence_sources (
    id          BIGSERIAL PRIMARY KEY,
    source_type TEXT NOT NULL,
    source_id   TEXT NOT NULL,
    format      TEXT NOT NULL DEFAULT 'all',
    deck_count  INTEGER NOT NULL DEFAULT 0,
    card_count  INTEGER NOT NULL DEFAULT 0,
    last_synced TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(source_type, source_id, format)
);

-- Card frequency: per-card frequency across analyzed decks
CREATE TABLE IF NOT EXISTS card_frequency (
    id            BIGSERIAL PRIMARY KEY,
    card_arena_id INTEGER NOT NULL,
    format        TEXT NOT NULL DEFAULT 'all',
    deck_count    INTEGER NOT NULL DEFAULT 0,
    total_decks   INTEGER NOT NULL DEFAULT 0,
    frequency     DOUBLE PRECISION GENERATED ALWAYS AS
                    (CASE WHEN total_decks = 0 THEN 0
                          ELSE CAST(deck_count AS DOUBLE PRECISION) / total_decks END) STORED,
    last_updated  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(card_arena_id, format)
);

CREATE INDEX IF NOT EXISTS idx_frequency_card   ON card_frequency(card_arena_id, format);
CREATE INDEX IF NOT EXISTS idx_frequency_format ON card_frequency(format);

-- Card embeddings: semantic similarity vectors (stored as JSON; upgrade to vector type when pgvector added)
CREATE TABLE IF NOT EXISTS card_embeddings (
    id                  BIGSERIAL PRIMARY KEY,
    arena_id            INTEGER NOT NULL UNIQUE,
    card_name           TEXT NOT NULL,
    embedding           TEXT NOT NULL,
    embedding_version   INTEGER NOT NULL DEFAULT 1,
    source              TEXT NOT NULL DEFAULT 'characteristics',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_card_embeddings_arena_id ON card_embeddings(arena_id);
CREATE INDEX IF NOT EXISTS idx_card_embeddings_name     ON card_embeddings(card_name);
CREATE INDEX IF NOT EXISTS idx_card_embeddings_version  ON card_embeddings(embedding_version);

-- Card similarity cache: pre-computed top-k similar cards
CREATE TABLE IF NOT EXISTS card_similarity_cache (
    id              BIGSERIAL PRIMARY KEY,
    card_arena_id   INTEGER NOT NULL,
    similar_arena_id INTEGER NOT NULL,
    similarity_score DOUBLE PRECISION NOT NULL,
    rank            INTEGER NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(card_arena_id, similar_arena_id)
);

CREATE INDEX IF NOT EXISTS idx_similarity_cache_card  ON card_similarity_cache(card_arena_id);
CREATE INDEX IF NOT EXISTS idx_similarity_cache_score ON card_similarity_cache(similarity_score DESC);

-- Card combination stats: how card pairs perform together
CREATE TABLE IF NOT EXISTS card_combination_stats (
    id                  BIGSERIAL PRIMARY KEY,
    card_id_1           INTEGER NOT NULL,
    card_id_2           INTEGER NOT NULL,
    deck_id             TEXT,
    format              TEXT NOT NULL DEFAULT 'Standard',
    games_together      INTEGER NOT NULL DEFAULT 0,
    games_card1_only    INTEGER NOT NULL DEFAULT 0,
    games_card2_only    INTEGER NOT NULL DEFAULT 0,
    wins_together       INTEGER NOT NULL DEFAULT 0,
    wins_card1_only     INTEGER NOT NULL DEFAULT 0,
    wins_card2_only     INTEGER NOT NULL DEFAULT 0,
    synergy_score       DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    confidence_score    DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(card_id_1, card_id_2, deck_id, format),
    CHECK(card_id_1 < card_id_2)
);

CREATE INDEX IF NOT EXISTS idx_combo_stats_card1   ON card_combination_stats(card_id_1);
CREATE INDEX IF NOT EXISTS idx_combo_stats_card2   ON card_combination_stats(card_id_2);
CREATE INDEX IF NOT EXISTS idx_combo_stats_deck    ON card_combination_stats(deck_id);
CREATE INDEX IF NOT EXISTS idx_combo_stats_format  ON card_combination_stats(format);
CREATE INDEX IF NOT EXISTS idx_combo_stats_synergy ON card_combination_stats(synergy_score DESC);

-- Card affinity: pre-computed synergy scores between card pairs
CREATE TABLE IF NOT EXISTS card_affinity (
    id              BIGSERIAL PRIMARY KEY,
    card_id_1       INTEGER NOT NULL,
    card_id_2       INTEGER NOT NULL,
    format          TEXT NOT NULL DEFAULT 'Standard',
    affinity_score  DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    sample_size     INTEGER NOT NULL DEFAULT 0,
    confidence      DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    source          TEXT NOT NULL DEFAULT 'historical',
    computed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(card_id_1, card_id_2, format),
    CHECK(card_id_1 < card_id_2)
);

CREATE INDEX IF NOT EXISTS idx_affinity_card1 ON card_affinity(card_id_1);
CREATE INDEX IF NOT EXISTS idx_affinity_card2 ON card_affinity(card_id_2);
CREATE INDEX IF NOT EXISTS idx_affinity_score ON card_affinity(affinity_score DESC);

-- Card individual stats: per-card win rate for synergy calculation
CREATE TABLE IF NOT EXISTS card_individual_stats (
    card_id     INTEGER NOT NULL,
    format      TEXT NOT NULL,
    total_games INTEGER NOT NULL DEFAULT 0,
    wins        INTEGER NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (card_id, format)
);

CREATE INDEX IF NOT EXISTS idx_card_individual_stats_format ON card_individual_stats(format);

-- Deck archetypes: archetype definitions and statistics
CREATE TABLE IF NOT EXISTS deck_archetypes (
    id                  BIGSERIAL PRIMARY KEY,
    name                TEXT NOT NULL,
    set_code            TEXT,
    format              TEXT NOT NULL,
    color_identity      TEXT NOT NULL,
    signature_cards     TEXT,
    synergy_patterns    TEXT,
    total_matches       INTEGER NOT NULL DEFAULT 0,
    total_wins          INTEGER NOT NULL DEFAULT 0,
    avg_win_rate        DOUBLE PRECISION,
    source              TEXT NOT NULL DEFAULT 'system'
                            CHECK(source IN ('system', '17lands', 'user', 'ml')),
    external_id         TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(name, set_code, format)
);

CREATE INDEX IF NOT EXISTS idx_deck_archetypes_set    ON deck_archetypes(set_code);
CREATE INDEX IF NOT EXISTS idx_deck_archetypes_format ON deck_archetypes(format);
CREATE INDEX IF NOT EXISTS idx_deck_archetypes_colors ON deck_archetypes(color_identity);

-- Archetype card weights: cards associated with archetypes
CREATE TABLE IF NOT EXISTS archetype_card_weights (
    id              BIGSERIAL PRIMARY KEY,
    archetype_id    BIGINT NOT NULL REFERENCES deck_archetypes(id) ON DELETE CASCADE,
    card_id         INTEGER NOT NULL,
    weight          DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    is_signature    BOOLEAN NOT NULL DEFAULT FALSE,
    source          TEXT NOT NULL DEFAULT 'system'
                        CHECK(source IN ('system', '17lands', 'user', 'ml')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(archetype_id, card_id)
);

CREATE INDEX IF NOT EXISTS idx_archetype_card_weights_archetype ON archetype_card_weights(archetype_id);
CREATE INDEX IF NOT EXISTS idx_archetype_card_weights_card      ON archetype_card_weights(card_id);

-- Archetype expected cards: commonly played cards per archetype
CREATE TABLE IF NOT EXISTS archetype_expected_cards (
    id              BIGSERIAL PRIMARY KEY,
    archetype_name  TEXT NOT NULL,
    format          TEXT NOT NULL,
    card_id         INTEGER NOT NULL,
    card_name       TEXT NOT NULL,
    inclusion_rate  DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    avg_copies      DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    is_signature    BOOLEAN NOT NULL DEFAULT FALSE,
    category        TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(archetype_name, format, card_id)
);

CREATE INDEX IF NOT EXISTS idx_expected_cards_archetype ON archetype_expected_cards(archetype_name);
CREATE INDEX IF NOT EXISTS idx_expected_cards_format    ON archetype_expected_cards(format);

-- ML model metadata: model versions and performance metrics
CREATE TABLE IF NOT EXISTS ml_model_metadata (
    id                  BIGSERIAL PRIMARY KEY,
    model_name          TEXT NOT NULL,
    model_version       TEXT NOT NULL,
    training_samples    INTEGER NOT NULL DEFAULT 0,
    training_date       TIMESTAMPTZ,
    accuracy            DOUBLE PRECISION,
    precision_score     DOUBLE PRECISION,
    recall              DOUBLE PRECISION,
    f1_score            DOUBLE PRECISION,
    is_active           BOOLEAN NOT NULL DEFAULT FALSE,
    model_data          BYTEA,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(model_name, model_version)
);

-- EDHREC synergy data
CREATE TABLE IF NOT EXISTS edhrec_synergy (
    id                  BIGSERIAL PRIMARY KEY,
    card_name           TEXT NOT NULL,
    synergy_card_name   TEXT NOT NULL,
    synergy_score       DOUBLE PRECISION NOT NULL,
    inclusion_count     INTEGER NOT NULL DEFAULT 0,
    num_decks           INTEGER NOT NULL DEFAULT 0,
    lift                DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    last_updated        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(card_name, synergy_card_name)
);

CREATE INDEX IF NOT EXISTS idx_edhrec_synergy_card  ON edhrec_synergy(card_name);
CREATE INDEX IF NOT EXISTS idx_edhrec_synergy_score ON edhrec_synergy(synergy_score DESC);

-- EDHREC card metadata
CREATE TABLE IF NOT EXISTS edhrec_card_metadata (
    id              BIGSERIAL PRIMARY KEY,
    card_name       TEXT NOT NULL UNIQUE,
    sanitized_name  TEXT NOT NULL,
    num_decks       INTEGER NOT NULL DEFAULT 0,
    salt_score      DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    color_identity  TEXT,
    last_updated    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_edhrec_metadata_name ON edhrec_card_metadata(card_name);

-- EDHREC theme cards
CREATE TABLE IF NOT EXISTS edhrec_theme_cards (
    id              BIGSERIAL PRIMARY KEY,
    theme_name      TEXT NOT NULL,
    card_name       TEXT NOT NULL,
    synergy_score   DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    is_top_card     BOOLEAN NOT NULL DEFAULT FALSE,
    is_high_synergy BOOLEAN NOT NULL DEFAULT FALSE,
    last_updated    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(theme_name, card_name)
);

CREATE INDEX IF NOT EXISTS idx_edhrec_theme_cards_theme ON edhrec_theme_cards(theme_name);
CREATE INDEX IF NOT EXISTS idx_edhrec_theme_cards_card  ON edhrec_theme_cards(card_name);

-- MTGZone archetypes
CREATE TABLE IF NOT EXISTS mtgzone_archetypes (
    id              BIGSERIAL PRIMARY KEY,
    name            TEXT NOT NULL,
    format          TEXT NOT NULL,
    tier            TEXT,
    description     TEXT,
    play_style      TEXT,
    source_url      TEXT,
    last_updated    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(name, format)
);

CREATE INDEX IF NOT EXISTS idx_mtgzone_archetypes_format ON mtgzone_archetypes(format);
CREATE INDEX IF NOT EXISTS idx_mtgzone_archetypes_tier   ON mtgzone_archetypes(tier);

-- MTGZone archetype core cards
CREATE TABLE IF NOT EXISTS mtgzone_archetype_cards (
    id              BIGSERIAL PRIMARY KEY,
    archetype_id    BIGINT NOT NULL REFERENCES mtgzone_archetypes(id) ON DELETE CASCADE,
    card_name       TEXT NOT NULL,
    role            TEXT NOT NULL,
    copies          INTEGER NOT NULL DEFAULT 4,
    importance      TEXT,
    notes           TEXT,
    last_updated    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(archetype_id, card_name)
);

CREATE INDEX IF NOT EXISTS idx_mtgzone_archetype_cards_archetype ON mtgzone_archetype_cards(archetype_id);
CREATE INDEX IF NOT EXISTS idx_mtgzone_archetype_cards_card      ON mtgzone_archetype_cards(card_name);
CREATE INDEX IF NOT EXISTS idx_mtgzone_archetype_cards_role      ON mtgzone_archetype_cards(role);

-- MTGZone synergies
CREATE TABLE IF NOT EXISTS mtgzone_synergies (
    id                  BIGSERIAL PRIMARY KEY,
    card_a              TEXT NOT NULL,
    card_b              TEXT NOT NULL,
    reason              TEXT NOT NULL,
    source_url          TEXT,
    archetype_context   TEXT,
    confidence          DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    last_updated        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(card_a, card_b, archetype_context)
);

CREATE INDEX IF NOT EXISTS idx_mtgzone_synergies_card_a ON mtgzone_synergies(card_a);
CREATE INDEX IF NOT EXISTS idx_mtgzone_synergies_card_b ON mtgzone_synergies(card_b);

-- MTGZone articles
CREATE TABLE IF NOT EXISTS mtgzone_articles (
    id              BIGSERIAL PRIMARY KEY,
    url             TEXT NOT NULL UNIQUE,
    title           TEXT NOT NULL,
    article_type    TEXT,
    format          TEXT,
    archetype       TEXT,
    published_at    TIMESTAMPTZ,
    processed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cards_mentioned TEXT
);

CREATE INDEX IF NOT EXISTS idx_mtgzone_articles_type   ON mtgzone_articles(article_type);
CREATE INDEX IF NOT EXISTS idx_mtgzone_articles_format ON mtgzone_articles(format);

-- Migration log: Scryfall card migration tracking
CREATE TABLE IF NOT EXISTS migration_log (
    id              BIGSERIAL PRIMARY KEY,
    migration_id    TEXT NOT NULL UNIQUE,
    old_scryfall_id TEXT NOT NULL,
    new_scryfall_id TEXT,
    strategy        TEXT NOT NULL CHECK(strategy IN ('merge', 'delete')),
    note            TEXT,
    performed_at    TIMESTAMPTZ NOT NULL,
    processed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_migration_log_migration_id    ON migration_log(migration_id);
CREATE INDEX IF NOT EXISTS idx_migration_log_old_scryfall_id ON migration_log(old_scryfall_id);
CREATE INDEX IF NOT EXISTS idx_migration_log_processed_at    ON migration_log(processed_at);

-- Metadata: application configuration and state (key-value)
CREATE TABLE IF NOT EXISTS metadata (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_metadata_updated_at ON metadata(updated_at);

-- Settings: user preferences (key-value, JSON values)
CREATE TABLE IF NOT EXISTS settings (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Processed log files: prevents duplicate log processing
CREATE TABLE IF NOT EXISTS processed_log_files (
    filename            TEXT PRIMARY KEY,
    processed_at        TIMESTAMPTZ NOT NULL,
    entry_count         INTEGER NOT NULL DEFAULT 0,
    matches_found       INTEGER NOT NULL DEFAULT 0,
    file_size_bytes     INTEGER NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_processed_log_files_processed_at ON processed_log_files(processed_at);

-- Standard config: rotation configuration (singleton, id must = 1)
CREATE TABLE IF NOT EXISTS standard_config (
    id                  INTEGER PRIMARY KEY CHECK (id = 1),
    next_rotation_date  TEXT NOT NULL,
    rotation_enabled    BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO standard_config (id, next_rotation_date, rotation_enabled)
VALUES (1, '2027-01-23', TRUE)
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- TENANT-SCOPED BASE TABLES
-- ---------------------------------------------------------------------------

-- Accounts: MTGA Arena accounts (multi-account support)
-- user_id added by migration 000051; kept nullable here for backwards compat
CREATE TABLE IF NOT EXISTS accounts (
    id              BIGSERIAL PRIMARY KEY,
    name            TEXT NOT NULL,
    screen_name     TEXT,
    client_id       TEXT,
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    daily_wins      INTEGER NOT NULL DEFAULT 0,
    weekly_wins     INTEGER NOT NULL DEFAULT 0,
    mastery_level   INTEGER NOT NULL DEFAULT 0,
    mastery_pass    TEXT NOT NULL DEFAULT 'Basic',
    mastery_max     INTEGER NOT NULL DEFAULT 80,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_accounts_is_default ON accounts(is_default);
-- Only one default account allowed
-- Note: is_default is stored as INTEGER (migration 000002 CREATE TABLE runs first).
-- Use = 1 not = TRUE to avoid "operator does not exist: integer = boolean".
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_default ON accounts(is_default) WHERE is_default = 1;

-- Matches: match results and metadata
CREATE TABLE IF NOT EXISTS matches (
    id                  TEXT PRIMARY KEY,
    account_id          BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    event_id            TEXT NOT NULL,
    event_name          TEXT NOT NULL,
    timestamp           TIMESTAMPTZ NOT NULL,
    duration_seconds    INTEGER,
    player_wins         INTEGER NOT NULL,
    opponent_wins       INTEGER NOT NULL,
    player_team_id      INTEGER NOT NULL,
    deck_id             TEXT,
    rank_before         TEXT,
    rank_after          TEXT,
    format              TEXT NOT NULL,
    result              TEXT NOT NULL CHECK(result IN ('win', 'loss')),
    result_reason       TEXT,
    opponent_name       TEXT,
    opponent_id         TEXT,
    notes               TEXT NOT NULL DEFAULT '',
    rating              INTEGER NOT NULL DEFAULT 0,
    processed_for_ml    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_matches_account_id           ON matches(account_id);
CREATE INDEX IF NOT EXISTS idx_matches_timestamp            ON matches(timestamp);
CREATE INDEX IF NOT EXISTS idx_matches_event_id             ON matches(event_id);
CREATE INDEX IF NOT EXISTS idx_matches_format               ON matches(format);
CREATE INDEX IF NOT EXISTS idx_matches_result               ON matches(result);
CREATE INDEX IF NOT EXISTS idx_matches_opponent_id          ON matches(opponent_id);
CREATE INDEX IF NOT EXISTS idx_matches_opponent_name        ON matches(opponent_name);
CREATE INDEX IF NOT EXISTS idx_matches_processed_for_ml     ON matches(processed_for_ml) WHERE processed_for_ml = FALSE;
-- Composite indexes for multi-tenant queries (account_id leading)
CREATE INDEX IF NOT EXISTS idx_matches_account_id_timestamp
    ON matches(account_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_matches_account_id_format
    ON matches(account_id, format);
CREATE INDEX IF NOT EXISTS idx_matches_account_id_format_timestamp
    ON matches(account_id, format, timestamp DESC);

-- Games: individual game results within a match
CREATE TABLE IF NOT EXISTS games (
    id              BIGSERIAL PRIMARY KEY,
    match_id        TEXT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    game_number     INTEGER NOT NULL,
    result          TEXT NOT NULL CHECK(result IN ('win', 'loss')),
    result_reason   TEXT,
    duration_seconds INTEGER,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(match_id, game_number)
);

CREATE INDEX IF NOT EXISTS idx_games_match_id     ON games(match_id);
CREATE INDEX IF NOT EXISTS idx_games_result_reason ON games(result_reason);

-- Game plays: individual card plays and actions during a game
CREATE TABLE IF NOT EXISTS game_plays (
    id              BIGSERIAL PRIMARY KEY,
    game_id         BIGINT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    match_id        TEXT NOT NULL,
    turn_number     INTEGER NOT NULL,
    phase           TEXT,
    step            TEXT,
    player_type     TEXT NOT NULL,
    action_type     TEXT NOT NULL,
    card_id         INTEGER,
    card_name       TEXT,
    zone_from       TEXT,
    zone_to         TEXT,
    life_from       INTEGER,
    life_to         INTEGER,
    timestamp       TIMESTAMPTZ NOT NULL,
    sequence_number INTEGER NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_game_plays_unique   ON game_plays(game_id, sequence_number);
CREATE INDEX IF NOT EXISTS idx_game_plays_game_id         ON game_plays(game_id);
CREATE INDEX IF NOT EXISTS idx_game_plays_match_id        ON game_plays(match_id);
CREATE INDEX IF NOT EXISTS idx_game_plays_turn            ON game_plays(game_id, turn_number);

-- Game state snapshots: board state at each turn
CREATE TABLE IF NOT EXISTS game_state_snapshots (
    id                      BIGSERIAL PRIMARY KEY,
    game_id                 BIGINT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    match_id                TEXT NOT NULL,
    turn_number             INTEGER NOT NULL,
    active_player           TEXT NOT NULL,
    player_life             INTEGER,
    opponent_life           INTEGER,
    player_cards_in_hand    INTEGER,
    opponent_cards_in_hand  INTEGER,
    player_lands_in_play    INTEGER,
    opponent_lands_in_play  INTEGER,
    board_state_json        TEXT,
    timestamp               TIMESTAMPTZ NOT NULL,
    UNIQUE(game_id, turn_number)
);

CREATE INDEX IF NOT EXISTS idx_game_snapshots_game_id  ON game_state_snapshots(game_id);
CREATE INDEX IF NOT EXISTS idx_game_snapshots_match_id ON game_state_snapshots(match_id);

-- Opponent cards observed: cards revealed by opponent during a game
CREATE TABLE IF NOT EXISTS opponent_cards_observed (
    id              BIGSERIAL PRIMARY KEY,
    game_id         BIGINT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    match_id        TEXT NOT NULL,
    card_id         INTEGER NOT NULL,
    card_name       TEXT,
    zone_observed   TEXT,
    turn_first_seen INTEGER,
    times_seen      INTEGER NOT NULL DEFAULT 1,
    UNIQUE(game_id, card_id)
);

CREATE INDEX IF NOT EXISTS idx_opponent_cards_game_id  ON opponent_cards_observed(game_id);
CREATE INDEX IF NOT EXISTS idx_opponent_cards_match_id ON opponent_cards_observed(match_id);

-- Player stats: aggregated statistics by date and format
CREATE TABLE IF NOT EXISTS player_stats (
    id              BIGSERIAL PRIMARY KEY,
    account_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    date            DATE NOT NULL,
    format          TEXT NOT NULL,
    matches_played  INTEGER NOT NULL DEFAULT 0,
    matches_won     INTEGER NOT NULL DEFAULT 0,
    games_played    INTEGER NOT NULL DEFAULT 0,
    games_won       INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(account_id, date, format)
);

CREATE INDEX IF NOT EXISTS idx_player_stats_account_id  ON player_stats(account_id);
CREATE INDEX IF NOT EXISTS idx_player_stats_date        ON player_stats(date);
CREATE INDEX IF NOT EXISTS idx_player_stats_format      ON player_stats(format);
-- Composite indexes for multi-tenant queries
CREATE INDEX IF NOT EXISTS idx_player_stats_account_id_date
    ON player_stats(account_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_player_stats_account_id_format_date
    ON player_stats(account_id, format, date DESC);

-- Rank history: rank progression over time
CREATE TABLE IF NOT EXISTS rank_history (
    id              BIGSERIAL PRIMARY KEY,
    account_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    timestamp       TIMESTAMPTZ NOT NULL,
    format          TEXT NOT NULL CHECK(format IN ('constructed', 'limited')),
    season_ordinal  INTEGER NOT NULL,
    rank_class      TEXT,
    rank_level      INTEGER,
    rank_step       INTEGER,
    percentile      DOUBLE PRECISION,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rank_history_account_id ON rank_history(account_id);
CREATE INDEX IF NOT EXISTS idx_rank_history_timestamp  ON rank_history(timestamp);
CREATE INDEX IF NOT EXISTS idx_rank_history_format     ON rank_history(format);
CREATE INDEX IF NOT EXISTS idx_rank_history_season     ON rank_history(season_ordinal);

-- Collection: player card collection (scoped by account)
CREATE TABLE IF NOT EXISTS collection (
    account_id  BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    card_id     INTEGER NOT NULL,
    quantity    INTEGER NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, card_id)
);

CREATE INDEX IF NOT EXISTS idx_collection_account_id ON collection(account_id);

-- Collection history: changes to collection over time
CREATE TABLE IF NOT EXISTS collection_history (
    id              BIGSERIAL PRIMARY KEY,
    account_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    card_id         INTEGER NOT NULL,
    quantity_delta  INTEGER NOT NULL,
    quantity_after  INTEGER NOT NULL,
    timestamp       TIMESTAMPTZ NOT NULL,
    source          TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_collection_history_account_id ON collection_history(account_id);
CREATE INDEX IF NOT EXISTS idx_collection_history_timestamp  ON collection_history(timestamp);
CREATE INDEX IF NOT EXISTS idx_collection_history_card_id    ON collection_history(card_id);

-- Currency history: gems and gold tracking
CREATE TABLE IF NOT EXISTS currency_history (
    id          BIGSERIAL PRIMARY KEY,
    account_id  BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    timestamp   TIMESTAMPTZ NOT NULL,
    gems        INTEGER NOT NULL,
    gold        INTEGER NOT NULL,
    gems_delta  INTEGER NOT NULL DEFAULT 0,
    gold_delta  INTEGER NOT NULL DEFAULT 0,
    source      TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_currency_history_account_id          ON currency_history(account_id);
CREATE INDEX IF NOT EXISTS idx_currency_history_timestamp           ON currency_history(timestamp);
CREATE INDEX IF NOT EXISTS idx_currency_history_account_timestamp   ON currency_history(account_id, timestamp);
-- Composite index for multi-tenant DESC queries
CREATE INDEX IF NOT EXISTS idx_currency_history_account_id_timestamp_desc
    ON currency_history(account_id, timestamp DESC);

-- Inventory: current wildcard / currency snapshot (one row per account)
CREATE TABLE IF NOT EXISTS inventory (
    id              BIGSERIAL PRIMARY KEY,
    account_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    gold            INTEGER NOT NULL DEFAULT 0,
    gems            INTEGER NOT NULL DEFAULT 0,
    wc_common       INTEGER NOT NULL DEFAULT 0,
    wc_uncommon     INTEGER NOT NULL DEFAULT 0,
    wc_rare         INTEGER NOT NULL DEFAULT 0,
    wc_mythic       INTEGER NOT NULL DEFAULT 0,
    vault_progress  DOUBLE PRECISION NOT NULL DEFAULT 0,
    draft_tokens    INTEGER NOT NULL DEFAULT 0,
    sealed_tokens   INTEGER NOT NULL DEFAULT 0,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Inventory change history
CREATE TABLE IF NOT EXISTS inventory_history (
    id              BIGSERIAL PRIMARY KEY,
    account_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    field           TEXT NOT NULL,
    previous_value  INTEGER NOT NULL,
    new_value       INTEGER NOT NULL,
    delta           INTEGER NOT NULL,
    source          TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Guard: inventory and inventory_history were created in migration 000023 without account_id.
-- Add the column if it is missing (CREATE TABLE IF NOT EXISTS above skips the column addition).
ALTER TABLE inventory         ADD COLUMN IF NOT EXISTS account_id BIGINT REFERENCES accounts(id) ON DELETE CASCADE;
ALTER TABLE inventory_history ADD COLUMN IF NOT EXISTS account_id BIGINT REFERENCES accounts(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_inventory_history_account_id ON inventory_history(account_id);
CREATE INDEX IF NOT EXISTS idx_inventory_history_field      ON inventory_history(field);
CREATE INDEX IF NOT EXISTS idx_inventory_history_created_at ON inventory_history(created_at);

-- Quests: daily quest progress and completion
CREATE TABLE IF NOT EXISTS quests (
    id                  BIGSERIAL PRIMARY KEY,
    account_id          BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    quest_id            TEXT NOT NULL,
    quest_type          TEXT,
    goal                INTEGER NOT NULL,
    starting_progress   INTEGER NOT NULL DEFAULT 0,
    ending_progress     INTEGER NOT NULL DEFAULT 0,
    completed           BOOLEAN NOT NULL DEFAULT FALSE,
    can_swap            BOOLEAN NOT NULL DEFAULT TRUE,
    rewards             TEXT,
    assigned_at         TIMESTAMPTZ NOT NULL,
    completed_at        TIMESTAMPTZ,
    rerolled            BOOLEAN NOT NULL DEFAULT FALSE,
    last_seen_at        TIMESTAMPTZ,
    session_id          TEXT,
    completion_source   TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(account_id, quest_id, assigned_at)
);

-- Guard: quests was created in migration 000010 without account_id.
ALTER TABLE quests ADD COLUMN IF NOT EXISTS account_id BIGINT REFERENCES accounts(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_quests_account_id  ON quests(account_id);
CREATE INDEX IF NOT EXISTS idx_quests_completed   ON quests(completed);
CREATE INDEX IF NOT EXISTS idx_quests_assigned_at ON quests(assigned_at);
CREATE INDEX IF NOT EXISTS idx_quests_completed_at ON quests(completed_at);
CREATE INDEX IF NOT EXISTS idx_quests_last_seen_at ON quests(last_seen_at);
CREATE INDEX IF NOT EXISTS idx_quests_session_id   ON quests(session_id);

-- Decks: player deck lists
CREATE TABLE IF NOT EXISTS decks (
    id                      TEXT PRIMARY KEY,
    account_id              BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name                    TEXT NOT NULL,
    format                  TEXT NOT NULL,
    description             TEXT,
    color_identity          TEXT,
    source                  TEXT NOT NULL DEFAULT 'constructed'
                                CHECK(source IN ('draft', 'constructed', 'imported', 'arena')),
    draft_event_id          TEXT,
    matches_played          INTEGER NOT NULL DEFAULT 0,
    matches_won             INTEGER NOT NULL DEFAULT 0,
    games_played            INTEGER NOT NULL DEFAULT 0,
    games_won               INTEGER NOT NULL DEFAULT 0,
    current_permutation_id  BIGINT,           -- FK added after deck_permutations is created
    is_app_created          BOOLEAN NOT NULL DEFAULT FALSE,
    created_method          TEXT NOT NULL DEFAULT 'imported',
    seed_card_id            INTEGER,
    created_at              TIMESTAMPTZ NOT NULL,
    modified_at             TIMESTAMPTZ NOT NULL,
    last_played             TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_decks_account_id         ON decks(account_id);
CREATE INDEX IF NOT EXISTS idx_decks_format             ON decks(format);
CREATE INDEX IF NOT EXISTS idx_decks_modified_at        ON decks(modified_at);
CREATE INDEX IF NOT EXISTS idx_decks_source             ON decks(source);
CREATE INDEX IF NOT EXISTS idx_decks_draft_event_id     ON decks(draft_event_id);
CREATE INDEX IF NOT EXISTS idx_decks_app_created        ON decks(is_app_created) WHERE is_app_created = TRUE;
CREATE INDEX IF NOT EXISTS idx_decks_created_method     ON decks(created_method);
-- Composite indexes for multi-tenant queries
CREATE INDEX IF NOT EXISTS idx_decks_account_id_modified_at
    ON decks(account_id, modified_at DESC);
CREATE INDEX IF NOT EXISTS idx_decks_account_id_format
    ON decks(account_id, format);

-- Deck permutations: deck version history
CREATE TABLE IF NOT EXISTS deck_permutations (
    id                      BIGSERIAL PRIMARY KEY,
    deck_id                 TEXT NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    parent_permutation_id   BIGINT REFERENCES deck_permutations(id) ON DELETE SET NULL,
    cards                   TEXT NOT NULL,
    card_hash               TEXT NOT NULL,
    version_number          INTEGER NOT NULL DEFAULT 1,
    version_name            TEXT,
    change_summary          TEXT,
    matches_played          INTEGER NOT NULL DEFAULT 0,
    matches_won             INTEGER NOT NULL DEFAULT 0,
    games_played            INTEGER NOT NULL DEFAULT 0,
    games_won               INTEGER NOT NULL DEFAULT 0,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_played_at          TIMESTAMPTZ,
    UNIQUE(deck_id, card_hash)
);

CREATE INDEX IF NOT EXISTS idx_deck_permutations_deck_id  ON deck_permutations(deck_id);
CREATE INDEX IF NOT EXISTS idx_deck_permutations_parent   ON deck_permutations(parent_permutation_id);
CREATE INDEX IF NOT EXISTS idx_deck_permutations_created  ON deck_permutations(deck_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_deck_permutations_win_rate ON deck_permutations(deck_id, matches_won, matches_played);
CREATE INDEX IF NOT EXISTS idx_deck_permutations_version  ON deck_permutations(deck_id, version_number);

-- Add FK from decks.current_permutation_id -> deck_permutations
ALTER TABLE decks
    ADD CONSTRAINT fk_decks_current_permutation
        FOREIGN KEY (current_permutation_id) REFERENCES deck_permutations(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_decks_current_permutation ON decks(current_permutation_id);

-- Deck cards: cards within a deck
CREATE TABLE IF NOT EXISTS deck_cards (
    id              BIGSERIAL PRIMARY KEY,
    deck_id         TEXT NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    card_id         INTEGER NOT NULL,
    quantity        INTEGER NOT NULL,
    board           TEXT NOT NULL CHECK(board IN ('main', 'sideboard')),
    from_draft_pick BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE(deck_id, card_id, board)
);

CREATE INDEX IF NOT EXISTS idx_deck_cards_deck_id ON deck_cards(deck_id);

-- Deck tags: categorizing decks
CREATE TABLE IF NOT EXISTS deck_tags (
    id          BIGSERIAL PRIMARY KEY,
    deck_id     TEXT NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    tag         TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(deck_id, tag)
);

CREATE INDEX IF NOT EXISTS idx_deck_tags_deck_id ON deck_tags(deck_id);
CREATE INDEX IF NOT EXISTS idx_deck_tags_tag     ON deck_tags(tag);

-- Deck notes: timestamped notes per deck
CREATE TABLE IF NOT EXISTS deck_notes (
    id          BIGSERIAL PRIMARY KEY,
    deck_id     TEXT NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    content     TEXT NOT NULL,
    category    TEXT NOT NULL DEFAULT 'general',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_deck_notes_deck_id  ON deck_notes(deck_id);
CREATE INDEX IF NOT EXISTS idx_deck_notes_category ON deck_notes(category);

-- Deck performance history: ML training data
CREATE TABLE IF NOT EXISTS deck_performance_history (
    id                      BIGSERIAL PRIMARY KEY,
    account_id              BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    deck_id                 TEXT NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    match_id                TEXT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    archetype               TEXT,
    secondary_archetype     TEXT,
    archetype_confidence    DOUBLE PRECISION,
    color_identity          TEXT NOT NULL,
    card_count              INTEGER NOT NULL,
    result                  TEXT NOT NULL CHECK(result IN ('win', 'loss')),
    games_won               INTEGER NOT NULL,
    games_lost              INTEGER NOT NULL,
    duration_seconds        INTEGER,
    format                  TEXT NOT NULL,
    event_type              TEXT,
    opponent_archetype      TEXT,
    rank_tier               TEXT,
    opponent_color_identity TEXT,
    opponent_confidence     DOUBLE PRECISION NOT NULL DEFAULT 0,
    opponent_cards_seen     INTEGER NOT NULL DEFAULT 0,
    match_timestamp         TIMESTAMPTZ NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_deck_perf_history_account   ON deck_performance_history(account_id);
CREATE INDEX IF NOT EXISTS idx_deck_perf_history_deck      ON deck_performance_history(deck_id);
CREATE INDEX IF NOT EXISTS idx_deck_perf_history_archetype ON deck_performance_history(archetype);
CREATE INDEX IF NOT EXISTS idx_deck_perf_history_format    ON deck_performance_history(format);
CREATE INDEX IF NOT EXISTS idx_deck_perf_history_timestamp ON deck_performance_history(match_timestamp);
CREATE INDEX IF NOT EXISTS idx_deck_perf_history_result    ON deck_performance_history(result);
CREATE INDEX IF NOT EXISTS idx_deck_perf_history_archetype_result
    ON deck_performance_history(archetype, result, format);
-- Composite indexes for multi-tenant queries
CREATE INDEX IF NOT EXISTS idx_deck_perf_history_account_id_timestamp
    ON deck_performance_history(account_id, match_timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_deck_perf_history_account_id_format
    ON deck_performance_history(account_id, format);

-- Improvement suggestions
CREATE TABLE IF NOT EXISTS improvement_suggestions (
    id              BIGSERIAL PRIMARY KEY,
    deck_id         TEXT NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    suggestion_type TEXT NOT NULL,
    priority        TEXT NOT NULL DEFAULT 'medium',
    title           TEXT NOT NULL,
    description     TEXT NOT NULL,
    evidence        TEXT,
    card_references TEXT,
    is_dismissed    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_suggestions_deck_id   ON improvement_suggestions(deck_id);
CREATE INDEX IF NOT EXISTS idx_suggestions_type      ON improvement_suggestions(suggestion_type);
CREATE INDEX IF NOT EXISTS idx_suggestions_dismissed ON improvement_suggestions(is_dismissed);

-- ML suggestions
CREATE TABLE IF NOT EXISTS ml_suggestions (
    id                          BIGSERIAL PRIMARY KEY,
    deck_id                     TEXT NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    suggestion_type             TEXT NOT NULL,
    card_id                     INTEGER,
    card_name                   TEXT,
    swap_for_card_id            INTEGER,
    swap_for_card_name          TEXT,
    confidence                  DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    expected_win_rate_change    DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    title                       TEXT NOT NULL,
    description                 TEXT,
    reasoning                   TEXT,
    evidence                    TEXT,
    is_dismissed                BOOLEAN NOT NULL DEFAULT FALSE,
    was_applied                 BOOLEAN NOT NULL DEFAULT FALSE,
    outcome_win_rate_change     DOUBLE PRECISION,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    applied_at                  TIMESTAMPTZ,
    outcome_recorded_at         TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ml_suggestions_deck       ON ml_suggestions(deck_id);
CREATE INDEX IF NOT EXISTS idx_ml_suggestions_type       ON ml_suggestions(suggestion_type);
CREATE INDEX IF NOT EXISTS idx_ml_suggestions_confidence ON ml_suggestions(confidence DESC);
CREATE INDEX IF NOT EXISTS idx_ml_suggestions_active     ON ml_suggestions(deck_id, is_dismissed);

-- Recommendation feedback: user responses to recommendations
CREATE TABLE IF NOT EXISTS recommendation_feedback (
    id                      BIGSERIAL PRIMARY KEY,
    account_id              BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    recommendation_type     TEXT NOT NULL
                                CHECK(recommendation_type IN ('card_pick', 'deck_card', 'archetype', 'sideboard')),
    recommendation_id       TEXT NOT NULL,
    recommended_card_id     INTEGER,
    recommended_archetype   TEXT,
    context_data            TEXT NOT NULL,
    action                  TEXT NOT NULL CHECK(action IN ('accepted', 'rejected', 'ignored', 'alternate')),
    alternate_choice_id     INTEGER,
    outcome_match_id        TEXT,
    outcome_result          TEXT CHECK(outcome_result IN ('win', 'loss')),
    recommendation_score    DOUBLE PRECISION,
    recommendation_rank     INTEGER,
    recommended_at          TIMESTAMPTZ NOT NULL,
    responded_at            TIMESTAMPTZ,
    outcome_recorded_at     TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rec_feedback_account   ON recommendation_feedback(account_id);
CREATE INDEX IF NOT EXISTS idx_rec_feedback_type      ON recommendation_feedback(recommendation_type);
CREATE INDEX IF NOT EXISTS idx_rec_feedback_action    ON recommendation_feedback(action);
CREATE INDEX IF NOT EXISTS idx_rec_feedback_timestamp ON recommendation_feedback(recommended_at);
CREATE INDEX IF NOT EXISTS idx_rec_feedback_card      ON recommendation_feedback(recommended_card_id);
CREATE INDEX IF NOT EXISTS idx_rec_feedback_type_action
    ON recommendation_feedback(recommendation_type, action, recommended_at);

-- User play patterns: aggregated play style for personalization
-- Note: account_id here is a text key (as in original), kept for compatibility.
-- In the multi-tenant model, this should be scoped to accounts.id BIGINT.
CREATE TABLE IF NOT EXISTS user_play_patterns (
    id                  BIGSERIAL PRIMARY KEY,
    account_id          TEXT NOT NULL UNIQUE,
    preferred_archetype TEXT,
    aggro_affinity      DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    midrange_affinity   DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    control_affinity    DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    combo_affinity      DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    color_preferences   TEXT,
    avg_game_length     DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    aggression_score    DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    interaction_score   DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    total_matches       INTEGER NOT NULL DEFAULT 0,
    total_decks         INTEGER NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_play_patterns_account ON user_play_patterns(account_id);

-- Draft sessions: draft event records
CREATE TABLE IF NOT EXISTS draft_sessions (
    id                          TEXT PRIMARY KEY,
    account_id                  BIGINT REFERENCES accounts(id) ON DELETE CASCADE,
    event_name                  TEXT NOT NULL,
    set_code                    TEXT NOT NULL,
    draft_type                  TEXT NOT NULL DEFAULT 'quick_draft',
    start_time                  TIMESTAMPTZ NOT NULL,
    end_time                    TIMESTAMPTZ,
    status                      TEXT NOT NULL DEFAULT 'in_progress',
    total_picks                 INTEGER NOT NULL DEFAULT 0,
    overall_grade               TEXT,
    overall_score               INTEGER,
    pick_quality_score          DOUBLE PRECISION,
    color_discipline_score      DOUBLE PRECISION,
    deck_composition_score      DOUBLE PRECISION,
    strategic_score             DOUBLE PRECISION,
    predicted_win_rate          DOUBLE PRECISION,
    predicted_win_rate_min      DOUBLE PRECISION,
    predicted_win_rate_max      DOUBLE PRECISION,
    prediction_factors          TEXT,
    predicted_at                TIMESTAMPTZ,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_draft_sessions_account_id ON draft_sessions(account_id);
CREATE INDEX IF NOT EXISTS idx_draft_sessions_status     ON draft_sessions(status);
CREATE INDEX IF NOT EXISTS idx_draft_sessions_set_code   ON draft_sessions(set_code);
CREATE INDEX IF NOT EXISTS idx_draft_sessions_start_time ON draft_sessions(start_time DESC);
CREATE INDEX IF NOT EXISTS idx_draft_sessions_grade      ON draft_sessions(overall_grade);
CREATE INDEX IF NOT EXISTS idx_draft_sessions_score      ON draft_sessions(overall_score DESC);
CREATE INDEX IF NOT EXISTS idx_draft_sessions_predicted_win_rate
    ON draft_sessions(predicted_win_rate) WHERE predicted_win_rate IS NOT NULL;
-- Composite indexes for multi-tenant queries
CREATE INDEX IF NOT EXISTS idx_draft_sessions_account_id_created_at
    ON draft_sessions(account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_draft_sessions_account_id_set_code
    ON draft_sessions(account_id, set_code);

-- Draft picks: individual pick decisions
CREATE TABLE IF NOT EXISTS draft_picks (
    id                  BIGSERIAL PRIMARY KEY,
    session_id          TEXT NOT NULL REFERENCES draft_sessions(id) ON DELETE CASCADE,
    pack_number         INTEGER NOT NULL,
    pick_number         INTEGER NOT NULL,
    card_id             TEXT NOT NULL,
    timestamp           TIMESTAMPTZ NOT NULL,
    pick_quality_grade  TEXT,
    pick_quality_rank   INTEGER,
    pack_best_gihwr     DOUBLE PRECISION,
    picked_card_gihwr   DOUBLE PRECISION,
    alternatives_json   TEXT,
    UNIQUE(session_id, pack_number, pick_number)
);

CREATE INDEX IF NOT EXISTS idx_draft_picks_session       ON draft_picks(session_id);
CREATE INDEX IF NOT EXISTS idx_draft_picks_timestamp     ON draft_picks(timestamp);
CREATE INDEX IF NOT EXISTS idx_draft_picks_quality_grade ON draft_picks(pick_quality_grade);

-- Draft packs: pack contents per pick
CREATE TABLE IF NOT EXISTS draft_packs (
    id          BIGSERIAL PRIMARY KEY,
    session_id  TEXT NOT NULL REFERENCES draft_sessions(id) ON DELETE CASCADE,
    pack_number INTEGER NOT NULL,
    pick_number INTEGER NOT NULL,
    card_ids    TEXT NOT NULL,
    timestamp   TIMESTAMPTZ NOT NULL,
    UNIQUE(session_id, pack_number, pick_number)
);

CREATE INDEX IF NOT EXISTS idx_draft_packs_session ON draft_packs(session_id);

-- Draft match results: links drafts to matches
CREATE TABLE IF NOT EXISTS draft_match_results (
    id              BIGSERIAL PRIMARY KEY,
    session_id      TEXT NOT NULL REFERENCES draft_sessions(id) ON DELETE CASCADE,
    match_id        TEXT NOT NULL,
    result          TEXT NOT NULL CHECK(result IN ('win', 'loss')),
    opponent_colors TEXT,
    game_wins       INTEGER NOT NULL DEFAULT 0,
    game_losses     INTEGER NOT NULL DEFAULT 0,
    match_timestamp TIMESTAMPTZ NOT NULL,
    UNIQUE(session_id, match_id)
);

CREATE INDEX IF NOT EXISTS idx_draft_match_results_session   ON draft_match_results(session_id);
CREATE INDEX IF NOT EXISTS idx_draft_match_results_timestamp ON draft_match_results(match_timestamp);

-- Draft archetype stats: archetype performance aggregation
CREATE TABLE IF NOT EXISTS draft_archetype_stats (
    id                  BIGSERIAL PRIMARY KEY,
    set_code            TEXT NOT NULL,
    color_combination   TEXT NOT NULL,
    archetype_name      TEXT NOT NULL,
    matches_played      INTEGER NOT NULL DEFAULT 0,
    matches_won         INTEGER NOT NULL DEFAULT 0,
    drafts_count        INTEGER NOT NULL DEFAULT 0,
    avg_draft_grade     DOUBLE PRECISION,
    last_played_at      TIMESTAMPTZ,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(set_code, color_combination)
);

CREATE INDEX IF NOT EXISTS idx_draft_archetype_stats_set ON draft_archetype_stats(set_code);

-- Draft community comparison: win rate vs community
CREATE TABLE IF NOT EXISTS draft_community_comparison (
    id                      BIGSERIAL PRIMARY KEY,
    set_code                TEXT NOT NULL,
    draft_format            TEXT NOT NULL,
    user_win_rate           DOUBLE PRECISION NOT NULL,
    community_avg_win_rate  DOUBLE PRECISION NOT NULL,
    percentile_rank         DOUBLE PRECISION,
    sample_size             INTEGER NOT NULL DEFAULT 0,
    calculated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(set_code, draft_format)
);

CREATE INDEX IF NOT EXISTS idx_draft_community_comparison_set ON draft_community_comparison(set_code);

-- Draft temporal trends: time-series win rate data
CREATE TABLE IF NOT EXISTS draft_temporal_trends (
    id              BIGSERIAL PRIMARY KEY,
    period_type     TEXT NOT NULL CHECK(period_type IN ('week', 'month')),
    period_start    DATE NOT NULL,
    period_end      DATE NOT NULL,
    set_code        TEXT NOT NULL DEFAULT '',
    drafts_count    INTEGER NOT NULL DEFAULT 0,
    matches_played  INTEGER NOT NULL DEFAULT 0,
    matches_won     INTEGER NOT NULL DEFAULT 0,
    avg_draft_grade DOUBLE PRECISION,
    calculated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(period_type, period_start, set_code)
);

CREATE INDEX IF NOT EXISTS idx_draft_temporal_trends_period ON draft_temporal_trends(period_type, period_start);

-- Draft pattern analysis: color/type preference cache
CREATE TABLE IF NOT EXISTS draft_pattern_analysis (
    id                      BIGSERIAL PRIMARY KEY,
    set_code                TEXT NOT NULL DEFAULT '',
    color_preference_json   TEXT,
    type_preference_json    TEXT,
    pick_order_pattern_json TEXT,
    archetype_affinity_json TEXT,
    sample_size             INTEGER NOT NULL DEFAULT 0,
    calculated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(set_code)
);

-- Opponent deck profiles: reconstructed opponent decks
CREATE TABLE IF NOT EXISTS opponent_deck_profiles (
    id                  BIGSERIAL PRIMARY KEY,
    match_id            TEXT NOT NULL UNIQUE REFERENCES matches(id) ON DELETE CASCADE,
    detected_archetype  TEXT,
    archetype_confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
    color_identity      TEXT NOT NULL,
    deck_style          TEXT,
    cards_observed      INTEGER NOT NULL DEFAULT 0,
    estimated_deck_size INTEGER NOT NULL DEFAULT 60,
    observed_card_ids   TEXT,
    inferred_card_ids   TEXT,
    signature_cards     TEXT,
    format              TEXT,
    meta_archetype_id   INTEGER,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_opponent_profiles_match_id  ON opponent_deck_profiles(match_id);
CREATE INDEX IF NOT EXISTS idx_opponent_profiles_archetype ON opponent_deck_profiles(detected_archetype);
CREATE INDEX IF NOT EXISTS idx_opponent_profiles_format    ON opponent_deck_profiles(format);

-- Matchup statistics: win rates against each archetype
CREATE TABLE IF NOT EXISTS matchup_statistics (
    id                  BIGSERIAL PRIMARY KEY,
    account_id          BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    player_archetype    TEXT NOT NULL,
    opponent_archetype  TEXT NOT NULL,
    format              TEXT NOT NULL,
    total_matches       INTEGER NOT NULL DEFAULT 0,
    wins                INTEGER NOT NULL DEFAULT 0,
    losses              INTEGER NOT NULL DEFAULT 0,
    avg_game_duration   INTEGER,
    last_match_at       TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(account_id, player_archetype, opponent_archetype, format)
);

CREATE INDEX IF NOT EXISTS idx_matchup_stats_account            ON matchup_statistics(account_id);
CREATE INDEX IF NOT EXISTS idx_matchup_stats_player_archetype   ON matchup_statistics(player_archetype);
CREATE INDEX IF NOT EXISTS idx_matchup_stats_opponent_archetype ON matchup_statistics(opponent_archetype);
CREATE INDEX IF NOT EXISTS idx_matchup_stats_format             ON matchup_statistics(format);
-- Composite indexes for multi-tenant queries
CREATE INDEX IF NOT EXISTS idx_matchup_stats_account_id_format
    ON matchup_statistics(account_id, format);
CREATE INDEX IF NOT EXISTS idx_matchup_stats_account_id_format_archetype
    ON matchup_statistics(account_id, format, player_archetype);

-- Default seed data
INSERT INTO settings (key, value) VALUES
    ('autoRefresh',       'false'),
    ('refreshInterval',   '30'),
    ('showNotifications', 'true'),
    ('theme',             '"dark"'),
    ('daemonPort',        '9999'),
    ('daemonMode',        '"standalone"')
ON CONFLICT (key) DO NOTHING;
