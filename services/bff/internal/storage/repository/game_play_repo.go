package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// GamePlayInsert holds the data needed to insert a single game_plays row.
type GamePlayInsert struct {
	AccountID     int64
	MatchID       string
	GameNumber    int
	WinningTeamID int
	TurnCount     int
	DurationSecs  int
	Sequence      uint64
	OccurredAt    time.Time
	// Partial indicates the row was emitted before the game was confirmed
	// complete — the GRE buffer hit its flush threshold or the stale sweep
	// evicted it.  Maps to the partial column added in migration 000074.
	Partial bool
}

// LifeChangeInsert holds one life-change row to be written to
// life_change_tracking.
type LifeChangeInsert struct {
	AccountID  int64
	GamePlayID int64
	TeamID     int
	LifeTotal  int
	Delta      int
	TurnNumber int
}

// GamePlayRow is returned when reading a game_plays row.
type GamePlayRow struct {
	ID            int64
	AccountID     int64
	MatchID       string
	GameNumber    int
	WinningTeamID int
	TurnCount     int
	DurationSecs  int
	Sequence      uint64
	OccurredAt    time.Time
	Partial       bool
}

// GamePlayRepository provides write and read access to game_plays and
// life_change_tracking, always scoped by account_id.
type GamePlayRepository struct {
	db DB
}

// NewGamePlayRepository returns a GamePlayRepository backed by db.
func NewGamePlayRepository(db DB) *GamePlayRepository {
	return &GamePlayRepository{db: db}
}

// InsertGamePlay inserts or updates a game_plays row identified by
// (account_id, match_id, game_number) and returns the row's id.
//
// On conflict the row is updated only when the incoming sequence is strictly
// greater than the stored one, preserving causal ordering across out-of-order
// daemon retransmissions.
func (r *GamePlayRepository) InsertGamePlay(ctx context.Context, ins GamePlayInsert) (int64, error) {
	const q = `
		INSERT INTO game_plays
			(account_id, match_id, game_number, winning_team_id, turn_count,
			 duration_secs, sequence, occurred_at, partial)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (account_id, match_id, game_number)
		DO UPDATE SET
			winning_team_id = EXCLUDED.winning_team_id,
			turn_count      = EXCLUDED.turn_count,
			duration_secs   = EXCLUDED.duration_secs,
			sequence        = EXCLUDED.sequence,
			occurred_at     = EXCLUDED.occurred_at,
			partial         = EXCLUDED.partial
		WHERE game_plays.sequence < EXCLUDED.sequence
		RETURNING id`

	var id int64
	err := r.db.QueryRowContext(
		ctx, q,
		ins.AccountID,
		ins.MatchID,
		ins.GameNumber,
		ins.WinningTeamID,
		ins.TurnCount,
		ins.DurationSecs,
		ins.Sequence,
		ins.OccurredAt,
		ins.Partial,
	).Scan(&id)

	if err == sql.ErrNoRows {
		// ON CONFLICT DO UPDATE WHERE clause was false (sequence not greater).
		// Fetch the existing id so callers can still insert life_changes.
		return r.getGamePlayID(ctx, ins.AccountID, ins.MatchID, ins.GameNumber)
	}

	return id, err
}

// getGamePlayID returns the id of an existing game_plays row.
func (r *GamePlayRepository) getGamePlayID(ctx context.Context, accountID int64, matchID string, gameNumber int) (int64, error) {
	const q = `
		SELECT id FROM game_plays
		WHERE account_id = $1 AND match_id = $2 AND game_number = $3`

	var id int64
	err := r.db.QueryRowContext(ctx, q, accountID, matchID, gameNumber).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("getGamePlayID: %w", err)
	}

	return id, nil
}

// InsertLifeChanges bulk-inserts life_change_tracking rows for a game.
// Each row is scoped by account_id and references game_play_id.
// Duplicate inserts (same game_play_id, team_id, turn_number, life_total) are
// silently ignored so replaying the same event is safe.
func (r *GamePlayRepository) InsertLifeChanges(ctx context.Context, changes []LifeChangeInsert) error {
	if len(changes) == 0 {
		return nil
	}

	const q = `
		INSERT INTO life_change_tracking
			(account_id, game_play_id, team_id, life_total, delta, turn_number)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (game_play_id, team_id, turn_number) DO NOTHING`

	for i := range changes {
		c := changes[i]
		if _, err := r.db.ExecContext(
			ctx, q,
			c.AccountID,
			c.GamePlayID,
			c.TeamID,
			c.LifeTotal,
			c.Delta,
			c.TurnNumber,
		); err != nil {
			return fmt.Errorf("InsertLifeChanges[%d]: %w", i, err)
		}
	}

	return nil
}

// GetGamePlay returns a single game_plays row by (account_id, match_id, game_number).
// Returns sql.ErrNoRows when no row exists.
func (r *GamePlayRepository) GetGamePlay(ctx context.Context, accountID int64, matchID string, gameNumber int) (GamePlayRow, error) {
	const q = `
		SELECT id, account_id, match_id, game_number, winning_team_id,
		       turn_count, duration_secs, sequence, occurred_at, partial
		FROM game_plays
		WHERE account_id = $1 AND match_id = $2 AND game_number = $3`

	var row GamePlayRow
	err := r.db.QueryRowContext(ctx, q, accountID, matchID, gameNumber).Scan(
		&row.ID,
		&row.AccountID,
		&row.MatchID,
		&row.GameNumber,
		&row.WinningTeamID,
		&row.TurnCount,
		&row.DurationSecs,
		&row.Sequence,
		&row.OccurredAt,
		&row.Partial,
	)

	return row, err
}

// ListGamePlaysByMatch returns all game_plays rows for a match ordered by
// (occurred_at, sequence) — the canonical per-session ordering defined in the
// projection layer v2 spec.
func (r *GamePlayRepository) ListGamePlaysByMatch(ctx context.Context, accountID int64, matchID string) ([]GamePlayRow, error) {
	const q = `
		SELECT id, account_id, match_id, game_number, winning_team_id,
		       turn_count, duration_secs, sequence, occurred_at, partial
		FROM game_plays
		WHERE account_id = $1 AND match_id = $2
		ORDER BY occurred_at, sequence`

	rows, err := r.db.QueryContext(ctx, q, accountID, matchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []GamePlayRow
	for rows.Next() {
		var row GamePlayRow
		if err := rows.Scan(
			&row.ID,
			&row.AccountID,
			&row.MatchID,
			&row.GameNumber,
			&row.WinningTeamID,
			&row.TurnCount,
			&row.DurationSecs,
			&row.Sequence,
			&row.OccurredAt,
			&row.Partial,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}

	return out, rows.Err()
}

// CountLifeChangesByGame returns the number of life_change_tracking rows for
// the given game_play_id.  Used in integration tests.
func (r *GamePlayRepository) CountLifeChangesByGame(ctx context.Context, gamePlayID int64) (int, error) {
	const q = `SELECT COUNT(*) FROM life_change_tracking WHERE game_play_id = $1`

	var n int
	err := r.db.QueryRowContext(ctx, q, gamePlayID).Scan(&n)

	return n, err
}
