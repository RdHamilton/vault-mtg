package datasets

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ramonehamilton/mtga-sync/internal/draftdata"
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

// UpsertRatings inserts or updates card ratings for the given set in draft_card_ratings.
func (s *PostgresStore) UpsertRatings(ctx context.Context, ratings draftdata.SetRatings) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const query = `
		INSERT INTO draft_card_ratings (set_code, draft_format, arena_id, name, gihwr, ohwr, alsa, ata, gih_count, cached_at)
		VALUES ($1, $2, 0, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (set_code, draft_format, arena_id)
		DO UPDATE SET
			name      = EXCLUDED.name,
			gihwr     = EXCLUDED.gihwr,
			ohwr      = EXCLUDED.ohwr,
			alsa      = EXCLUDED.alsa,
			ata       = EXCLUDED.ata,
			gih_count = EXCLUDED.gih_count,
			cached_at = EXCLUDED.cached_at
	`

	for _, card := range ratings.Cards {
		if _, err := tx.Exec(ctx, query,
			ratings.SetCode,
			ratings.DraftFormat,
			card.Name,
			card.GIHWR,
			card.OHW,
			card.ALSA,
			card.ATA,
			card.SeenCount,
			ratings.FetchedAt,
		); err != nil {
			return fmt.Errorf("upsert card %q: %w", card.Name, err)
		}
	}

	return tx.Commit(ctx)
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
