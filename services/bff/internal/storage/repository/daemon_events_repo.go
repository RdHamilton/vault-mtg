package repository

import (
	"context"
	"encoding/json"
	"time"
)

// DaemonEventRow is a single row from the daemon_events table.
type DaemonEventRow struct {
	ID          int64
	UserID      int64
	AccountID   string
	EventType   string
	Payload     json.RawMessage
	OccurredAt  time.Time
	ReceivedAt  time.Time
	EventID     *string
	ProjectedAt *time.Time
	Sequence    uint64
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
// eventID is the daemon-issued idempotency key (may be empty string for legacy rows).
// When eventID is non-empty the unique index (user_id, event_id) prevents duplicate inserts.
// sequence is the monotonically-increasing counter from the daemon (ADR-013).
func (r *DaemonEventsRepository) Insert(
	ctx context.Context,
	userID int64,
	accountID string,
	eventType string,
	payload json.RawMessage,
	occurredAt time.Time,
	eventID string,
	sequence uint64,
) error {
	// Normalise empty eventID to NULL so the partial unique index
	// (WHERE event_id IS NOT NULL) does not deduplicate rows without a key.
	var nullableEventID *string
	if eventID != "" {
		nullableEventID = &eventID
	}

	const q = `
		INSERT INTO daemon_events (user_id, account_id, event_type, payload, occurred_at, event_id, sequence)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT DO NOTHING`

	_, err := r.db.ExecContext(ctx, q, userID, accountID, eventType, payload, occurredAt, nullableEventID, sequence)

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
		SELECT id, user_id, account_id, event_type, payload, occurred_at, received_at,
		       event_id, projected_at
		FROM daemon_events
		WHERE user_id = $1
		ORDER BY occurred_at DESC
		LIMIT $2`

	rows, err := r.db.QueryContext(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var events []DaemonEventRow

	for rows.Next() {
		var e DaemonEventRow

		if err := rows.Scan(
			&e.ID, &e.UserID, &e.AccountID, &e.EventType,
			&e.Payload, &e.OccurredAt, &e.ReceivedAt,
			&e.EventID, &e.ProjectedAt,
		); err != nil {
			return nil, err
		}

		events = append(events, e)
	}

	return events, rows.Err()
}

// ListPendingProjection returns up to limit daemon_events rows that have not
// yet been projected (projected_at IS NULL), ordered by received_at ASC so
// events are projected in ingest order.
func (r *DaemonEventsRepository) ListPendingProjection(
	ctx context.Context,
	limit int,
) ([]DaemonEventRow, error) {
	const q = `
		SELECT id, user_id, account_id, event_type, payload, occurred_at, received_at,
		       event_id, projected_at
		FROM daemon_events
		WHERE projected_at IS NULL
		ORDER BY received_at ASC
		LIMIT $1`

	rows, err := r.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var events []DaemonEventRow

	for rows.Next() {
		var e DaemonEventRow

		if err := rows.Scan(
			&e.ID, &e.UserID, &e.AccountID, &e.EventType,
			&e.Payload, &e.OccurredAt, &e.ReceivedAt,
			&e.EventID, &e.ProjectedAt,
		); err != nil {
			return nil, err
		}

		events = append(events, e)
	}

	return events, rows.Err()
}

// MarkProjected sets projected_at = NOW() for the given daemon_events row.
func (r *DaemonEventsRepository) MarkProjected(ctx context.Context, id int64) error {
	const q = `UPDATE daemon_events SET projected_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, q, id)
	return err
}

// HasRecentEventByUserID returns true when the given user has at least one
// daemon_events row with received_at within the last window duration.
// This is used by the health endpoint to determine whether the daemon is
// actively connected (i.e. heartbeating).
func (r *DaemonEventsRepository) HasRecentEventByUserID(ctx context.Context, userID int64, window time.Duration) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1
			FROM daemon_events
			WHERE user_id = $1
			  AND received_at >= NOW() - ($2 * INTERVAL '1 second')
		)`

	seconds := int64(window.Seconds())
	row := r.db.QueryRowContext(ctx, q, userID, seconds)

	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}
