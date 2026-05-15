// Phase 2 PR #11 — ml-suggestions + synergy + play-patterns repository.
//
// Covers the read+write surface for:
//   - ml_suggestions apply (was_applied=TRUE, applied_at=NOW())
//   - card_combination_stats (synergy report, per-card synergies, exact-pair lookup)
//   - user_play_patterns (read + stub upsert)
//   - account-scoped wipe of user-owned learned data
//
// Read of ml_suggestions for the list/generate/dismiss aliases stays on
// NotesRepository (PR #7) — MLHandler composes both repos.
//
// card_combination_stats has CHECK(card_id_1 < card_id_2) — callers pass
// any pair and the repo normalises ordering. The table is global (no
// account_id) since synergy is computed across all users' matches.

package repository

import (
	"context"
	"database/sql"
	"strconv"
	"time"
)

// CardCombinationStatsRow mirrors the card_combination_stats row.
type CardCombinationStatsRow struct {
	ID              int64
	CardID1         int
	CardID2         int
	DeckID          *string
	Format          string
	GamesTogether   int
	GamesCard1Only  int
	GamesCard2Only  int
	WinsTogether    int
	WinsCard1Only   int
	WinsCard2Only   int
	SynergyScore    float64
	ConfidenceScore float64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// SynergyReportPair is one card pair inside a deck's synergy report.
// CardName columns come from the cards table join when available.
type SynergyReportPair struct {
	Card1ID       int
	Card1Name     *string
	Card2ID       int
	Card2Name     *string
	SynergyScore  float64
	GamesTogether int
	WinRate       float64
}

// SynergyReportRow is the aggregated payload for GET /decks/{id}/synergy-report.
type SynergyReportRow struct {
	DeckID          string
	CardCount       int
	TotalPairs      int
	AvgSynergyScore float64
	Synergies       []SynergyReportPair
}

// UserPlayPatternsRow mirrors user_play_patterns.
type UserPlayPatternsRow struct {
	ID                 int64
	AccountIDText      string
	PreferredArchetype *string
	AggroAffinity      float64
	MidrangeAffinity   float64
	ControlAffinity    float64
	ComboAffinity      float64
	ColorPreferences   *string
	AvgGameLength      float64
	AggressionScore    float64
	InteractionScore   float64
	TotalMatches       int
	TotalDecks         int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// MLRepository owns the synergy + play-patterns + apply/wipe surface.
type MLRepository struct {
	db *sql.DB
}

// NewMLRepository constructs an MLRepository.
func NewMLRepository(db *sql.DB) *MLRepository {
	return &MLRepository{db: db}
}

// ApplySuggestion sets was_applied=TRUE and applied_at=NOW() on the suggestion.
// Account ownership is verified through decks.account_id. Returns true when
// a row was updated.
func (r *MLRepository) ApplySuggestion(ctx context.Context, accountID, suggestionID int64) (bool, error) {
	const q = `UPDATE ml_suggestions
	           SET was_applied = TRUE, applied_at = NOW()
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

// SynergyReport returns a deck-scoped synergy report. The deck must belong
// to accountID. The card list comes from deck_cards; pair lookups join
// card_combination_stats and the cards table for names.
func (r *MLRepository) SynergyReport(ctx context.Context, accountID int64, deckID string) (*SynergyReportRow, error) {
	owned, err := r.deckOwned(ctx, accountID, deckID)
	if err != nil {
		return nil, err
	}
	if !owned {
		return nil, nil
	}

	const cardsQ = `SELECT DISTINCT card_id FROM deck_cards WHERE deck_id = $1 ORDER BY card_id`
	rows, err := r.db.QueryContext(ctx, cardsQ, deckID)
	if err != nil {
		return nil, err
	}
	var cardIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		cardIDs = append(cardIDs, id)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	report := &SynergyReportRow{DeckID: deckID, CardCount: len(cardIDs)}
	if len(cardIDs) < 2 {
		return report, nil
	}

	// Build the in-list once and pass it as an int[] array for performance.
	pairsQ := `SELECT ccs.card_id_1, c1.name, ccs.card_id_2, c2.name,
	                  ccs.synergy_score, ccs.games_together, ccs.wins_together
	           FROM card_combination_stats ccs
	           LEFT JOIN cards c1 ON c1.arena_id = ccs.card_id_1
	           LEFT JOIN cards c2 ON c2.arena_id = ccs.card_id_2
	           WHERE ccs.card_id_1 = ANY($1) AND ccs.card_id_2 = ANY($1)
	           ORDER BY ccs.synergy_score DESC`
	prows, err := r.db.QueryContext(ctx, pairsQ, intSliceToInt64Slice(cardIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = prows.Close() }()
	var total float64
	for prows.Next() {
		var p SynergyReportPair
		var games int
		var wins int
		if err := prows.Scan(&p.Card1ID, &p.Card1Name, &p.Card2ID, &p.Card2Name, &p.SynergyScore, &games, &wins); err != nil {
			return nil, err
		}
		p.GamesTogether = games
		if games > 0 {
			p.WinRate = float64(wins) / float64(games)
		}
		report.Synergies = append(report.Synergies, p)
		total += p.SynergyScore
	}
	if err := prows.Err(); err != nil {
		return nil, err
	}
	report.TotalPairs = len(report.Synergies)
	if report.TotalPairs > 0 {
		report.AvgSynergyScore = total / float64(report.TotalPairs)
	}
	return report, nil
}

// CardSynergies returns the top synergistic pairs that include cardID,
// scoped to format and limited to `limit` rows (max 50). The opposite
// card in each row is the one that is not cardID.
func (r *MLRepository) CardSynergies(ctx context.Context, cardID int, format string, limit int) ([]CardCombinationStatsRow, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	const q = `SELECT id, card_id_1, card_id_2, deck_id, format,
	                  games_together, games_card1_only, games_card2_only,
	                  wins_together, wins_card1_only, wins_card2_only,
	                  synergy_score, confidence_score, created_at, updated_at
	           FROM card_combination_stats
	           WHERE format = $2 AND (card_id_1 = $1 OR card_id_2 = $1)
	           ORDER BY synergy_score DESC
	           LIMIT $3`
	rows, err := r.db.QueryContext(ctx, q, cardID, format, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]CardCombinationStatsRow, 0, limit)
	for rows.Next() {
		var c CardCombinationStatsRow
		if err := rows.Scan(
			&c.ID, &c.CardID1, &c.CardID2, &c.DeckID, &c.Format,
			&c.GamesTogether, &c.GamesCard1Only, &c.GamesCard2Only,
			&c.WinsTogether, &c.WinsCard1Only, &c.WinsCard2Only,
			&c.SynergyScore, &c.ConfidenceScore, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CombinationStats returns the exact pair row for (card1, card2, format).
// Pair ordering is normalised to satisfy the CHECK(card_id_1 < card_id_2)
// constraint. Returns nil when no row exists.
func (r *MLRepository) CombinationStats(ctx context.Context, card1, card2 int, format string) (*CardCombinationStatsRow, error) {
	a, b := card1, card2
	if a > b {
		a, b = b, a
	}
	const q = `SELECT id, card_id_1, card_id_2, deck_id, format,
	                  games_together, games_card1_only, games_card2_only,
	                  wins_together, wins_card1_only, wins_card2_only,
	                  synergy_score, confidence_score, created_at, updated_at
	           FROM card_combination_stats
	           WHERE card_id_1 = $1 AND card_id_2 = $2 AND format = $3
	           LIMIT 1`
	var c CardCombinationStatsRow
	err := r.db.QueryRowContext(ctx, q, a, b, format).Scan(
		&c.ID, &c.CardID1, &c.CardID2, &c.DeckID, &c.Format,
		&c.GamesTogether, &c.GamesCard1Only, &c.GamesCard2Only,
		&c.WinsTogether, &c.WinsCard1Only, &c.WinsCard2Only,
		&c.SynergyScore, &c.ConfidenceScore, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// PlayPatterns returns the user_play_patterns row for the given account
// (stored as TEXT in the schema; we pass the stringified internal id).
// Returns nil when the row does not yet exist — the SPA renders defaults.
func (r *MLRepository) PlayPatterns(ctx context.Context, accountIDText string) (*UserPlayPatternsRow, error) {
	const q = `SELECT id, account_id, preferred_archetype, aggro_affinity, midrange_affinity,
	                  control_affinity, combo_affinity, color_preferences, avg_game_length,
	                  aggression_score, interaction_score, total_matches, total_decks,
	                  created_at, updated_at
	           FROM user_play_patterns
	           WHERE account_id = $1
	           LIMIT 1`
	var u UserPlayPatternsRow
	err := r.db.QueryRowContext(ctx, q, accountIDText).Scan(
		&u.ID, &u.AccountIDText, &u.PreferredArchetype, &u.AggroAffinity, &u.MidrangeAffinity,
		&u.ControlAffinity, &u.ComboAffinity, &u.ColorPreferences, &u.AvgGameLength,
		&u.AggressionScore, &u.InteractionScore, &u.TotalMatches, &u.TotalDecks,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpsertPlayPatternsStub ensures a user_play_patterns row exists for the
// account and bumps updated_at. STUB until the analytics pipeline lands —
// the affinity / interaction columns are left at their defaults.
func (r *MLRepository) UpsertPlayPatternsStub(ctx context.Context, accountIDText string) (*UserPlayPatternsRow, error) {
	const q = `INSERT INTO user_play_patterns (account_id, updated_at)
	           VALUES ($1, NOW())
	           ON CONFLICT (account_id)
	           DO UPDATE SET updated_at = NOW()`
	if _, err := r.db.ExecContext(ctx, q, accountIDText); err != nil {
		return nil, err
	}
	return r.PlayPatterns(ctx, accountIDText)
}

// ClearLearnedDataForAccount removes account-scoped learned data:
//   - ml_suggestions for the user's decks
//   - user_play_patterns row for this account
//
// Global card_combination_stats / card_affinity rows are NOT touched —
// they encode cross-user synergy learnings and survive single-user resets.
func (r *MLRepository) ClearLearnedDataForAccount(ctx context.Context, accountID int64, accountIDText string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	const delSuggestions = `DELETE FROM ml_suggestions
	                        WHERE deck_id IN (SELECT id FROM decks WHERE account_id = $1)`
	if _, err := tx.ExecContext(ctx, delSuggestions, accountID); err != nil {
		return err
	}
	const delPatterns = `DELETE FROM user_play_patterns WHERE account_id = $1`
	if _, err := tx.ExecContext(ctx, delPatterns, accountIDText); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *MLRepository) deckOwned(ctx context.Context, accountID int64, deckID string) (bool, error) {
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

// AccountIDToText renders an internal int64 account id as the TEXT key the
// user_play_patterns table stores. Exported so callers can keep that
// conversion in one place.
func AccountIDToText(accountID int64) string {
	return strconv.FormatInt(accountID, 10)
}
