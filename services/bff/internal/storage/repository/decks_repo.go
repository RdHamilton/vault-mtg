package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DecksRepository serves the Phase 2 /api/v1/decks/* surface — CRUD on
// decks + deck_cards + deck_tags, plus the deck_permutations versioning
// table. Account scoping is enforced by joining decks.account_id in
// every query.
//
// Note: a DeckListRepository already exists for the v2 cursor list; this
// new repository is a richer shape covering the per-deck reads + writes
// the SPA's decks.ts wraps.
type DecksRepository struct {
	db DB
}

// NewDecksRepository returns a DecksRepository backed by db.
func NewDecksRepository(db DB) *DecksRepository {
	return &DecksRepository{db: db}
}

// DeckSummaryRow is the row shape for /decks list endpoints. Mirrors
// gui.DeckListItem on the SPA (camelCase).
type DeckSummaryRow struct {
	ID            string
	Name          string
	Format        string
	Source        string
	DraftEventID  *string
	MatchesPlayed int
	MatchesWon    int
	GamesPlayed   int
	GamesWon      int
	IsAppCreated  bool
	CreatedAt     time.Time
	ModifiedAt    time.Time
	LastPlayed    *time.Time
	ColorIdentity *string
	Description   *string
	CreatedMethod string
	SeedCardID    *int
	CardCount     int // computed via subquery
	Tags          []string
}

// DeckDetailRow is the rich per-deck shape for /decks/{id}. Carries the
// summary fields + the deck_cards rows.
type DeckDetailRow struct {
	DeckSummaryRow
	Cards []DeckCardRow
}

// DeckCardRow is one row from deck_cards joined to set_cards (for name/metadata).
type DeckCardRow struct {
	CardID        int
	Quantity      int
	Board         string
	FromDraftPick bool
	Name          string
	SetCode       string
	ManaCost      string
	CMC           float64
	Colors        string
	TypeLine      string
	Rarity        string
	ImageURIs     string
}

// DeckListFilter narrows ListDecks queries.
type DeckListFilter struct {
	Format string
	Source string
	Tags   []string
}

// ListDecks returns the account's decks (no cards) optionally filtered
// by format/source. Sorted by modified_at DESC, capped at 200.
func (r *DecksRepository) ListDecks(ctx context.Context, accountID int64, f DeckListFilter) ([]DeckSummaryRow, error) {
	clauses := []string{"d.account_id = $1"}
	args := []any{accountID}
	next := 2
	if f.Format != "" {
		clauses = append(clauses, "lower(d.format) = lower($"+strconv.Itoa(next)+")")
		args = append(args, f.Format)
		next++
	}
	if f.Source != "" {
		clauses = append(clauses, "lower(d.source) = lower($"+strconv.Itoa(next)+")")
		args = append(args, f.Source)
		next++
	}
	if len(f.Tags) > 0 {
		clauses = append(clauses, "d.id IN (SELECT deck_id FROM deck_tags WHERE lower(tag) = ANY($"+strconv.Itoa(next)+"))")
		args = append(args, lowerSlice(f.Tags))
	}
	q := `SELECT d.id, d.name, d.format, d.source, d.draft_event_id,
	             d.matches_played, d.matches_won, d.games_played, d.games_won,
	             d.is_app_created, d.created_at, d.modified_at, d.last_played,
	             d.color_identity, d.description, d.created_method, d.seed_card_id,
	             COALESCE((SELECT SUM(quantity) FROM deck_cards WHERE deck_id = d.id), 0) AS card_count
	      FROM decks d
	      WHERE ` + strings.Join(clauses, " AND ") + `
	      ORDER BY d.modified_at DESC
	      LIMIT 200`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []DeckSummaryRow
	deckIDs := []string{}
	for rows.Next() {
		var d DeckSummaryRow
		if err := rows.Scan(
			&d.ID, &d.Name, &d.Format, &d.Source, &d.DraftEventID,
			&d.MatchesPlayed, &d.MatchesWon, &d.GamesPlayed, &d.GamesWon,
			&d.IsAppCreated, &d.CreatedAt, &d.ModifiedAt, &d.LastPlayed,
			&d.ColorIdentity, &d.Description, &d.CreatedMethod, &d.SeedCardID,
			&d.CardCount,
		); err != nil {
			return nil, err
		}
		out = append(out, d)
		deckIDs = append(deckIDs, d.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Bulk-load tags so the list response carries tags inline.
	tagMap, err := r.tagsForDecks(ctx, deckIDs)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Tags = tagMap[out[i].ID]
	}
	return out, nil
}

// GetDeck returns a single deck (with cards + tags) scoped to account.
// Returns nil when not found / not owned.
func (r *DecksRepository) GetDeck(ctx context.Context, accountID int64, deckID string) (*DeckDetailRow, error) {
	const q = `SELECT d.id, d.name, d.format, d.source, d.draft_event_id,
	                  d.matches_played, d.matches_won, d.games_played, d.games_won,
	                  d.is_app_created, d.created_at, d.modified_at, d.last_played,
	                  d.color_identity, d.description, d.created_method, d.seed_card_id,
	                  COALESCE((SELECT SUM(quantity) FROM deck_cards WHERE deck_id = d.id), 0) AS card_count
	           FROM decks d
	           WHERE d.account_id = $1 AND d.id = $2
	           LIMIT 1`
	var d DeckDetailRow
	err := r.db.QueryRowContext(ctx, q, accountID, deckID).Scan(
		&d.ID, &d.Name, &d.Format, &d.Source, &d.DraftEventID,
		&d.MatchesPlayed, &d.MatchesWon, &d.GamesPlayed, &d.GamesWon,
		&d.IsAppCreated, &d.CreatedAt, &d.ModifiedAt, &d.LastPlayed,
		&d.ColorIdentity, &d.Description, &d.CreatedMethod, &d.SeedCardID,
		&d.CardCount,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cards, err := r.deckCards(ctx, deckID)
	if err != nil {
		return nil, err
	}
	d.Cards = cards
	tagMap, err := r.tagsForDecks(ctx, []string{deckID})
	if err != nil {
		return nil, err
	}
	d.Tags = tagMap[deckID]
	return &d, nil
}

// GetDeckByDraftEvent returns the deck associated with a draft event id.
func (r *DecksRepository) GetDeckByDraftEvent(ctx context.Context, accountID int64, draftEventID string) (*DeckDetailRow, error) {
	const q = `SELECT d.id, d.name, d.format, d.source, d.draft_event_id,
	                  d.matches_played, d.matches_won, d.games_played, d.games_won,
	                  d.is_app_created, d.created_at, d.modified_at, d.last_played,
	                  d.color_identity, d.description, d.created_method, d.seed_card_id,
	                  COALESCE((SELECT SUM(quantity) FROM deck_cards WHERE deck_id = d.id), 0) AS card_count
	           FROM decks d
	           WHERE d.account_id = $1 AND d.draft_event_id = $2
	           ORDER BY d.modified_at DESC
	           LIMIT 1`
	var d DeckDetailRow
	err := r.db.QueryRowContext(ctx, q, accountID, draftEventID).Scan(
		&d.ID, &d.Name, &d.Format, &d.Source, &d.DraftEventID,
		&d.MatchesPlayed, &d.MatchesWon, &d.GamesPlayed, &d.GamesWon,
		&d.IsAppCreated, &d.CreatedAt, &d.ModifiedAt, &d.LastPlayed,
		&d.ColorIdentity, &d.Description, &d.CreatedMethod, &d.SeedCardID,
		&d.CardCount,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cards, err := r.deckCards(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	d.Cards = cards
	return &d, nil
}

// CreateDeckInput holds the inserted-deck fields. ID is generated server-
// side as a v4-ish hash of (account_id + name + timestamp) so the SPA
// gets a stable deck handle without coordinating UUID generation.
type CreateDeckInput struct {
	AccountID    int64
	Name         string
	Format       string
	Source       string
	DraftEventID *string
}

// CreateDeck inserts a deck row and returns it.
func (r *DecksRepository) CreateDeck(ctx context.Context, in CreateDeckInput) (*DeckDetailRow, error) {
	id := generateDeckID(in.AccountID, in.Name)
	const q = `INSERT INTO decks (
	             id, account_id, name, format, source, draft_event_id,
	             is_app_created, created_method, created_at, modified_at
	           ) VALUES ($1, $2, $3, $4, $5, $6, FALSE, 'imported', NOW(), NOW())
	           RETURNING id, name, format, source, draft_event_id,
	                     matches_played, matches_won, games_played, games_won,
	                     is_app_created, created_at, modified_at, last_played,
	                     color_identity, description, created_method, seed_card_id`
	var d DeckDetailRow
	if err := r.db.QueryRowContext(
		ctx, q,
		id, in.AccountID, in.Name, in.Format, in.Source, in.DraftEventID,
	).Scan(
		&d.ID, &d.Name, &d.Format, &d.Source, &d.DraftEventID,
		&d.MatchesPlayed, &d.MatchesWon, &d.GamesPlayed, &d.GamesWon,
		&d.IsAppCreated, &d.CreatedAt, &d.ModifiedAt, &d.LastPlayed,
		&d.ColorIdentity, &d.Description, &d.CreatedMethod, &d.SeedCardID,
	); err != nil {
		return nil, err
	}
	d.CardCount = 0
	d.Cards = []DeckCardRow{}
	return &d, nil
}

// UpdateDeckInput lists the fields that can be patched.
type UpdateDeckInput struct {
	Name   *string
	Format *string
}

// UpdateDeck modifies the deck. Only set non-nil fields are updated.
// Returns the updated deck (or nil when the deck doesn't belong to the
// account).
func (r *DecksRepository) UpdateDeck(ctx context.Context, accountID int64, deckID string, in UpdateDeckInput) (*DeckDetailRow, error) {
	clauses := []string{}
	args := []any{accountID, deckID}
	next := 3
	if in.Name != nil {
		clauses = append(clauses, "name = $"+strconv.Itoa(next))
		args = append(args, *in.Name)
		next++
	}
	if in.Format != nil {
		clauses = append(clauses, "format = $"+strconv.Itoa(next))
		args = append(args, *in.Format)
	}
	if len(clauses) == 0 {
		return r.GetDeck(ctx, accountID, deckID)
	}
	clauses = append(clauses, "modified_at = NOW()")
	q := "UPDATE decks SET " + strings.Join(clauses, ", ") +
		" WHERE account_id = $1 AND id = $2"
	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	return r.GetDeck(ctx, accountID, deckID)
}

// DeleteDeck removes the deck. Returns true when a row was deleted.
func (r *DecksRepository) DeleteDeck(ctx context.Context, accountID int64, deckID string) (bool, error) {
	const q = `DELETE FROM decks WHERE account_id = $1 AND id = $2`
	res, err := r.db.ExecContext(ctx, q, accountID, deckID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// CloneDeck duplicates an existing deck (and its cards) under a new id +
// name. Returns the new deck.
func (r *DecksRepository) CloneDeck(ctx context.Context, accountID int64, deckID, newName string) (*DeckDetailRow, error) {
	src, err := r.GetDeck(ctx, accountID, deckID)
	if err != nil {
		return nil, err
	}
	if src == nil {
		return nil, nil
	}
	newID := generateDeckID(accountID, newName)
	const insertDeck = `INSERT INTO decks (id, account_id, name, format, source, created_method, is_app_created, created_at, modified_at)
	                    VALUES ($1, $2, $3, $4, $5, 'cloned', FALSE, NOW(), NOW())`
	if _, err := r.db.ExecContext(ctx, insertDeck, newID, accountID, newName, src.Format, src.Source); err != nil {
		return nil, err
	}
	for _, c := range src.Cards {
		if _, err := r.db.ExecContext(
			ctx,
			`INSERT INTO deck_cards (deck_id, card_id, quantity, board) VALUES ($1, $2, $3, $4)`,
			newID, c.CardID, c.Quantity, c.Board,
		); err != nil {
			return nil, err
		}
	}
	return r.GetDeck(ctx, accountID, newID)
}

// AddCardInput is one card add operation. Quantity may be > 1.
type AddCardInput struct {
	CardID    int
	Quantity  int
	Board     string
	FromDraft bool
}

// AddCard inserts or increments a deck_cards row. Account ownership
// verified via decks.account_id.
func (r *DecksRepository) AddCard(ctx context.Context, accountID int64, deckID string, in AddCardInput) (bool, error) {
	owns, err := r.deckOwned(ctx, accountID, deckID)
	if err != nil || !owns {
		return false, err
	}
	const q = `INSERT INTO deck_cards (deck_id, card_id, quantity, board, from_draft_pick)
	           VALUES ($1, $2, $3, $4, $5)
	           ON CONFLICT (deck_id, card_id, board) DO UPDATE
	             SET quantity = deck_cards.quantity + EXCLUDED.quantity`
	_, err = r.db.ExecContext(ctx, q, deckID, in.CardID, in.Quantity, in.Board, in.FromDraft)
	if err != nil {
		return false, err
	}
	return true, nil
}

// RemoveCardOne decrements a deck_cards row by 1. When the resulting
// quantity is 0, the row is deleted. Returns true when a row was changed.
func (r *DecksRepository) RemoveCardOne(ctx context.Context, accountID int64, deckID string, cardID int, board string) (bool, error) {
	owns, err := r.deckOwned(ctx, accountID, deckID)
	if err != nil || !owns {
		return false, err
	}
	const q = `UPDATE deck_cards SET quantity = quantity - 1
	           WHERE deck_id = $1 AND card_id = $2 AND board = $3 AND quantity > 0`
	res, err := r.db.ExecContext(ctx, q, deckID, cardID, board)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	// Clean up zero-quantity rows.
	_, _ = r.db.ExecContext(ctx, `DELETE FROM deck_cards WHERE deck_id = $1 AND card_id = $2 AND board = $3 AND quantity <= 0`, deckID, cardID, board)
	return true, nil
}

// RemoveAllCopies deletes the deck_cards row entirely.
func (r *DecksRepository) RemoveAllCopies(ctx context.Context, accountID int64, deckID string, cardID int, board string) (bool, error) {
	owns, err := r.deckOwned(ctx, accountID, deckID)
	if err != nil || !owns {
		return false, err
	}
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM deck_cards WHERE deck_id = $1 AND card_id = $2 AND board = $3`,
		deckID, cardID, board)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// AddTag inserts a (deck_id, tag) row. No-op when (deck, tag) already exists.
func (r *DecksRepository) AddTag(ctx context.Context, accountID int64, deckID, tag string) (bool, error) {
	owns, err := r.deckOwned(ctx, accountID, deckID)
	if err != nil || !owns {
		return false, err
	}
	const q = `INSERT INTO deck_tags (deck_id, tag) VALUES ($1, $2)
	           ON CONFLICT (deck_id, tag) DO NOTHING`
	_, err = r.db.ExecContext(ctx, q, deckID, tag)
	if err != nil {
		return false, err
	}
	return true, nil
}

// RemoveTag deletes a (deck_id, tag) row.
func (r *DecksRepository) RemoveTag(ctx context.Context, accountID int64, deckID, tag string) (bool, error) {
	owns, err := r.deckOwned(ctx, accountID, deckID)
	if err != nil || !owns {
		return false, err
	}
	res, err := r.db.ExecContext(ctx, `DELETE FROM deck_tags WHERE deck_id = $1 AND tag = $2`, deckID, tag)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// PermutationRow mirrors deck_permutations.
type PermutationRow struct {
	ID                  int64
	DeckID              string
	ParentPermutationID *int64
	Cards               string // JSON
	VersionNumber       int
	VersionName         *string
	ChangeSummary       *string
	MatchesPlayed       int
	MatchesWon          int
	GamesPlayed         int
	GamesWon            int
	CreatedAt           time.Time
	LastPlayedAt        *time.Time
}

// ListPermutations returns every permutation for the deck, scoped via
// decks.account_id. Newest first.
func (r *DecksRepository) ListPermutations(ctx context.Context, accountID int64, deckID string) ([]PermutationRow, error) {
	const q = `SELECT p.id, p.deck_id, p.parent_permutation_id, p.cards,
	                  p.version_number, p.version_name, p.change_summary,
	                  p.matches_played, p.matches_won, p.games_played, p.games_won,
	                  p.created_at, p.last_played_at
	           FROM deck_permutations p
	           JOIN decks d ON d.id = p.deck_id
	           WHERE d.account_id = $1 AND p.deck_id = $2
	           ORDER BY p.version_number DESC, p.created_at DESC`
	return r.scanPermutations(ctx, q, accountID, deckID)
}

// GetPermutation returns a single permutation by id, scoped to account.
func (r *DecksRepository) GetPermutation(ctx context.Context, accountID int64, deckID string, permutationID int64) (*PermutationRow, error) {
	const q = `SELECT p.id, p.deck_id, p.parent_permutation_id, p.cards,
	                  p.version_number, p.version_name, p.change_summary,
	                  p.matches_played, p.matches_won, p.games_played, p.games_won,
	                  p.created_at, p.last_played_at
	           FROM deck_permutations p
	           JOIN decks d ON d.id = p.deck_id
	           WHERE d.account_id = $1 AND p.deck_id = $2 AND p.id = $3
	           LIMIT 1`
	rows, err := r.scanPermutations(ctx, q, accountID, deckID, permutationID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// CurrentPermutation returns the permutation referenced by
// decks.current_permutation_id, scoped to account.
func (r *DecksRepository) CurrentPermutation(ctx context.Context, accountID int64, deckID string) (*PermutationRow, error) {
	const q = `SELECT p.id, p.deck_id, p.parent_permutation_id, p.cards,
	                  p.version_number, p.version_name, p.change_summary,
	                  p.matches_played, p.matches_won, p.games_played, p.games_won,
	                  p.created_at, p.last_played_at
	           FROM deck_permutations p
	           JOIN decks d ON d.id = p.deck_id
	           WHERE d.account_id = $1 AND d.id = $2 AND d.current_permutation_id = p.id
	           LIMIT 1`
	rows, err := r.scanPermutations(ctx, q, accountID, deckID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// UpdatePermutationName updates the version_name field. Returns true on
// success.
func (r *DecksRepository) UpdatePermutationName(ctx context.Context, accountID int64, deckID string, permutationID int64, name string) (bool, error) {
	owns, err := r.deckOwned(ctx, accountID, deckID)
	if err != nil || !owns {
		return false, err
	}
	const q = `UPDATE deck_permutations SET version_name = $3
	           WHERE id = $1 AND deck_id = $2`
	res, err := r.db.ExecContext(ctx, q, permutationID, deckID, name)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// RestorePermutation sets decks.current_permutation_id to the given id.
// The actual deck_cards re-population happens in a separate flow (the
// projection worker writes the new state). For now, we just update the
// pointer and leave the SPA to refresh.
func (r *DecksRepository) RestorePermutation(ctx context.Context, accountID int64, deckID string, permutationID int64) (bool, error) {
	const q = `UPDATE decks SET current_permutation_id = $1, modified_at = NOW()
	           WHERE id = $2 AND account_id = $3
	             AND $1 IN (SELECT id FROM deck_permutations WHERE deck_id = $2)`
	res, err := r.db.ExecContext(ctx, q, permutationID, deckID, accountID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// DeckMatchesRow returns aggregate match counts for the deck.
type DeckMatchesAggregate struct {
	TotalMatches int
	MatchesWon   int
	GamesPlayed  int
	GamesWon     int
}

// DeckMatchesAggregate returns per-deck performance counts joined to
// matches.
func (r *DecksRepository) DeckMatchesAggregate(ctx context.Context, accountID int64, deckID string) (DeckMatchesAggregate, error) {
	const q = `SELECT COUNT(*),
	                  COUNT(*) FILTER (WHERE lower(result) = 'win'),
	                  COALESCE(SUM(player_wins), 0) + COALESCE(SUM(opponent_wins), 0),
	                  COALESCE(SUM(player_wins), 0)
	           FROM matches
	           WHERE account_id = $1 AND deck_id = $2`
	var a DeckMatchesAggregate
	if err := r.db.QueryRowContext(ctx, q, accountID, deckID).Scan(
		&a.TotalMatches, &a.MatchesWon, &a.GamesPlayed, &a.GamesWon,
	); err != nil {
		return DeckMatchesAggregate{}, err
	}
	return a, nil
}

// DistinctTagsForAccount returns the set of unique tags the account uses.
// Used to populate a tag picker.
func (r *DecksRepository) DistinctTagsForAccount(ctx context.Context, accountID int64) ([]string, error) {
	const q = `SELECT DISTINCT t.tag
	           FROM deck_tags t
	           JOIN decks d ON d.id = t.deck_id
	           WHERE d.account_id = $1
	           ORDER BY t.tag`
	rows, err := r.db.QueryContext(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ─── helpers ────────────────────────────────────────────────────────────────

// deckOwned returns true when the (deck, account) pair exists.
func (r *DecksRepository) deckOwned(ctx context.Context, accountID int64, deckID string) (bool, error) {
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

// deckCards returns the joined deck_cards rows for a deck.
// NOTE: the legacy `cards` table was dropped in migration 000025; the
// canonical card-metadata table is now `set_cards` (created in 000014).
// When the same arena_id appears in multiple sets we take the first row by
// set_cards.id (DISTINCT ON) to avoid duplicate output rows.
func (r *DecksRepository) deckCards(ctx context.Context, deckID string) ([]DeckCardRow, error) {
	const q = `SELECT dc.card_id, dc.quantity, dc.board, (dc.from_draft_pick::boolean),
	                  COALESCE(c.name, ''), COALESCE(c.set_code, ''),
	                  COALESCE(c.mana_cost, ''), COALESCE(c.cmc, 0),
	                  COALESCE(c.colors, '[]'), COALESCE(c.types, ''),
	                  COALESCE(c.rarity, ''), COALESCE(json_build_object('normal', c.image_url)::TEXT, '{}')
	           FROM deck_cards dc
	           LEFT JOIN LATERAL (
	               SELECT * FROM set_cards
	               WHERE arena_id = dc.card_id::TEXT
	               ORDER BY id
	               LIMIT 1
	           ) c ON TRUE
	           WHERE dc.deck_id = $1
	           ORDER BY dc.board, c.name NULLS LAST, dc.card_id`
	rows, err := r.db.QueryContext(ctx, q, deckID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []DeckCardRow
	for rows.Next() {
		var c DeckCardRow
		if err := rows.Scan(
			&c.CardID, &c.Quantity, &c.Board, &c.FromDraftPick,
			&c.Name, &c.SetCode, &c.ManaCost, &c.CMC,
			&c.Colors, &c.TypeLine, &c.Rarity, &c.ImageURIs,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// tagsForDecks bulk-loads tags grouped by deck_id.
func (r *DecksRepository) tagsForDecks(ctx context.Context, deckIDs []string) (map[string][]string, error) {
	out := map[string][]string{}
	if len(deckIDs) == 0 {
		return out, nil
	}
	const q = `SELECT deck_id, tag FROM deck_tags WHERE deck_id = ANY($1) ORDER BY tag`
	rows, err := r.db.QueryContext(ctx, q, deckIDs)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var deckID, tag string
		if err := rows.Scan(&deckID, &tag); err != nil {
			return nil, err
		}
		out[deckID] = append(out[deckID], tag)
	}
	return out, rows.Err()
}

// scanPermutations centralises the row scan for permutation queries.
func (r *DecksRepository) scanPermutations(ctx context.Context, q string, args ...any) ([]PermutationRow, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []PermutationRow
	for rows.Next() {
		var p PermutationRow
		if err := rows.Scan(
			&p.ID, &p.DeckID, &p.ParentPermutationID, &p.Cards,
			&p.VersionNumber, &p.VersionName, &p.ChangeSummary,
			&p.MatchesPlayed, &p.MatchesWon, &p.GamesPlayed, &p.GamesWon,
			&p.CreatedAt, &p.LastPlayedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// generateDeckID returns a stable pseudo-random deck id derived from
// (accountID, name, time.Now()). The schema requires a TEXT primary key
// and the SPA never inspects the format — any unique string works.
func generateDeckID(accountID int64, name string) string {
	now := time.Now().UTC().UnixNano()
	src := strconv.FormatInt(accountID, 10) + ":" + name + ":" + strconv.FormatInt(now, 10)
	sum := sha256.Sum256([]byte(src))
	return "deck_" + hex.EncodeToString(sum[:8])
}

// ParsePermutationCards returns the deck cards stored on a permutation.
// The cards column is a JSON array — empty when malformed.
func ParsePermutationCards(raw string) []DeckPermutationCard {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var out []DeckPermutationCard
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	// Sort for determinism so the diff handler emits stable output.
	sort.Slice(out, func(i, j int) bool { return out[i].CardID < out[j].CardID })
	return out
}

// DeckPermutationCard mirrors the SPA's DeckPermutationCard.
type DeckPermutationCard struct {
	CardID   int    `json:"card_id"`
	Quantity int    `json:"quantity"`
	Board    string `json:"board"`
}
