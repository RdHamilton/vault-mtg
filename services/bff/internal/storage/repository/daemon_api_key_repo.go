package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrDaemonAPIKeyNotFound is returned when no active daemon API key exists for an account.
var ErrDaemonAPIKeyNotFound = errors.New("daemon api key not found")

// DaemonAPIKey is the in-memory representation of a daemon_api_keys row.
// KeyHash contains the bcrypt hash — never the plaintext key.
type DaemonAPIKey struct {
	ID         string
	AccountID  string
	KeyHash    string
	KeyPrefix  string
	DeviceID   string
	Platform   string
	DaemonVer  string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

// daemonAPIKeyDB is the minimal interface required by DaemonAPIKeyRepository.
type daemonAPIKeyDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// DaemonAPIKeyRepository handles persistence for daemon_api_keys rows.
type DaemonAPIKeyRepository struct {
	db daemonAPIKeyDB
}

// NewDaemonAPIKeyRepository returns a repository backed by db.
func NewDaemonAPIKeyRepository(db daemonAPIKeyDB) *DaemonAPIKeyRepository {
	return &DaemonAPIKeyRepository{db: db}
}

// UpsertKey inserts a new key for accountID, or returns the existing non-revoked key.
// deviceID, platform, and daemonVer identify the daemon installation.
// Returns (record, true, nil) when a new key was created.
// Returns (record, false, nil) when the existing key was returned.
func (r *DaemonAPIKeyRepository) UpsertKey(ctx context.Context, accountID, keyHash, keyPrefix, deviceID, platform, daemonVer string) (*DaemonAPIKey, bool, error) {
	// Try to fetch existing non-revoked key first.
	existing, err := r.GetActive(ctx, accountID)
	if err != nil && err != ErrDaemonAPIKeyNotFound {
		return nil, false, err
	}
	if existing != nil {
		return existing, false, nil
	}

	// No active key — insert a new one.
	const q = `
		INSERT INTO daemon_api_keys (account_id, key_hash, key_prefix, device_id, platform, daemon_ver)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (account_id) DO NOTHING
		RETURNING id, account_id, key_hash, key_prefix, device_id, platform, daemon_ver, created_at, last_used_at, revoked_at`

	row := r.db.QueryRowContext(ctx, q, accountID, keyHash, keyPrefix, deviceID, platform, daemonVer)
	k, err := scanDaemonAPIKey(row)
	if err != nil {
		// ON CONFLICT DO NOTHING means no row returned if a concurrent insert won.
		// Retry the read.
		if err == sql.ErrNoRows {
			existing, err2 := r.GetActive(ctx, accountID)
			if err2 != nil {
				return nil, false, err2
			}
			return existing, false, nil
		}
		return nil, false, err
	}

	return k, true, nil
}

// GetActive returns the active (non-revoked) key for accountID.
// Returns ErrDaemonAPIKeyNotFound when no active key exists.
func (r *DaemonAPIKeyRepository) GetActive(ctx context.Context, accountID string) (*DaemonAPIKey, error) {
	const q = `
		SELECT id, account_id, key_hash, key_prefix, device_id, platform, daemon_ver, created_at, last_used_at, revoked_at
		FROM   daemon_api_keys
		WHERE  account_id = $1 AND revoked_at IS NULL
		LIMIT  1`

	row := r.db.QueryRowContext(ctx, q, accountID)
	k, err := scanDaemonAPIKey(row)
	if err == sql.ErrNoRows {
		return nil, ErrDaemonAPIKeyNotFound
	}
	return k, err
}

// UpdateLastUsed sets last_used_at to now for the given key id.
// updated_at is bumped in lockstep to keep the audit trail current.
func (r *DaemonAPIKeyRepository) UpdateLastUsed(ctx context.Context, id string) error {
	const q = `UPDATE daemon_api_keys SET last_used_at = now(), updated_at = now() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, q, id)
	return err
}

// ListAllActive returns all non-revoked daemon_api_keys rows. Used by the
// daemon API key auth middleware to bcrypt-compare an incoming Bearer token
// against every known hash.
//
// NOTE: full-table scan + bcrypt per request. Acceptable for v0.3.1 beta
// scale; revisit with a prefix-index lookup if the key count grows large.
func (r *DaemonAPIKeyRepository) ListAllActive(ctx context.Context) ([]DaemonAPIKey, error) {
	const q = `
		SELECT id, account_id, key_hash, key_prefix, device_id, platform, daemon_ver, created_at, last_used_at, revoked_at
		FROM   daemon_api_keys
		WHERE  revoked_at IS NULL`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var keys []DaemonAPIKey
	for rows.Next() {
		var k DaemonAPIKey
		if err := rows.Scan(&k.ID, &k.AccountID, &k.KeyHash, &k.KeyPrefix, &k.DeviceID, &k.Platform, &k.DaemonVer, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// scanDaemonAPIKey scans a single row into a DaemonAPIKey.
func scanDaemonAPIKey(row *sql.Row) (*DaemonAPIKey, error) {
	var k DaemonAPIKey
	err := row.Scan(&k.ID, &k.AccountID, &k.KeyHash, &k.KeyPrefix, &k.DeviceID, &k.Platform, &k.DaemonVer, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt)
	if err != nil {
		return nil, err
	}
	return &k, nil
}
