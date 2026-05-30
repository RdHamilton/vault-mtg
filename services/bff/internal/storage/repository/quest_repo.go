package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// QuestProgressUpsert holds a single quest entry written to the quests table
// from a quest.progress daemon event.
type QuestProgressUpsert struct {
	AccountID int64 // accounts.id FK (resolved from client_id by the projection worker)
	QuestID   string
	QuestName string
	Progress  int
	Goal      int
	CanSwap   bool
	SeenAt    time.Time
}

// QuestCompletedInsert holds the fields written to quest_session_tracking
// from a quest.completed daemon event.  AccountID is the resolved accounts.id
// BIGINT FK (migration 000080 converts the column from TEXT client_id to
// BIGINT FK so every write is properly tenant-scoped).
type QuestCompletedInsert struct {
	AccountID        int64
	QuestID          string
	QuestName        string
	Progress         int
	Goal             int
	XPReward         int
	CompletionSource string
	OccurredAt       time.Time
}

// QuestRepository writes quest data to the quests and quest_session_tracking tables.
type QuestRepository struct {
	db DB
}

// NewQuestRepository returns a QuestRepository backed by db.
func NewQuestRepository(db DB) *QuestRepository {
	return &QuestRepository{db: db}
}

// UpsertQuestProgress inserts or updates a quest row scoped by account_id.
// The quests table unique constraint is (account_id, quest_id) after migration
// 000096.  On conflict the progress fields are updated in place so that repeated
// daemon sync events for the same quest produce exactly one row, not one row
// per event.
func (r *QuestRepository) UpsertQuestProgress(ctx context.Context, u QuestProgressUpsert) error {
	const q = `
		INSERT INTO quests (
			account_id,
			quest_id,
			quest_type,
			goal,
			starting_progress,
			ending_progress,
			completed,
			can_swap,
			first_seen_at,
			last_seen_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
		ON CONFLICT (account_id, quest_id) DO UPDATE
			SET ending_progress = EXCLUDED.ending_progress,
			    can_swap        = EXCLUDED.can_swap,
			    last_seen_at    = EXCLUDED.last_seen_at`

	_, err := r.db.ExecContext(
		ctx, q,
		u.AccountID, // $1 account_id
		u.QuestID,   // $2 quest_id
		u.QuestName, // $3 quest_type (name used as type label from daemon payload)
		u.Goal,      // $4 goal
		u.Progress,  // $5 starting_progress
		u.Progress,  // $6 ending_progress
		false,       // $7 completed — quest.progress event means it is still active
		u.CanSwap,   // $8 can_swap
		u.SeenAt,    // $9 first_seen_at / last_seen_at
	)

	return err
}

// InsertQuestCompleted writes a quest completion record to quest_session_tracking.
// The unique constraint on (account_id, quest_id, occurred_at) makes repeated
// projection of the same event a no-op.
func (r *QuestRepository) InsertQuestCompleted(ctx context.Context, ins QuestCompletedInsert) error {
	const q = `
		INSERT INTO quest_session_tracking (
			account_id,
			quest_id,
			quest_name,
			progress,
			goal,
			xp_reward,
			completion_source,
			occurred_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (account_id, quest_id, occurred_at) DO NOTHING`

	_, err := r.db.ExecContext(
		ctx, q,
		ins.AccountID,
		ins.QuestID,
		ins.QuestName,
		ins.Progress,
		ins.Goal,
		ins.XPReward,
		nullableString(ins.CompletionSource),
		ins.OccurredAt,
	)

	return err
}

// nullableString returns nil for an empty string so it is stored as NULL.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

// ─── Phase 2 PR #3 reads ─────────────────────────────────────────────────────

// QuestRow is the read-side projection of a quests row, scoped to one
// account. The handler maps it into the SPA's models.Quest snake_case shape.
type QuestRow struct {
	ID               int64
	QuestID          string
	QuestType        *string
	Goal             int
	StartingProgress int
	EndingProgress   int
	Completed        bool
	CanSwap          bool
	Rewards          *string
	FirstSeenAt      time.Time
	CompletedAt      *time.Time
	LastSeenAt       *time.Time
	Rerolled         bool
	CreatedAt        time.Time
	SessionID        *string
	CompletionSource *string
}

// ListActiveByAccountID returns the account's currently active quests
// (completed = false), most recent assignment first. Limited to 50 — the
// SPA only shows a handful at a time.
func (r *QuestRepository) ListActiveByAccountID(ctx context.Context, accountID int64) ([]QuestRow, error) {
	const q = `SELECT id, quest_id, quest_type, goal, starting_progress, ending_progress,
	                  completed, can_swap, rewards, first_seen_at, completed_at, last_seen_at,
	                  rerolled, created_at, session_id, completion_source
	           FROM quests
	           WHERE account_id = $1 AND completed = FALSE
	           ORDER BY first_seen_at DESC, id DESC
	           LIMIT 50`
	return r.scanQuestRows(ctx, q, accountID)
}

// ListHistoryByAccountID returns completed quests in the optional date window,
// newest-first, capped at limit (default 100, max 500). Empty start/end means
// "no bound on that side".
func (r *QuestRepository) ListHistoryByAccountID(ctx context.Context, accountID int64, start, end *time.Time, limit int) ([]QuestRow, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	clauses := []string{"account_id = $1", "completed = TRUE"}
	args := []any{accountID}
	next := 2
	if start != nil {
		clauses = append(clauses, "completed_at >= $"+itoa(next))
		args = append(args, *start)
		next++
	}
	if end != nil {
		clauses = append(clauses, "completed_at <= $"+itoa(next))
		args = append(args, *end)
		next++
	}
	q := `SELECT id, quest_id, quest_type, goal, starting_progress, ending_progress,
	             completed, can_swap, rewards, first_seen_at, completed_at, last_seen_at,
	             rerolled, created_at, session_id, completion_source
	      FROM quests
	      WHERE ` + strings.Join(clauses, " AND ") + `
	      ORDER BY completed_at DESC NULLS LAST, id DESC
	      LIMIT $` + itoa(next)
	args = append(args, limit)
	return r.scanQuestRows(ctx, q, args...)
}

// CountWinsSince returns the number of matches the account has won since `since`.
// Used for daily/weekly wins progress tracking.
func (r *QuestRepository) CountWinsSince(ctx context.Context, accountID int64, since time.Time) (int, error) {
	const q = `SELECT COUNT(*) FROM matches
	           WHERE account_id = $1 AND lower(result) = 'win' AND timestamp >= $2`
	var n int
	if err := r.db.QueryRowContext(ctx, q, accountID, since).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// QuestStatsAggregate is the result of QuestStats — compatible with the SPA's
// models.QuestStats wire shape (the handler just renames the fields to
// snake_case).
type QuestStatsAggregate struct {
	TotalQuests         int
	CompletedQuests     int
	ActiveQuests        int
	TotalGoldEarned     int
	AverageCompletionMS int64
	RerollCount         int
}

// QuestStats returns aggregate quest counts for the account in the given
// window. CompletionRate is computed by the handler so the repo stays SQL-
// only. Gold reward is parsed from rewards (TEXT, free-form) — for now we
// approximate as 0 (proper parsing requires a richer rewards schema).
func (r *QuestRepository) QuestStats(ctx context.Context, accountID int64, start, end time.Time) (QuestStatsAggregate, error) {
	const q = `SELECT
	             COUNT(*)                                                                     AS total_quests,
	             COUNT(*) FILTER (WHERE completed = TRUE)                                     AS completed_quests,
	             COUNT(*) FILTER (WHERE completed = FALSE)                                    AS active_quests,
	             COUNT(*) FILTER (WHERE rerolled = TRUE)                                      AS reroll_count,
	             COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - first_seen_at)) * 1000)
	                FILTER (WHERE completed_at IS NOT NULL), 0)::BIGINT                       AS avg_completion_ms
	           FROM quests
	           WHERE account_id = $1 AND first_seen_at >= $2 AND first_seen_at <= $3`
	var s QuestStatsAggregate
	if err := r.db.QueryRowContext(ctx, q, accountID, start, end).Scan(
		&s.TotalQuests, &s.CompletedQuests, &s.ActiveQuests,
		&s.RerollCount, &s.AverageCompletionMS,
	); err != nil {
		return QuestStatsAggregate{}, err
	}
	// Gold rewards are stored in the rewards TEXT column as a free-form
	// blob. Parsing them requires knowing the daemon's reward serialisation
	// format, which is not in scope for this PR. Surface 0 for now and
	// revisit when the rewards schema gets formalised.
	s.TotalGoldEarned = 0
	return s, nil
}

// LastQuestSeenAt returns the most recent last_seen_at across all quest rows
// for the account. Used by /quests/active to expose a "data freshness" hint
// in ActiveQuestsResponse.last_updated. Returns the zero time + ok=false
// when the account has no quests yet.
func (r *QuestRepository) LastQuestSeenAt(ctx context.Context, accountID int64) (time.Time, bool, error) {
	const q = `SELECT COALESCE(MAX(last_seen_at), MAX(first_seen_at))
	           FROM quests
	           WHERE account_id = $1`
	var ts sql.NullTime
	if err := r.db.QueryRowContext(ctx, q, accountID).Scan(&ts); err != nil {
		return time.Time{}, false, err
	}
	if !ts.Valid {
		return time.Time{}, false, nil
	}
	return ts.Time, true, nil
}

// scanQuestRows is the shared decoder used by ListActive/ListHistory.
func (r *QuestRepository) scanQuestRows(ctx context.Context, q string, args ...any) ([]QuestRow, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []QuestRow
	for rows.Next() {
		var qr QuestRow
		if err := rows.Scan(
			&qr.ID, &qr.QuestID, &qr.QuestType, &qr.Goal, &qr.StartingProgress, &qr.EndingProgress,
			&qr.Completed, &qr.CanSwap, &qr.Rewards, &qr.FirstSeenAt, &qr.CompletedAt, &qr.LastSeenAt,
			&qr.Rerolled, &qr.CreatedAt, &qr.SessionID, &qr.CompletionSource,
		); err != nil {
			return nil, err
		}
		out = append(out, qr)
	}
	return out, rows.Err()
}
