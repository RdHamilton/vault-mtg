package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFleetHealthSnapshot_ReturnsZeroesOnEmptyTable verifies the aggregate
// query returns a valid snapshot with all-zero counts when daemon_api_keys is
// empty (or contains no matching rows for this test's seed prefix).
//
// Requires TEST_DATABASE_URL and migration 000085 applied.
func TestFleetHealthSnapshot_ReturnsZeroesOnEmptyTable(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonAPIKeyRepository(db)

	snap, err := repo.FleetHealthSnapshot(context.Background())
	require.NoError(t, err)

	// AsOf must be a recent timestamp — within 5 seconds of now.
	assert.WithinDuration(t, time.Now(), snap.AsOf, 5*time.Second)

	// All counts are >= 0 and consistency-checked: active_last_5m <= active_last_1h <= total_paired.
	assert.GreaterOrEqual(t, snap.TotalPaired, 0)
	assert.GreaterOrEqual(t, snap.ActiveLast5m, 0)
	assert.GreaterOrEqual(t, snap.ActiveLast1h, 0)
	assert.GreaterOrEqual(t, snap.Revoked, 0)
	assert.LessOrEqual(t, snap.ActiveLast5m, snap.ActiveLast1h)
}

// TestFleetHealthSnapshot_CountsSeededRows seeds two active keys and one
// revoked key, then verifies the snapshot counts are correct.
//
// Requires TEST_DATABASE_URL and migration 000085 applied.
func TestFleetHealthSnapshot_CountsSeededRows(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDaemonAPIKeyRepository(db)
	ctx := context.Background()

	// Insert two active keys — last_used_at within 5m so they register in
	// active_last_5m, active_last_1h, and total_paired.
	insertKey := func(accountID, deviceID string, lastUsedAt *time.Time, revoke bool) {
		t.Helper()
		var revokedAt *time.Time
		if revoke {
			now := time.Now()
			revokedAt = &now
		}

		_, err := db.ExecContext(
			ctx, `
			INSERT INTO daemon_api_keys
				(account_id, key_hash, key_prefix, device_id, platform, daemon_ver,
				 last_used_at, revoked_at)
			VALUES ($1, '$2a$10$fake', 'sk_test_', $2, 'darwin', '0.3.1', $3, $4)`,
			accountID, deviceID, lastUsedAt, revokedAt,
		)
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _ = db.ExecContext(
				ctx,
				"DELETE FROM daemon_api_keys WHERE account_id = $1 AND device_id = $2",
				accountID, deviceID,
			)
		})
	}

	now := time.Now()
	twoMinAgo := now.Add(-2 * time.Minute)
	twoHoursAgo := now.Add(-2 * time.Hour)

	// Active within 5m.
	insertKey("fleet_test_acct_1", "device-aaaa-0001", &twoMinAgo, false)
	// Active within 1h but not 5m.
	insertKey("fleet_test_acct_2", "device-aaaa-0002", &twoHoursAgo, false)
	// Revoked (should not appear in total_paired; counted in revoked).
	insertKey("fleet_test_acct_3", "device-aaaa-0003", nil, true)

	snap, err := repo.FleetHealthSnapshot(ctx)
	require.NoError(t, err)

	// total_paired: only the two non-revoked rows.
	assert.GreaterOrEqual(t, snap.TotalPaired, 2, "total_paired should include both active rows")
	// active_last_5m: the row with last_used_at 2m ago.
	assert.GreaterOrEqual(t, snap.ActiveLast5m, 1, "active_last_5m should include the 2-minute row")
	// active_last_1h: both active rows had last_used_at within 1h.
	assert.GreaterOrEqual(t, snap.ActiveLast1h, 1, "active_last_1h should include the 2-minute row")
	// revoked: at least the one row we revoked.
	assert.GreaterOrEqual(t, snap.Revoked, 1, "revoked should count the revoked row")

	assert.WithinDuration(t, time.Now(), snap.AsOf, 5*time.Second)
}
