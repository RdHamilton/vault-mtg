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

// UpsertKey inserts a new daemon_api_keys row for the given (accountID, deviceID).
// Under the multi-device schema (migration 000085) every register call mints a
// new row — the single-device "return existing key" branch is gone. The handler
// upsert semantics (collision handling on duplicate device_id, etc.) are
// redesigned in #2631; this repo method only owns the INSERT.
//
// Returns (record, true, nil) on a successful insert. The bool return is
// preserved to keep the handler signature stable until #2631 lands.
// A duplicate (account_id, device_id) surfaces as the underlying pgx
// unique-violation error — caller's responsibility to map to 409.
func (r *DaemonAPIKeyRepository) UpsertKey(ctx context.Context, accountID, keyHash, keyPrefix, deviceID, platform, daemonVer string) (*DaemonAPIKey, bool, error) {
	const q = `
		INSERT INTO daemon_api_keys (account_id, key_hash, key_prefix, device_id, platform, daemon_ver)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, account_id, key_hash, key_prefix, device_id, platform, daemon_ver, created_at, last_used_at, revoked_at`

	row := r.db.QueryRowContext(ctx, q, accountID, keyHash, keyPrefix, deviceID, platform, daemonVer)
	k, err := scanDaemonAPIKey(row)
	if err != nil {
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
