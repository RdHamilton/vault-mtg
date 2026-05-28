package repository

import (
	"context"
	"database/sql"
)

// WaitlistEntry is the in-memory representation of a waitlist_entries row.
type WaitlistEntry struct {
	ID              string
	Email           string
	MailchimpStatus string
	UTMSource       *string
	UTMMedium       *string
	UTMCampaign     *string
	Referrer        *string
}

// waitlistDB is the minimal DB interface required by WaitlistRepository.
type waitlistDB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// WaitlistRepository handles persistence for the waitlist_entries table.
type WaitlistRepository struct {
	db waitlistDB
}

// NewWaitlistRepository returns a repository backed by db.
func NewWaitlistRepository(db waitlistDB) *WaitlistRepository {
	return &WaitlistRepository{db: db}
}

// InsertIfNew inserts a new waitlist_entries row for email and attribution fields
// using ON CONFLICT DO NOTHING RETURNING id. Returns (id, true, nil) when a new
// row was created, or ("", false, nil) when the email already existed.
// The initial mailchimp_status is 'failed' per the table DEFAULT; the happy
// path calls UpdateMailchimpStatus afterwards.
func (r *WaitlistRepository) InsertIfNew(
	ctx context.Context,
	email string,
	utmSource, utmMedium, utmCampaign *string,
	referrer *string,
) (id string, created bool, err error) {
	const q = `
		INSERT INTO waitlist_entries (email, utm_source, utm_medium, utm_campaign, referrer)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (email) DO NOTHING
		RETURNING id`

	row := r.db.QueryRowContext(ctx, q, email, utmSource, utmMedium, utmCampaign, referrer)
	if err := row.Scan(&id); err == sql.ErrNoRows {
		return "", false, nil
	} else if err != nil {
		return "", false, err
	}
	return id, true, nil
}

// UpdateMailchimpStatus sets mailchimp_status and bumps updated_at for the row
// with the given id. status is expected to be "subscribed" or "failed".
func (r *WaitlistRepository) UpdateMailchimpStatus(ctx context.Context, id, status string) error {
	const q = `
		UPDATE waitlist_entries
		SET    mailchimp_status = $2, updated_at = now()
		WHERE  id = $1`

	_, err := r.db.ExecContext(ctx, q, id, status)
	return err
}
