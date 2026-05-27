package repository_test

import (
	"context"
	"database/sql"
	"strings"
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

// ─── integration tests (require TEST_DATABASE_URL + migration 000085 applied) ─

// TestDaemonAPIKeyRepository_UpsertKey_InsertsRow exercises the post-#2654
// schema: UpsertKey must succeed against the composite-UNIQUE table without an
// ON CONFLICT clause. This guards against the regression Lee caught on the
// first iteration of PR #2654 where ON CONFLICT (account_id) referenced a
// constraint that no longer exists.
func TestDaemonAPIKeyRepository_UpsertKey_InsertsRow(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonAPIKeyRepository(db)

	accountID := "user_upsert_insert_" + t.Name()
	deviceID := "550e8400-e29b-41d4-a716-446655440001"

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM daemon_api_keys WHERE account_id = $1`, accountID)
	})

	rec, created, err := repo.UpsertKey(context.Background(),
		accountID, "$2a$10$test_hash", "sk_test_pref01", deviceID, "darwin", "0.3.3")
	require.NoError(t, err, "UpsertKey must succeed against post-000085 schema (no ON CONFLICT regression)")
	require.NotNil(t, rec)
	assert.True(t, created, "every multi-device register is a fresh row")
	assert.Equal(t, accountID, rec.AccountID)
	assert.Equal(t, deviceID, rec.DeviceID)
	assert.Equal(t, "darwin", rec.Platform)
	assert.Equal(t, "0.3.3", rec.DaemonVer)
	assert.NotEmpty(t, rec.ID)
	assert.False(t, rec.CreatedAt.IsZero())
}

// TestDaemonAPIKeyRepository_UpsertKey_MultiDevicePerAccount verifies the
// multi-device authorization principal from ADR-031 §1: a single account can
// hold multiple active keys, one per device_id.
func TestDaemonAPIKeyRepository_UpsertKey_MultiDevicePerAccount(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonAPIKeyRepository(db)

	accountID := "user_upsert_multidev_" + t.Name()
	deviceA := "11111111-1111-1111-1111-111111111111"
	deviceB := "22222222-2222-2222-2222-222222222222"

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM daemon_api_keys WHERE account_id = $1`, accountID)
	})

	_, _, err := repo.UpsertKey(context.Background(),
		accountID, "hash_a", "sk_pref_a01", deviceA, "darwin", "0.3.3")
	require.NoError(t, err)

	_, _, err = repo.UpsertKey(context.Background(),
		accountID, "hash_b", "sk_pref_b01", deviceB, "windows", "0.3.3")
	require.NoError(t, err, "second device for same account must succeed under composite UNIQUE")

	var count int
	err = db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM daemon_api_keys WHERE account_id = $1`, accountID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "both devices should persist as separate rows")
}

// TestDaemonAPIKeyRepository_UpsertKey_DuplicateDeviceRejected verifies the
// composite UNIQUE(account_id, device_id) constraint surfaces as an error
// when the same (account, device) pair is registered twice. The handler
// rewrite in #2631 owns mapping this to a 409 response — this test only
// confirms the SQL-level signal reaches the caller.
func TestDaemonAPIKeyRepository_UpsertKey_DuplicateDeviceRejected(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonAPIKeyRepository(db)

	accountID := "user_upsert_dupdev_" + t.Name()
	deviceID := "33333333-3333-3333-3333-333333333333"

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM daemon_api_keys WHERE account_id = $1`, accountID)
	})

	_, _, err := repo.UpsertKey(context.Background(),
		accountID, "hash1", "sk_pref_dup1", deviceID, "darwin", "0.3.3")
	require.NoError(t, err)

	_, _, err = repo.UpsertKey(context.Background(),
		accountID, "hash2", "sk_pref_dup2", deviceID, "darwin", "0.3.3")
	require.Error(t, err, "duplicate (account_id, device_id) must violate composite UNIQUE")
	assert.True(t,
		strings.Contains(err.Error(), "daemon_api_keys_account_device_unique") ||
			strings.Contains(err.Error(), "duplicate key"),
		"error must come from the composite UNIQUE constraint, got: %v", err)
}
