package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"
)

// ListByAccountIDCursor returns up to limit+1 matches using keyset (cursor)
// pagination. The extra row signals has_more=true to the caller.
//
// When cursorTS and cursorID are both non-zero the query applies the keyset
// predicate (timestamp, id) < (cursorTS, cursorID), restricting results to
// rows that come after the cursor in the default DESC ordering. When cursorTS
// is nil (first page) no keyset predicate is applied.
//
// format may be empty to return all formats.
func (r *MatchesRepository) ListByAccountIDCursor(
	ctx context.Context,
	accountID int64,
	format string,
	cursorTS *time.Time,
	cursorID string,
	limit int,
) ([]MatchRow, error) {
	fetch := limit + 1 // fetch one extra to detect has_more

	var (
		rows *sql.Rows
		err  error
	)

	switch {
	case format != "" && cursorTS != nil:
		const q = `
			SELECT id, format, result, timestamp, duration_seconds, deck_id, rank_before, rank_after,
			       player_wins, opponent_wins
			FROM matches
			WHERE account_id = $1
			  AND lower(format) = lower($2)
			  AND (timestamp < $3 OR (timestamp = $3 AND id < $4))
			ORDER BY timestamp DESC, id DESC
			LIMIT $5`

		rows, err = r.db.QueryContext(ctx, q, accountID, format, *cursorTS, cursorID, fetch)

	case format != "" && cursorTS == nil:
		const q = `
			SELECT id, format, result, timestamp, duration_seconds, deck_id, rank_before, rank_after,
			       player_wins, opponent_wins
			FROM matches
			WHERE account_id = $1 AND lower(format) = lower($2)
			ORDER BY timestamp DESC, id DESC
			LIMIT $3`

		rows, err = r.db.QueryContext(ctx, q, accountID, format, fetch)

	case format == "" && cursorTS != nil:
		const q = `
			SELECT id, format, result, timestamp, duration_seconds, deck_id, rank_before, rank_after,
			       player_wins, opponent_wins
			FROM matches
			WHERE account_id = $1
			  AND (timestamp < $2 OR (timestamp = $2 AND id < $3))
			ORDER BY timestamp DESC, id DESC
			LIMIT $4`

		rows, err = r.db.QueryContext(ctx, q, accountID, *cursorTS, cursorID, fetch)

	default: // format == "" && cursorTS == nil (first page, no filter)
		const q = `
			SELECT id, format, result, timestamp, duration_seconds, deck_id, rank_before, rank_after,
			       player_wins, opponent_wins
			FROM matches
			WHERE account_id = $1
			ORDER BY timestamp DESC, id DESC
			LIMIT $2`

		rows, err = r.db.QueryContext(ctx, q, accountID, fetch)
	}

	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var matches []MatchRow

	for rows.Next() {
		var m MatchRow
		if err := rows.Scan(
			&m.ID, &m.Format, &m.Result, &m.Timestamp,
			&m.DurationSeconds, &m.DeckID, &m.RankBefore, &m.RankAfter,
			&m.PlayerWins, &m.OpponentWins,
		); err != nil {
			return nil, err
		}

		matches = append(matches, m)
	}

	return matches, rows.Err()
}

// MatchRow is a row returned from the matches table for history reads.
type MatchRow struct {
	ID              string
	Format          string
	Result          string
	Timestamp       time.Time
	DurationSeconds *int
	DeckID          *string
	RankBefore      *string
	RankAfter       *string
	PlayerWins      int
	OpponentWins    int
}

// MatchesRepository provides read access to the matches table scoped by account_id.
type MatchesRepository struct {
	db DB
}

// NewMatchesRepository returns a MatchesRepository backed by db.
func NewMatchesRepository(db DB) *MatchesRepository {
	return &MatchesRepository{db: db}
}

// ListByAccountID returns a page of matches for the given account, ordered by
// timestamp DESC.  format may be empty to return all formats.
// Returns rows and total count (for pagination).
func (r *MatchesRepository) ListByAccountID(
	ctx context.Context,
	accountID int64,
	format string,
	page int,
	limit int,
) ([]MatchRow, int, error) {
	offset := (page - 1) * limit

	var (
		rows *sql.Rows
		err  error
	)

	if format != "" {
		const q = `
			SELECT id, format, result, timestamp, duration_seconds, deck_id, rank_before, rank_after,
			       player_wins, opponent_wins
			FROM matches
			WHERE account_id = $1 AND lower(format) = lower($2)
			ORDER BY timestamp DESC
			LIMIT $3 OFFSET $4`

		rows, err = r.db.QueryContext(ctx, q, accountID, format, limit, offset)
	} else {
		const q = `
			SELECT id, format, result, timestamp, duration_seconds, deck_id, rank_before, rank_after,
			       player_wins, opponent_wins
			FROM matches
			WHERE account_id = $1
			ORDER BY timestamp DESC
			LIMIT $2 OFFSET $3`

		rows, err = r.db.QueryContext(ctx, q, accountID, limit, offset)
	}

	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var matches []MatchRow

	for rows.Next() {
		var m MatchRow
		if err := rows.Scan(
			&m.ID, &m.Format, &m.Result, &m.Timestamp,
			&m.DurationSeconds, &m.DeckID, &m.RankBefore, &m.RankAfter,
			&m.PlayerWins, &m.OpponentWins,
		); err != nil {
			return nil, 0, err
		}

		matches = append(matches, m)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	total, err := r.countByAccountID(ctx, accountID, format)
	if err != nil {
		return nil, 0, err
	}

	return matches, total, nil
}

func (r *MatchesRepository) countByAccountID(ctx context.Context, accountID int64, format string) (int, error) {
	var total int

	if format != "" {
		const q = `SELECT COUNT(*) FROM matches WHERE account_id = $1 AND lower(format) = lower($2)`
		row := r.db.QueryRowContext(ctx, q, accountID, format)

		if err := row.Scan(&total); err != nil {
			return 0, err
		}
	} else {
		const q = `SELECT COUNT(*) FROM matches WHERE account_id = $1`
		row := r.db.QueryRowContext(ctx, q, accountID)

		if err := row.Scan(&total); err != nil {
			return 0, err
		}
	}

	return total, nil
}

// UpsertMatch inserts or updates a match row.  Used by the projection worker.
func (r *MatchesRepository) UpsertMatch(ctx context.Context, m MatchUpsert) error {
	const q = `
		INSERT INTO matches (
			id, account_id, event_id, event_name, timestamp, duration_seconds,
			player_wins, opponent_wins, player_team_id, deck_id, rank_before, rank_after,
			format, result, result_reason, opponent_name, opponent_id
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (id) DO UPDATE
			SET event_name       = EXCLUDED.event_name,
			    timestamp        = EXCLUDED.timestamp,
			    duration_seconds = EXCLUDED.duration_seconds,
			    player_wins      = EXCLUDED.player_wins,
			    opponent_wins    = EXCLUDED.opponent_wins,
			    deck_id          = EXCLUDED.deck_id,
			    rank_before      = EXCLUDED.rank_before,
			    rank_after       = EXCLUDED.rank_after,
			    format           = EXCLUDED.format,
			    result           = EXCLUDED.result,
			    result_reason    = EXCLUDED.result_reason,
			    opponent_name    = EXCLUDED.opponent_name,
			    opponent_id      = EXCLUDED.opponent_id`

	_, err := r.db.ExecContext(
		ctx, q,
		m.ID, m.AccountID, m.EventID, m.EventName, m.Timestamp, m.DurationSeconds,
		m.PlayerWins, m.OpponentWins, m.PlayerTeamID, m.DeckID, m.RankBefore, m.RankAfter,
		m.Format, m.Result, m.ResultReason, m.OpponentName, m.OpponentID,
	)
	return err
}

// MatchUpsert holds the fields needed to write a match row from the projection worker.
type MatchUpsert struct {
	ID              string
	AccountID       int64
	EventID         string
	EventName       string
	Timestamp       time.Time
	DurationSeconds *int
	PlayerWins      int
	OpponentWins    int
	PlayerTeamID    int
	DeckID          *string
	RankBefore      *string
	RankAfter       *string
	Format          string
	Result          string
	ResultReason    *string
	OpponentName    *string
	OpponentID      *string
}

// MatchFilter captures every filterable dimension the Phase 2 /api/v1/matches
// endpoint supports. Zero-valued fields are treated as "no filter on this
// dimension" so callers can pass a partially-populated struct.
type MatchFilter struct {
	StartDate *time.Time
	EndDate   *time.Time
	Format    string
	Formats   []string
	DeckID    string
	Result    string // "win" | "loss" | "draw"
	Page      int
	Limit     int
}

// ListByAccountIDFiltered returns a page of matches scoped to accountID,
// filtered by the non-zero fields of f, ordered by timestamp DESC.  Returns
// the page rows and a total count for pagination.
func (r *MatchesRepository) ListByAccountIDFiltered(ctx context.Context, accountID int64, f MatchFilter) ([]MatchRow, int, error) {
	where, args := buildMatchWhere(accountID, f)
	offset := (f.Page - 1) * f.Limit
	args = append(args, f.Limit, offset)

	q := `SELECT id, format, result, timestamp, duration_seconds, deck_id,
	             rank_before, rank_after, player_wins, opponent_wins
	      FROM matches ` + where + `
	      ORDER BY timestamp DESC
	      LIMIT $` + itoa(len(args)-1) + ` OFFSET $` + itoa(len(args))

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var matches []MatchRow
	for rows.Next() {
		var m MatchRow
		if err := rows.Scan(
			&m.ID, &m.Format, &m.Result, &m.Timestamp,
			&m.DurationSeconds, &m.DeckID, &m.RankBefore, &m.RankAfter,
			&m.PlayerWins, &m.OpponentWins,
		); err != nil {
			return nil, 0, err
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// Drop pagination args for the count query.
	countWhere, countArgs := buildMatchWhere(accountID, f)
	countQ := "SELECT COUNT(*) FROM matches " + countWhere
	var total int
	if err := r.db.QueryRowContext(ctx, countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return matches, total, nil
}

// GetByID returns a single match row scoped to accountID, or nil when the
// row does not exist or belongs to a different account. The "scoped to
// accountID" check is the security boundary — never trust matchID alone.
func (r *MatchesRepository) GetByID(ctx context.Context, accountID int64, matchID string) (*MatchRow, error) {
	const q = `SELECT id, format, result, timestamp, duration_seconds, deck_id,
	                  rank_before, rank_after, player_wins, opponent_wins
	           FROM matches
	           WHERE account_id = $1 AND id = $2`
	row := r.db.QueryRowContext(ctx, q, accountID, matchID)
	var m MatchRow
	err := row.Scan(
		&m.ID, &m.Format, &m.Result, &m.Timestamp,
		&m.DurationSeconds, &m.DeckID, &m.RankBefore, &m.RankAfter,
		&m.PlayerWins, &m.OpponentWins,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DistinctFormats returns every distinct format the account has matches in,
// sorted alphabetically. Used by the SPA's format-filter dropdown.
func (r *MatchesRepository) DistinctFormats(ctx context.Context, accountID int64) ([]string, error) {
	const q = `SELECT DISTINCT format
	           FROM matches
	           WHERE account_id = $1 AND format <> ''
	           ORDER BY format`
	rows, err := r.db.QueryContext(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// buildMatchWhere assembles the WHERE clause + args for ListByAccountIDFiltered
// and the matching count query.  Returns "WHERE ..." with $1..$N placeholders
// in the same order as the args slice.
func buildMatchWhere(accountID int64, f MatchFilter) (string, []any) {
	clauses := []string{"account_id = $1"}
	args := []any{accountID}
	next := 2

	if f.StartDate != nil {
		clauses = append(clauses, "timestamp >= $"+itoa(next))
		args = append(args, *f.StartDate)
		next++
	}
	if f.EndDate != nil {
		clauses = append(clauses, "timestamp <= $"+itoa(next))
		args = append(args, *f.EndDate)
		next++
	}
	switch {
	case f.Format != "" && len(f.Formats) > 0:
		clauses = append(clauses, "(lower(format) = lower($"+itoa(next)+") OR lower(format) = ANY($"+itoa(next+1)+"))")
		args = append(args, f.Format, lowerSlice(f.Formats))
		next += 2
	case f.Format != "":
		clauses = append(clauses, "lower(format) = lower($"+itoa(next)+")")
		args = append(args, f.Format)
		next++
	case len(f.Formats) > 0:
		clauses = append(clauses, "lower(format) = ANY($"+itoa(next)+")")
		args = append(args, lowerSlice(f.Formats))
		next++
	}
	if f.DeckID != "" {
		clauses = append(clauses, "deck_id = $"+itoa(next))
		args = append(args, f.DeckID)
		next++
	}
	if f.Result != "" {
		clauses = append(clauses, "lower(result) = lower($"+itoa(next)+")")
		args = append(args, f.Result)
	}

	return "WHERE " + strings.Join(clauses, " AND "), args
}

func lowerSlice(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, strings.ToLower(s))
	}
	return out
}

// itoa is a small int-to-string helper used inside SQL builders; preferred
// over strconv.Itoa here to avoid an extra import and to keep the SQL
// concatenation readable.
func itoa(i int) string { return strconv.Itoa(i) }

// ─── Phase 2 PR #1 expansion: analytics + comparison + games ──────────────────

// GameRow is one row from the games table for read-side use.
type GameRow struct {
	ID              int64
	MatchID         string
	GameNumber      int
	Result          string
	ResultReason    *string
	DurationSeconds *int
	CreatedAt       time.Time
}

// GamesByMatchID returns the games belonging to matchID, scoped to accountID
// via a join through matches. Returned in game_number ASC order so the SPA
// can render them in play order.
func (r *MatchesRepository) GamesByMatchID(ctx context.Context, accountID int64, matchID string) ([]GameRow, error) {
	const q = `
		SELECT g.id, g.match_id, g.game_number, g.result, g.result_reason, g.duration_seconds, g.created_at
		FROM games g
		JOIN matches m ON m.id = g.match_id
		WHERE m.account_id = $1 AND g.match_id = $2
		ORDER BY g.game_number`
	rows, err := r.db.QueryContext(ctx, q, accountID, matchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []GameRow
	for rows.Next() {
		var g GameRow
		if err := rows.Scan(
			&g.ID, &g.MatchID, &g.GameNumber, &g.Result, &g.ResultReason,
			&g.DurationSeconds, &g.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// StatsAggregate is the row returned by a single stats query — match counts +
// game counts. Win rate is computed in the handler so the repo stays SQL-only.
type StatsAggregate struct {
	TotalMatches int
	MatchesWon   int
	MatchesLost  int
	TotalGames   int
	GamesWon     int
	GamesLost    int
}

// AggregateStats runs a filtered stats query against matches+games for the
// account. Empty filter returns whole-account stats. Used by /matches/stats
// and as a building block for compare endpoints.
func (r *MatchesRepository) AggregateStats(ctx context.Context, accountID int64, f MatchFilter) (StatsAggregate, error) {
	where, args := buildMatchWhere(accountID, f)
	q := `
		SELECT
			COUNT(*)                                           AS total_matches,
			COUNT(*) FILTER (WHERE lower(result) = 'win')      AS matches_won,
			COUNT(*) FILTER (WHERE lower(result) = 'loss')     AS matches_lost,
			COALESCE(SUM(player_wins), 0) + COALESCE(SUM(opponent_wins), 0) AS total_games,
			COALESCE(SUM(player_wins), 0)                      AS games_won,
			COALESCE(SUM(opponent_wins), 0)                    AS games_lost
		FROM matches ` + where
	var s StatsAggregate
	if err := r.db.QueryRowContext(ctx, q, args...).Scan(
		&s.TotalMatches, &s.MatchesWon, &s.MatchesLost,
		&s.TotalGames, &s.GamesWon, &s.GamesLost,
	); err != nil {
		return StatsAggregate{}, err
	}
	return s, nil
}

// FormatStatsRow pairs a format string with its aggregated stats. Used by
// /matches/format-distribution.
type FormatStatsRow struct {
	Format string
	Stats  StatsAggregate
}

// FormatDistribution returns per-format aggregated stats for the account,
// honoring the rest of the filter (date range, deck, result, etc.). The
// per-format breakdown happens in SQL; the handler maps it into a
// {format: Statistics} response object.
func (r *MatchesRepository) FormatDistribution(ctx context.Context, accountID int64, f MatchFilter) ([]FormatStatsRow, error) {
	// Reuse buildMatchWhere but ignore the inbound format/formats filter —
	// distribution by-definition spans formats.
	scoped := f
	scoped.Format = ""
	scoped.Formats = nil
	where, args := buildMatchWhere(accountID, scoped)
	q := `
		SELECT
			format,
			COUNT(*),
			COUNT(*) FILTER (WHERE lower(result) = 'win'),
			COUNT(*) FILTER (WHERE lower(result) = 'loss'),
			COALESCE(SUM(player_wins), 0) + COALESCE(SUM(opponent_wins), 0),
			COALESCE(SUM(player_wins), 0),
			COALESCE(SUM(opponent_wins), 0)
		FROM matches ` + where + `
		GROUP BY format
		ORDER BY format`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []FormatStatsRow
	for rows.Next() {
		var fr FormatStatsRow
		if err := rows.Scan(
			&fr.Format,
			&fr.Stats.TotalMatches, &fr.Stats.MatchesWon, &fr.Stats.MatchesLost,
			&fr.Stats.TotalGames, &fr.Stats.GamesWon, &fr.Stats.GamesLost,
		); err != nil {
			return nil, err
		}
		out = append(out, fr)
	}
	return out, rows.Err()
}

// HourBucket is one hour-of-day's aggregated performance metrics. Hour is the
// 0-23 integer extracted from the match timestamp in the server's TZ.
type HourBucket struct {
	Hour                 int
	MatchCount           int
	AvgMatchDurationSecs *float64
	FastestMatchSecs     *int
	SlowestMatchSecs     *int
}

// PerformanceByHour returns one bucket per hour-of-day (0..23) with counts +
// duration aggregates. Hours with no matches are not emitted (handler fills
// gaps if it needs a complete 24-bucket array). Honors the inbound filter.
func (r *MatchesRepository) PerformanceByHour(ctx context.Context, accountID int64, f MatchFilter) ([]HourBucket, error) {
	where, args := buildMatchWhere(accountID, f)
	q := `
		SELECT
			EXTRACT(HOUR FROM timestamp)::int AS hour,
			COUNT(*) AS match_count,
			AVG(duration_seconds) FILTER (WHERE duration_seconds IS NOT NULL) AS avg_duration,
			MIN(duration_seconds) FILTER (WHERE duration_seconds IS NOT NULL) AS fastest,
			MAX(duration_seconds) FILTER (WHERE duration_seconds IS NOT NULL) AS slowest
		FROM matches ` + where + `
		GROUP BY hour
		ORDER BY hour`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []HourBucket
	for rows.Next() {
		var b HourBucket
		if err := rows.Scan(&b.Hour, &b.MatchCount, &b.AvgMatchDurationSecs, &b.FastestMatchSecs, &b.SlowestMatchSecs); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// MatchupRow pairs an opponent label (archetype name when available, falling
// back to opponent name) with aggregated stats vs that opponent. Used by
// /matches/matchup-matrix.
type MatchupRow struct {
	OpponentLabel string
	Stats         StatsAggregate
}

// MatchupMatrix returns aggregated stats grouped by opponent (opponent_name
// is used as the label since this schema does not carry archetype).
// Honors the inbound filter; rows with empty opponent_name are bucketed under
// "Unknown" so the SPA always has a usable label.
func (r *MatchesRepository) MatchupMatrix(ctx context.Context, accountID int64, f MatchFilter) ([]MatchupRow, error) {
	where, args := buildMatchWhere(accountID, f)
	q := `
		SELECT
			COALESCE(NULLIF(opponent_name, ''), 'Unknown') AS label,
			COUNT(*),
			COUNT(*) FILTER (WHERE lower(result) = 'win'),
			COUNT(*) FILTER (WHERE lower(result) = 'loss'),
			COALESCE(SUM(player_wins), 0) + COALESCE(SUM(opponent_wins), 0),
			COALESCE(SUM(player_wins), 0),
			COALESCE(SUM(opponent_wins), 0)
		FROM matches ` + where + `
		GROUP BY label
		ORDER BY label`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []MatchupRow
	for rows.Next() {
		var mr MatchupRow
		if err := rows.Scan(
			&mr.OpponentLabel,
			&mr.Stats.TotalMatches, &mr.Stats.MatchesWon, &mr.Stats.MatchesLost,
			&mr.Stats.TotalGames, &mr.Stats.GamesWon, &mr.Stats.GamesLost,
		); err != nil {
			return nil, err
		}
		out = append(out, mr)
	}
	return out, rows.Err()
}

// DistinctArchetypes returns the distinct opponent_name values for the
// account, sorted alphabetically. Empty/null names are excluded — the SPA
// uses this for a filter dropdown so a blank entry would be useless.
func (r *MatchesRepository) DistinctArchetypes(ctx context.Context, accountID int64) ([]string, error) {
	const q = `SELECT DISTINCT opponent_name FROM matches
	           WHERE account_id = $1 AND opponent_name IS NOT NULL AND opponent_name <> ''
	           ORDER BY opponent_name`
	rows, err := r.db.QueryContext(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// TrendBucket is a single time-bucketed slice of aggregated stats. Used by
// /matches/trends. The bucket boundary semantics live in the caller — the
// repo just executes the requested date_trunc + grouping.
type TrendBucket struct {
	BucketStart time.Time
	Stats       StatsAggregate
}

// Trends returns aggregated stats bucketed by the requested period. period
// is one of: "day" | "week" | "month" — anything else returns
// ErrInvalidTrendPeriod. Honors the inbound filter for any non-period
// dimensions.
func (r *MatchesRepository) Trends(ctx context.Context, accountID int64, period string, f MatchFilter) ([]TrendBucket, error) {
	if period != "day" && period != "week" && period != "month" {
		return nil, ErrInvalidTrendPeriod
	}
	where, args := buildMatchWhere(accountID, f)
	q := `
		SELECT
			date_trunc('` + period + `', timestamp) AS bucket,
			COUNT(*),
			COUNT(*) FILTER (WHERE lower(result) = 'win'),
			COUNT(*) FILTER (WHERE lower(result) = 'loss'),
			COALESCE(SUM(player_wins), 0) + COALESCE(SUM(opponent_wins), 0),
			COALESCE(SUM(player_wins), 0),
			COALESCE(SUM(opponent_wins), 0)
		FROM matches ` + where + `
		GROUP BY bucket
		ORDER BY bucket`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []TrendBucket
	for rows.Next() {
		var b TrendBucket
		if err := rows.Scan(
			&b.BucketStart,
			&b.Stats.TotalMatches, &b.Stats.MatchesWon, &b.Stats.MatchesLost,
			&b.Stats.TotalGames, &b.Stats.GamesWon, &b.Stats.GamesLost,
		); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// RankSnapshot is the most recent rank-tagged match for a format. Used by
// /matches/rank-progression/{format} to produce a current-rank summary.
type RankSnapshot struct {
	Format     string
	RankAfter  string
	OccurredAt time.Time
}

// LatestRankInFormat returns the most recent match with a non-null rank_after
// for the format. Returns (nil, nil) when the account has no ranked matches
// in that format yet.
func (r *MatchesRepository) LatestRankInFormat(ctx context.Context, accountID int64, format string) (*RankSnapshot, error) {
	const q = `SELECT format, rank_after, timestamp
	           FROM matches
	           WHERE account_id = $1 AND lower(format) = lower($2)
	             AND rank_after IS NOT NULL AND rank_after <> ''
	           ORDER BY timestamp DESC
	           LIMIT 1`
	row := r.db.QueryRowContext(ctx, q, accountID, format)
	var s RankSnapshot
	if err := row.Scan(&s.Format, &s.RankAfter, &s.OccurredAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// RankTimelineRow is a single rank-change event for a format. Used by
// /matches/rank-progression-timeline.
type RankTimelineRow struct {
	MatchID    string
	OccurredAt time.Time
	RankBefore *string
	RankAfter  *string
	Result     string
}

// RankTimelineForFormat returns rank-change events for the format between
// startDate and endDate, ordered chronologically. period is currently
// informational only — bucketing happens in the handler so the repo stays
// terse. Returns rows where rank_before/rank_after are present.
func (r *MatchesRepository) RankTimelineForFormat(ctx context.Context, accountID int64, format string, startDate, endDate time.Time) ([]RankTimelineRow, error) {
	const q = `SELECT id, timestamp, rank_before, rank_after, result
	           FROM matches
	           WHERE account_id = $1 AND lower(format) = lower($2)
	             AND timestamp >= $3 AND timestamp <= $4
	             AND (rank_before IS NOT NULL OR rank_after IS NOT NULL)
	           ORDER BY timestamp`
	rows, err := r.db.QueryContext(ctx, q, accountID, format, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []RankTimelineRow
	for rows.Next() {
		var rt RankTimelineRow
		if err := rows.Scan(&rt.MatchID, &rt.OccurredAt, &rt.RankBefore, &rt.RankAfter, &rt.Result); err != nil {
			return nil, err
		}
		out = append(out, rt)
	}
	return out, rows.Err()
}

// ExportRow is a single matches-table row in export-friendly form. Used by
// /matches/export. We do not include game-level details — exports stay flat
// so CSV consumers don't have to deal with nested structures.
type ExportRow struct {
	ID              string
	Format          string
	Result          string
	ResultReason    *string
	Timestamp       time.Time
	DurationSeconds *int
	DeckID          *string
	RankBefore      *string
	RankAfter       *string
	OpponentName    *string
	OpponentID      *string
	PlayerWins      int
	OpponentWins    int
	EventName       string
}

// ExportAll returns every match for the account, ordered newest-first. Caller
// is responsible for streaming / paging when exporting very large accounts.
func (r *MatchesRepository) ExportAll(ctx context.Context, accountID int64) ([]ExportRow, error) {
	const q = `SELECT id, format, result, result_reason, timestamp, duration_seconds,
	                  deck_id, rank_before, rank_after, opponent_name, opponent_id,
	                  player_wins, opponent_wins, event_name
	           FROM matches
	           WHERE account_id = $1
	           ORDER BY timestamp DESC`
	rows, err := r.db.QueryContext(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []ExportRow
	for rows.Next() {
		var ex ExportRow
		if err := rows.Scan(
			&ex.ID, &ex.Format, &ex.Result, &ex.ResultReason, &ex.Timestamp, &ex.DurationSeconds,
			&ex.DeckID, &ex.RankBefore, &ex.RankAfter, &ex.OpponentName, &ex.OpponentID,
			&ex.PlayerWins, &ex.OpponentWins, &ex.EventName,
		); err != nil {
			return nil, err
		}
		out = append(out, ex)
	}
	return out, rows.Err()
}

// ErrInvalidTrendPeriod is returned by Trends when the period argument is
// outside the allowed set (day|week|month). Handlers translate this to a
// 400 response.
var ErrInvalidTrendPeriod = errors.New("trend period must be day|week|month")
