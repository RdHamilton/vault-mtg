package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
)

// RankHistoryRepository defines the interface for rank history data operations.
type RankHistoryRepository interface {
	// Create stores a new rank snapshot in the database.
	Create(ctx context.Context, rank *models.RankHistory) error

	// GetByFormat retrieves all rank history entries for a specific format.
	GetByFormat(ctx context.Context, accountID int, format string) ([]*models.RankHistory, error)

	// GetBySeason retrieves all rank history entries for a specific season.
	GetBySeason(ctx context.Context, accountID int, seasonOrdinal int) ([]*models.RankHistory, error)

	// GetByDateRange retrieves rank history entries within a date range.
	GetByDateRange(ctx context.Context, accountID int, startDate, endDate time.Time) ([]*models.RankHistory, error)

	// GetLatestByFormat retrieves the most recent rank snapshot for a format.
	GetLatestByFormat(ctx context.Context, accountID int, format string) (*models.RankHistory, error)

	// GetAll retrieves all rank history entries for an account.
	GetAll(ctx context.Context, accountID int) ([]*models.RankHistory, error)
}

type rankHistoryRepository struct {
	db *sql.DB
}

// NewRankHistoryRepository creates a new rank history repository.
func NewRankHistoryRepository(db *sql.DB) RankHistoryRepository {
	return &rankHistoryRepository{db: db}
}

// Create stores a new rank snapshot in the database.
func (r *rankHistoryRepository) Create(ctx context.Context, rank *models.RankHistory) error {
	query := `
		INSERT INTO rank_history (
			account_id, timestamp, format, season_ordinal,
			rank_class, rank_level, rank_step, percentile, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	err := r.db.QueryRowContext(
		ctx, query,
		rank.AccountID,
		rank.Timestamp,
		rank.Format,
		rank.SeasonOrdinal,
		rank.RankClass,
		rank.RankLevel,
		rank.RankStep,
		rank.Percentile,
		rank.CreatedAt,
	).Scan(&rank.ID)
	if err != nil {
		return err
	}

	return nil
}

// GetByFormat retrieves all rank history entries for a specific format.
func (r *rankHistoryRepository) GetByFormat(ctx context.Context, accountID int, format string) ([]*models.RankHistory, error) {
	query := `
		SELECT
			id, account_id, timestamp, format, season_ordinal,
			rank_class, rank_level, rank_step, percentile, created_at
		FROM rank_history
		WHERE account_id = $1 AND format = $2
		ORDER BY timestamp DESC
	`

	rows, err := r.db.QueryContext(ctx, query, accountID, format)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []*models.RankHistory
	for rows.Next() {
		rank := &models.RankHistory{}
		err := rows.Scan(
			&rank.ID,
			&rank.AccountID,
			&rank.Timestamp,
			&rank.Format,
			&rank.SeasonOrdinal,
			&rank.RankClass,
			&rank.RankLevel,
			&rank.RankStep,
			&rank.Percentile,
			&rank.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, rank)
	}

	return results, rows.Err()
}

// GetBySeason retrieves all rank history entries for a specific season.
func (r *rankHistoryRepository) GetBySeason(ctx context.Context, accountID int, seasonOrdinal int) ([]*models.RankHistory, error) {
	query := `
		SELECT
			id, account_id, timestamp, format, season_ordinal,
			rank_class, rank_level, rank_step, percentile, created_at
		FROM rank_history
		WHERE account_id = $1 AND season_ordinal = $2
		ORDER BY timestamp DESC
	`

	rows, err := r.db.QueryContext(ctx, query, accountID, seasonOrdinal)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []*models.RankHistory
	for rows.Next() {
		rank := &models.RankHistory{}
		err := rows.Scan(
			&rank.ID,
			&rank.AccountID,
			&rank.Timestamp,
			&rank.Format,
			&rank.SeasonOrdinal,
			&rank.RankClass,
			&rank.RankLevel,
			&rank.RankStep,
			&rank.Percentile,
			&rank.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, rank)
	}

	return results, rows.Err()
}

// GetByDateRange retrieves rank history entries within a date range.
func (r *rankHistoryRepository) GetByDateRange(ctx context.Context, accountID int, startDate, endDate time.Time) ([]*models.RankHistory, error) {
	query := `
		SELECT
			id, account_id, timestamp, format, season_ordinal,
			rank_class, rank_level, rank_step, percentile, created_at
		FROM rank_history
		WHERE account_id = $1 AND timestamp >= $2 AND timestamp <= $3
		ORDER BY timestamp DESC
	`

	rows, err := r.db.QueryContext(ctx, query, accountID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []*models.RankHistory
	for rows.Next() {
		rank := &models.RankHistory{}
		err := rows.Scan(
			&rank.ID,
			&rank.AccountID,
			&rank.Timestamp,
			&rank.Format,
			&rank.SeasonOrdinal,
			&rank.RankClass,
			&rank.RankLevel,
			&rank.RankStep,
			&rank.Percentile,
			&rank.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, rank)
	}

	return results, rows.Err()
}

// GetLatestByFormat retrieves the most recent rank snapshot for a format.
func (r *rankHistoryRepository) GetLatestByFormat(ctx context.Context, accountID int, format string) (*models.RankHistory, error) {
	query := `
		SELECT
			id, account_id, timestamp, format, season_ordinal,
			rank_class, rank_level, rank_step, percentile, created_at
		FROM rank_history
		WHERE account_id = $1 AND format = $2
		ORDER BY timestamp DESC
		LIMIT 1
	`

	rank := &models.RankHistory{}
	err := r.db.QueryRowContext(ctx, query, accountID, format).Scan(
		&rank.ID,
		&rank.AccountID,
		&rank.Timestamp,
		&rank.Format,
		&rank.SeasonOrdinal,
		&rank.RankClass,
		&rank.RankLevel,
		&rank.RankStep,
		&rank.Percentile,
		&rank.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return rank, nil
}

// GetAll retrieves all rank history entries for an account.
func (r *rankHistoryRepository) GetAll(ctx context.Context, accountID int) ([]*models.RankHistory, error) {
	query := `
		SELECT
			id, account_id, timestamp, format, season_ordinal,
			rank_class, rank_level, rank_step, percentile, created_at
		FROM rank_history
		WHERE account_id = $1
		ORDER BY timestamp DESC
	`

	rows, err := r.db.QueryContext(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []*models.RankHistory
	for rows.Next() {
		rank := &models.RankHistory{}
		err := rows.Scan(
			&rank.ID,
			&rank.AccountID,
			&rank.Timestamp,
			&rank.Format,
			&rank.SeasonOrdinal,
			&rank.RankClass,
			&rank.RankLevel,
			&rank.RankStep,
			&rank.Percentile,
			&rank.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, rank)
	}

	return results, rows.Err()
}
