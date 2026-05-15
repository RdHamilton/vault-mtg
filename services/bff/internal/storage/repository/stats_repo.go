package repository

import (
	"context"
	"database/sql"
	"time"
)

// ─── DeckPerformance ─────────────────────────────────────────────────────────

// DeckPerformanceRow holds win/loss/draw counts for a single deck.
type DeckPerformanceRow struct {
	DeckID     string
	DeckName   string
	Format     string
	Wins       int
	Losses     int
	Draws      int
	TotalGames int
}

// ─── WinRateTrend ────────────────────────────────────────────────────────────

// WinRateBucket holds win-rate stats for a single time bucket.
type WinRateBucket struct {
	// BucketStart is the start of the day (UTC, truncated to midnight) or week
	// bucket depending on the granularity requested.
	BucketStart time.Time
	Wins        int
	Losses      int
	Draws       int
	TotalGames  int
	// WinRate is expressed as a float in [0, 1].  0 when TotalGames == 0.
	WinRate float64
}

// ─── FormatDistribution ──────────────────────────────────────────────────────

// FormatDistributionRow holds game count for a single format.
type FormatDistributionRow struct {
	Format    string
	GameCount int
}

// ─── StatsRepository ─────────────────────────────────────────────────────────

// StatsRepository provides read access to stats derived from the matches table,
// always scoped by account_id.
type StatsRepository struct {
	db DB
}

// NewStatsRepository returns a StatsRepository backed by db.
func NewStatsRepository(db DB) *StatsRepository {
	return &StatsRepository{db: db}
}

// GetDeckPerformance returns win/loss/draw counts per deck for the given
// account.  Only decks that appear in at least one match row are returned.
// Rows are ordered by total_games DESC, deck_id ASC so the most-played decks
// appear first.
func (r *StatsRepository) GetDeckPerformance(ctx context.Context, accountID int64) ([]DeckPerformanceRow, error) {
	const q = `
		SELECT
			m.deck_id,
			COALESCE(d.name, m.deck_id) AS deck_name,
			COALESCE(d.format, m.format) AS format,
			COUNT(*) FILTER (WHERE lower(m.result) = 'win')  AS wins,
			COUNT(*) FILTER (WHERE lower(m.result) = 'loss') AS losses,
			COUNT(*) FILTER (WHERE lower(m.result) = 'draw') AS draws,
			COUNT(*) AS total_games
		FROM matches m
		LEFT JOIN decks d ON d.id = m.deck_id AND d.account_id = m.account_id
		WHERE m.account_id = $1
		  AND m.deck_id IS NOT NULL
		GROUP BY m.deck_id, d.name, d.format, m.format
		ORDER BY total_games DESC, m.deck_id ASC`

	rows, err := r.db.QueryContext(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []DeckPerformanceRow
	for rows.Next() {
		var row DeckPerformanceRow
		if err := rows.Scan(
			&row.DeckID,
			&row.DeckName,
			&row.Format,
			&row.Wins,
			&row.Losses,
			&row.Draws,
			&row.TotalGames,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}

	return out, rows.Err()
}

// GetWinRateTrend returns win-rate buckets over the last 90 days for the given
// account.  granularity must be "daily" or "weekly"; any other value defaults
// to "daily".  Buckets with no matches are excluded from the result.
func (r *StatsRepository) GetWinRateTrend(ctx context.Context, accountID int64, granularity string) ([]WinRateBucket, error) {
	var trunc string
	if granularity == "weekly" {
		trunc = "week"
	} else {
		trunc = "day"
	}

	// Build the query dynamically using the sanitised trunc string (not from user
	// input directly — only "day" or "week" are ever assigned).
	q := `
		SELECT
			date_trunc('` + trunc + `', timestamp AT TIME ZONE 'UTC') AS bucket_start,
			COUNT(*) FILTER (WHERE lower(result) = 'win')  AS wins,
			COUNT(*) FILTER (WHERE lower(result) = 'loss') AS losses,
			COUNT(*) FILTER (WHERE lower(result) = 'draw') AS draws,
			COUNT(*) AS total_games
		FROM matches
		WHERE account_id = $1
		  AND timestamp >= NOW() - INTERVAL '90 days'
		GROUP BY bucket_start
		ORDER BY bucket_start ASC`

	rows, err := r.db.QueryContext(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []WinRateBucket
	for rows.Next() {
		var b WinRateBucket
		if err := rows.Scan(
			&b.BucketStart,
			&b.Wins,
			&b.Losses,
			&b.Draws,
			&b.TotalGames,
		); err != nil {
			return nil, err
		}
		if b.TotalGames > 0 {
			b.WinRate = float64(b.Wins) / float64(b.TotalGames)
		}
		out = append(out, b)
	}

	return out, rows.Err()
}

// GetFormatDistribution returns game count per format for the given account,
// ordered by game_count DESC.
func (r *StatsRepository) GetFormatDistribution(ctx context.Context, accountID int64) ([]FormatDistributionRow, error) {
	const q = `
		SELECT format, COUNT(*) AS game_count
		FROM matches
		WHERE account_id = $1
		  AND format IS NOT NULL
		  AND format <> ''
		GROUP BY format
		ORDER BY game_count DESC`

	rows, err := r.db.QueryContext(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []FormatDistributionRow
	for rows.Next() {
		var row FormatDistributionRow
		if err := rows.Scan(&row.Format, &row.GameCount); err != nil {
			return nil, err
		}
		out = append(out, row)
	}

	return out, rows.Err()
}

// ─── Draft Analytics ─────────────────────────────────────────────────────────

// DraftAnalyticsRow holds per-draft pick-efficiency and record data.
type DraftAnalyticsRow struct {
	SessionID   string
	SetCode     string
	DraftType   string
	StartTime   time.Time
	Wins        int
	Losses      int
	TotalPicks  int
	AvgGIHWR    *float64 // average picked_card_gihwr across this session's picks; nil when no pick data
	AvgPickRank *float64 // average pick_quality_rank across this session's picks; nil when no pick data
}

// RankProgressionRow holds a single rank-change event from the matches table.
type RankProgressionRow struct {
	MatchID    string
	OccurredAt time.Time
	Format     string
	RankBefore *string
	RankAfter  *string
	Result     string
}

// ResultBreakdownRow holds aggregate wins/losses grouped by format.
// Draws is always 0 (MTGA has no draw result) but is included for schema
// completeness and future use.
type ResultBreakdownRow struct {
	Format string
	Wins   int
	Losses int
	Draws  int
}

// ListDraftAnalytics returns per-session draft analytics for the given account
// ordered by start_time DESC.  setCode may be empty to return all sets.
// Keyset pagination: afterID is the session ID from the last row of the
// previous page ("" on the first page).  Returns up to limit+1 rows so the
// caller can detect has_more.
func (r *StatsRepository) ListDraftAnalytics(
	ctx context.Context,
	accountID int64,
	setCode string,
	afterStartTime *time.Time,
	afterID string,
	limit int,
) ([]DraftAnalyticsRow, error) {
	fetch := limit + 1

	var (
		rows *sql.Rows
		err  error
	)

	switch {
	case setCode != "" && afterStartTime != nil:
		const q = `
			SELECT ds.id, ds.set_code, ds.draft_type, ds.start_time,
			       COALESCE(SUM(CASE WHEN dmr.result = 'win'  THEN 1 ELSE 0 END), 0) AS wins,
			       COALESCE(SUM(CASE WHEN dmr.result = 'loss' THEN 1 ELSE 0 END), 0) AS losses,
			       ds.total_picks,
			       AVG(dp.picked_card_gihwr) AS avg_gihwr,
			       AVG(dp.pick_quality_rank) AS avg_pick_rank
			FROM draft_sessions ds
			LEFT JOIN draft_match_results dmr ON dmr.session_id = ds.id
			LEFT JOIN draft_picks dp ON dp.session_id = ds.id
			WHERE ds.account_id = $1
			  AND ds.set_code = $2
			  AND (ds.start_time < $3 OR (ds.start_time = $3 AND ds.id < $4))
			GROUP BY ds.id, ds.set_code, ds.draft_type, ds.start_time, ds.total_picks
			ORDER BY ds.start_time DESC, ds.id DESC
			LIMIT $5`

		rows, err = r.db.QueryContext(ctx, q, accountID, setCode, *afterStartTime, afterID, fetch)

	case setCode != "" && afterStartTime == nil:
		const q = `
			SELECT ds.id, ds.set_code, ds.draft_type, ds.start_time,
			       COALESCE(SUM(CASE WHEN dmr.result = 'win'  THEN 1 ELSE 0 END), 0) AS wins,
			       COALESCE(SUM(CASE WHEN dmr.result = 'loss' THEN 1 ELSE 0 END), 0) AS losses,
			       ds.total_picks,
			       AVG(dp.picked_card_gihwr) AS avg_gihwr,
			       AVG(dp.pick_quality_rank) AS avg_pick_rank
			FROM draft_sessions ds
			LEFT JOIN draft_match_results dmr ON dmr.session_id = ds.id
			LEFT JOIN draft_picks dp ON dp.session_id = ds.id
			WHERE ds.account_id = $1 AND ds.set_code = $2
			GROUP BY ds.id, ds.set_code, ds.draft_type, ds.start_time, ds.total_picks
			ORDER BY ds.start_time DESC, ds.id DESC
			LIMIT $3`

		rows, err = r.db.QueryContext(ctx, q, accountID, setCode, fetch)

	case setCode == "" && afterStartTime != nil:
		const q = `
			SELECT ds.id, ds.set_code, ds.draft_type, ds.start_time,
			       COALESCE(SUM(CASE WHEN dmr.result = 'win'  THEN 1 ELSE 0 END), 0) AS wins,
			       COALESCE(SUM(CASE WHEN dmr.result = 'loss' THEN 1 ELSE 0 END), 0) AS losses,
			       ds.total_picks,
			       AVG(dp.picked_card_gihwr) AS avg_gihwr,
			       AVG(dp.pick_quality_rank) AS avg_pick_rank
			FROM draft_sessions ds
			LEFT JOIN draft_match_results dmr ON dmr.session_id = ds.id
			LEFT JOIN draft_picks dp ON dp.session_id = ds.id
			WHERE ds.account_id = $1
			  AND (ds.start_time < $2 OR (ds.start_time = $2 AND ds.id < $3))
			GROUP BY ds.id, ds.set_code, ds.draft_type, ds.start_time, ds.total_picks
			ORDER BY ds.start_time DESC, ds.id DESC
			LIMIT $4`

		rows, err = r.db.QueryContext(ctx, q, accountID, *afterStartTime, afterID, fetch)

	default:
		const q = `
			SELECT ds.id, ds.set_code, ds.draft_type, ds.start_time,
			       COALESCE(SUM(CASE WHEN dmr.result = 'win'  THEN 1 ELSE 0 END), 0) AS wins,
			       COALESCE(SUM(CASE WHEN dmr.result = 'loss' THEN 1 ELSE 0 END), 0) AS losses,
			       ds.total_picks,
			       AVG(dp.picked_card_gihwr) AS avg_gihwr,
			       AVG(dp.pick_quality_rank) AS avg_pick_rank
			FROM draft_sessions ds
			LEFT JOIN draft_match_results dmr ON dmr.session_id = ds.id
			LEFT JOIN draft_picks dp ON dp.session_id = ds.id
			WHERE ds.account_id = $1
			GROUP BY ds.id, ds.set_code, ds.draft_type, ds.start_time, ds.total_picks
			ORDER BY ds.start_time DESC, ds.id DESC
			LIMIT $2`

		rows, err = r.db.QueryContext(ctx, q, accountID, fetch)
	}

	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var out []DraftAnalyticsRow

	for rows.Next() {
		var row DraftAnalyticsRow

		if err := rows.Scan(
			&row.SessionID,
			&row.SetCode,
			&row.DraftType,
			&row.StartTime,
			&row.Wins,
			&row.Losses,
			&row.TotalPicks,
			&row.AvgGIHWR,
			&row.AvgPickRank,
		); err != nil {
			return nil, err
		}

		out = append(out, row)
	}

	return out, rows.Err()
}

// ListRankProgression returns matches that carry rank information for the given
// account ordered by timestamp DESC.  format may be empty to return all
// formats.  Keyset pagination by (timestamp, id).  Returns up to limit+1 rows.
func (r *StatsRepository) ListRankProgression(
	ctx context.Context,
	accountID int64,
	format string,
	cursorTS *time.Time,
	cursorID string,
	limit int,
) ([]RankProgressionRow, error) {
	fetch := limit + 1

	var (
		rows *sql.Rows
		err  error
	)

	switch {
	case format != "" && cursorTS != nil:
		const q = `
			SELECT id, timestamp, format, rank_before, rank_after, result
			FROM matches
			WHERE account_id = $1
			  AND lower(format) = lower($2)
			  AND (rank_before IS NOT NULL OR rank_after IS NOT NULL)
			  AND (timestamp < $3 OR (timestamp = $3 AND id < $4))
			ORDER BY timestamp DESC, id DESC
			LIMIT $5`

		rows, err = r.db.QueryContext(ctx, q, accountID, format, *cursorTS, cursorID, fetch)

	case format != "" && cursorTS == nil:
		const q = `
			SELECT id, timestamp, format, rank_before, rank_after, result
			FROM matches
			WHERE account_id = $1
			  AND lower(format) = lower($2)
			  AND (rank_before IS NOT NULL OR rank_after IS NOT NULL)
			ORDER BY timestamp DESC, id DESC
			LIMIT $3`

		rows, err = r.db.QueryContext(ctx, q, accountID, format, fetch)

	case format == "" && cursorTS != nil:
		const q = `
			SELECT id, timestamp, format, rank_before, rank_after, result
			FROM matches
			WHERE account_id = $1
			  AND (rank_before IS NOT NULL OR rank_after IS NOT NULL)
			  AND (timestamp < $2 OR (timestamp = $2 AND id < $3))
			ORDER BY timestamp DESC, id DESC
			LIMIT $4`

		rows, err = r.db.QueryContext(ctx, q, accountID, *cursorTS, cursorID, fetch)

	default:
		const q = `
			SELECT id, timestamp, format, rank_before, rank_after, result
			FROM matches
			WHERE account_id = $1
			  AND (rank_before IS NOT NULL OR rank_after IS NOT NULL)
			ORDER BY timestamp DESC, id DESC
			LIMIT $2`

		rows, err = r.db.QueryContext(ctx, q, accountID, fetch)
	}

	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var out []RankProgressionRow

	for rows.Next() {
		var row RankProgressionRow

		if err := rows.Scan(
			&row.MatchID,
			&row.OccurredAt,
			&row.Format,
			&row.RankBefore,
			&row.RankAfter,
			&row.Result,
		); err != nil {
			return nil, err
		}

		out = append(out, row)
	}

	return out, rows.Err()
}

// GetResultBreakdown returns aggregate wins/losses grouped by format for the
// given account.  format may be empty to return all formats; when set it
// filters to that specific format.
func (r *StatsRepository) GetResultBreakdown(
	ctx context.Context,
	accountID int64,
	format string,
) ([]ResultBreakdownRow, error) {
	var (
		rows *sql.Rows
		err  error
	)

	if format != "" {
		const q = `
			SELECT format,
			       COALESCE(SUM(CASE WHEN result = 'win'  THEN 1 ELSE 0 END), 0) AS wins,
			       COALESCE(SUM(CASE WHEN result = 'loss' THEN 1 ELSE 0 END), 0) AS losses,
			       0 AS draws
			FROM matches
			WHERE account_id = $1 AND lower(format) = lower($2)
			GROUP BY format
			ORDER BY format`

		rows, err = r.db.QueryContext(ctx, q, accountID, format)
	} else {
		const q = `
			SELECT format,
			       COALESCE(SUM(CASE WHEN result = 'win'  THEN 1 ELSE 0 END), 0) AS wins,
			       COALESCE(SUM(CASE WHEN result = 'loss' THEN 1 ELSE 0 END), 0) AS losses,
			       0 AS draws
			FROM matches
			WHERE account_id = $1
			GROUP BY format
			ORDER BY format`

		rows, err = r.db.QueryContext(ctx, q, accountID)
	}

	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var out []ResultBreakdownRow

	for rows.Next() {
		var row ResultBreakdownRow

		if err := rows.Scan(&row.Format, &row.Wins, &row.Losses, &row.Draws); err != nil {
			return nil, err
		}

		out = append(out, row)
	}

	return out, rows.Err()
}
