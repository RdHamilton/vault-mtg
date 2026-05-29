package repository

import (
	"context"
	"database/sql"
	"time"
)

// StandardRepository serves the Phase 2 /api/v1/standard/* read paths.
// All queries are global (sets, standard_config) or scoped by account_id
// (decks, deck_cards). Card legality is read from set_cards.legalities
// (migration 000029). set_cards.arena_id is TEXT; casts are applied where
// comparisons with INTEGER arena ids are needed.
type StandardRepository struct {
	db DB
}

// NewStandardRepository returns a StandardRepository backed by db.
func NewStandardRepository(db DB) *StandardRepository {
	return &StandardRepository{db: db}
}

// StandardSetRow is one Standard-legal set row joined with the rotation
// metadata. RotationDate is the set's own rotation_date (text in the schema).
type StandardSetRow struct {
	Code         string
	Name         string
	ReleasedAt   string
	RotationDate *string
	IconSvgURI   string
	CardCount    int
}

// ListStandardSets returns every set marked is_standard_legal=TRUE. Order is
// release date desc so newest set is first.
func (r *StandardRepository) ListStandardSets(ctx context.Context) ([]StandardSetRow, error) {
	const q = `SELECT code, name, COALESCE(released_at, ''), rotation_date,
	                  COALESCE(icon_svg_uri, ''), COALESCE(card_count, 0)
	           FROM sets
	           WHERE is_standard_legal = TRUE
	           ORDER BY released_at DESC NULLS LAST, code`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []StandardSetRow
	for rows.Next() {
		var s StandardSetRow
		if err := rows.Scan(&s.Code, &s.Name, &s.ReleasedAt, &s.RotationDate, &s.IconSvgURI, &s.CardCount); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// StandardConfigRow mirrors the standard_config singleton row.
type StandardConfigRow struct {
	ID               int
	NextRotationDate string
	RotationEnabled  bool
	UpdatedAt        time.Time
}

// GetStandardConfig returns the singleton standard_config row.
func (r *StandardRepository) GetStandardConfig(ctx context.Context) (StandardConfigRow, error) {
	const q = `SELECT id, next_rotation_date, rotation_enabled, updated_at
	           FROM standard_config WHERE id = 1`
	var c StandardConfigRow
	if err := r.db.QueryRowContext(ctx, q).Scan(&c.ID, &c.NextRotationDate, &c.RotationEnabled, &c.UpdatedAt); err != nil {
		return StandardConfigRow{}, err
	}
	return c, nil
}

// CardLegalityRow holds a card's name + set + parsed legalities JSON. The
// raw legalities text is kept so the handler can extract per-format strings
// without re-querying.
type CardLegalityRow struct {
	ArenaID    int
	Name       string
	SetCode    string
	Legalities string // JSON object stored as TEXT
}

// CardByArenaID fetches a single card by its MTGA Arena id. Returns
// sql.ErrNoRows when not found.
//
// set_cards.arena_id is TEXT (migration 000014); cast to INTEGER for comparison.
// DISTINCT ON guards against the same arena_id appearing in multiple sets.
func (r *StandardRepository) CardByArenaID(ctx context.Context, arenaID int) (CardLegalityRow, error) {
	const q = `SELECT arena_id::INTEGER, name, COALESCE(set_code, ''), COALESCE(legalities, '{}')
	           FROM set_cards
	           WHERE arena_id::INTEGER = $1
	           LIMIT 1`
	var c CardLegalityRow
	if err := r.db.QueryRowContext(ctx, q, arenaID).Scan(&c.ArenaID, &c.Name, &c.SetCode, &c.Legalities); err != nil {
		return CardLegalityRow{}, err
	}
	return c, nil
}

// StandardDeckRow is the deck-level metadata used by validate / affected-decks.
type StandardDeckRow struct {
	ID     string
	Name   string
	Format string
}

// DeckByID returns deck metadata if it belongs to accountID. Returns nil
// when the deck does not exist or belongs to another account — the same
// "scope by account is the security boundary" rule the matches handler uses.
func (r *StandardRepository) DeckByID(ctx context.Context, accountID int64, deckID string) (*StandardDeckRow, error) {
	const q = `SELECT id, name, format FROM decks WHERE account_id = $1 AND id = $2`
	var d StandardDeckRow
	err := r.db.QueryRowContext(ctx, q, accountID, deckID).Scan(&d.ID, &d.Name, &d.Format)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// DeckCardsForValidation returns every card in deckID joined to set_cards (for
// name + set_code + legalities) and sets (for rotation_date). The handler
// applies the standard-format predicate using the parsed JSON legalities;
// the repo just delivers the rows.
//
// set_cards.arena_id is TEXT (migration 000014); cast dc.card_id to TEXT for
// the join. DISTINCT ON (arena_id) prevents a card that appears in multiple
// sets from producing duplicate rows.
func (r *StandardRepository) DeckCardsForValidation(ctx context.Context, deckID string) ([]DeckCardForValidation, error) {
	const q = `SELECT dc.card_id, dc.quantity, dc.board,
	                  COALESCE(c.name, ''), COALESCE(c.set_code, ''),
	                  COALESCE(c.legalities, '{}'),
	                  s.rotation_date, COALESCE(s.is_standard_legal, FALSE)
	           FROM deck_cards dc
	           LEFT JOIN (
	               SELECT DISTINCT ON (arena_id) arena_id, name, set_code, legalities
	               FROM set_cards
	               ORDER BY arena_id, id
	           ) c ON c.arena_id = dc.card_id::TEXT
	           LEFT JOIN sets s ON s.code = c.set_code
	           WHERE dc.deck_id = $1
	           ORDER BY dc.board, c.name`
	rows, err := r.db.QueryContext(ctx, q, deckID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []DeckCardForValidation
	for rows.Next() {
		var d DeckCardForValidation
		if err := rows.Scan(
			&d.CardID, &d.Quantity, &d.Board, &d.Name, &d.SetCode,
			&d.Legalities, &d.RotationDate, &d.SetIsStandardLegal,
		); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DeckCardForValidation is a deck-card joined with everything the validator
// needs. Kept separate from DeckCardRow so future callers can opt into
// either shape.
type DeckCardForValidation struct {
	CardID             int
	Quantity           int
	Board              string
	Name               string
	SetCode            string
	Legalities         string  // JSON
	RotationDate       *string // from sets.rotation_date
	SetIsStandardLegal bool
}

// AccountStandardDeckRow is the minimal deck row needed by the affected-decks
// summary. Format filter is applied in SQL since the SPA only cares about
// Standard decks.
type AccountStandardDeckRow struct {
	ID     string
	Name   string
	Format string
}

// ListAccountStandardDecks returns the user's decks with format='standard'
// (case-insensitive) for the affected-decks scan. We return all matching
// decks; the handler iterates and computes per-deck rotation impact via
// DeckCardsForValidation.
func (r *StandardRepository) ListAccountStandardDecks(ctx context.Context, accountID int64) ([]AccountStandardDeckRow, error) {
	const q = `SELECT id, name, format
	           FROM decks
	           WHERE account_id = $1 AND lower(format) IN ('standard', 'standard_bo1', 'standard_bo3')
	           ORDER BY modified_at DESC
	           LIMIT 100`
	rows, err := r.db.QueryContext(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []AccountStandardDeckRow
	for rows.Next() {
		var d AccountStandardDeckRow
		if err := rows.Scan(&d.ID, &d.Name, &d.Format); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// SetByCode returns the set's display fields. Used to populate
// DeckSetInfo / RotatingCard.SetName when those nil-out from the
// LEFT JOIN in DeckCardsForValidation.
func (r *StandardRepository) SetByCode(ctx context.Context, code string) (*StandardSetRow, error) {
	const q = `SELECT code, name, COALESCE(released_at, ''), rotation_date,
	                  COALESCE(icon_svg_uri, ''), COALESCE(card_count, 0)
	           FROM sets
	           WHERE lower(code) = lower($1)`
	var s StandardSetRow
	err := r.db.QueryRowContext(ctx, q, code).Scan(&s.Code, &s.Name, &s.ReleasedAt, &s.RotationDate, &s.IconSvgURI, &s.CardCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// CountStandardCardsAcrossSets returns the total number of cards across all
// rotating sets — sets whose rotation_date matches the next rotation. Used
// by the rotation summary.
func (r *StandardRepository) CountStandardCardsAcrossSets(ctx context.Context, rotationDate string) (int, error) {
	const q = `SELECT COALESCE(SUM(card_count), 0)
	           FROM sets
	           WHERE rotation_date = $1`
	var n int
	if err := r.db.QueryRowContext(ctx, q, rotationDate).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// CountStandardSetsRotatingOn returns the number of sets rotating on the
// given date. Used by the rotation summary.
func (r *StandardRepository) CountStandardSetsRotatingOn(ctx context.Context, rotationDate string) (int, error) {
	const q = `SELECT COUNT(*) FROM sets WHERE rotation_date = $1`
	var n int
	if err := r.db.QueryRowContext(ctx, q, rotationDate).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// SetsRotatingOn returns the set rows that rotate on the given date.
func (r *StandardRepository) SetsRotatingOn(ctx context.Context, rotationDate string) ([]StandardSetRow, error) {
	const q = `SELECT code, name, COALESCE(released_at, ''), rotation_date,
	                  COALESCE(icon_svg_uri, ''), COALESCE(card_count, 0)
	           FROM sets
	           WHERE rotation_date = $1
	           ORDER BY released_at`
	rows, err := r.db.QueryContext(ctx, q, rotationDate)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []StandardSetRow
	for rows.Next() {
		var s StandardSetRow
		if err := rows.Scan(&s.Code, &s.Name, &s.ReleasedAt, &s.RotationDate, &s.IconSvgURI, &s.CardCount); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
