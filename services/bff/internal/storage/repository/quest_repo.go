package repository

import (
	"context"
	"time"
)

// QuestProgressUpsert holds a single quest entry written to the quests table
// from a quest.progress daemon event.
type QuestProgressUpsert struct {
	AccountID string
	QuestID   string
	QuestName string
	Progress  int
	Goal      int
	CanSwap   bool
	SeenAt    time.Time
}

// QuestCompletedInsert holds the fields written to quest_session_tracking
// from a quest.completed daemon event.
type QuestCompletedInsert struct {
	AccountID        string
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
// The quests table unique constraint is (quest_id, assigned_at); we treat
// seen_at as assigned_at when upserting from daemon events so each observed
// quest state is recorded.  On conflict, progress fields are updated.
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
			assigned_at,
			last_seen_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
		ON CONFLICT (quest_id, assigned_at) DO UPDATE
			SET ending_progress = EXCLUDED.ending_progress,
			    can_swap        = EXCLUDED.can_swap,
			    last_seen_at    = EXCLUDED.last_seen_at,
			    account_id      = EXCLUDED.account_id`

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
		u.SeenAt,    // $9 assigned_at / last_seen_at
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
