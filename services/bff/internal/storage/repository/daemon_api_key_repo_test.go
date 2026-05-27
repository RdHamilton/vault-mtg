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

// ─── #21 integration tests (ListByAccountID / RevokeByAccountIDAndDeviceID / GetByAccountAndDevice) ─

// TestListByAccountID_OnlyActive verifies the ADR-031 §4 contract: the list
// endpoint MUST exclude revoked rows, MUST scope to the caller's account_id,
// and MUST order by paired_at DESC. Two active rows + one revoked row + one
// other-account row are inserted; only the two active A rows are returned.
func TestListByAccountID_OnlyActive(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonAPIKeyRepository(db)

	accountA := "user_list_active_A_" + t.Name()
	accountB := "user_list_active_B_" + t.Name()
	deviceX := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaa01"
	deviceY := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaa02"
	deviceZRevoked := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaa03"
	deviceB1 := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbb01"

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM daemon_api_keys WHERE account_id IN ($1, $2)`, accountA, accountB)
	})

	// Insert two active rows for A with controlled paired_at ordering.
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO daemon_api_keys (account_id, key_hash, key_prefix, device_id, platform, daemon_ver, paired_at)
		 VALUES ($1, 'h1', 'p1', $2, 'darwin', '0.3.3', now() - interval '1 hour')`,
		accountA, deviceX)
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO daemon_api_keys (account_id, key_hash, key_prefix, device_id, platform, daemon_ver, paired_at)
		 VALUES ($1, 'h2', 'p2', $2, 'windows', '0.3.3', now())`,
		accountA, deviceY)
	require.NoError(t, err)
	// Insert one revoked row for A.
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO daemon_api_keys (account_id, key_hash, key_prefix, device_id, platform, daemon_ver, paired_at, revoked_at)
		 VALUES ($1, 'h3', 'p3', $2, 'darwin', '0.3.3', now() - interval '2 hour', now())`,
		accountA, deviceZRevoked)
	require.NoError(t, err)
	// Insert one row for B (must not appear in A's list).
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO daemon_api_keys (account_id, key_hash, key_prefix, device_id, platform, daemon_ver)
		 VALUES ($1, 'h4', 'p4', $2, 'darwin', '0.3.3')`,
		accountB, deviceB1)
	require.NoError(t, err)

	keys, err := repo.ListByAccountID(context.Background(), accountA)
	require.NoError(t, err)
	require.Len(t, keys, 2, "expected exactly two active rows for accountA (revoked excluded, accountB excluded)")

	// ORDER BY paired_at DESC: deviceY (now) first, then deviceX (1 hour ago).
	assert.Equal(t, deviceY, keys[0].DeviceID, "newest paired_at first")
	assert.Equal(t, deviceX, keys[1].DeviceID, "oldest paired_at last")
	assert.Equal(t, accountA, keys[0].AccountID)
	for _, k := range keys {
		assert.Nil(t, k.RevokedAt, "no revoked rows expected in the result")
		assert.NotEqual(t, deviceZRevoked, k.DeviceID, "revoked row must not appear")
		assert.NotEqual(t, deviceB1, k.DeviceID, "cross-tenant row must not appear")
		assert.False(t, k.PairedAt.IsZero(), "paired_at must be populated")
	}
}

// TestListByAccountID_EmptyState verifies the empty-list path returns
// (nil/empty slice, nil error) for a user with no devices. AC1.
func TestListByAccountID_EmptyState(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonAPIKeyRepository(db)

	accountID := "user_list_empty_" + t.Name()

	keys, err := repo.ListByAccountID(context.Background(), accountID)
	require.NoError(t, err, "empty list must not return an error")
	assert.Empty(t, keys, "expected empty slice for user with no devices")
}

// TestRevokeByAccountIDAndDeviceID_HappyPath verifies the soft-delete primitive:
// one active row → revoke → returns true and the row's revoked_at becomes NOT NULL.
// Per ADR-031 §3.
func TestRevokeByAccountIDAndDeviceID_HappyPath(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonAPIKeyRepository(db)

	accountID := "user_revoke_happy_" + t.Name()
	deviceID := "cccccccc-cccc-cccc-cccc-cccccccccc01"

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM daemon_api_keys WHERE account_id = $1`, accountID)
	})

	_, _, err := repo.UpsertKey(context.Background(),
		accountID, "hash_revoke", "sk_pref_rev1", deviceID, "darwin", "0.3.3")
	require.NoError(t, err)

	revoked, err := repo.RevokeByAccountIDAndDeviceID(context.Background(), accountID, deviceID)
	require.NoError(t, err)
	assert.True(t, revoked, "exactly one row must be affected")

	var revokedAt *time.Time
	err = db.QueryRowContext(context.Background(),
		`SELECT revoked_at FROM daemon_api_keys WHERE account_id = $1 AND device_id = $2`,
		accountID, deviceID).Scan(&revokedAt)
	require.NoError(t, err)
	require.NotNil(t, revokedAt, "revoked_at MUST be set after a successful revoke")
}

// TestRevokeByAccountIDAndDeviceID_CrossTenant is the load-bearing assertion
// for ADR-031 §3 + AC4: User B's revoke against User A's device MUST return
// false (no row affected) AND User A's row MUST remain non-revoked.
func TestRevokeByAccountIDAndDeviceID_CrossTenant(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonAPIKeyRepository(db)

	accountA := "user_revoke_xt_A_" + t.Name()
	accountB := "user_revoke_xt_B_" + t.Name()
	deviceID := "dddddddd-dddd-dddd-dddd-dddddddddd01"

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM daemon_api_keys WHERE account_id IN ($1, $2)`, accountA, accountB)
	})

	_, _, err := repo.UpsertKey(context.Background(),
		accountA, "hash_A", "sk_pref_A01", deviceID, "darwin", "0.3.3")
	require.NoError(t, err)

	revoked, err := repo.RevokeByAccountIDAndDeviceID(context.Background(), accountB, deviceID)
	require.NoError(t, err, "cross-tenant revoke must be a no-op, not an error")
	assert.False(t, revoked, "User B's revoke against User A's device MUST NOT affect any row")

	var revokedAt *time.Time
	err = db.QueryRowContext(context.Background(),
		`SELECT revoked_at FROM daemon_api_keys WHERE account_id = $1 AND device_id = $2`,
		accountA, deviceID).Scan(&revokedAt)
	require.NoError(t, err)
	assert.Nil(t, revokedAt, "User A's row revoked_at MUST remain NULL after cross-tenant attempt")
}

// TestRevokeByAccountIDAndDeviceID_AlreadyRevoked verifies the second revoke
// returns false (no row matches because of the WHERE revoked_at IS NULL filter).
// Per ADR-031 §3: 404 collapse.
func TestRevokeByAccountIDAndDeviceID_AlreadyRevoked(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonAPIKeyRepository(db)

	accountID := "user_revoke_dup_" + t.Name()
	deviceID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeee01"

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM daemon_api_keys WHERE account_id = $1`, accountID)
	})

	_, _, err := repo.UpsertKey(context.Background(),
		accountID, "hash_dup", "sk_pref_dup1", deviceID, "darwin", "0.3.3")
	require.NoError(t, err)

	revoked, err := repo.RevokeByAccountIDAndDeviceID(context.Background(), accountID, deviceID)
	require.NoError(t, err)
	require.True(t, revoked)

	revoked, err = repo.RevokeByAccountIDAndDeviceID(context.Background(), accountID, deviceID)
	require.NoError(t, err)
	assert.False(t, revoked, "second revoke must be a no-op (already revoked)")
}

// TestRevokeByAccountIDAndDeviceID_NonExistent verifies revoke against a
// device_id that simply doesn't exist returns false (no row matched).
func TestRevokeByAccountIDAndDeviceID_NonExistent(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonAPIKeyRepository(db)

	accountID := "user_revoke_nx_" + t.Name()
	deviceID := "ffffffff-ffff-ffff-ffff-ffffffffff01"

	revoked, err := repo.RevokeByAccountIDAndDeviceID(context.Background(), accountID, deviceID)
	require.NoError(t, err)
	assert.False(t, revoked, "revoke against non-existent device_id must be a no-op")
}

// TestListAllActive_ExcludesRevoked is the ADR-031 Fitness Function §1
// regression test: a future refactor that drops the WHERE revoked_at IS NULL
// filter from ListAllActive (used by DaemonAPIKeyAuth middleware) would
// silently break revocation. This test catches that.
func TestListAllActive_ExcludesRevoked(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonAPIKeyRepository(db)

	accountID := "user_list_all_active_" + t.Name()
	deviceActive := "11111111-2222-3333-4444-555555555501"
	deviceRevoked := "11111111-2222-3333-4444-555555555502"

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM daemon_api_keys WHERE account_id = $1`, accountID)
	})

	_, err := db.ExecContext(context.Background(),
		`INSERT INTO daemon_api_keys (account_id, key_hash, key_prefix, device_id, platform, daemon_ver)
		 VALUES ($1, 'h_act', 'p_act', $2, 'darwin', '0.3.3')`,
		accountID, deviceActive)
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO daemon_api_keys (account_id, key_hash, key_prefix, device_id, platform, daemon_ver, revoked_at)
		 VALUES ($1, 'h_rev', 'p_rev', $2, 'darwin', '0.3.3', now())`,
		accountID, deviceRevoked)
	require.NoError(t, err)

	all, err := repo.ListAllActive(context.Background())
	require.NoError(t, err)

	var sawActive, sawRevoked bool
	for _, k := range all {
		if k.DeviceID == deviceActive {
			sawActive = true
		}
		if k.DeviceID == deviceRevoked {
			sawRevoked = true
		}
	}
	assert.True(t, sawActive, "ListAllActive must include the active row")
	assert.False(t, sawRevoked, "ListAllActive MUST exclude revoked rows (ADR-031 Fitness Function §1)")
}

// TestGetByAccountAndDevice_IncludesRevoked verifies the lookup used by the
// daemon_register revoked-row-resurrection guard returns a row regardless of
// its revoked_at state — the register handler needs to detect a stale,
// revoked device_id replay to mint a fresh row (ADR-031 §5 + ADR-028).
func TestGetByAccountAndDevice_IncludesRevoked(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonAPIKeyRepository(db)

	accountID := "user_getbyad_" + t.Name()
	deviceActive := "22222222-3333-4444-5555-666666666601"
	deviceRevoked := "22222222-3333-4444-5555-666666666602"

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM daemon_api_keys WHERE account_id = $1`, accountID)
	})

	_, err := db.ExecContext(context.Background(),
		`INSERT INTO daemon_api_keys (account_id, key_hash, key_prefix, device_id, platform, daemon_ver)
		 VALUES ($1, 'h_act', 'p_act', $2, 'darwin', '0.3.3')`,
		accountID, deviceActive)
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO daemon_api_keys (account_id, key_hash, key_prefix, device_id, platform, daemon_ver, revoked_at)
		 VALUES ($1, 'h_rev', 'p_rev', $2, 'darwin', '0.3.3', now())`,
		accountID, deviceRevoked)
	require.NoError(t, err)

	// Active row: must return non-nil with revoked_at == nil.
	got, err := repo.GetByAccountAndDevice(context.Background(), accountID, deviceActive)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, deviceActive, got.DeviceID)
	assert.Nil(t, got.RevokedAt)

	// Revoked row: must return non-nil with revoked_at != nil (so the
	// register handler can detect replay).
	got, err = repo.GetByAccountAndDevice(context.Background(), accountID, deviceRevoked)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, deviceRevoked, got.DeviceID)
	require.NotNil(t, got.RevokedAt, "revoked row MUST surface revoked_at to the handler")

	// Non-existent row: must return ErrDaemonAPIKeyNotFound.
	_, err = repo.GetByAccountAndDevice(context.Background(), accountID, "99999999-9999-9999-9999-999999999999")
	assert.ErrorIs(t, err, repository.ErrDaemonAPIKeyNotFound)
}
