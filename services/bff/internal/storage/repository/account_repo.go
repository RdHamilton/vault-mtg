package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/observability"
)

// ErrCrosstenantAccount is returned by GetOrCreateByClientID when the supplied
// client_id resolves to an account that belongs to a different user_id.  The
// caller must treat this as an authorization failure and skip the write.
var ErrCrosstenantAccount = errors.New("client_id belongs to a different user")

// AccountRepository resolves accounts for a given DB user_id.
type AccountRepository struct {
	db DB
}

// NewAccountRepository returns an AccountRepository backed by db.
func NewAccountRepository(db DB) *AccountRepository {
	return &AccountRepository{db: db}
}

// GetAccountIDByUserID returns the first accounts.id for the given users.id.
// For v0.2.0, one account per user is assumed (multi-account fan-out is v0.3.0).
// Returns (0, false, nil) when the user has no account row yet.
func (r *AccountRepository) GetAccountIDByUserID(ctx context.Context, userID int64) (int64, bool, error) {
	const q = `SELECT id FROM accounts WHERE user_id = $1 LIMIT 1`

	var accountID int64

	row := r.db.QueryRowContext(ctx, q, userID)
	if err := row.Scan(&accountID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		observability.ReportError(ctx, err, map[string]string{"component": "db", "table": "accounts"})
		return 0, false, err
	}

	return accountID, true, nil
}

// GetOrCreateByClientID returns the accounts.id for the given MTGA client_id
// (the raw Arena account string), verifying that the resolved account belongs
// to userID.  If the client_id is registered under a different user_id,
// ErrCrosstenantAccount is returned and no INSERT is attempted — this prevents
// a daemon authenticated as user A from writing into user B's tenant.
//
// If no account row exists for the client_id, a new one is created linked to
// userID (the legitimate first-run path).
func (r *AccountRepository) GetOrCreateByClientID(ctx context.Context, clientID string, userID int64) (int64, error) {
	// Fetch both id and owner so we can detect cross-tenant attempts in a
	// single round-trip.
	const selectQ = `SELECT id, user_id FROM accounts WHERE client_id = $1 LIMIT 1`

	var accountID, ownerUserID int64

	row := r.db.QueryRowContext(ctx, selectQ, clientID)

	switch err := row.Scan(&accountID, &ownerUserID); {
	case err == nil:
		// Account exists — verify it belongs to the authenticated user.
		if ownerUserID != userID {
			return 0, fmt.Errorf("%w: client_id=%s authenticated_user=%d owner_user=%d",
				ErrCrosstenantAccount, clientID, userID, ownerUserID)
		}

		return accountID, nil

	case errors.Is(err, sql.ErrNoRows):
		// Account does not exist yet — fall through to INSERT.

	default:
		observability.ReportError(ctx, err, map[string]string{"component": "db", "table": "accounts"})
		return 0, err
	}

	// Insert a minimal account row linked to the authenticated user.
	const insertQ = `
		INSERT INTO accounts (name, client_id, user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
		RETURNING id`

	insertRow := r.db.QueryRowContext(ctx, insertQ, clientID, clientID, userID)
	if err := insertRow.Scan(&accountID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Another goroutine raced us to the INSERT and won the conflict.
			// Re-read with user_id check.
			retryRow := r.db.QueryRowContext(ctx, selectQ, clientID)
			if err2 := retryRow.Scan(&accountID, &ownerUserID); err2 != nil {
				observability.ReportError(ctx, err2, map[string]string{"component": "db", "table": "accounts"})
				return 0, err2
			}

			if ownerUserID != userID {
				return 0, fmt.Errorf("%w: client_id=%s authenticated_user=%d owner_user=%d",
					ErrCrosstenantAccount, clientID, userID, ownerUserID)
			}

			return accountID, nil
		}

		observability.ReportError(ctx, err, map[string]string{"component": "db", "table": "accounts"})
		return 0, err
	}

	return accountID, nil
}
