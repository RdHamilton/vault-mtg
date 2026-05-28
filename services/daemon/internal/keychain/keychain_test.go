package keychain_test

import (
	"testing"

	"github.com/RdHamilton/vault-mtg/services/daemon/internal/keychain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

// useMemoryKeyring switches go-keyring to its in-memory mock backend for the
// duration of the test.  This avoids touching the real OS keychain and works
// on every platform including headless CI runners.
func useMemoryKeyring(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	t.Cleanup(func() { keyring.MockInitWithError(nil) }) // reset after test
}

// TestConstants verifies the exported constants hold the correct values so
// callers can rely on the string literals (e.g. for launchd label matching).
func TestConstants(t *testing.T) {
	assert.Equal(t, "com.vaultmtg.daemon", keychain.ServiceNameNew)
	assert.Equal(t, "com.mtga-companion.daemon", keychain.ServiceNameLegacy)
	assert.Equal(t, "api-key", keychain.AccountKey)
}

// ── Scenario 1: new entry present ────────────────────────────────────────────

// TestGet_NewEntryPresent verifies that when ServiceNameNew has an entry
// Get() returns it without touching the legacy service name.
func TestGet_NewEntryPresent(t *testing.T) {
	useMemoryKeyring(t)

	const wantKey = "sk_live_newentry"
	require.NoError(t, keyring.Set(keychain.ServiceNameNew, keychain.AccountKey, wantKey))

	got, err := keychain.Get()
	require.NoError(t, err)
	assert.Equal(t, wantKey, got)
}

// TestSet_WritesToNewServiceName confirms that Set() stores under ServiceNameNew.
func TestSet_WritesToNewServiceName(t *testing.T) {
	useMemoryKeyring(t)

	const wantKey = "sk_live_writtenkey"
	require.NoError(t, keychain.Set(wantKey))

	got, err := keyring.Get(keychain.ServiceNameNew, keychain.AccountKey)
	require.NoError(t, err)
	assert.Equal(t, wantKey, got)
}

// TestSetAndGet is the basic round-trip test: Set then Get returns the same value.
func TestSetAndGet(t *testing.T) {
	useMemoryKeyring(t)

	const key = "sk_live_test1234"
	require.NoError(t, keychain.Set(key))

	got, err := keychain.Get()
	require.NoError(t, err)
	assert.Equal(t, key, got)
}

// TestSet_Overwrite verifies that a second Set() replaces the first.
func TestSet_Overwrite(t *testing.T) {
	useMemoryKeyring(t)

	require.NoError(t, keychain.Set("sk_live_first"))
	require.NoError(t, keychain.Set("sk_live_second"))

	got, err := keychain.Get()
	require.NoError(t, err)
	assert.Equal(t, "sk_live_second", got)
}

// ── Scenario 2: legacy entry present (upgrade path) ──────────────────────────

// TestGet_LegacyEntryPresent_CopiedForward verifies that when only the legacy
// service name has an entry, Get() returns the key AND copies it to ServiceNameNew.
// The legacy entry must be retained (not deleted).
func TestGet_LegacyEntryPresent_CopiedForward(t *testing.T) {
	useMemoryKeyring(t)

	const wantKey = "sk_live_legacykey"
	// Seed only the legacy entry — simulating an upgrade from the old daemon.
	require.NoError(t, keyring.Set(keychain.ServiceNameLegacy, keychain.AccountKey, wantKey))

	got, err := keychain.Get()
	require.NoError(t, err, "Get() must succeed when only legacy entry is present")
	assert.Equal(t, wantKey, got, "Get() must return the legacy key")

	// ── Copy-forward assertion ────────────────────────────────────────────────
	copiedVal, copyErr := keyring.Get(keychain.ServiceNameNew, keychain.AccountKey)
	require.NoError(t, copyErr, "legacy key must have been copied to ServiceNameNew")
	assert.Equal(t, wantKey, copiedVal, "copied value must equal the original legacy key")

	// ── Retention assertion ───────────────────────────────────────────────────
	legacyVal, legacyErr := keyring.Get(keychain.ServiceNameLegacy, keychain.AccountKey)
	require.NoError(t, legacyErr, "legacy entry must be retained after migration (not deleted)")
	assert.Equal(t, wantKey, legacyVal, "retained legacy entry must be unchanged")
}

// TestGet_LegacyPresent_SubsequentCallHitsNew verifies that a second Get() call
// after the copy-forward reads from ServiceNameNew, not from legacy.
// This proves the copy-forward is effective and persistent within the same mock store.
func TestGet_LegacyPresent_SubsequentCallHitsNew(t *testing.T) {
	useMemoryKeyring(t)

	const wantKey = "sk_live_subseqcall"
	require.NoError(t, keyring.Set(keychain.ServiceNameLegacy, keychain.AccountKey, wantKey))

	// First call triggers migration.
	_, err := keychain.Get()
	require.NoError(t, err)

	// Remove the legacy entry to confirm subsequent reads come from ServiceNameNew.
	require.NoError(t, keyring.Delete(keychain.ServiceNameLegacy, keychain.AccountKey))

	got, err := keychain.Get()
	require.NoError(t, err)
	assert.Equal(t, wantKey, got)
}

// ── Scenario 3: neither entry present ────────────────────────────────────────

// TestGet_NotFound verifies that ErrNotFound is returned when no entry exists
// under either service name (fresh install).
func TestGet_NotFound(t *testing.T) {
	useMemoryKeyring(t)
	_, err := keychain.Get()
	assert.ErrorIs(t, err, keychain.ErrNotFound)
}

// ── Scenario 4: neither entry present (via global mock error) ────────────────

// TestGet_NeitherEntryPresent_GlobalMockError verifies the neither-entry-present
// branch when go-keyring's MockInitWithError(ErrNotFound) is used. Because that
// mock applies the same error to every keyring call, both Get(ServiceNameNew)
// and Get(ServiceNameLegacy) return ErrNotFound — which routes through the
// "neither present" branch in keychain.Get() and returns ErrNotFound.
//
// This is intentionally distinct from TestGet_NotFound (which uses the
// in-memory mock and an empty store). It also distinct from
// TestGet_CorruptedLegacyEntry (in keychain_internal_test.go), which uses a
// per-service-name seam to truly exercise the corrupted-legacy branch — the
// branch this test was previously mis-claimed to cover (#2255).
func TestGet_NeitherEntryPresent_GlobalMockError(t *testing.T) {
	// MockInitWithError makes ALL keyring operations return the given error.
	keyring.MockInitWithError(keyring.ErrNotFound)
	t.Cleanup(func() { keyring.MockInitWithError(nil) })

	_, err := keychain.Get()
	// Must return ErrNotFound (not a raw keyring error) so callers that check
	// errors.Is(err, keychain.ErrNotFound) can trigger re-auth cleanly.
	assert.ErrorIs(t, err, keychain.ErrNotFound,
		"both keyring ops returning ErrNotFound must yield keychain.ErrNotFound")
}

// ── Delete tests ──────────────────────────────────────────────────────────────

// TestDelete_Existing verifies that Delete() removes the ServiceNameNew entry.
func TestDelete_Existing(t *testing.T) {
	useMemoryKeyring(t)

	require.NoError(t, keychain.Set("sk_live_todelete"))
	require.NoError(t, keychain.Delete())

	_, err := keychain.Get()
	// After Delete, the new entry is gone.  If no legacy entry exists either
	// this should be ErrNotFound.
	assert.ErrorIs(t, err, keychain.ErrNotFound)
}

// TestDelete_Idempotent verifies that Delete() on an empty keychain returns nil.
func TestDelete_Idempotent(t *testing.T) {
	useMemoryKeyring(t)
	assert.NoError(t, keychain.Delete())
}

// TestDelete_DoesNotRemoveLegacy verifies that Delete() only removes ServiceNameNew
// and leaves the legacy entry intact (important for downgrade safety).
func TestDelete_DoesNotRemoveLegacy(t *testing.T) {
	useMemoryKeyring(t)

	const legacyKey = "sk_live_legacyretained"
	require.NoError(t, keyring.Set(keychain.ServiceNameLegacy, keychain.AccountKey, legacyKey))
	require.NoError(t, keychain.Set("sk_live_new"))

	require.NoError(t, keychain.Delete())

	// Legacy entry must still be present.
	legacyVal, err := keyring.Get(keychain.ServiceNameLegacy, keychain.AccountKey)
	require.NoError(t, err, "legacy entry must survive Delete()")
	assert.Equal(t, legacyKey, legacyVal)
}
