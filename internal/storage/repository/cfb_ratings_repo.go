package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
)

// CFBRatingsRepository handles database operations for ChannelFireball card ratings.
type CFBRatingsRepository interface {
	// GetRating retrieves a CFB rating by card name and set code.
	GetRating(ctx context.Context, cardName, setCode string) (*models.CFBRating, error)

	// GetRatingByArenaID retrieves a CFB rating by Arena ID.
	GetRatingByArenaID(ctx context.Context, arenaID int) (*models.CFBRating, error)

	// GetRatingsForSet retrieves all CFB ratings for a set.
	GetRatingsForSet(ctx context.Context, setCode string) ([]*models.CFBRating, error)

	// UpsertRating inserts or updates a CFB rating.
	UpsertRating(ctx context.Context, rating *models.CFBRating) error

	// BulkUpsertRatings inserts or updates multiple CFB ratings.
	BulkUpsertRatings(ctx context.Context, ratings []*models.CFBRating) error

	// DeleteRatingsForSet deletes all CFB ratings for a set.
	DeleteRatingsForSet(ctx context.Context, setCode string) error

	// GetRatingsCount returns the number of CFB ratings for a set.
	GetRatingsCount(ctx context.Context, setCode string) (int, error)

	// LinkArenaIDs updates Arena IDs for ratings based on card name matching.
	// Returns the number of ratings that were linked.
	LinkArenaIDs(ctx context.Context, setCode string, cardNameToArenaID map[string]int) (int, error)
}

// cfbRatingsRepository is the concrete implementation of CFBRatingsRepository.
type cfbRatingsRepository struct {
	db *sql.DB
}

// NewCFBRatingsRepository creates a new CFB ratings repository.
func NewCFBRatingsRepository(db *sql.DB) CFBRatingsRepository {
	return &cfbRatingsRepository{db: db}
}

// GetRating retrieves a CFB rating by card name and set code.
func (r *cfbRatingsRepository) GetRating(ctx context.Context, cardName, setCode string) (*models.CFBRating, error) {
	query := `
		SELECT id, card_name, set_code, arena_id, limited_rating, limited_score,
		       constructed_rating, constructed_score, archetype_fit, commentary,
		       source_url, author, imported_at, updated_at
		FROM cfb_ratings
		WHERE card_name = $1 AND set_code = $2
	`

	rating := &models.CFBRating{}
	var arenaID sql.NullInt64
	var archetypeFit, commentary, sourceURL, author sql.NullString

	err := r.db.QueryRowContext(ctx, query, cardName, setCode).Scan(
		&rating.ID,
		&rating.CardName,
		&rating.SetCode,
		&arenaID,
		&rating.LimitedRating,
		&rating.LimitedScore,
		&rating.ConstructedRating,
		&rating.ConstructedScore,
		&archetypeFit,
		&commentary,
		&sourceURL,
		&author,
		&rating.ImportedAt,
		&rating.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if arenaID.Valid {
		id := int(arenaID.Int64)
		rating.ArenaID = &id
	}
	if archetypeFit.Valid {
		rating.ArchetypeFit = archetypeFit.String
	}
	if commentary.Valid {
		rating.Commentary = commentary.String
	}
	if sourceURL.Valid {
		rating.SourceURL = sourceURL.String
	}
	if author.Valid {
		rating.Author = author.String
	}

	return rating, nil
}

// GetRatingByArenaID retrieves a CFB rating by Arena ID.
func (r *cfbRatingsRepository) GetRatingByArenaID(ctx context.Context, arenaID int) (*models.CFBRating, error) {
	query := `
		SELECT id, card_name, set_code, arena_id, limited_rating, limited_score,
		       constructed_rating, constructed_score, archetype_fit, commentary,
		       source_url, author, imported_at, updated_at
		FROM cfb_ratings
		WHERE arena_id = $1
	`

	rating := &models.CFBRating{}
	var arenaIDVal sql.NullInt64
	var archetypeFit, commentary, sourceURL, author sql.NullString

	err := r.db.QueryRowContext(ctx, query, arenaID).Scan(
		&rating.ID,
		&rating.CardName,
		&rating.SetCode,
		&arenaIDVal,
		&rating.LimitedRating,
		&rating.LimitedScore,
		&rating.ConstructedRating,
		&rating.ConstructedScore,
		&archetypeFit,
		&commentary,
		&sourceURL,
		&author,
		&rating.ImportedAt,
		&rating.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if arenaIDVal.Valid {
		id := int(arenaIDVal.Int64)
		rating.ArenaID = &id
	}
	if archetypeFit.Valid {
		rating.ArchetypeFit = archetypeFit.String
	}
	if commentary.Valid {
		rating.Commentary = commentary.String
	}
	if sourceURL.Valid {
		rating.SourceURL = sourceURL.String
	}
	if author.Valid {
		rating.Author = author.String
	}

	return rating, nil
}

// GetRatingsForSet retrieves all CFB ratings for a set.
func (r *cfbRatingsRepository) GetRatingsForSet(ctx context.Context, setCode string) ([]*models.CFBRating, error) {
	query := `
		SELECT id, card_name, set_code, arena_id, limited_rating, limited_score,
		       constructed_rating, constructed_score, archetype_fit, commentary,
		       source_url, author, imported_at, updated_at
		FROM cfb_ratings
		WHERE set_code = $1
		ORDER BY card_name
	`

	rows, err := r.db.QueryContext(ctx, query, setCode)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return r.scanRatings(rows)
}

// UpsertRating inserts or updates a CFB rating.
func (r *cfbRatingsRepository) UpsertRating(ctx context.Context, rating *models.CFBRating) error {
	query := `
		INSERT INTO cfb_ratings (
			card_name, set_code, arena_id, limited_rating, limited_score,
			constructed_rating, constructed_score, archetype_fit, commentary,
			source_url, author, imported_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT(card_name, set_code) DO UPDATE SET
			arena_id = excluded.arena_id,
			limited_rating = excluded.limited_rating,
			limited_score = excluded.limited_score,
			constructed_rating = excluded.constructed_rating,
			constructed_score = excluded.constructed_score,
			archetype_fit = excluded.archetype_fit,
			commentary = excluded.commentary,
			source_url = excluded.source_url,
			author = excluded.author,
			updated_at = excluded.updated_at
	`

	now := time.Now().UTC()
	if rating.ImportedAt.IsZero() {
		rating.ImportedAt = now
	}
	rating.UpdatedAt = now

	var arenaID interface{}
	if rating.ArenaID != nil {
		arenaID = *rating.ArenaID
	}

	queryWithReturning := query + " RETURNING id"
	err := r.db.QueryRowContext(ctx, queryWithReturning,
		rating.CardName,
		rating.SetCode,
		arenaID,
		rating.LimitedRating,
		rating.LimitedScore,
		rating.ConstructedRating,
		rating.ConstructedScore,
		nullIfEmpty(rating.ArchetypeFit),
		nullIfEmpty(rating.Commentary),
		nullIfEmpty(rating.SourceURL),
		nullIfEmpty(rating.Author),
		rating.ImportedAt.UTC(),
		rating.UpdatedAt.UTC(),
	).Scan(&rating.ID)
	if err != nil {
		return err
	}

	return nil
}

// BulkUpsertRatings inserts or updates multiple CFB ratings.
func (r *cfbRatingsRepository) BulkUpsertRatings(ctx context.Context, ratings []*models.CFBRating) error {
	for _, rating := range ratings {
		if err := r.UpsertRating(ctx, rating); err != nil {
			return err
		}
	}
	return nil
}

// DeleteRatingsForSet deletes all CFB ratings for a set.
func (r *cfbRatingsRepository) DeleteRatingsForSet(ctx context.Context, setCode string) error {
	query := `DELETE FROM cfb_ratings WHERE set_code = $1`
	_, err := r.db.ExecContext(ctx, query, setCode)
	return err
}

// GetRatingsCount returns the number of CFB ratings for a set.
func (r *cfbRatingsRepository) GetRatingsCount(ctx context.Context, setCode string) (int, error) {
	query := `SELECT COUNT(*) FROM cfb_ratings WHERE set_code = $1`
	var count int
	err := r.db.QueryRowContext(ctx, query, setCode).Scan(&count)
	return count, err
}

// LinkArenaIDs updates Arena IDs for ratings based on card name matching.
// Uses case-insensitive matching and skips already-linked ratings.
// Returns the number of ratings that were linked.
func (r *cfbRatingsRepository) LinkArenaIDs(ctx context.Context, setCode string, cardNameToArenaID map[string]int) (int, error) {
	query := `
		UPDATE cfb_ratings
		SET arena_id = $1
		WHERE LOWER(TRIM(card_name)) = $2 AND set_code = $3 AND arena_id IS NULL
	`

	linked := 0
	for cardName, arenaID := range cardNameToArenaID {
		// Normalize the card name to match the query
		normalizedName := strings.TrimSpace(strings.ToLower(cardName))
		result, err := r.db.ExecContext(ctx, query, arenaID, normalizedName, setCode)
		if err != nil {
			return linked, err
		}
		rowsAffected, _ := result.RowsAffected()
		linked += int(rowsAffected)
	}

	return linked, nil
}

// scanRatings scans multiple CFB rating rows.
func (r *cfbRatingsRepository) scanRatings(rows *sql.Rows) ([]*models.CFBRating, error) {
	var ratings []*models.CFBRating

	for rows.Next() {
		rating := &models.CFBRating{}
		var arenaID sql.NullInt64
		var archetypeFit, commentary, sourceURL, author sql.NullString

		err := rows.Scan(
			&rating.ID,
			&rating.CardName,
			&rating.SetCode,
			&arenaID,
			&rating.LimitedRating,
			&rating.LimitedScore,
			&rating.ConstructedRating,
			&rating.ConstructedScore,
			&archetypeFit,
			&commentary,
			&sourceURL,
			&author,
			&rating.ImportedAt,
			&rating.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if arenaID.Valid {
			id := int(arenaID.Int64)
			rating.ArenaID = &id
		}
		if archetypeFit.Valid {
			rating.ArchetypeFit = archetypeFit.String
		}
		if commentary.Valid {
			rating.Commentary = commentary.String
		}
		if sourceURL.Valid {
			rating.SourceURL = sourceURL.String
		}
		if author.Valid {
			rating.Author = author.String
		}

		ratings = append(ratings, rating)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ratings, nil
}

// nullIfEmpty returns nil if the string is empty, otherwise returns the string.
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
