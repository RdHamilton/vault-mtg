package config_test

import (
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
	assert.Equal(t, "/v1/ingest/events", cfg.IngestPath)
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
