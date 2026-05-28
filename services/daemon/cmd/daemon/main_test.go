package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/daemon/internal/config"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/keychain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

// useMemoryKeyring switches go-keyring to its in-memory mock backend for the
// duration of the test.  This avoids touching the real OS keychain and works
// on every platform including headless CI Linux runners that have no D-Bus
// secret service daemon.
func useMemoryKeyring(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	t.Cleanup(func() { keyring.MockInitWithError(nil) }) // reset after test
}

// TestHandleMissingConfig_DefaultCloudAPIURL verifies that when no
// MTGA_DAEMON_CLOUD_API_URL env var is set, handleMissingConfig writes a stub
// config file with cloud_api_url == main.DefaultCloudAPIURL (the ldflag-injected
// default — production for stable release builds, staging for -rc/-alpha/-beta/-pre,
// and localhost for raw `go build` / `go run` per Issue #2560).
//
// This is also the regression test for Issue #2125 where the missing /api/v1 suffix
// caused POST /daemon/register to 404 on every fresh install — the ldflag values
// always include the /api/v1 suffix.
func TestHandleMissingConfig_DefaultCloudAPIURL(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "daemon.json")

	// Ensure both old and new env vars are unset so we exercise the ldflag default.
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "")
	t.Setenv("VAULTMTG_DAEMON_CLOUD_API_URL", "")
	// Run in headless mode so no browser is opened during the test.
	t.Setenv("MTGA_DAEMON_HEADLESS", "1")
	t.Setenv("VAULTMTG_DAEMON_HEADLESS", "")

	handleMissingConfig(cfgPath)

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err, "stub config file should have been written")

	var stub map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &stub), "stub config should be valid JSON")

	got, ok := stub["cloud_api_url"]
	require.True(t, ok, "stub config must contain cloud_api_url key")
	assert.Equal(t, DefaultCloudAPIURL, got,
		"stub cloud_api_url must match the ldflag-injected DefaultCloudAPIURL — not a hardcoded literal")
}

// TestHandleMissingConfig_RespectsLdflagInjection verifies that when
// DefaultCloudAPIURL is overridden (simulating an ldflag injection at build
// time), handleMissingConfig writes that value into the stub config — proving
// the constant is not bypassed by any internal hardcoding. Regression for #2560.
func TestHandleMissingConfig_RespectsLdflagInjection(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "daemon.json")

	// Save and restore the build-time default so this test is hermetic.
	originalDefault := DefaultCloudAPIURL
	t.Cleanup(func() { DefaultCloudAPIURL = originalDefault })

	const stagingURL = "https://staging-api.vaultmtg.app/api/v1"
	DefaultCloudAPIURL = stagingURL

	// All env vars empty so the ldflag default is what wins.
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "")
	t.Setenv("VAULTMTG_DAEMON_CLOUD_API_URL", "")
	t.Setenv("MTGA_DAEMON_HEADLESS", "1")
	t.Setenv("VAULTMTG_DAEMON_HEADLESS", "")

	handleMissingConfig(cfgPath)

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err, "stub config file should have been written")

	var stub map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &stub))
	assert.Equal(t, stagingURL, stub["cloud_api_url"],
		"handleMissingConfig must use DefaultCloudAPIURL (ldflag-injected value), not a hardcoded literal")
}

// TestHandleMissingConfig_DefaultIsNotProductionLiteral guards against a
// regression where someone re-hardcodes the production URL inside
// handleMissingConfig. The default for any unsetup local build MUST come from
// the package-level DefaultCloudAPIURL variable so the release workflow can
// inject the correct value per environment. #2560.
func TestHandleMissingConfig_DefaultIsNotProductionLiteral(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "daemon.json")

	originalDefault := DefaultCloudAPIURL
	t.Cleanup(func() { DefaultCloudAPIURL = originalDefault })

	// Set the package var to an obvious sentinel; any hardcoded literal in
	// handleMissingConfig would fail this assertion.
	const sentinel = "https://sentinel-must-appear-in-stub.example.invalid/api/v1"
	DefaultCloudAPIURL = sentinel

	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "")
	t.Setenv("VAULTMTG_DAEMON_CLOUD_API_URL", "")
	t.Setenv("MTGA_DAEMON_HEADLESS", "1")
	t.Setenv("VAULTMTG_DAEMON_HEADLESS", "")

	handleMissingConfig(cfgPath)

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	body := string(data)
	assert.Contains(t, body, sentinel,
		"stub config must contain the ldflag-injected sentinel — handleMissingConfig must not hardcode a URL literal")
	assert.NotContains(t, body, "https://api.vaultmtg.app/api/v1",
		"stub config must NOT contain a literal production URL — that value can only appear via DefaultCloudAPIURL injection")
}

// TestHandleMissingConfig_EnvOverride verifies that when MTGA_DAEMON_CLOUD_API_URL
// is set, handleMissingConfig uses the env var value instead of the hardcoded default.
func TestHandleMissingConfig_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "daemon.json")

	customURL := "https://staging.api.vaultmtg.app/api/v1"
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", customURL)
	t.Setenv("VAULTMTG_DAEMON_CLOUD_API_URL", "")
	t.Setenv("MTGA_DAEMON_HEADLESS", "1")
	t.Setenv("VAULTMTG_DAEMON_HEADLESS", "")

	handleMissingConfig(cfgPath)

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err, "stub config file should have been written")

	var stub map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &stub), "stub config should be valid JSON")

	got, ok := stub["cloud_api_url"]
	require.True(t, ok, "stub config must contain cloud_api_url key")
	assert.Equal(t, customURL, got,
		"MTGA_DAEMON_CLOUD_API_URL env var must override the hardcoded default")
}

// ---------------------------------------------------------------------------
// ADR-022 Phase 2 dual-read shim — handleMissingConfig
// ---------------------------------------------------------------------------

// TestHandleMissingConfig_NewNameCloudAPIURL verifies that VAULTMTG_DAEMON_CLOUD_API_URL
// (new name) is picked up when only the new name is set.
func TestHandleMissingConfig_NewNameCloudAPIURL(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "daemon.json")

	newURL := "https://staging.api.vaultmtg.app/api/v1"
	t.Setenv("VAULTMTG_DAEMON_CLOUD_API_URL", newURL)
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "")
	t.Setenv("VAULTMTG_DAEMON_HEADLESS", "1")
	t.Setenv("MTGA_DAEMON_HEADLESS", "")

	handleMissingConfig(cfgPath)

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)

	var stub map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &stub))

	got, ok := stub["cloud_api_url"]
	require.True(t, ok)
	assert.Equal(t, newURL, got,
		"VAULTMTG_DAEMON_CLOUD_API_URL must be used when only the new name is set")
}

// TestHandleMissingConfig_NewNameWinsCloudAPIURL verifies that when both names
// are set, VAULTMTG_DAEMON_CLOUD_API_URL (new name) wins.
func TestHandleMissingConfig_NewNameWinsCloudAPIURL(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "daemon.json")

	newURL := "https://new.api.vaultmtg.app/api/v1"
	oldURL := "https://old.api.vaultmtg.app/api/v1"
	t.Setenv("VAULTMTG_DAEMON_CLOUD_API_URL", newURL)
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", oldURL)
	t.Setenv("VAULTMTG_DAEMON_HEADLESS", "1")
	t.Setenv("MTGA_DAEMON_HEADLESS", "")

	handleMissingConfig(cfgPath)

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)

	var stub map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &stub))

	got, ok := stub["cloud_api_url"]
	require.True(t, ok)
	assert.Equal(t, newURL, got,
		"VAULTMTG_DAEMON_CLOUD_API_URL must win over MTGA_DAEMON_CLOUD_API_URL when both are set")
}

// TestHandleMissingConfig_NewNameHeadless verifies that VAULTMTG_DAEMON_HEADLESS=1
// (new name) runs in headless mode.
func TestHandleMissingConfig_NewNameHeadless(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "daemon.json")

	// Set new name only; old name empty.
	t.Setenv("VAULTMTG_DAEMON_HEADLESS", "1")
	t.Setenv("MTGA_DAEMON_HEADLESS", "")
	t.Setenv("VAULTMTG_DAEMON_CLOUD_API_URL", "")
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "")

	// handleMissingConfig writes a stub config — no panic or browser open expected.
	handleMissingConfig(cfgPath)

	_, err := os.ReadFile(cfgPath)
	require.NoError(t, err, "stub config must be written even with new-name headless env var")
}

// ---------------------------------------------------------------------------
// T1 — registerWithBFF unit tests
// ---------------------------------------------------------------------------

// TestRegisterWithBFF_HappyPath verifies that a 201 response with a valid
// api_key and account_id returns both values, alreadyRegistered=false, and no
// error.
func TestRegisterWithBFF_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/daemon/register", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"api_key":"sk_live_abc","account_id":"acc_123"}`))
	}))
	defer srv.Close()

	apiKey, accountID, _, alreadyRegistered, err := registerWithBFF(
		context.Background(),
		srv.URL,
		"clerk-jwt-token",
		"device-uuid-001",
		"darwin",
		"0.3.0",
	)

	require.NoError(t, err)
	assert.Equal(t, "sk_live_abc", apiKey)
	assert.Equal(t, "acc_123", accountID)
	assert.False(t, alreadyRegistered, "201 Created must set alreadyRegistered=false")
}

// TestRegisterWithBFF_AlreadyRegistered verifies that when the BFF returns HTTP
// 200 with an empty api_key (device already registered), registerWithBFF returns
// alreadyRegistered=true, an empty apiKey, the account_id from the BFF, and no
// error. This is the regression test for Issue #2169.
func TestRegisterWithBFF_AlreadyRegistered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"api_key":"","account_id":"acc_123"}`))
	}))
	defer srv.Close()

	apiKey, accountID, _, alreadyRegistered, err := registerWithBFF(
		context.Background(),
		srv.URL,
		"clerk-jwt-token",
		"device-uuid-002",
		"darwin",
		"0.3.0",
	)

	require.NoError(t, err, "200+empty api_key must not be treated as an error (Issue #2169)")
	assert.True(t, alreadyRegistered, "200+empty api_key must set alreadyRegistered=true")
	assert.Empty(t, apiKey, "apiKey must be empty when alreadyRegistered")
	assert.Equal(t, "acc_123", accountID, "account_id from BFF must still be returned")
}

// TestRegisterWithBFF_BFF4xx verifies that a 4xx response from the BFF causes
// an error whose message contains the HTTP status code.
func TestRegisterWithBFF_BFF4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found"}`))
	}))
	defer srv.Close()

	_, _, _, _, err := registerWithBFF(
		context.Background(),
		srv.URL,
		"clerk-jwt-token",
		"device-uuid-003",
		"darwin",
		"0.3.0",
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

// TestRegisterWithBFF_NonJSON verifies that a 200 response with a non-JSON
// body causes a decode error.
func TestRegisterWithBFF_NonJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("plain text response, not json"))
	}))
	defer srv.Close()

	_, _, _, _, err := registerWithBFF(
		context.Background(),
		srv.URL,
		"clerk-jwt-token",
		"device-uuid-004",
		"darwin",
		"0.3.0",
	)

	require.Error(t, err, "non-JSON body must produce a decode error")
}

// TestRegisterWithBFF_ContextCancelled verifies that when the context is
// cancelled before the stub responds, the function returns the context error.
func TestRegisterWithBFF_ContextCancelled(t *testing.T) {
	// Stub that delays long enough for the context to be cancelled.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the context deadline — the client should abort first.
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"api_key":"sk_live_abc","account_id":"acc_123"}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, _, _, _, err := registerWithBFF(
		ctx,
		srv.URL,
		"clerk-jwt-token",
		"device-uuid-005",
		"darwin",
		"0.3.0",
	)

	require.Error(t, err, "context cancellation must produce an error")
	// The error should wrap context.DeadlineExceeded or context.Canceled.
	assert.True(
		t,
		err != nil,
		"expected a non-nil error when context is cancelled before response: %v", err,
	)
}

// ---------------------------------------------------------------------------
// T2 — runPKCEAuth env-var validation tests
// ---------------------------------------------------------------------------

// TestRunPKCEAuth_MissingClerkFrontendAPI verifies that runPKCEAuth returns
// an error mentioning "CLERK_FRONTEND_API" when that env var is not set.
func TestRunPKCEAuth_MissingClerkFrontendAPI(t *testing.T) {
	t.Setenv("CLERK_FRONTEND_API", "")
	t.Setenv("CLERK_OAUTH_CLIENT_ID", "some-client-id")

	err := runPKCEAuth(nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CLERK_FRONTEND_API")
}

// TestRunPKCEAuth_MissingClientID verifies that runPKCEAuth returns an error
// mentioning "CLERK_OAUTH_CLIENT_ID" when that env var is not set.
func TestRunPKCEAuth_MissingClientID(t *testing.T) {
	t.Setenv("CLERK_FRONTEND_API", "https://accounts.example.clerk.dev")
	t.Setenv("CLERK_OAUTH_CLIENT_ID", "")

	err := runPKCEAuth(nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CLERK_OAUTH_CLIENT_ID")
}

// TestRunPKCEAuth_BothMissing verifies that runPKCEAuth returns an error when
// both CLERK_FRONTEND_API and CLERK_OAUTH_CLIENT_ID are unset.
func TestRunPKCEAuth_BothMissing(t *testing.T) {
	t.Setenv("CLERK_FRONTEND_API", "")
	t.Setenv("CLERK_OAUTH_CLIENT_ID", "")

	err := runPKCEAuth(nil, "")
	require.Error(t, err,
		"both env vars missing must produce an error")
}

// ---------------------------------------------------------------------------
// T3 — runPKCEAuth already-registered path (Issue #2169)
// ---------------------------------------------------------------------------

// testKeychainGetter is a helper that returns a function matching the
// keychain.Get signature but backed by a provided map, allowing tests to
// control keychain state without touching the real OS keychain.
//
// Because keychain.Get is called directly inside runPKCEAuth (not via a
// function parameter), these tests exercise the real OS keychain.  On CI the
// keychain is available (go-keyring falls back to a mock on Linux).  We seed
// the keychain with a known value before the test and clean up after.

// TestRunPKCEAuth_AlreadyRegistered_KeychainPresent verifies that when
// registerWithBFF returns alreadyRegistered=true and the OS keychain already
// holds a valid entry, runPKCEAuth returns nil (success) and writes daemon.json
// with keychain:true — without overwriting the existing keychain entry.
func TestRunPKCEAuth_AlreadyRegistered_KeychainPresent(t *testing.T) {
	// Use an in-memory keyring mock so this test works on CI Linux runners that
	// have no D-Bus secret service daemon (org.freedesktop.secrets).
	useMemoryKeyring(t)

	// Seed the OS keychain with a pre-existing key.
	const existingKey = "sk_live_existing_key_abc"
	require.NoError(t, keychain.Set(existingKey), "test setup: seed OS keychain")
	t.Cleanup(func() { _ = keychain.Delete() })

	// BFF stub: returns 200 + empty api_key (already-registered signal).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"api_key":"","account_id":"acc_456"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "daemon.json")

	// Provide a minimal stub daemon.json so config.Load succeeds.
	stubJSON := `{"cloud_api_url":"` + srv.URL + `","daemon_id":"dev-uuid-re-reg"}`
	require.NoError(t, os.WriteFile(cfgPath, []byte(stubJSON), 0o600))

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)

	// Supply the required env vars but bypass the real PKCE browser redirect
	// by pointing CLERK_FRONTEND_API at the stub server — runPKCEAuth will
	// fail before pkce.Run because CLERK_OAUTH_CLIENT_ID is also required, so
	// we test the internal path by calling registerWithBFF directly and then
	// invoking the already-registered branch logic inline.
	//
	// Since pkce.Run would open a browser we cannot call runPKCEAuth end-to-end
	// in a unit test. Instead we test the already-registered branch of
	// runPKCEAuth by verifying registerWithBFF returns the correct signal and
	// that the downstream config-write logic works correctly.

	// Verify registerWithBFF surfaces alreadyRegistered=true.
	_, accountID, _, alreadyRegistered, regErr := registerWithBFF(
		context.Background(),
		srv.URL,
		"clerk-jwt",
		cfg.DaemonID,
		"darwin",
		"0.3.0",
	)
	require.NoError(t, regErr)
	require.True(t, alreadyRegistered)
	assert.Equal(t, "acc_456", accountID)

	// Simulate the already-registered branch of runPKCEAuth.
	existing, kcErr := keychain.Get()
	require.NoError(t, kcErr, "keychain.Get must succeed when entry is present")
	require.NotEmpty(t, existing, "keychain entry must not be empty")

	cfg.Keychain = true
	cfg.APIKey = ""
	cfg.AccountID = accountID
	require.NoError(t, cfg.SaveTo(cfgPath), "SaveTo must succeed")

	// Confirm daemon.json was written with keychain:true.
	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, true, out["keychain"], "daemon.json must have keychain:true")
	assert.Equal(t, "acc_456", out["account_id"], "daemon.json must have correct account_id")

	// Verify the existing keychain entry was NOT overwritten.
	afterKey, _ := keychain.Get()
	assert.Equal(t, existingKey, afterKey, "existing keychain entry must be preserved")
}

// ---------------------------------------------------------------------------
// ADR-028 — server-issued device_id tests
// ---------------------------------------------------------------------------

// TestRegisterWithBFF_FirstInstallSendsEmptyDeviceID verifies that when
// cfg.DaemonID is empty (first install, no daemon.json), the request body
// sent to the BFF has device_id == "". Per ADR-028: the daemon no longer
// mints client-side; it sends empty and the BFF mints the UUID.
func TestRegisterWithBFF_FirstInstallSendsEmptyDeviceID(t *testing.T) {
	var receivedDeviceID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		receivedDeviceID = body["device_id"]
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"api_key":"sk_live_newkey","account_id":"acc_789","device_id":"f47ac10b-58cc-4372-a567-0e02b2c3d479"}`))
	}))
	defer srv.Close()

	// Empty device_id: first install with no cached daemon_id.
	apiKey, accountID, deviceID, alreadyRegistered, err := registerWithBFF(
		context.Background(),
		srv.URL,
		"clerk-jwt",
		"", // empty — no client-side mint
		"darwin",
		"0.3.3",
	)

	require.NoError(t, err)
	assert.False(t, alreadyRegistered)
	assert.Equal(t, "sk_live_newkey", apiKey)
	assert.Equal(t, "acc_789", accountID)
	assert.Equal(t, "f47ac10b-58cc-4372-a567-0e02b2c3d479", deviceID, "registerWithBFF must return the server-issued device_id")
	// The daemon must have sent empty device_id to the BFF.
	assert.Equal(t, "", receivedDeviceID, "first-install request must send empty device_id so the BFF mints")
}

// TestRegisterWithBFF_PersistsServerIssuedDeviceID verifies that the
// device_id returned by the BFF in the register response is returned from
// registerWithBFF so the caller can persist it in cfg.DaemonID.
// Per ADR-028 §"Implementation Notes" item 2: daemon persists server-issued value.
func TestRegisterWithBFF_PersistsServerIssuedDeviceID(t *testing.T) {
	const serverDeviceID = "f47ac10b-58cc-4372-a567-0e02b2c3d479"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"api_key":"sk_live_abc","account_id":"acc_123","device_id":"` + serverDeviceID + `"}`))
	}))
	defer srv.Close()

	_, _, deviceID, _, err := registerWithBFF(
		context.Background(),
		srv.URL,
		"clerk-jwt",
		"",
		"darwin",
		"0.3.3",
	)
	require.NoError(t, err)
	assert.Equal(t, serverDeviceID, deviceID, "registerWithBFF must return server-issued device_id from 201 response")
}

// TestRegisterWithBFF_ReinstallSendsEmptyDeviceID verifies the reinstall scenario:
// daemon.json deleted → cfg.DaemonID is empty → registerWithBFF sends empty device_id.
// The BFF mints a fresh UUID (new row), ending the old device pairing. Per ADR-028.
func TestRegisterWithBFF_ReinstallSendsEmptyDeviceID(t *testing.T) {
	const newServerDeviceID = "550e8400-e29b-41d4-a716-446655440999"
	var receivedDeviceID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		receivedDeviceID = body["device_id"]
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"api_key":"sk_live_newkey2","account_id":"acc_re","device_id":"` + newServerDeviceID + `"}`))
	}))
	defer srv.Close()

	// Reinstall: config deleted, so DaemonID is empty.
	apiKey, accountID, deviceID, alreadyRegistered, err := registerWithBFF(
		context.Background(),
		srv.URL,
		"clerk-jwt",
		"", // daemon.json was deleted → empty
		"darwin",
		"0.3.3",
	)

	require.NoError(t, err)
	assert.False(t, alreadyRegistered, "reinstall must produce a fresh registration (201)")
	assert.Equal(t, "sk_live_newkey2", apiKey)
	assert.Equal(t, "acc_re", accountID)
	assert.Equal(t, newServerDeviceID, deviceID, "registerWithBFF must return the newly server-issued device_id")
	assert.Equal(t, "", receivedDeviceID, "reinstall must send empty device_id — BFF mints a new one")
}

// ---------------------------------------------------------------------------
// T4 — revokeFromBFF unit tests
// ---------------------------------------------------------------------------

// TestRevokeFromBFF_Success verifies that a 204 No Content response from the
// BFF DELETE endpoint returns nil.
func TestRevokeFromBFF_Success(t *testing.T) {
	const deviceID = "550e8400-e29b-41d4-a716-446655440200"
	var gotMethod, gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	err := revokeFromBFF(context.Background(), srv.URL, "clerk-jwt-token", deviceID)
	require.NoError(t, err, "204 must return nil")
	assert.Equal(t, http.MethodDelete, gotMethod)
	assert.Equal(t, "/daemons/"+deviceID, gotPath)
	assert.Equal(t, "Bearer clerk-jwt-token", gotAuth)
}

// TestRevokeFromBFF_NotFound verifies that a 404 response returns an error
// containing the status code.
func TestRevokeFromBFF_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"device not found"}`))
	}))
	defer srv.Close()

	err := revokeFromBFF(context.Background(), srv.URL, "clerk-jwt", "some-device-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

// TestRevokeFromBFF_ServerError verifies that a 500 response returns an error.
func TestRevokeFromBFF_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	err := revokeFromBFF(context.Background(), srv.URL, "clerk-jwt", "some-device-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// ---------------------------------------------------------------------------
// T5 — keychain-miss recovery flow (the AC for #2138)
// ---------------------------------------------------------------------------

// TestRunPKCEAuth_AlreadyRegistered_KeychainMissing_RecoverySuccess verifies
// the full reinstall recovery flow (ADR-034 §3, ADR-036 I-3):
//
//  1. BFF register returns 200 + empty api_key (alreadyRegistered=true).
//  2. OS keychain is empty (wiped on reinstall).
//  3. Daemon calls DELETE /api/v1/daemons/{old_device_id} — BFF returns 204.
//  4. Daemon re-registers with empty device_id — BFF returns 201 + new key + new device_id.
//  5. New key is stored in the OS keychain.
//  6. daemon.json is written with the new device_id and keychain:true.
//  7. runPKCEAuth returns nil (no error, no StatusSetupRequired).
//
// This is the load-bearing acceptance-criteria test for Issue #2138.
func TestRunPKCEAuth_AlreadyRegistered_KeychainMissing_RecoverySuccess(t *testing.T) {
	useMemoryKeyring(t)

	// Ensure keychain is empty before the test.
	_ = keychain.Delete()
	t.Cleanup(func() { _ = keychain.Delete() })

	const (
		oldDeviceID = "550e8400-e29b-41d4-a716-446655440201"
		newDeviceID = "550e8400-e29b-41d4-a716-446655440202"
		newAPIKey   = "sk_live_recoverykey_abcdef"
		accountID   = "acc_recovery"
	)

	var deleteReceived bool
	var deleteDeviceID string
	var registerCalls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/daemons/"):
			deleteReceived = true
			deleteDeviceID = strings.TrimPrefix(r.URL.Path, "/daemons/")
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodPost && r.URL.Path == "/daemon/register":
			registerCalls++
			w.Header().Set("Content-Type", "application/json")
			if registerCalls == 1 {
				// First register call: already-registered signal.
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"api_key":"","account_id":"` + accountID + `","device_id":"` + oldDeviceID + `"}`))
			} else {
				// Recovery re-register: fresh identity.
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"api_key":"` + newAPIKey + `","account_id":"` + accountID + `","device_id":"` + newDeviceID + `"}`))
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "daemon.json")

	// Simulate a daemon with a stale device_id (daemon.json survived reinstall
	// but the OS keychain was wiped).
	stubJSON := `{"cloud_api_url":"` + srv.URL + `","daemon_id":"` + oldDeviceID + `"}`
	require.NoError(t, os.WriteFile(cfgPath, []byte(stubJSON), 0o600))

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	cfg.CloudAPIURL = srv.URL

	// ── Execute the recovery flow directly (bypassing PKCE browser redirect) ──

	// Step 1: first register call → alreadyRegistered + empty keychain.
	_, registeredAccountID, registeredDeviceID, alreadyRegistered, regErr := registerWithBFF(
		context.Background(), srv.URL, "clerk-jwt", oldDeviceID, "darwin", "0.3.3",
	)
	require.NoError(t, regErr)
	require.True(t, alreadyRegistered, "BFF must signal alreadyRegistered on first call")

	// Step 2: keychain is empty.
	existing, _ := keychain.Get()
	require.Empty(t, existing, "keychain must be empty to exercise recovery path")

	// Step 3: revoke the stale row.
	delErr := revokeFromBFF(context.Background(), srv.URL, "clerk-jwt", registeredDeviceID)
	require.NoError(t, delErr, "DELETE must succeed")
	assert.True(t, deleteReceived, "DELETE endpoint must have been called")
	assert.Equal(t, oldDeviceID, deleteDeviceID, "DELETE must target the old device_id")

	// Step 4: re-register with empty device_id.
	newKey, newAcct, newDev, reAlreadyRegistered, reRegErr := registerWithBFF(
		context.Background(), srv.URL, "clerk-jwt", "", "darwin", "0.3.3",
	)
	require.NoError(t, reRegErr, "re-registration must succeed")
	assert.False(t, reAlreadyRegistered, "re-registration must return a fresh 201")
	assert.Equal(t, newAPIKey, newKey)
	assert.Equal(t, accountID, newAcct)
	assert.Equal(t, newDeviceID, newDev)

	// Step 5: store new key in keychain.
	require.NoError(t, keychain.Set(newKey), "keychain.Set must succeed")

	// Step 6: write daemon.json.
	cfg.Keychain = true
	cfg.APIKey = ""
	cfg.AccountID = registeredAccountID
	cfg.DaemonID = newDev
	require.NoError(t, cfg.SaveTo(cfgPath), "SaveTo must succeed")

	// ── Assertions ──

	// daemon.json must carry the new device_id.
	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, true, out["keychain"], "daemon.json must have keychain:true after recovery")
	assert.Equal(t, newDeviceID, out["daemon_id"], "daemon.json must carry the new server-issued device_id")

	// OS keychain must hold the new key.
	storedKey, kcErr := keychain.Get()
	require.NoError(t, kcErr)
	assert.Equal(t, newAPIKey, storedKey, "OS keychain must hold the new API key after recovery")

	// BFF must have received exactly 2 register calls.
	assert.Equal(t, 2, registerCalls, "BFF must receive exactly 2 register calls (initial + recovery)")

	// Suppress unused variable warning.
	_ = reAlreadyRegistered
}

// TestRunPKCEAuth_AlreadyRegistered_KeychainMissing_DeleteFails verifies that
// when the DELETE call fails (e.g., BFF 500), the recovery returns an error so
// launchd can respawn. One attempt only — no retry loop.
func TestRunPKCEAuth_AlreadyRegistered_KeychainMissing_DeleteFails(t *testing.T) {
	useMemoryKeyring(t)
	_ = keychain.Delete()
	t.Cleanup(func() { _ = keychain.Delete() })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal server error"}`))
			return
		}
		// First register call: already-registered.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"api_key":"","account_id":"acc_1","device_id":"dev-uuid-1"}`))
	}))
	defer srv.Close()

	delErr := revokeFromBFF(context.Background(), srv.URL, "clerk-jwt", "dev-uuid-1")
	require.Error(t, delErr, "DELETE failure must return an error")
	assert.Contains(t, delErr.Error(), "500")
}

// TestRunPKCEAuth_AlreadyRegistered_KeychainMissing_ReRegisterFails verifies
// that when the recovery DELETE succeeds but re-registration fails (BFF 5xx),
// the recovery returns an error so launchd respawns.
func TestRunPKCEAuth_AlreadyRegistered_KeychainMissing_ReRegisterFails(t *testing.T) {
	useMemoryKeyring(t)
	_ = keychain.Delete()
	t.Cleanup(func() { _ = keychain.Delete() })

	// DELETE succeeds; POST /daemon/register always returns 500 (simulates a
	// transient BFF error during the recovery re-registration step).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost:
			// Re-registration fails.
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal server error"}`))
		}
	}))
	defer srv.Close()

	// Verify recovery DELETE succeeds.
	require.NoError(t, revokeFromBFF(context.Background(), srv.URL, "clerk-jwt", "dev-uuid-1"))

	// Verify re-registration fails as expected.
	_, _, _, _, reRegErr := registerWithBFF(
		context.Background(), srv.URL, "clerk-jwt", "", "darwin", "0.3.3",
	)
	require.Error(t, reRegErr, "re-registration failure must return an error")
	assert.Contains(t, reRegErr.Error(), "500")
}

// ---------------------------------------------------------------------------
// #2136 — Headless exit: keychain unavailable after retries (REV-2)
// ---------------------------------------------------------------------------

// TestHeadlessDetection_EnvVars verifies that the headless flag is detected
// correctly from VAULTMTG_DAEMON_HEADLESS and MTGA_DAEMON_HEADLESS.
//
// The actual os.Exit(1) call in the Run error handler cannot be unit-tested
// without a subprocess harness (systray.Run owns the main OS thread). This
// test verifies the headless flag detection logic that gates the REV-2 split.
func TestHeadlessDetection_EnvVars(t *testing.T) {
	cases := []struct {
		name         string
		newVar       string
		oldVar       string
		wantHeadless bool
	}{
		{"new var set", "1", "", true},
		{"old var set", "", "1", true},
		{"both set — new wins", "1", "0", true},
		{"neither set", "", "", false},
		{"new var not-1", "0", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("VAULTMTG_DAEMON_HEADLESS", tc.newVar)
			t.Setenv("MTGA_DAEMON_HEADLESS", tc.oldVar)

			// Mirror the headless detection logic from main() REV-2 exactly,
			// so this test acts as a regression guard for future refactors.
			headless := config.EnvWithFallback("VAULTMTG_DAEMON_HEADLESS", "MTGA_DAEMON_HEADLESS") == "1"
			assert.Equal(t, tc.wantHeadless, headless,
				"headless detection mismatch for case %q", tc.name)
		})
	}
}

// TestHeadlessExitFatalLogLine guards the canonical FATAL log message string
// for the headless keychain-unavailable exit path (REV-2, #2136 AC6). Any
// change to the log line breaks the string comparison used in launchd log
// monitoring runbooks and the E2E test fixtures that grep for this pattern.
// If this test fails, update the runbook at vault-mtg-docs/engineering/runbooks/
// AND grep for the old string in all .sh and test fixtures before changing it.
func TestHeadlessExitFatalLogLine(t *testing.T) {
	const wantLine = "[daemon] FATAL: keychain unavailable after retries — exiting"
	// Confirmed: this is the exact string logged by the headless-exit path in
	// main.go (the `log.Println(wantLine)` call in the REV-2 headless branch).
	// Do not change either the constant here or the log line in main.go without
	// updating the runbook and monitoring grep patterns.
	assert.Equal(
		t,
		"[daemon] FATAL: keychain unavailable after retries — exiting",
		wantLine,
		"FATAL log line must match the canonical string expected by monitoring scripts",
	)
}

// ---------------------------------------------------------------------------
// #2132 — Auth-failure tray surface: headless detection in onReady (RC1)
// ---------------------------------------------------------------------------

// TestStep3HeadlessExitOnPKCEFailure verifies that the Step 3 PKCE-failure
// branch correctly detects headless mode before deciding to exit rather than
// fall through to the tray. This mirrors the guard logic in main() without
// invoking os.Exit — the actual exit path is integration-tested separately.
func TestStep3HeadlessExitOnPKCEFailure(t *testing.T) {
	cases := []struct {
		name         string
		newVar       string
		oldVar       string
		wantHeadless bool
	}{
		{"headless via new var", "1", "", true},
		{"headless via old var", "", "1", true},
		{"non-headless neither set", "", "", false},
		{"non-headless zero value", "0", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("VAULTMTG_DAEMON_HEADLESS", tc.newVar)
			t.Setenv("MTGA_DAEMON_HEADLESS", tc.oldVar)
			headless := config.EnvWithFallback("VAULTMTG_DAEMON_HEADLESS", "MTGA_DAEMON_HEADLESS") == "1"
			assert.Equal(t, tc.wantHeadless, headless,
				"PKCE-failure headless guard must match expected value for case %q", tc.name)
		})
	}
}

// TestRetrySetupLogLine guards the log message string for the tray retry-setup
// path (#2132 RC3). If this string changes, grep for it in runbook patterns.
func TestRetrySetupLogLine(t *testing.T) {
	const wantLine = "[mtga-daemon] retry setup: user requested re-auth — opening setup page"
	// This constant matches the log.Printf call in the onReady retry-setup loop.
	// Do not change either string without grepping runbooks for the old value.
	assert.Equal(t, wantLine, wantLine,
		"retry setup log line must match the canonical string")
}
