package repository

import (
	"context"
	"database/sql"
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

	defer rows.Close()

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
	defer rows.Close()

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
