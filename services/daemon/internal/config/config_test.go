package config_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-daemon/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:8080")
	t.Setenv("MTGA_DAEMON_API_KEY", "my-token")
	t.Setenv("MTGA_DAEMON_ACCOUNT_ID", "acc-999")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080", cfg.CloudAPIURL)
	assert.Equal(t, "my-token", cfg.APIKey)
	assert.Equal(t, "acc-999", cfg.AccountID)
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.json")
	content := `{"cloud_api_url":"http://bff.example.com","api_key":"file-key","account_id":"acc-file"}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "http://bff.example.com", cfg.CloudAPIURL)
	assert.Equal(t, "file-key", cfg.APIKey)
}

func TestLoadMissingCloudAPIURL(t *testing.T) {
	// Clear env var to ensure validation triggers
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "")
	_, err := config.Load("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cloud_api_url")
}

func TestEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.json")
	content := `{"cloud_api_url":"http://from-file.example.com"}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://from-env.example.com")

	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "http://from-env.example.com", cfg.CloudAPIURL)
}

func TestDefaults(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, "/ingest/events", cfg.IngestPath)
	assert.True(t, cfg.UseFSNotify)
}

func TestSyncEnabledDefaultsTrue(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	t.Setenv("MTGA_DAEMON_API_KEY", "some-key")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.True(t, cfg.SyncEnabled)
}

func TestSyncEnabledWithMissingAPIKeyNoError(t *testing.T) {
	// sync_enabled=true with no api_key should warn but NOT return an error
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	t.Setenv("MTGA_DAEMON_API_KEY", "")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.True(t, cfg.SyncEnabled)
	assert.Empty(t, cfg.APIKey)
}

func TestDefaultLogPreserveOnStart(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.True(t, cfg.LogPreserveOnStart)
}

func TestDefaultLogArchiveMaxAge(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, 7*24*time.Hour, cfg.LogArchiveMaxAge)
}

func TestDefaultLogArchiveDirNonEmpty(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.LogArchiveDir)
}

func TestLogArchiveDirEnvOverride(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	t.Setenv("MTGA_DAEMON_LOG_ARCHIVE_DIR", "/custom/archive/dir")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, "/custom/archive/dir", cfg.LogArchiveDir)
}

// TestLoadFromFileAllFields verifies the canonical JSON round-trip for every
// key name documented in config.go.  This is the format written by the install
// scripts on both platforms.
func TestLoadFromFileAllFields(t *testing.T) {
	for _, key := range []string{
		"MTGA_DAEMON_CLOUD_API_URL", "MTGA_DAEMON_API_KEY",
		"MTGA_DAEMON_SYNC_ENABLED", "MTGA_DAEMON_LOG_PATH",
		"MTGA_DAEMON_INGEST_PATH", "MTGA_DAEMON_ACCOUNT_ID",
		"MTGA_DAEMON_LOG_ARCHIVE_DIR", "MTGA_DAEMON_LOG_PRESERVE_ON_START",
	} {
		os.Unsetenv(key)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.json")
	content := `{
		"cloud_api_url":         "https://bff.example.com",
		"api_key":               "tok-abc123",
		"sync_enabled":          false,
		"log_path":              "/tmp/Player.log",
		"ingest_path":           "/ingest/events",
		"account_id":            "acc-42",
		"log_archive_dir":       "/tmp/archives",
		"log_preserve_on_start": false
	}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "https://bff.example.com", cfg.CloudAPIURL)
	assert.Equal(t, "tok-abc123", cfg.APIKey)
	assert.False(t, cfg.SyncEnabled)
	assert.Equal(t, "/tmp/Player.log", cfg.LogPath)
	assert.Equal(t, "/ingest/events", cfg.IngestPath)
	assert.Equal(t, "acc-42", cfg.AccountID)
	assert.Equal(t, "/tmp/archives", cfg.LogArchiveDir)
	assert.False(t, cfg.LogPreserveOnStart)
}

// TestOldKeyNamesIgnored confirms that the old installer key names (bff_url,
// daemon_auth_token) written by the pre-fix Windows installer are silently
// ignored by the JSON decoder — they do not satisfy cloud_api_url/api_key.
// The test captures that these stale configs should fail validation so users
// get a clear error rather than a silent misconfiguration.
func TestOldKeyNamesIgnored(t *testing.T) {
	for _, key := range []string{
		"MTGA_DAEMON_CLOUD_API_URL", "MTGA_DAEMON_API_KEY",
		"MTGA_DAEMON_SYNC_ENABLED", "MTGA_DAEMON_LOG_PATH",
		"MTGA_DAEMON_INGEST_PATH", "MTGA_DAEMON_ACCOUNT_ID",
		"MTGA_DAEMON_LOG_ARCHIVE_DIR", "MTGA_DAEMON_LOG_PRESERVE_ON_START",
	} {
		os.Unsetenv(key)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.json")
	// Simulate a JSON file with the old key names (bff_url, daemon_auth_token).
	// Note: plain YAML (e.g. "bff_url: foo") is not valid JSON, so this tests
	// the JSON-with-wrong-keys scenario.
	content := `{"bff_url":"https://bff.example.com","daemon_auth_token":"tok-old"}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	_, err := config.Load(path)
	// cloud_api_url will be empty because "bff_url" is unknown; validation fails.
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cloud_api_url")
}

// ---- New fields: DaemonJWT, DaemonID, UserID ----

// TestLoadDaemonJWTFields verifies that daemon_jwt, daemon_id, and user_id
// are read from the config file.
func TestLoadDaemonJWTFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.json")
	content := `{
		"cloud_api_url": "https://bff.example.com",
		"api_key": "user-key",
		"user_id": 42,
		"daemon_jwt": "eyJhbGciOiJIUzI1NiJ9.eyJleHAiOjk5OTk5OTk5OTl9.sig",
		"daemon_id": "d1e2f3a4-b5c6-7890-1234-567890abcdef"
	}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, 42, cfg.UserID)
	assert.Equal(t, "eyJhbGciOiJIUzI1NiJ9.eyJleHAiOjk5OTk5OTk5OTl9.sig", cfg.DaemonJWT)
	assert.Equal(t, "d1e2f3a4-b5c6-7890-1234-567890abcdef", cfg.DaemonID)
}

// ---- Save / SaveTo ----

// TestSaveRoundTrip verifies that Save() writes back to the same file and
// the reloaded config contains the mutated values.
func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.json")
	content := `{"cloud_api_url":"https://bff.example.com","api_key":"key"}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := config.Load(path)
	require.NoError(t, err)

	cfg.DaemonJWT = "new-jwt-token"
	cfg.DaemonID = "new-daemon-id"
	require.NoError(t, cfg.Save())

	// Reload and verify persisted values.
	cfg2, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "new-jwt-token", cfg2.DaemonJWT)
	assert.Equal(t, "new-daemon-id", cfg2.DaemonID)
}

// TestSaveWithoutFilePathReturnsError verifies Save() fails when config was
// not loaded from a file (env-only config has no file path to write back to).
func TestSaveWithoutFilePathReturnsError(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Empty(t, cfg.FilePath())

	err = cfg.Save()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no file path")
}

// TestSaveToCreatesFile verifies SaveTo() creates the file and records the path.
func TestSaveToCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new-daemon.json")

	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	t.Setenv("MTGA_DAEMON_API_KEY", "key")
	cfg, err := config.Load("")
	require.NoError(t, err)

	cfg.DaemonJWT = "jwt-abc"
	require.NoError(t, cfg.SaveTo(path))
	assert.Equal(t, path, cfg.FilePath())

	// File should exist and contain daemon_jwt.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "jwt-abc")

	// Subsequent Save() should use the recorded path.
	cfg.DaemonJWT = "jwt-updated"
	require.NoError(t, cfg.Save())
	data2, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data2), "jwt-updated")
}

// TestSaveDoesNotWriteFilePathField verifies the unexported filePath field is
// not serialised into the JSON output.
func TestSaveDoesNotWriteFilePathField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.json")
	content := `{"cloud_api_url":"https://bff.example.com","api_key":"key"}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.NoError(t, cfg.Save())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "file_path")
	assert.NotContains(t, string(data), "filePath")
}

// ---- JWTNeedsRefresh ----

// makeJWT constructs a minimal unsigned JWT with the given exp Unix timestamp.
func makeJWT(exp int64) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	claims := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, exp)))
	return header + "." + claims + ".fakesig"
}

// TestJWTNeedsRefreshEmptyJWT verifies that an empty DaemonJWT always needs refresh.
func TestJWTNeedsRefreshEmptyJWT(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.True(t, cfg.JWTNeedsRefresh())
}

// TestJWTNeedsRefreshFarFutureExpiry verifies that a JWT expiring 30 days from
// now does NOT need refresh.
func TestJWTNeedsRefreshFarFutureExpiry(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	cfg, err := config.Load("")
	require.NoError(t, err)
	cfg.DaemonJWT = makeJWT(time.Now().Add(30 * 24 * time.Hour).Unix())
	assert.False(t, cfg.JWTNeedsRefresh())
}

// TestJWTNeedsRefreshWithin24h verifies that a JWT expiring within 24 hours
// needs refresh.
func TestJWTNeedsRefreshWithin24h(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	cfg, err := config.Load("")
	require.NoError(t, err)
	cfg.DaemonJWT = makeJWT(time.Now().Add(23 * time.Hour).Unix())
	assert.True(t, cfg.JWTNeedsRefresh())
}

// TestJWTNeedsRefreshExpired verifies that an already-expired JWT needs refresh.
func TestJWTNeedsRefreshExpired(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	cfg, err := config.Load("")
	require.NoError(t, err)
	cfg.DaemonJWT = makeJWT(time.Now().Add(-1 * time.Hour).Unix())
	assert.True(t, cfg.JWTNeedsRefresh())
}

// TestJWTNeedsRefreshMalformedJWT verifies that a malformed JWT is treated as
// needing refresh.
func TestJWTNeedsRefreshMalformedJWT(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	cfg, err := config.Load("")
	require.NoError(t, err)
	cfg.DaemonJWT = "not-a-valid-jwt"
	assert.True(t, cfg.JWTNeedsRefresh())
}

// TestJWTNeedsRefreshMissingExpClaim verifies that a JWT without an exp claim
// is treated as needing refresh.
func TestJWTNeedsRefreshMissingExpClaim(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	cfg, err := config.Load("")
	require.NoError(t, err)
	// JWT with no exp field.
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	claims := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"daemon-1"}`))
	cfg.DaemonJWT = header + "." + claims + ".sig"
	assert.True(t, cfg.JWTNeedsRefresh())
}

// TestFilePath verifies FilePath returns the file the config was loaded from.
func TestFilePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.json")
	content := `{"cloud_api_url":"https://bff.example.com"}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, path, cfg.FilePath())
}

// TestFilePathEmptyForEnvOnlyConfig verifies FilePath is empty when loaded from env.
func TestFilePathEmptyForEnvOnlyConfig(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Empty(t, cfg.FilePath())
}

// TestSavePreservesAllExistingFields verifies that Save does not clobber fields
// that were not mutated.
func TestSavePreservesAllExistingFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.json")
	content := `{
		"cloud_api_url": "https://bff.example.com",
		"api_key": "user-key",
		"account_id": "acc-99",
		"sync_enabled": true
	}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := config.Load(path)
	require.NoError(t, err)

	cfg.DaemonJWT = "jwt-xyz"
	require.NoError(t, cfg.Save())

	// Re-read the raw JSON and verify old fields are intact.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "https://bff.example.com", m["cloud_api_url"])
	assert.Equal(t, "user-key", m["api_key"])
	assert.Equal(t, "acc-99", m["account_id"])
	assert.Equal(t, "jwt-xyz", m["daemon_jwt"])
}

// ---- DisableUpdateCheck ----

// TestDisableUpdateCheckDefaultsFalse verifies that update checks are enabled by default.
func TestDisableUpdateCheckDefaultsFalse(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.False(t, cfg.DisableUpdateCheck)
}

// TestDisableUpdateCheckEnvVar verifies that MTGA_DAEMON_DISABLE_UPDATE_CHECK=1 disables checks.
func TestDisableUpdateCheckEnvVar(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	t.Setenv("MTGA_DAEMON_DISABLE_UPDATE_CHECK", "1")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.True(t, cfg.DisableUpdateCheck)
}

// TestDisableUpdateCheckEnvVarZeroDoesNotDisable verifies that
// MTGA_DAEMON_DISABLE_UPDATE_CHECK=0 does NOT disable checks.
func TestDisableUpdateCheckEnvVarZeroDoesNotDisable(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	t.Setenv("MTGA_DAEMON_DISABLE_UPDATE_CHECK", "0")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.False(t, cfg.DisableUpdateCheck)
}

// ---- GRE session buffer config ----

// TestGRESessionFlushThresholdDefault verifies the default is 500.
func TestGRESessionFlushThresholdDefault(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, 500, cfg.GRESessionFlushThreshold)
}

// TestGRESessionStaleMinutesDefault verifies the default is 15.
func TestGRESessionStaleMinutesDefault(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, 15, cfg.GRESessionStaleMinutes)
}

// TestGRESessionFlushThresholdEnvVar verifies the env var is respected.
func TestGRESessionFlushThresholdEnvVar(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	t.Setenv("GRE_SESSION_FLUSH_THRESHOLD", "800")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, 800, cfg.GRESessionFlushThreshold)
}

// TestGRESessionStaleMinutesEnvVar verifies the env var is respected.
func TestGRESessionStaleMinutesEnvVar(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	t.Setenv("GRE_SESSION_STALE_MINUTES", "30")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, 30, cfg.GRESessionStaleMinutes)
}

// TestGRESessionFlushThresholdOutOfRangeLow verifies values below 50 revert to
// default with a warning (no error returned).
func TestGRESessionFlushThresholdOutOfRangeLow(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	t.Setenv("GRE_SESSION_FLUSH_THRESHOLD", "10")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, 500, cfg.GRESessionFlushThreshold,
		"out-of-range low value must revert to default 500")
}

// TestGRESessionFlushThresholdOutOfRangeHigh verifies values above 2000 revert
// to default with a warning.
func TestGRESessionFlushThresholdOutOfRangeHigh(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	t.Setenv("GRE_SESSION_FLUSH_THRESHOLD", "9999")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, 500, cfg.GRESessionFlushThreshold,
		"out-of-range high value must revert to default 500")
}

// TestGRESessionFlushThresholdBoundaryMin verifies the minimum boundary value
// (50) is accepted.
func TestGRESessionFlushThresholdBoundaryMin(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	t.Setenv("GRE_SESSION_FLUSH_THRESHOLD", "50")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, 50, cfg.GRESessionFlushThreshold)
}

// TestGRESessionFlushThresholdBoundaryMax verifies the maximum boundary value
// (2000) is accepted.
func TestGRESessionFlushThresholdBoundaryMax(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	t.Setenv("GRE_SESSION_FLUSH_THRESHOLD", "2000")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, 2000, cfg.GRESessionFlushThreshold)
}

// ── NeedsFirstRunAuth ────────────────────────────────────────────────────────

// TestNeedsFirstRunAuth_NoCredentials returns true when no key or JWT is set.
func TestNeedsFirstRunAuth_NoCredentials(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.True(t, cfg.NeedsFirstRunAuth())
}

// TestNeedsFirstRunAuth_PlaintextAPIKey returns false when api_key is present.
func TestNeedsFirstRunAuth_PlaintextAPIKey(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	t.Setenv("MTGA_DAEMON_API_KEY", "sk_live_somekey")
	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.False(t, cfg.NeedsFirstRunAuth())
}

// TestNeedsFirstRunAuth_KeychainTrue returns false when keychain sentinel is set.
func TestNeedsFirstRunAuth_KeychainTrue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.json")
	content := `{"cloud_api_url":"https://bff.example.com","keychain":true}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.False(t, cfg.NeedsFirstRunAuth())
}

// TestNeedsFirstRunAuth_DaemonJWT returns false when a daemon JWT is present.
func TestNeedsFirstRunAuth_DaemonJWT(t *testing.T) {
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "http://localhost:9000")
	cfg, err := config.Load("")
	require.NoError(t, err)
	cfg.DaemonJWT = "some.jwt.token"
	assert.False(t, cfg.NeedsFirstRunAuth())
}

// ── Keychain field serialisation ─────────────────────────────────────────────

// TestKeychainFieldRoundTrip verifies that keychain:true is written and read back.
func TestKeychainFieldRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.json")
	content := `{"cloud_api_url":"https://bff.example.com"}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := config.Load(path)
	require.NoError(t, err)

	cfg.Keychain = true
	require.NoError(t, cfg.Save())

	cfg2, err := config.Load(path)
	require.NoError(t, err)
	assert.True(t, cfg2.Keychain)
}

// TestAPIKeyOmittedFromJSONWhenKeychain verifies that api_key is omitted from
// the JSON output when Keychain is true (omitempty on the field).
func TestAPIKeyOmittedFromJSONWhenKeychain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.json")
	content := `{"cloud_api_url":"https://bff.example.com","keychain":true}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := config.Load(path)
	require.NoError(t, err)
	cfg.APIKey = "" // ensure empty

	require.NoError(t, cfg.Save())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), `"api_key"`)
}

func TestLoadMigratesOldIngestPath(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/daemon.json"
	body := `{"cloud_api_url":"https://api.example.com/api/v1","ingest_path":"/v1/ingest/events","sync_enabled":true}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.IngestPath != "/ingest/events" {
		t.Fatalf("IngestPath migration: got %q, want /ingest/events", cfg.IngestPath)
	}
}
