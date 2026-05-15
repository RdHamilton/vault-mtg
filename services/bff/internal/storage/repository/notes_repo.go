package repository

import (
	"context"
	"database/sql"
	"time"
)

// NotesRepository serves the Phase 2 notes + suggestions surface:
//   - deck_notes (CRUD)
//   - matches.notes / matches.rating (per-match annotations)
//   - ml_suggestions (read + dismiss)
//
// Account scoping is enforced by joining decks.account_id /
// matches.account_id in every query — IDs alone are never trusted.
type NotesRepository struct {
	db DB
}

// NewNotesRepository returns a NotesRepository backed by db.
func NewNotesRepository(db DB) *NotesRepository {
	return &NotesRepository{db: db}
}

// DeckNoteRow mirrors deck_notes.
type DeckNoteRow struct {
	ID        int64
	DeckID    string
	Content   string
	Category  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ListDeckNotes returns notes for deckID, scoped to account, optionally
// filtered by category. Newest first.
func (r *NotesRepository) ListDeckNotes(ctx context.Context, accountID int64, deckID, category string) ([]DeckNoteRow, error) {
	q := `SELECT n.id, n.deck_id, n.content, n.category, n.created_at, n.updated_at
	      FROM deck_notes n
	      JOIN decks d ON d.id = n.deck_id
	      WHERE d.account_id = $1 AND n.deck_id = $2`
	args := []any{accountID, deckID}
	if category != "" {
		q += " AND lower(n.category) = lower($3)"
		args = append(args, category)
	}
	q += " ORDER BY n.created_at DESC, n.id DESC"
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []DeckNoteRow
	for rows.Next() {
		var n DeckNoteRow
		if err := rows.Scan(&n.ID, &n.DeckID, &n.Content, &n.Category, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// GetDeckNote returns a single note scoped to (account, deck, note).
// Returns nil when not found or not owned by the account.
func (r *NotesRepository) GetDeckNote(ctx context.Context, accountID int64, deckID string, noteID int64) (*DeckNoteRow, error) {
	const q = `SELECT n.id, n.deck_id, n.content, n.category, n.created_at, n.updated_at
	           FROM deck_notes n
	           JOIN decks d ON d.id = n.deck_id
	           WHERE d.account_id = $1 AND n.deck_id = $2 AND n.id = $3
	           LIMIT 1`
	var n DeckNoteRow
	err := r.db.QueryRowContext(ctx, q, accountID, deckID, noteID).Scan(
		&n.ID, &n.DeckID, &n.Content, &n.Category, &n.CreatedAt, &n.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// CreateDeckNote inserts a note for deckID after verifying ownership.
// Returns the inserted row.
func (r *NotesRepository) CreateDeckNote(ctx context.Context, accountID int64, deckID, content, category string) (*DeckNoteRow, error) {
	owns, err := r.deckOwnedByAccount(ctx, accountID, deckID)
	if err != nil {
		return nil, err
	}
	if !owns {
		return nil, nil
	}
	if category == "" {
		category = "general"
	}
	const q = `INSERT INTO deck_notes (deck_id, content, category)
	           VALUES ($1, $2, $3)
	           RETURNING id, deck_id, content, category, created_at, updated_at`
	var n DeckNoteRow
	if err := r.db.QueryRowContext(ctx, q, deckID, content, category).Scan(
		&n.ID, &n.DeckID, &n.Content, &n.Category, &n.CreatedAt, &n.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &n, nil
}

// UpdateDeckNote updates content + category for the note. Returns the
// updated row, or nil when the note isn't owned by the account.
func (r *NotesRepository) UpdateDeckNote(ctx context.Context, accountID int64, deckID string, noteID int64, content, category string) (*DeckNoteRow, error) {
	if category == "" {
		category = "general"
	}
	const q = `UPDATE deck_notes
	           SET content = $4, category = $5, updated_at = NOW()
	           WHERE id = $3 AND deck_id = $2
	             AND deck_id IN (SELECT id FROM decks WHERE account_id = $1)
	           RETURNING id, deck_id, content, category, created_at, updated_at`
	var n DeckNoteRow
	err := r.db.QueryRowContext(ctx, q, accountID, deckID, noteID, content, category).Scan(
		&n.ID, &n.DeckID, &n.Content, &n.Category, &n.CreatedAt, &n.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// DeleteDeckNote deletes the note. Returns true if a row was deleted,
// false when the note doesn't exist or isn't owned by the account.
func (r *NotesRepository) DeleteDeckNote(ctx context.Context, accountID int64, deckID string, noteID int64) (bool, error) {
	const q = `DELETE FROM deck_notes
	           WHERE id = $3 AND deck_id = $2
	             AND deck_id IN (SELECT id FROM decks WHERE account_id = $1)`
	res, err := r.db.ExecContext(ctx, q, accountID, deckID, noteID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// MatchNotesRow holds the (notes, rating) tuple from the matches table.
type MatchNotesRow struct {
	MatchID string
	Notes   string
	Rating  int
}

// GetMatchNotes returns notes + rating for matchID, scoped to account.
// Returns nil when the match doesn't belong to the account.
func (r *NotesRepository) GetMatchNotes(ctx context.Context, accountID int64, matchID string) (*MatchNotesRow, error) {
	const q = `SELECT id, COALESCE(notes, ''), COALESCE(rating, 0)
	           FROM matches
	           WHERE account_id = $1 AND id = $2
	           LIMIT 1`
	var m MatchNotesRow
	err := r.db.QueryRowContext(ctx, q, accountID, matchID).Scan(&m.MatchID, &m.Notes, &m.Rating)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// UpdateMatchNotes updates notes + rating for the match. Returns the
// updated row, or nil when the match isn't owned by the account.
func (r *NotesRepository) UpdateMatchNotes(ctx context.Context, accountID int64, matchID, notes string, rating int) (*MatchNotesRow, error) {
	const q = `UPDATE matches
	           SET notes = $3, rating = $4
	           WHERE account_id = $1 AND id = $2
	           RETURNING id, COALESCE(notes, ''), COALESCE(rating, 0)`
	var m MatchNotesRow
	err := r.db.QueryRowContext(ctx, q, accountID, matchID, notes, rating).Scan(
		&m.MatchID, &m.Notes, &m.Rating,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// SuggestionRow mirrors ml_suggestions for the read-side. The handler
// derives priority from confidence and constructs cardReferences from
// the card_id / swap_for_card_id pair.
type SuggestionRow struct {
	ID                    int64
	DeckID                string
	SuggestionType        string
	CardID                *int
	CardName              *string
	SwapForCardID         *int
	SwapForCardName       *string
	Confidence            float64
	ExpectedWinRateChange float64
	Title                 string
	Description           *string
	Reasoning             *string
	Evidence              *string
	IsDismissed           bool
	WasApplied            bool
	OutcomeWinRateChange  *float64
	CreatedAt             time.Time
	AppliedAt             *time.Time
	OutcomeRecordedAt     *time.Time
}

// ListSuggestions returns ml_suggestions for the deck, scoped to account.
// activeOnly=true filters out dismissed rows. Sorted by confidence DESC.
func (r *NotesRepository) ListSuggestions(ctx context.Context, accountID int64, deckID string, activeOnly bool) ([]SuggestionRow, error) {
	clauses := "ms.deck_id = $2 AND d.account_id = $1"
	if activeOnly {
		clauses += " AND ms.is_dismissed = FALSE"
	}
	q := `SELECT ms.id, ms.deck_id, ms.suggestion_type, ms.card_id, ms.card_name,
	             ms.swap_for_card_id, ms.swap_for_card_name, ms.confidence,
	             ms.expected_win_rate_change, ms.title, ms.description, ms.reasoning,
	             ms.evidence, ms.is_dismissed, ms.was_applied,
	             ms.outcome_win_rate_change, ms.created_at, ms.applied_at,
	             ms.outcome_recorded_at
	      FROM ml_suggestions ms
	      JOIN decks d ON d.id = ms.deck_id
	      WHERE ` + clauses + `
	      ORDER BY ms.confidence DESC, ms.created_at DESC`
	rows, err := r.db.QueryContext(ctx, q, accountID, deckID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []SuggestionRow
	for rows.Next() {
		var s SuggestionRow
		if err := rows.Scan(
			&s.ID, &s.DeckID, &s.SuggestionType, &s.CardID, &s.CardName,
			&s.SwapForCardID, &s.SwapForCardName, &s.Confidence,
			&s.ExpectedWinRateChange, &s.Title, &s.Description, &s.Reasoning,
			&s.Evidence, &s.IsDismissed, &s.WasApplied,
			&s.OutcomeWinRateChange, &s.CreatedAt, &s.AppliedAt,
			&s.OutcomeRecordedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DismissSuggestion sets is_dismissed=TRUE for the suggestion. Account
// ownership is verified via decks.account_id. Returns true when a row was
// updated.
func (r *NotesRepository) DismissSuggestion(ctx context.Context, accountID, suggestionID int64) (bool, error) {
	const q = `UPDATE ml_suggestions
	           SET is_dismissed = TRUE
	           WHERE id = $2
	             AND deck_id IN (SELECT id FROM decks WHERE account_id = $1)`
	res, err := r.db.ExecContext(ctx, q, accountID, suggestionID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// deckOwnedByAccount returns true when (deckID, accountID) match a row in
// decks. Used as a precheck for write paths.
func (r *NotesRepository) deckOwnedByAccount(ctx context.Context, accountID int64, deckID string) (bool, error) {
	const q = `SELECT 1 FROM decks WHERE id = $1 AND account_id = $2 LIMIT 1`
	var n int
	err := r.db.QueryRowContext(ctx, q, deckID, accountID).Scan(&n)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
