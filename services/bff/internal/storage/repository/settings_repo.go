// Phase 2 PR #12 — user_settings repository.
//
// Account-scoped key/value store. Values are stored as JSONB so callers
// can persist any JSON-encodable type under any key without schema churn.
// The SPA's AppSettings constructor applies defaults for missing keys, so
// the repo never has to know about the canonical setting list.

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
)

// SettingsRepository owns the user_settings table.
type SettingsRepository struct {
	db *sql.DB
}

// NewSettingsRepository constructs a SettingsRepository.
func NewSettingsRepository(db *sql.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

// ListByAccount returns every key/value pair for the account as a map.
// Missing rows surface as an empty map — callers (e.g. the SPA) supply
// the defaults.
func (r *SettingsRepository) ListByAccount(ctx context.Context, accountID int64) (map[string]json.RawMessage, error) {
	const q = `SELECT key, value FROM user_settings WHERE account_id = $1`
	rows, err := r.db.QueryContext(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]json.RawMessage{}
	for rows.Next() {
		var key string
		var value json.RawMessage
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		out[key] = value
	}
	return out, rows.Err()
}

// Get returns the value for a single key. The bool is false when the key
// is unset for that account.
func (r *SettingsRepository) Get(ctx context.Context, accountID int64, key string) (json.RawMessage, bool, error) {
	const q = `SELECT value FROM user_settings WHERE account_id = $1 AND key = $2`
	var value json.RawMessage
	err := r.db.QueryRowContext(ctx, q, accountID, key).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

// Upsert writes (or replaces) a single key/value pair for the account.
func (r *SettingsRepository) Upsert(ctx context.Context, accountID int64, key string, value json.RawMessage) error {
	const q = `INSERT INTO user_settings (account_id, key, value, updated_at)
	           VALUES ($1, $2, $3, NOW())
	           ON CONFLICT (account_id, key)
	           DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`
	_, err := r.db.ExecContext(ctx, q, accountID, key, []byte(value))
	return err
}

// UpsertMany writes every entry in `values` for the account in a single
// transaction. Empty maps short-circuit to a no-op.
func (r *SettingsRepository) UpsertMany(ctx context.Context, accountID int64, values map[string]json.RawMessage) error {
	if len(values) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	const q = `INSERT INTO user_settings (account_id, key, value, updated_at)
	           VALUES ($1, $2, $3, NOW())
	           ON CONFLICT (account_id, key)
	           DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`
	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()
	for key, value := range values {
		if _, err := stmt.ExecContext(ctx, accountID, key, []byte(value)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
