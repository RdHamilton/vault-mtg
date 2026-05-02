package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
)

// SuggestionRepository handles database operations for improvement suggestions.
type SuggestionRepository interface {
	// CreateSuggestion creates a new improvement suggestion.
	CreateSuggestion(ctx context.Context, suggestion *models.ImprovementSuggestion) error

	// GetSuggestionsByDeck retrieves all suggestions for a deck.
	GetSuggestionsByDeck(ctx context.Context, deckID string) ([]*models.ImprovementSuggestion, error)

	// GetActiveSuggestions retrieves non-dismissed suggestions for a deck.
	GetActiveSuggestions(ctx context.Context, deckID string) ([]*models.ImprovementSuggestion, error)

	// GetSuggestionByID retrieves a single suggestion by ID.
	GetSuggestionByID(ctx context.Context, id int64) (*models.ImprovementSuggestion, error)

	// GetSuggestionsByType retrieves suggestions for a deck filtered by type.
	GetSuggestionsByType(ctx context.Context, deckID, suggestionType string) ([]*models.ImprovementSuggestion, error)

	// DismissSuggestion marks a suggestion as dismissed.
	DismissSuggestion(ctx context.Context, id int64) error

	// UndismissSuggestion marks a suggestion as not dismissed.
	UndismissSuggestion(ctx context.Context, id int64) error

	// DeleteSuggestion deletes a suggestion by ID.
	DeleteSuggestion(ctx context.Context, id int64) error

	// DeleteSuggestionsByDeck deletes all suggestions for a deck.
	DeleteSuggestionsByDeck(ctx context.Context, deckID string) error

	// DeleteActiveSuggestionsByDeck deletes all non-dismissed suggestions for a deck.
	// Useful before regenerating suggestions.
	DeleteActiveSuggestionsByDeck(ctx context.Context, deckID string) error
}

// suggestionRepository is the concrete implementation of SuggestionRepository.
type suggestionRepository struct {
	db *sql.DB
}

// NewSuggestionRepository creates a new suggestion repository.
func NewSuggestionRepository(db *sql.DB) SuggestionRepository {
	return &suggestionRepository{db: db}
}

// CreateSuggestion creates a new improvement suggestion.
func (r *suggestionRepository) CreateSuggestion(ctx context.Context, suggestion *models.ImprovementSuggestion) error {
	query := `
		INSERT INTO improvement_suggestions (
			deck_id, suggestion_type, priority, title, description,
			evidence, card_references, is_dismissed, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	now := time.Now().UTC()
	suggestion.CreatedAt = now

	err := r.db.QueryRowContext(ctx, query,
		suggestion.DeckID,
		suggestion.SuggestionType,
		suggestion.Priority,
		suggestion.Title,
		suggestion.Description,
		suggestion.Evidence,
		suggestion.CardReferences,
		suggestion.IsDismissed,
		now,
	).Scan(&suggestion.ID)
	if err != nil {
		return err
	}

	return nil
}

// GetSuggestionsByDeck retrieves all suggestions for a deck.
func (r *suggestionRepository) GetSuggestionsByDeck(ctx context.Context, deckID string) ([]*models.ImprovementSuggestion, error) {
	query := `
		SELECT id, deck_id, suggestion_type, priority, title, description,
			   evidence, card_references, is_dismissed, created_at
		FROM improvement_suggestions
		WHERE deck_id = $1
		ORDER BY
			CASE priority
				WHEN 'high' THEN 1
				WHEN 'medium' THEN 2
				WHEN 'low' THEN 3
			END,
			created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, deckID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return r.scanSuggestions(rows)
}

// GetActiveSuggestions retrieves non-dismissed suggestions for a deck.
func (r *suggestionRepository) GetActiveSuggestions(ctx context.Context, deckID string) ([]*models.ImprovementSuggestion, error) {
	query := `
		SELECT id, deck_id, suggestion_type, priority, title, description,
			   evidence, card_references, is_dismissed, created_at
		FROM improvement_suggestions
		WHERE deck_id = $1 AND is_dismissed = FALSE
		ORDER BY
			CASE priority
				WHEN 'high' THEN 1
				WHEN 'medium' THEN 2
				WHEN 'low' THEN 3
			END,
			created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, deckID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return r.scanSuggestions(rows)
}

// GetSuggestionByID retrieves a single suggestion by ID.
func (r *suggestionRepository) GetSuggestionByID(ctx context.Context, id int64) (*models.ImprovementSuggestion, error) {
	query := `
		SELECT id, deck_id, suggestion_type, priority, title, description,
			   evidence, card_references, is_dismissed, created_at
		FROM improvement_suggestions
		WHERE id = $1
	`

	suggestion := &models.ImprovementSuggestion{}
	var createdAt string

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&suggestion.ID,
		&suggestion.DeckID,
		&suggestion.SuggestionType,
		&suggestion.Priority,
		&suggestion.Title,
		&suggestion.Description,
		&suggestion.Evidence,
		&suggestion.CardReferences,
		&suggestion.IsDismissed,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	parsedTime, err := time.Parse("2006-01-02 15:04:05.999999", createdAt)
	if err != nil {
		// Log warning but don't fail - use zero time if parse fails
		parsedTime = time.Time{}
	}
	suggestion.CreatedAt = parsedTime

	return suggestion, nil
}

// GetSuggestionsByType retrieves suggestions for a deck filtered by type.
func (r *suggestionRepository) GetSuggestionsByType(ctx context.Context, deckID, suggestionType string) ([]*models.ImprovementSuggestion, error) {
	query := `
		SELECT id, deck_id, suggestion_type, priority, title, description,
			   evidence, card_references, is_dismissed, created_at
		FROM improvement_suggestions
		WHERE deck_id = $1 AND suggestion_type = $2
		ORDER BY
			CASE priority
				WHEN 'high' THEN 1
				WHEN 'medium' THEN 2
				WHEN 'low' THEN 3
			END,
			created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, deckID, suggestionType)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return r.scanSuggestions(rows)
}

// DismissSuggestion marks a suggestion as dismissed.
func (r *suggestionRepository) DismissSuggestion(ctx context.Context, id int64) error {
	query := `UPDATE improvement_suggestions SET is_dismissed = TRUE WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// UndismissSuggestion marks a suggestion as not dismissed.
func (r *suggestionRepository) UndismissSuggestion(ctx context.Context, id int64) error {
	query := `UPDATE improvement_suggestions SET is_dismissed = FALSE WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// DeleteSuggestion deletes a suggestion by ID.
func (r *suggestionRepository) DeleteSuggestion(ctx context.Context, id int64) error {
	query := `DELETE FROM improvement_suggestions WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// DeleteSuggestionsByDeck deletes all suggestions for a deck.
func (r *suggestionRepository) DeleteSuggestionsByDeck(ctx context.Context, deckID string) error {
	query := `DELETE FROM improvement_suggestions WHERE deck_id = $1`
	_, err := r.db.ExecContext(ctx, query, deckID)
	return err
}

// DeleteActiveSuggestionsByDeck deletes all non-dismissed suggestions for a deck.
func (r *suggestionRepository) DeleteActiveSuggestionsByDeck(ctx context.Context, deckID string) error {
	query := `DELETE FROM improvement_suggestions WHERE deck_id = $1 AND is_dismissed = FALSE`
	_, err := r.db.ExecContext(ctx, query, deckID)
	return err
}

// scanSuggestions scans multiple suggestion rows.
func (r *suggestionRepository) scanSuggestions(rows *sql.Rows) ([]*models.ImprovementSuggestion, error) {
	var suggestions []*models.ImprovementSuggestion

	for rows.Next() {
		suggestion := &models.ImprovementSuggestion{}
		var createdAt string

		err := rows.Scan(
			&suggestion.ID,
			&suggestion.DeckID,
			&suggestion.SuggestionType,
			&suggestion.Priority,
			&suggestion.Title,
			&suggestion.Description,
			&suggestion.Evidence,
			&suggestion.CardReferences,
			&suggestion.IsDismissed,
			&createdAt,
		)
		if err != nil {
			return nil, err
		}

		parsedTime, err := time.Parse("2006-01-02 15:04:05.999999", createdAt)
		if err != nil {
			// Use zero time if parse fails - don't break the scan loop
			parsedTime = time.Time{}
		}
		suggestion.CreatedAt = parsedTime

		suggestions = append(suggestions, suggestion)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return suggestions, nil
}
