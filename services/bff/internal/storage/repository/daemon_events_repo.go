package repository

import (
	"context"
	"encoding/json"
	"time"
)

// DaemonEventRow is a single row from the daemon_events table.
type DaemonEventRow struct {
	ID         int64
	UserID     int64
	AccountID  string
	EventType  string
	Payload    json.RawMessage
	OccurredAt time.Time
	ReceivedAt time.Time
}

// DaemonEventsRepository persists daemon events to the daemon_events table.
type DaemonEventsRepository struct {
	db DB
}

// NewDaemonEventsRepository returns a DaemonEventsRepository backed by db.
func NewDaemonEventsRepository(db DB) *DaemonEventsRepository {
	return &DaemonEventsRepository{db: db}
}

// Insert writes a daemon event row scoped to the given user_id and account_id.
// occurred_at is stored as-is; received_at defaults to NOW() via the column default.
func (r *DaemonEventsRepository) Insert(
	ctx context.Context,
	userID int64,
	accountID string,
	eventType string,
	payload json.RawMessage,
	occurredAt time.Time,
) error {
	const q = `
		INSERT INTO daemon_events (user_id, account_id, event_type, payload, occurred_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := r.db.ExecContext(ctx, q, userID, accountID, eventType, payload, occurredAt)

	return err
}

// ListByUserID returns up to limit daemon event rows for the given user,
// ordered newest-first.  It never returns rows belonging to other users.
func (r *DaemonEventsRepository) ListByUserID(
	ctx context.Context,
	userID int64,
	limit int,
) ([]DaemonEventRow, error) {
	const q = `
		SELECT id, user_id, account_id, event_type, payload, occurred_at, received_at
		FROM daemon_events
		WHERE user_id = $1
		ORDER BY occurred_at DESC
		LIMIT $2`

	rows, err := r.db.QueryContext(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []DaemonEventRow

	for rows.Next() {
		var e DaemonEventRow

		if err := rows.Scan(
			&e.ID, &e.UserID, &e.AccountID, &e.EventType,
			&e.Payload, &e.OccurredAt, &e.ReceivedAt,
		); err != nil {
			return nil, err
		}

		events = append(events, e)
	}

	return events, rows.Err()
}
