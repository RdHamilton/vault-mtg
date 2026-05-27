package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── stub DB ──────────────────────────────────────────────────────────────────

// stubDaemonDB is a test double for the daemonAPIKeyDB interface.
// It holds a single row that is returned by QueryRowContext calls and
// tracks ExecContext calls.
type stubDaemonDB struct {
	row     *stubRow
	execErr error
}

func (s *stubDaemonDB) ExecContext(_ context.Context, _ string, _ ...any) (sql.Result, error) {
	if s.execErr != nil {
		return nil, s.execErr
	}
	return stubResult{}, nil
}

func (s *stubDaemonDB) QueryContext(_ context.Context, _ string, _ ...any) (*sql.Rows, error) {
	// *sql.Rows cannot be constructed without a real driver; integration tests
	// in *_pg_test.go exercise ListAllActive against real Postgres.
	return nil, nil //nolint:nilnil
}

func (s *stubDaemonDB) QueryRowContext(_ context.Context, _ string, _ ...any) *sql.Row {
	// *sql.Row cannot be constructed directly in tests — use a real DB round trip.
	// Instead we rely on the real Postgres for integration tests; here we only
	// test the repository constructor and ensure it does not panic.
	return nil
}

// stubResult satisfies sql.Result.
type stubResult struct{}

func (stubResult) LastInsertId() (int64, error) { return 0, nil }
func (stubResult) RowsAffected() (int64, error) { return 1, nil }

// stubRow simulates a *sql.Row.  Unused in unit tests but kept for reference.
type stubRow struct {
	values []any
	err    error
}

// ─── unit tests ──────────────────────────────────────────────────────────────

// TestNewDaemonAPIKeyRepository_NotNil verifies the constructor returns a non-nil value.
func TestNewDaemonAPIKeyRepository_NotNil(t *testing.T) {
	db := &stubDaemonDB{}
	repo := repository.NewDaemonAPIKeyRepository(db)
	assert.NotNil(t, repo)
}

// TestDaemonAPIKey_Fields verifies that DaemonAPIKey fields are set correctly.
func TestDaemonAPIKey_Fields(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	k := repository.DaemonAPIKey{
		ID:        "uuid-abc",
		AccountID: "user_2abc",
		KeyHash:   "$2a$10$...",
		KeyPrefix: "sk_live_ab",
		DeviceID:  "550e8400-e29b-41d4-a716-446655440000",
		Platform:  "darwin",
		DaemonVer: "0.3.1",
		CreatedAt: now,
	}

	assert.Equal(t, "uuid-abc", k.ID)
	assert.Equal(t, "user_2abc", k.AccountID)
	assert.Equal(t, "$2a$10$...", k.KeyHash)
	assert.Equal(t, "sk_live_ab", k.KeyPrefix)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", k.DeviceID)
	assert.Equal(t, "darwin", k.Platform)
	assert.Equal(t, "0.3.1", k.DaemonVer)
	assert.Equal(t, now, k.CreatedAt)
	assert.Nil(t, k.LastUsedAt)
	assert.Nil(t, k.RevokedAt)
}

// TestDaemonAPIKey_LastUsedAt verifies the optional LastUsedAt pointer field.
func TestDaemonAPIKey_LastUsedAt(t *testing.T) {
	now := time.Now().UTC()
	k := repository.DaemonAPIKey{
		LastUsedAt: &now,
	}
	require.NotNil(t, k.LastUsedAt)
	assert.Equal(t, now, *k.LastUsedAt)
}

// TestDaemonAPIKey_RevokedAt verifies the optional RevokedAt pointer field.
func TestDaemonAPIKey_RevokedAt(t *testing.T) {
	now := time.Now().UTC()
	k := repository.DaemonAPIKey{
		RevokedAt: &now,
	}
	require.NotNil(t, k.RevokedAt)
	assert.Equal(t, now, *k.RevokedAt)
}

// TestErrDaemonAPIKeyNotFound verifies the sentinel error has the expected message.
func TestErrDaemonAPIKeyNotFound(t *testing.T) {
	assert.EqualError(t, repository.ErrDaemonAPIKeyNotFound, "daemon api key not found")
}
