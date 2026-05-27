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
//
// PairedAt is populated by methods added for ADR-031 #21 (ListByAccountID,
// GetByAccountAndDevice). Pre-#21 methods (UpsertKey, GetActive, ListAllActive)
// do not project paired_at — their scans leave it zero-valued. The handler
// surface that needs paired_at (GET /v1/daemons response) only consumes rows
// produced by ListByAccountID, so this is contained.
type DaemonAPIKey struct {
	ID         string
	AccountID  string
	KeyHash    string
	KeyPrefix  string
	DeviceID   string
	Platform   string
	DaemonVer  string
	PairedAt   time.Time
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

// ListByAccountID returns all non-revoked daemon_api_keys rows for the given
// account_id, ordered by paired_at DESC (newest first). Used by the
// `GET /v1/daemons` handler per ADR-031 §4.
//
// Sensitive columns (key_hash, key_prefix, id) are NOT projected — the
// handler must not be able to leak them even by accident. The returned
// DaemonAPIKey values populate only the fields included in the SELECT below;
// other fields are zero-valued.
func (r *DaemonAPIKeyRepository) ListByAccountID(ctx context.Context, accountID string) ([]DaemonAPIKey, error) {
	const q = `
		SELECT device_id, platform, daemon_ver, paired_at, last_used_at
		FROM   daemon_api_keys
		WHERE  account_id = $1 AND revoked_at IS NULL
		ORDER BY paired_at DESC`

	rows, err := r.db.QueryContext(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	keys := make([]DaemonAPIKey, 0)
	for rows.Next() {
		var k DaemonAPIKey
		if err := rows.Scan(&k.DeviceID, &k.Platform, &k.DaemonVer, &k.PairedAt, &k.LastUsedAt); err != nil {
			return nil, err
		}
		k.AccountID = accountID
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// RevokeByAccountIDAndDeviceID soft-deletes the daemon_api_keys row matching
// (accountID, deviceID) by setting revoked_at = now(). Returns true if exactly
// one row was updated, false if zero rows matched (either device_id doesn't
// exist, OR it belongs to another account_id, OR it was already revoked —
// all three collapse to false here so the handler can return 404 without
// leaking cross-tenant existence). Per ADR-031 §3.
func (r *DaemonAPIKeyRepository) RevokeByAccountIDAndDeviceID(ctx context.Context, accountID, deviceID string) (bool, error) {
	const q = `
		UPDATE daemon_api_keys
		SET    revoked_at = now(), updated_at = now()
		WHERE  account_id = $1 AND device_id = $2 AND revoked_at IS NULL
		RETURNING id`

	var id string
	err := r.db.QueryRowContext(ctx, q, accountID, deviceID).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// FleetHealthSnapshot holds aggregate counts returned by FleetHealthSnapshot.
// All fields are safe to expose to ops tooling — no PII, no per-user data.
type FleetHealthSnapshot struct {
	TotalPaired  int       // non-revoked keys (all time)
	ActiveLast5m int       // keys with last_used_at within 5 minutes
	ActiveLast1h int       // keys with last_used_at within 1 hour
	Revoked      int       // keys with revoked_at IS NOT NULL (all time)
	AsOf         time.Time // server-side timestamp at query execution
}

// FleetHealthSnapshot returns a point-in-time aggregate of daemon_api_keys
// state for operations dashboards. The query returns a single row of counts;
// no per-row or per-account data is projected — zero PII.
//
// "Active" is defined as last_used_at within the window (5m or 1h).
// Revoked rows are those with revoked_at IS NOT NULL (all time).
// TotalPaired counts all non-revoked rows regardless of last_used_at.
func (r *DaemonAPIKeyRepository) FleetHealthSnapshot(ctx context.Context) (FleetHealthSnapshot, error) {
	const q = `
		SELECT
			COUNT(*)                                                              AS total_paired,
			COUNT(*) FILTER (WHERE last_used_at > now() - INTERVAL '5 minutes')  AS active_last_5m,
			COUNT(*) FILTER (WHERE last_used_at > now() - INTERVAL '1 hour')     AS active_last_1h,
			(SELECT COUNT(*) FROM daemon_api_keys WHERE revoked_at IS NOT NULL)   AS revoked,
			now()                                                                  AS as_of
		FROM daemon_api_keys
		WHERE revoked_at IS NULL`

	var snap FleetHealthSnapshot
	row := r.db.QueryRowContext(ctx, q)
	if err := row.Scan(
		&snap.TotalPaired,
		&snap.ActiveLast5m,
		&snap.ActiveLast1h,
		&snap.Revoked,
		&snap.AsOf,
	); err != nil {
		return FleetHealthSnapshot{}, err
	}

	return snap, nil
}

// GetByAccountAndDevice returns the daemon_api_keys row matching
// (accountID, deviceID) regardless of revoked_at state. Returns
// ErrDaemonAPIKeyNotFound when no row exists. Used by the daemon_register
// handler's revoked-row-resurrection guard (ADR-031 §5 + ADR-028): if a
// daemon replays a stale device_id pointing at a revoked row, the handler
// detects it here and treats the request as a first pair (mints a fresh
// server-issued device_id) rather than resurrecting the revoked credential.
func (r *DaemonAPIKeyRepository) GetByAccountAndDevice(ctx context.Context, accountID, deviceID string) (*DaemonAPIKey, error) {
	const q = `
		SELECT id, account_id, key_hash, key_prefix, device_id, platform, daemon_ver, created_at, last_used_at, revoked_at
		FROM   daemon_api_keys
		WHERE  account_id = $1 AND device_id = $2
		LIMIT  1`

	row := r.db.QueryRowContext(ctx, q, accountID, deviceID)
	k, err := scanDaemonAPIKey(row)
	if err == sql.ErrNoRows {
		return nil, ErrDaemonAPIKeyNotFound
	}
	return k, err
}
