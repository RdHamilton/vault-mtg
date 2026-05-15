package repository

import (
	"context"
	"database/sql"
	"strconv"
	"time"
)

// GamePlaysRepository serves the Phase 2 /api/v1/matches/{id}/plays/* and
// /api/v1/gameplays/* read paths. It deals with the turn-by-turn play
// telemetry tables: game_plays, game_state_snapshots, and
// opponent_cards_observed. Scoping is enforced by joining game_plays.match_id
// → matches.id → matches.account_id.
//
// Note: a GamePlayRepository already exists for the projection worker's
// per-game/per-life-change writes (see game_play_repo.go). That type uses a
// different conceptual "GamePlayRow" — the per-game life-change tracker — so
// the play-by-play action data lives here in its own type to avoid
// collisions.
type GamePlaysRepository struct {
	db DB
}

// NewGamePlaysRepository returns a GamePlaysRepository backed by db.
func NewGamePlaysRepository(db DB) *GamePlaysRepository {
	return &GamePlaysRepository{db: db}
}

// GamePlayActionRow is one row from the game_plays table — a single in-game
// action (play_card, attack, mulligan, life_change, etc.). Pointer fields
// are nullable in the schema.
type GamePlayActionRow struct {
	ID             int64
	GameID         int64
	MatchID        string
	TurnNumber     int
	Phase          *string
	Step           *string
	PlayerType     string
	ActionType     string
	CardID         *int
	CardName       *string
	ZoneFrom       *string
	ZoneTo         *string
	LifeFrom       *int
	LifeTo         *int
	Timestamp      time.Time
	SequenceNumber int
	CreatedAt      time.Time
}

// PlaysByMatch returns every action recorded for the match in sequence
// order. Scoped to the account via a matches join.
func (r *GamePlaysRepository) PlaysByMatch(ctx context.Context, accountID int64, matchID string) ([]GamePlayActionRow, error) {
	const q = `SELECT gp.id, gp.game_id, gp.match_id, gp.turn_number, gp.phase, gp.step,
	                  gp.player_type, gp.action_type, gp.card_id, gp.card_name,
	                  gp.zone_from, gp.zone_to, gp.life_from, gp.life_to,
	                  gp.timestamp, gp.sequence_number, gp.created_at
	           FROM game_plays gp
	           JOIN matches m ON m.id = gp.match_id
	           WHERE m.account_id = $1 AND gp.match_id = $2
	           ORDER BY gp.turn_number, gp.sequence_number, gp.id`
	return r.scanGamePlayRows(ctx, q, accountID, matchID)
}

// PlaysByGameID returns every action for a single game_id, scoped to the
// account via games → matches.
func (r *GamePlaysRepository) PlaysByGameID(ctx context.Context, accountID int64, gameID int64) ([]GamePlayActionRow, error) {
	const q = `SELECT gp.id, gp.game_id, gp.match_id, gp.turn_number, gp.phase, gp.step,
	                  gp.player_type, gp.action_type, gp.card_id, gp.card_name,
	                  gp.zone_from, gp.zone_to, gp.life_from, gp.life_to,
	                  gp.timestamp, gp.sequence_number, gp.created_at
	           FROM game_plays gp
	           JOIN games g     ON g.id = gp.game_id
	           JOIN matches m   ON m.id = g.match_id
	           WHERE m.account_id = $1 AND gp.game_id = $2
	           ORDER BY gp.turn_number, gp.sequence_number, gp.id`
	return r.scanGamePlayRows(ctx, q, accountID, gameID)
}

// scanGamePlayRows centralises the row-scan boilerplate.
func (r *GamePlaysRepository) scanGamePlayRows(ctx context.Context, q string, args ...any) ([]GamePlayActionRow, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []GamePlayActionRow
	for rows.Next() {
		var p GamePlayActionRow
		if err := rows.Scan(
			&p.ID, &p.GameID, &p.MatchID, &p.TurnNumber, &p.Phase, &p.Step,
			&p.PlayerType, &p.ActionType, &p.CardID, &p.CardName,
			&p.ZoneFrom, &p.ZoneTo, &p.LifeFrom, &p.LifeTo,
			&p.Timestamp, &p.SequenceNumber, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GameSnapshotRow mirrors a row in game_state_snapshots.
type GameSnapshotRow struct {
	ID                  int64
	GameID              int64
	MatchID             string
	TurnNumber          int
	ActivePlayer        string
	PlayerLife          *int
	OpponentLife        *int
	PlayerCardsInHand   *int
	OpponentCardsInHand *int
	PlayerLandsInPlay   *int
	OpponentLandsInPlay *int
	BoardStateJSON      *string
	Timestamp           time.Time
}

// SnapshotsByMatch returns every snapshot for the match, scoped to account.
// gameID may be 0 to mean "all games for this match".
func (r *GamePlaysRepository) SnapshotsByMatch(ctx context.Context, accountID int64, matchID string, gameID int64) ([]GameSnapshotRow, error) {
	q := `SELECT s.id, s.game_id, s.match_id, s.turn_number, s.active_player,
	             s.player_life, s.opponent_life,
	             s.player_cards_in_hand, s.opponent_cards_in_hand,
	             s.player_lands_in_play, s.opponent_lands_in_play,
	             s.board_state_json, s.timestamp
	      FROM game_state_snapshots s
	      JOIN matches m ON m.id = s.match_id
	      WHERE m.account_id = $1 AND s.match_id = $2`
	args := []any{accountID, matchID}
	if gameID > 0 {
		q += " AND s.game_id = $" + strconv.Itoa(len(args)+1)
		args = append(args, gameID)
	}
	q += " ORDER BY s.turn_number, s.id"
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []GameSnapshotRow
	for rows.Next() {
		var s GameSnapshotRow
		if err := rows.Scan(
			&s.ID, &s.GameID, &s.MatchID, &s.TurnNumber, &s.ActivePlayer,
			&s.PlayerLife, &s.OpponentLife,
			&s.PlayerCardsInHand, &s.OpponentCardsInHand,
			&s.PlayerLandsInPlay, &s.OpponentLandsInPlay,
			&s.BoardStateJSON, &s.Timestamp,
		); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// OpponentCardRow mirrors a row in opponent_cards_observed.
type OpponentCardRow struct {
	ID            int64
	GameID        int64
	MatchID       string
	CardID        int
	CardName      *string
	ZoneObserved  *string
	TurnFirstSeen *int
	TimesSeen     int
}

// OpponentCardsByMatch returns the opponent-revealed cards for the match,
// scoped to account.
func (r *GamePlaysRepository) OpponentCardsByMatch(ctx context.Context, accountID int64, matchID string) ([]OpponentCardRow, error) {
	const q = `SELECT oc.id, oc.game_id, oc.match_id, oc.card_id, oc.card_name,
	                  oc.zone_observed, oc.turn_first_seen, oc.times_seen
	           FROM opponent_cards_observed oc
	           JOIN matches m ON m.id = oc.match_id
	           WHERE m.account_id = $1 AND oc.match_id = $2
	           ORDER BY oc.turn_first_seen, oc.id`
	rows, err := r.db.QueryContext(ctx, q, accountID, matchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []OpponentCardRow
	for rows.Next() {
		var o OpponentCardRow
		if err := rows.Scan(
			&o.ID, &o.GameID, &o.MatchID, &o.CardID, &o.CardName,
			&o.ZoneObserved, &o.TurnFirstSeen, &o.TimesSeen,
		); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// MatchExistsForAccount returns whether the match belongs to the account.
// Used by the play handlers as a cheap precheck (cheaper than running the
// full play query just to discover an empty result is "not yours" vs
// "no plays recorded yet").
func (r *GamePlaysRepository) MatchExistsForAccount(ctx context.Context, accountID int64, matchID string) (bool, error) {
	const q = `SELECT 1 FROM matches WHERE account_id = $1 AND id = $2 LIMIT 1`
	var n int
	err := r.db.QueryRowContext(ctx, q, accountID, matchID).Scan(&n)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
