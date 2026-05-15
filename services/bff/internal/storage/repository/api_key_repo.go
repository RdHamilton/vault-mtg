// Package repository provides data-access helpers for the BFF service.
package repository

import (
	"context"
	"database/sql"
	"time"
)

// APIKey is the in-memory representation of a row in the api_keys table.
// KeyHash contains the bcrypt hash — never the plaintext key.
type APIKey struct {
	ID         int64
	UserID     int64
	KeyHash    string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	Revoked    bool
}

// DB is the minimal interface required by APIKeyRepository.
// Both *sql.DB and *sql.Tx satisfy it.
type DB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// APIKeyRepository handles persistence for api_keys rows.
type APIKeyRepository struct {
	db DB
}

// NewAPIKeyRepository returns a repository backed by db.
func NewAPIKeyRepository(db DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

// Create inserts a new api_key row and returns the created record.
// keyHash must be a bcrypt hash of the plaintext key — never the plaintext itself.
func (r *APIKeyRepository) Create(ctx context.Context, userID int64, keyHash string) (*APIKey, error) {
	const q = `
		INSERT INTO api_keys (user_id, key_hash)
		VALUES ($1, $2)
		RETURNING id, user_id, key_hash, created_at, last_used_at, revoked`

	row := r.db.QueryRowContext(ctx, q, userID, keyHash)

	var k APIKey
	if err := row.Scan(&k.ID, &k.UserID, &k.KeyHash, &k.CreatedAt, &k.LastUsedAt, &k.Revoked); err != nil {
		return nil, err
	}

	return &k, nil
}

// ListActive returns all non-revoked api_keys rows for a user.
func (r *APIKeyRepository) ListActive(ctx context.Context, userID int64) ([]APIKey, error) {
	const q = `
		SELECT id, user_id, key_hash, created_at, last_used_at, revoked
		FROM   api_keys
		WHERE  user_id = $1 AND revoked = FALSE`

	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var keys []APIKey

	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.KeyHash, &k.CreatedAt, &k.LastUsedAt, &k.Revoked); err != nil {
			return nil, err
		}

		keys = append(keys, k)
	}

	return keys, rows.Err()
}

// ListAllActive returns all non-revoked api_keys rows across all users.
// Used by the auth middleware for key lookup.
//
// NOTE(v1): This performs a full-table scan + bcrypt compare per request.
// Acceptable for low daemon volume; revisit with a prefix-index approach if
// key count grows large (ticket #1000 tracks this optimization).
func (r *APIKeyRepository) ListAllActive(ctx context.Context) ([]APIKey, error) {
	const q = `
		SELECT id, user_id, key_hash, created_at, last_used_at, revoked
		FROM   api_keys
		WHERE  revoked = FALSE`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var keys []APIKey

	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.KeyHash, &k.CreatedAt, &k.LastUsedAt, &k.Revoked); err != nil {
			return nil, err
		}

		keys = append(keys, k)
	}

	return keys, rows.Err()
}

// UpdateLastUsedAt sets last_used_at to now for the given key id.
func (r *APIKeyRepository) UpdateLastUsedAt(ctx context.Context, id int64) error {
	const q = `UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, q, id)

	return err
}
