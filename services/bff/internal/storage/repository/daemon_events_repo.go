// Package repository provides data-access helpers for the BFF service.
package repository

import (
	"context"
	"encoding/json"
	"time"
)

// DaemonEventRow is the in-memory representation of a row in the daemon_events table.
type DaemonEventRow struct {
	ID         int64
	UserID     int64
	AccountID  string
	EventType  string
	Payload    json.RawMessage
	OccurredAt time.Time
	ReceivedAt time.Time
}

// DaemonEventsRepository handles persistence for daemon_events rows.
type DaemonEventsRepository struct {
	db DB
}

// NewDaemonEventsRepository returns a repository backed by db.
func NewDaemonEventsRepository(db DB) *DaemonEventsRepository {
	return &DaemonEventsRepository{db: db}
}

// Insert persists a daemon event scoped to a user and account.
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

	_, err := r.db.ExecContext(ctx, q, userID, accountID, eventType, []byte(payload), occurredAt)

	return err
}

// ListByUserID returns the most recent events for a user, newest first.
// At most limit rows are returned.
func (r *DaemonEventsRepository) ListByUserID(ctx context.Context, userID int64, limit int) ([]DaemonEventRow, error) {
	const q = `
		SELECT id, user_id, account_id, event_type, payload, occurred_at, received_at
		FROM   daemon_events
		WHERE  user_id = $1
		ORDER  BY occurred_at DESC
		LIMIT  $2`

	rows, err := r.db.QueryContext(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []DaemonEventRow

	for rows.Next() {
		var e DaemonEventRow

		if err := rows.Scan(
			&e.ID, &e.UserID, &e.AccountID, &e.EventType, &e.Payload, &e.OccurredAt, &e.ReceivedAt,
		); err != nil {
			return nil, err
		}

		events = append(events, e)
	}

	return events, rows.Err()
}
