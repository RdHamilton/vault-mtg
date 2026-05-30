package repository

import (
	"context"
	"time"
)

// ProjectionErrorRow is a single row from the projection_errors table.
type ProjectionErrorRow struct {
	ID            int64
	DaemonEventID int64
	AccountID     string
	EventType     string
	RawPayload    string
	ErrorMessage  string
	FailedAt      time.Time
}

// ProjectionErrorInsert holds the fields needed to write a DLQ row.
type ProjectionErrorInsert struct {
	DaemonEventID int64
	AccountID     string
	EventType     string
	RawPayload    string
	ErrorMessage  string
}

// ProjectionErrorsRepository persists permanently-failed projection attempts to
// the projection_errors dead-letter table.
type ProjectionErrorsRepository struct {
	db DB
}

// NewProjectionErrorsRepository returns a ProjectionErrorsRepository backed by db.
func NewProjectionErrorsRepository(db DB) *ProjectionErrorsRepository {
	return &ProjectionErrorsRepository{db: db}
}

// Insert writes a dead-letter row for a projection failure.
// failed_at defaults to NOW() via the column default.
func (r *ProjectionErrorsRepository) Insert(ctx context.Context, ins ProjectionErrorInsert) error {
	const q = `
		INSERT INTO projection_errors (daemon_event_id, account_id, event_type, raw_payload, error_message)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := r.db.ExecContext(
		ctx, q,
		ins.DaemonEventID,
		ins.AccountID,
		ins.EventType,
		ins.RawPayload,
		ins.ErrorMessage,
	)

	return err
}

// CountProjectionErrors returns the total number of rows in the
// projection_errors table. This is a global count (no account scoping) because
// the endpoint is admin-only and backed by AdminTokenAuth.
func (r *ProjectionErrorsRepository) CountProjectionErrors(ctx context.Context) (int64, error) {
	const q = `SELECT COUNT(*) FROM projection_errors`

	var n int64
	if err := r.db.QueryRowContext(ctx, q).Scan(&n); err != nil {
		return 0, err
	}

	return n, nil
}
