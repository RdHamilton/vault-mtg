package repository

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"
)

// OpponentsRepository serves the Phase 2 /api/v1/opponents/*,
// /api/v1/analytics/{matchups,opponent-history}, /api/v1/matches/{id}/
// opponent-analysis, and /api/v1/archetypes/{name}/expected-cards reads.
//
// Tables:
//   - opponent_deck_profiles — per-match reconstruction (scoped via matches join)
//   - matchup_statistics     — per-account, per-archetype-pair stats
//   - archetype_expected_cards — global archetype → expected card lookup
//   - opponent_cards_observed — per-game observed cards (already exposed by
//     GamePlaysRepository; here we expose a thin per-match wrapper for the
//     opponent-analysis composite response)
type OpponentsRepository struct {
	db DB
}

// NewOpponentsRepository returns an OpponentsRepository backed by db.
func NewOpponentsRepository(db DB) *OpponentsRepository {
	return &OpponentsRepository{db: db}
}

// OpponentDeckProfileRow mirrors opponent_deck_profiles.
type OpponentDeckProfileRow struct {
	ID                  int64
	MatchID             string
	DetectedArchetype   *string
	ArchetypeConfidence float64
	ColorIdentity       string
	DeckStyle           *string
	CardsObserved       int
	EstimatedDeckSize   int
	ObservedCardIDs     *string // JSON
	InferredCardIDs     *string // JSON
	SignatureCards      *string // JSON
	Format              *string
	MetaArchetypeID     *int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// OpponentProfileForMatch returns the opponent deck profile for matchID,
// scoped to accountID via a matches join. Returns nil when no profile
// exists for the match (or the match doesn't belong to the account).
func (r *OpponentsRepository) OpponentProfileForMatch(ctx context.Context, accountID int64, matchID string) (*OpponentDeckProfileRow, error) {
	const q = `SELECT odp.id, odp.match_id, odp.detected_archetype, odp.archetype_confidence,
	                  odp.color_identity, odp.deck_style, odp.cards_observed,
	                  odp.estimated_deck_size, odp.observed_card_ids,
	                  odp.inferred_card_ids, odp.signature_cards, odp.format,
	                  odp.meta_archetype_id, odp.created_at, odp.updated_at
	           FROM opponent_deck_profiles odp
	           JOIN matches m ON m.id = odp.match_id
	           WHERE m.account_id = $1 AND odp.match_id = $2
	           LIMIT 1`
	var p OpponentDeckProfileRow
	err := r.db.QueryRowContext(ctx, q, accountID, matchID).Scan(
		&p.ID, &p.MatchID, &p.DetectedArchetype, &p.ArchetypeConfidence,
		&p.ColorIdentity, &p.DeckStyle, &p.CardsObserved,
		&p.EstimatedDeckSize, &p.ObservedCardIDs, &p.InferredCardIDs,
		&p.SignatureCards, &p.Format, &p.MetaArchetypeID,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// OpponentDeckFilter narrows a ListOpponentDecks query. Empty fields are
// treated as "no filter" so partial structs are fine.
type OpponentDeckFilter struct {
	Archetype     string
	Format        string
	MinConfidence float64
	Limit         int
}

// ListOpponentDecks returns opponent_deck_profiles for the account
// (scoped via matches.account_id), filtered per f, newest first. Returns
// the rows + total count for pagination.
func (r *OpponentsRepository) ListOpponentDecks(ctx context.Context, accountID int64, f OpponentDeckFilter) ([]OpponentDeckProfileRow, int, error) {
	clauses := []string{"m.account_id = $1"}
	args := []any{accountID}
	next := 2
	if f.Archetype != "" {
		clauses = append(clauses, "lower(odp.detected_archetype) = lower($"+strconv.Itoa(next)+")")
		args = append(args, f.Archetype)
		next++
	}
	if f.Format != "" {
		clauses = append(clauses, "lower(odp.format) = lower($"+strconv.Itoa(next)+")")
		args = append(args, f.Format)
		next++
	}
	if f.MinConfidence > 0 {
		clauses = append(clauses, "odp.archetype_confidence >= $"+strconv.Itoa(next))
		args = append(args, f.MinConfidence)
		next++
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	where := "WHERE " + strings.Join(clauses, " AND ")
	q := `SELECT odp.id, odp.match_id, odp.detected_archetype, odp.archetype_confidence,
	             odp.color_identity, odp.deck_style, odp.cards_observed,
	             odp.estimated_deck_size, odp.observed_card_ids,
	             odp.inferred_card_ids, odp.signature_cards, odp.format,
	             odp.meta_archetype_id, odp.created_at, odp.updated_at
	      FROM opponent_deck_profiles odp
	      JOIN matches m ON m.id = odp.match_id
	      ` + where + `
	      ORDER BY odp.updated_at DESC, odp.id DESC
	      LIMIT $` + strconv.Itoa(next)
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []OpponentDeckProfileRow
	for rows.Next() {
		var p OpponentDeckProfileRow
		if err := rows.Scan(
			&p.ID, &p.MatchID, &p.DetectedArchetype, &p.ArchetypeConfidence,
			&p.ColorIdentity, &p.DeckStyle, &p.CardsObserved,
			&p.EstimatedDeckSize, &p.ObservedCardIDs, &p.InferredCardIDs,
			&p.SignatureCards, &p.Format, &p.MetaArchetypeID,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// Count query mirrors the WHERE without limit/offset.
	countQ := `SELECT COUNT(*) FROM opponent_deck_profiles odp
	           JOIN matches m ON m.id = odp.match_id ` + where
	var total int
	if err := r.db.QueryRowContext(ctx, countQ, args[:len(args)-1]...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// MatchupStatRow mirrors matchup_statistics. wins/losses are stored
// directly; the handler computes win_rate.
type MatchupStatRow struct {
	ID                int64
	AccountID         int64
	PlayerArchetype   string
	OpponentArchetype string
	Format            string
	TotalMatches      int
	Wins              int
	Losses            int
	AvgGameDuration   *int
	LastMatchAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// ListMatchups returns matchup_statistics for the account, optionally
// filtered by format. Sorted by total_matches DESC then last_match_at DESC.
func (r *OpponentsRepository) ListMatchups(ctx context.Context, accountID int64, format string) ([]MatchupStatRow, int, error) {
	clauses := []string{"account_id = $1"}
	args := []any{accountID}
	next := 2
	if format != "" {
		clauses = append(clauses, "lower(format) = lower($"+strconv.Itoa(next)+")")
		args = append(args, format)
		next++
	}
	q := `SELECT id, account_id, player_archetype, opponent_archetype, format,
	             total_matches, wins, losses, avg_game_duration, last_match_at,
	             created_at, updated_at
	      FROM matchup_statistics
	      WHERE ` + strings.Join(clauses, " AND ") + `
	      ORDER BY total_matches DESC, last_match_at DESC NULLS LAST
	      LIMIT 200`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []MatchupStatRow
	for rows.Next() {
		var m MatchupStatRow
		if err := rows.Scan(
			&m.ID, &m.AccountID, &m.PlayerArchetype, &m.OpponentArchetype, &m.Format,
			&m.TotalMatches, &m.Wins, &m.Losses, &m.AvgGameDuration, &m.LastMatchAt,
			&m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, len(out), nil
}

// MatchupForArchetypes returns the single matchup_statistics row for the
// (player, opponent, format) triple, or nil when none exists.
func (r *OpponentsRepository) MatchupForArchetypes(ctx context.Context, accountID int64, playerArch, opponentArch, format string) (*MatchupStatRow, error) {
	const q = `SELECT id, account_id, player_archetype, opponent_archetype, format,
	                  total_matches, wins, losses, avg_game_duration, last_match_at,
	                  created_at, updated_at
	           FROM matchup_statistics
	           WHERE account_id = $1
	             AND lower(player_archetype) = lower($2)
	             AND lower(opponent_archetype) = lower($3)
	             AND lower(format) = lower($4)
	           LIMIT 1`
	var m MatchupStatRow
	err := r.db.QueryRowContext(ctx, q, accountID, playerArch, opponentArch, format).Scan(
		&m.ID, &m.AccountID, &m.PlayerArchetype, &m.OpponentArchetype, &m.Format,
		&m.TotalMatches, &m.Wins, &m.Losses, &m.AvgGameDuration, &m.LastMatchAt,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ExpectedCardRow mirrors archetype_expected_cards.
type ExpectedCardRow struct {
	ID            int64
	ArchetypeName string
	Format        string
	CardID        int
	CardName      string
	InclusionRate float64
	AvgCopies     float64
	IsSignature   bool
	Category      *string
	CreatedAt     time.Time
}

// ExpectedCardsForArchetype returns archetype_expected_cards filtered by
// archetype name and optional format. Sorted by inclusion_rate DESC so the
// most likely cards come first.
func (r *OpponentsRepository) ExpectedCardsForArchetype(ctx context.Context, archetype, format string) ([]ExpectedCardRow, error) {
	clauses := []string{"lower(archetype_name) = lower($1)"}
	args := []any{archetype}
	next := 2
	if format != "" {
		clauses = append(clauses, "lower(format) = lower($"+strconv.Itoa(next)+")")
		args = append(args, format)
	}
	q := `SELECT id, archetype_name, format, card_id, card_name, inclusion_rate,
	             avg_copies, is_signature, category, created_at
	      FROM archetype_expected_cards
	      WHERE ` + strings.Join(clauses, " AND ") + `
	      ORDER BY is_signature DESC, inclusion_rate DESC, card_name
	      LIMIT 200`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExpectedCardRow
	for rows.Next() {
		var e ExpectedCardRow
		if err := rows.Scan(
			&e.ID, &e.ArchetypeName, &e.Format, &e.CardID, &e.CardName,
			&e.InclusionRate, &e.AvgCopies, &e.IsSignature, &e.Category, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// OpponentHistorySummaryRow is the aggregated summary for the opponent-
// history endpoint. Computed in SQL via subqueries.
type OpponentHistorySummaryRow struct {
	TotalOpponents      int
	UniqueArchetypes    int
	MostCommonArchetype string
	MostCommonCount     int
}

// OpponentHistorySummary returns the cross-cut aggregates over the
// account's opponent_deck_profiles, optionally filtered by format. Empty
// when the account has no profiles yet.
func (r *OpponentsRepository) OpponentHistorySummary(ctx context.Context, accountID int64, format string) (OpponentHistorySummaryRow, error) {
	clauses := []string{"m.account_id = $1"}
	args := []any{accountID}
	next := 2
	if format != "" {
		clauses = append(clauses, "lower(odp.format) = lower($"+strconv.Itoa(next)+")")
		args = append(args, format)
	}
	where := "WHERE " + strings.Join(clauses, " AND ")
	q := `SELECT COUNT(*),
	             COUNT(DISTINCT odp.detected_archetype),
	             COALESCE(MODE() WITHIN GROUP (ORDER BY odp.detected_archetype), ''),
	             (SELECT COUNT(*)
	              FROM opponent_deck_profiles odp2
	              JOIN matches m2 ON m2.id = odp2.match_id
	              ` + where + `
	                AND lower(odp2.detected_archetype) =
	                    lower(MODE() WITHIN GROUP (ORDER BY odp.detected_archetype)))
	      FROM opponent_deck_profiles odp
	      JOIN matches m ON m.id = odp.match_id
	      ` + where
	// Args are repeated because the inner subquery shares the same WHERE.
	doubleArgs := append([]any{}, args...)
	doubleArgs = append(doubleArgs, args...)
	var s OpponentHistorySummaryRow
	if err := r.db.QueryRowContext(ctx, q, doubleArgs...).Scan(
		&s.TotalOpponents, &s.UniqueArchetypes, &s.MostCommonArchetype, &s.MostCommonCount,
	); err != nil {
		if err == sql.ErrNoRows {
			return OpponentHistorySummaryRow{}, nil
		}
		return OpponentHistorySummaryRow{}, err
	}
	return s, nil
}

// ArchetypeBreakdownRow is one (archetype, count, win_rate) tuple in the
// opponent-history breakdown.
type ArchetypeBreakdownRow struct {
	Archetype string
	Count     int
	Wins      int
}

// ArchetypeBreakdown returns per-archetype counts + win counts for the
// account's opponent history, optionally filtered by format.
func (r *OpponentsRepository) ArchetypeBreakdown(ctx context.Context, accountID int64, format string) ([]ArchetypeBreakdownRow, error) {
	clauses := []string{"m.account_id = $1"}
	args := []any{accountID}
	next := 2
	if format != "" {
		clauses = append(clauses, "lower(odp.format) = lower($"+strconv.Itoa(next)+")")
		args = append(args, format)
	}
	q := `SELECT COALESCE(odp.detected_archetype, 'Unknown') AS archetype,
	             COUNT(*),
	             COUNT(*) FILTER (WHERE lower(m.result) = 'win')
	      FROM opponent_deck_profiles odp
	      JOIN matches m ON m.id = odp.match_id
	      WHERE ` + strings.Join(clauses, " AND ") + `
	      GROUP BY archetype
	      ORDER BY COUNT(*) DESC
	      LIMIT 50`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ArchetypeBreakdownRow
	for rows.Next() {
		var b ArchetypeBreakdownRow
		if err := rows.Scan(&b.Archetype, &b.Count, &b.Wins); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ColorIdentityBreakdownRow is one (color, count, wins) tuple in the
// opponent-history color breakdown.
type ColorIdentityBreakdownRow struct {
	ColorIdentity string
	Count         int
	Wins          int
}

// ColorIdentityBreakdown returns per-color-identity counts + win counts
// for the account's opponent history, optionally filtered by format.
func (r *OpponentsRepository) ColorIdentityBreakdown(ctx context.Context, accountID int64, format string) ([]ColorIdentityBreakdownRow, error) {
	clauses := []string{"m.account_id = $1"}
	args := []any{accountID}
	next := 2
	if format != "" {
		clauses = append(clauses, "lower(odp.format) = lower($"+strconv.Itoa(next)+")")
		args = append(args, format)
	}
	q := `SELECT COALESCE(odp.color_identity, ''),
	             COUNT(*),
	             COUNT(*) FILTER (WHERE lower(m.result) = 'win')
	      FROM opponent_deck_profiles odp
	      JOIN matches m ON m.id = odp.match_id
	      WHERE ` + strings.Join(clauses, " AND ") + `
	      GROUP BY 1
	      ORDER BY COUNT(*) DESC
	      LIMIT 50`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ColorIdentityBreakdownRow
	for rows.Next() {
		var c ColorIdentityBreakdownRow
		if err := rows.Scan(&c.ColorIdentity, &c.Count, &c.Wins); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// OpponentObservedCardRow is a slim alias for the GamePlaysRepository's
// OpponentCardRow — defined here so the OpponentsHandler does not have to
// import that repo just for the type. The handler maps from this row.
type OpponentObservedCardRow struct {
	CardID        int
	CardName      *string
	ZoneObserved  *string
	TurnFirstSeen *int
	TimesSeen     int
}

// OpponentCardsForMatch returns observed-card rows for matchID, scoped to
// account via matches join. Mirrors the same query used by
// GamePlaysRepository.OpponentCardsByMatch but returns the slimmer shape
// the OpponentAnalysis composite needs.
func (r *OpponentsRepository) OpponentCardsForMatch(ctx context.Context, accountID int64, matchID string) ([]OpponentObservedCardRow, error) {
	const q = `SELECT oc.card_id, oc.card_name, oc.zone_observed, oc.turn_first_seen, oc.times_seen
	           FROM opponent_cards_observed oc
	           JOIN matches m ON m.id = oc.match_id
	           WHERE m.account_id = $1 AND oc.match_id = $2
	           ORDER BY oc.turn_first_seen, oc.id`
	rows, err := r.db.QueryContext(ctx, q, accountID, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OpponentObservedCardRow
	for rows.Next() {
		var c OpponentObservedCardRow
		if err := rows.Scan(&c.CardID, &c.CardName, &c.ZoneObserved, &c.TurnFirstSeen, &c.TimesSeen); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
