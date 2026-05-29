package datasets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/RdHamilton/vault-mtg/services/sync/internal/draftdata"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/scryfall"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/seventeenlands"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements Store using a pgxpool connection pool.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a PostgresStore wrapping the given pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// GetActiveSets returns sets where is_draft_active = TRUE.
// Each SyncSet carries the Scryfall Code (used as the DB key for all writes)
// and the ExpansionCode to send to 17Lands API requests.
// COALESCE(seventeenlands_code, code) means a NULL seventeenlands_code falls
// back to the Scryfall code — correct for the majority of sets.
func (s *PostgresStore) GetActiveSets(ctx context.Context) ([]SyncSet, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT code, COALESCE(seventeenlands_code, code) AS expansion_code
		FROM sets
		WHERE is_draft_active = TRUE
		ORDER BY code
	`)
	if err != nil {
		return nil, fmt.Errorf("query active sets: %w", err)
	}
	defer rows.Close()

	var sets []SyncSet
	for rows.Next() {
		var ss SyncSet
		if err := rows.Scan(&ss.Code, &ss.ExpansionCode); err != nil {
			return nil, fmt.Errorf("scan set: %w", err)
		}
		sets = append(sets, ss)
	}

	return sets, rows.Err()
}

// UpsertRatings replaces all card ratings for the given set/format in draft_card_ratings.
// It deletes existing rows for the set/format and inserts fresh rows in a single transaction,
// avoiding the arena_id=0 conflict that would collapse all cards into one row.
//
// If ratings.FetchedAt is zero (caller forgot to set it), time.Now().UTC() is used as a
// defensive fallback so that cached_at is never stored as 0001-01-01 in Postgres — which
// would cause the BFF staleness check to always fire X-Cache-Degraded: true.
func (s *PostgresStore) UpsertRatings(ctx context.Context, ratings draftdata.SetRatings) error {
	if ratings.FetchedAt.IsZero() {
		ratings.FetchedAt = time.Now().UTC()
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			log.Printf("[sync] UpsertRatings: rollback error: %v", err)
		}
	}()

	if _, err := tx.Exec(
		ctx,
		`DELETE FROM draft_card_ratings WHERE set_code = $1 AND draft_format = $2`,
		ratings.SetCode,
		ratings.DraftFormat,
	); err != nil {
		return fmt.Errorf("delete stale ratings for %s/%s: %w", ratings.SetCode, ratings.DraftFormat, err)
	}

	const insertQuery = `
		INSERT INTO draft_card_ratings (set_code, draft_format, arena_id, name, gihwr, ohwr, alsa, ata, gih_count, cached_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	for _, card := range ratings.Cards {
		if card.MtgaID == 0 {
			log.Printf("[sync] skipping paper-only card %q (no arena ID)", card.Name)
			continue
		}

		if _, err := tx.Exec(
			ctx, insertQuery,
			ratings.SetCode,
			ratings.DraftFormat,
			card.MtgaID,
			card.Name,
			card.GIHWR,
			card.OHW,
			card.ALSA,
			card.ATA,
			card.SeenCount,
			ratings.FetchedAt,
		); err != nil {
			return fmt.Errorf("insert card %q: %w", card.Name, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	log.Printf("[sync] UpsertRatings: inserted %d rows for %s/%s", len(ratings.Cards), ratings.SetCode, ratings.DraftFormat)
	return nil
}

// UpsertSets upserts set metadata into the sets table and marks each set as
// draft-active (is_draft_active = TRUE). Existing rows are updated via ON CONFLICT;
// rows not present in the incoming slice are left unchanged (they may have been
// deactivated manually or by a prior migration).
// Note: is_standard_legal is intentionally not touched here — Standard legality
// is a separate concept from draft availability and is managed by BFF migrations.
func (s *PostgresStore) UpsertSets(ctx context.Context, sets []scryfall.ScryfallSet) error {
	const q = `
		INSERT INTO sets (code, name, released_at, set_type, card_count, is_draft_active, last_updated)
		VALUES ($1, $2, $3, $4, $5, TRUE, NOW())
		ON CONFLICT (code) DO UPDATE SET
			name            = EXCLUDED.name,
			released_at     = EXCLUDED.released_at,
			set_type        = EXCLUDED.set_type,
			card_count      = EXCLUDED.card_count,
			is_draft_active = TRUE,
			last_updated    = NOW()
	`

	for _, set := range sets {
		if _, err := s.pool.Exec(
			ctx, q,
			set.Code,
			set.Name,
			set.ReleasedAt,
			set.SetType,
			set.CardCount,
		); err != nil {
			return fmt.Errorf("upsert set %q: %w", set.Code, err)
		}
	}

	log.Printf("[sync] UpsertSets: upserted %d sets", len(sets))
	return nil
}

// UpsertColorRatings replaces all color-combination ratings for the given
// set/format in draft_color_ratings using a DELETE + batch INSERT transaction.
func (s *PostgresStore) UpsertColorRatings(ctx context.Context, setCode, draftFormat string, ratings []seventeenlands.ColorRating) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			log.Printf("[sync] UpsertColorRatings: rollback error: %v", err)
		}
	}()

	if _, err := tx.Exec(
		ctx,
		`DELETE FROM draft_color_ratings WHERE set_code = $1 AND draft_format = $2`,
		setCode,
		draftFormat,
	); err != nil {
		return fmt.Errorf("delete stale color ratings for %s/%s: %w", setCode, draftFormat, err)
	}

	const insertQuery = `
		INSERT INTO draft_color_ratings (set_code, draft_format, color_combination, win_rate, games_played, cached_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
	`

	for _, r := range ratings {
		if r.ShortName == "" {
			continue
		}

		if _, err := tx.Exec(
			ctx, insertQuery,
			setCode,
			draftFormat,
			r.ShortName,
			r.WinRate(),
			r.Games,
		); err != nil {
			return fmt.Errorf("insert color rating %q: %w", r.ShortName, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	log.Printf("[sync] UpsertColorRatings: inserted %d rows for %s/%s", len(ratings), setCode, draftFormat)

	return nil
}

// UpsertSetCards upserts per-set card entries into the set_cards table keyed on
// (set_code, arena_id). arena_id in set_cards is TEXT, so the integer ArenaID
// from ScryfallCard is cast via fmt.Sprintf. The upsert updates all mutable
// fields on conflict — no WHERE guard (always-upsert for v1).
// image_url_small and image_url_art are written from the "small" and "art_crop"
// keys in the ImageURIs map using the same extractImageURLKey helper as image_url.
// Price columns (price_usd, price_eur, etc.) have no Scryfall source and are
// left null — they are populated by the separate price-sync path.
func (s *PostgresStore) UpsertSetCards(ctx context.Context, cards []scryfall.ScryfallCard) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			log.Printf("[sync] UpsertSetCards: rollback error: %v", err)
		}
	}()

	const q = `
		INSERT INTO set_cards (
			set_code, arena_id, scryfall_id, name, mana_cost, cmc, types,
			colors, rarity, text, power, toughness, image_url, image_url_small,
			image_url_art, legalities, fetched_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13, $14,
			$15, $16, NOW()
		)
		ON CONFLICT (set_code, arena_id) DO UPDATE SET
			scryfall_id     = EXCLUDED.scryfall_id,
			name            = EXCLUDED.name,
			mana_cost       = EXCLUDED.mana_cost,
			cmc             = EXCLUDED.cmc,
			types           = EXCLUDED.types,
			colors          = EXCLUDED.colors,
			rarity          = EXCLUDED.rarity,
			text            = EXCLUDED.text,
			power           = EXCLUDED.power,
			toughness       = EXCLUDED.toughness,
			image_url       = EXCLUDED.image_url,
			image_url_small = EXCLUDED.image_url_small,
			image_url_art   = EXCLUDED.image_url_art,
			legalities      = EXCLUDED.legalities,
			fetched_at      = NOW()
	`

	inserted := 0
	for i := range cards {
		c := &cards[i]
		if c.ArenaID == nil {
			continue
		}

		// set_cards.arena_id is TEXT; cast integer ArenaID to string.
		arenaIDText := fmt.Sprintf("%d", *c.ArenaID)

		colorsJSON, err := json.Marshal(c.Colors)
		if err != nil {
			return fmt.Errorf("marshal colors for set_card %q: %w", c.Name, err)
		}
		legalitiesJSON, err := json.Marshal(c.Legalities)
		if err != nil {
			return fmt.Errorf("marshal legalities for set_card %q: %w", c.Name, err)
		}

		// Extract image URLs from the ImageURIs map using the shared helper.
		imageURL := extractImageURL(c.ImageURIs)
		imageURLSmall := extractImageURLKey(c.ImageURIs, "small")
		imageURLArt := extractImageURLKey(c.ImageURIs, "art_crop")

		if _, err := tx.Exec(
			ctx, q,
			c.SetCode,
			arenaIDText,
			c.ScryfallID,
			c.Name,
			c.ManaCost,
			int(c.CMC),
			c.TypeLine,
			string(colorsJSON),
			c.Rarity,
			c.OracleText,
			c.Power,
			c.Toughness,
			imageURL,
			imageURLSmall,
			imageURLArt,
			string(legalitiesJSON),
		); err != nil {
			return fmt.Errorf("upsert set_card %q (set=%s arena_id=%s): %w", c.Name, c.SetCode, arenaIDText, err)
		}
		inserted++
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	log.Printf("[sync] UpsertSetCards: upserted %d set_cards", inserted)
	return nil
}

// extractImageURL pulls the "normal" image URL out of the ImageURIs field.
// ImageURIs is decoded as any (interface{}) from JSON, so a type assertion
// is used to avoid a secondary parse. Returns empty string if absent.
func extractImageURL(imageURIs any) string {
	return extractImageURLKey(imageURIs, "normal")
}

// extractImageURLKey pulls the named key's URL out of the ImageURIs field.
// ImageURIs is decoded as any (interface{}) from JSON. Returns empty string
// when the field is absent, not a map, or the key is missing.
func extractImageURLKey(imageURIs any, key string) string {
	if imageURIs == nil {
		return ""
	}
	m, ok := imageURIs.(map[string]any)
	if !ok {
		return ""
	}
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetHash returns the stored hash for the given key, or ("", nil) if none exists.
func (s *PostgresStore) GetHash(ctx context.Context, key string) (string, error) {
	var hash string
	err := s.pool.QueryRow(ctx, `SELECT hash FROM sync_hashes WHERE key = $1`, key).Scan(&hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}

		return "", fmt.Errorf("get hash %q: %w", key, err)
	}

	return hash, nil
}

// SetHash upserts the hash for the given key.
func (s *PostgresStore) SetHash(ctx context.Context, key string, hash string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sync_hashes (key, hash, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET
			hash       = EXCLUDED.hash,
			updated_at = NOW()
	`, key, hash)
	if err != nil {
		return fmt.Errorf("set hash %q: %w", key, err)
	}

	return nil
}

// GetRatings retrieves all card ratings for a set/format combination.
func (s *PostgresStore) GetRatings(ctx context.Context, setCode, draftFormat string) (*draftdata.SetRatings, error) {
	const query = `
		SELECT name, gihwr, ohwr, alsa, ata, gih_count, cached_at
		FROM draft_card_ratings
		WHERE set_code = $1 AND draft_format = $2
		ORDER BY name
	`

	rows, err := s.pool.Query(ctx, query, setCode, draftFormat)
	if err != nil {
		return nil, fmt.Errorf("query ratings: %w", err)
	}
	defer rows.Close()

	result := &draftdata.SetRatings{
		SetCode:     setCode,
		DraftFormat: draftFormat,
	}

	for rows.Next() {
		var card seventeenlands.CardRating
		if err := rows.Scan(
			&card.Name,
			&card.GIHWR,
			&card.OHW,
			&card.ALSA,
			&card.ATA,
			&card.SeenCount,
			&result.FetchedAt,
		); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		result.Cards = append(result.Cards, card)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}

	return result, nil
}
