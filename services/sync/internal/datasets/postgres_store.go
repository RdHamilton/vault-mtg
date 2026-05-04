package datasets

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ramonehamilton/mtga-sync/internal/draftdata"
	"github.com/ramonehamilton/mtga-sync/internal/scryfall"
	"github.com/ramonehamilton/mtga-sync/internal/seventeenlands"
)

// PostgresStore implements Store using a pgxpool connection pool.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a PostgresStore wrapping the given pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// GetActiveSets returns set codes where is_standard_legal = TRUE.
func (s *PostgresStore) GetActiveSets(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT code FROM sets WHERE is_standard_legal = TRUE ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("query active sets: %w", err)
	}
	defer rows.Close()

	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("scan code: %w", err)
		}
		codes = append(codes, code)
	}

	return codes, rows.Err()
}

// UpsertRatings replaces all card ratings for the given set/format in draft_card_ratings.
// It deletes existing rows for the set/format and inserts fresh rows in a single transaction,
// avoiding the arena_id=0 conflict that would collapse all cards into one row.
func (s *PostgresStore) UpsertRatings(ctx context.Context, ratings draftdata.SetRatings) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
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

		if _, err := tx.Exec(ctx, insertQuery,
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
// standard legal. Existing rows are updated via ON CONFLICT; rows not present
// in the incoming slice are left unchanged (they may have been rotated out
// manually or by a prior migration).
func (s *PostgresStore) UpsertSets(ctx context.Context, sets []scryfall.ScryfallSet) error {
	const q = `
		INSERT INTO sets (code, name, released_at, set_type, card_count, is_standard_legal, last_updated)
		VALUES ($1, $2, $3, $4, $5, TRUE, NOW())
		ON CONFLICT (code) DO UPDATE SET
			name              = EXCLUDED.name,
			released_at       = EXCLUDED.released_at,
			set_type          = EXCLUDED.set_type,
			card_count        = EXCLUDED.card_count,
			is_standard_legal = TRUE,
			last_updated      = NOW()
	`

	for _, set := range sets {
		if _, err := s.pool.Exec(ctx, q,
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
