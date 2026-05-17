package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleMissingConfig_DefaultCloudAPIURL verifies that when no
// MTGA_DAEMON_CLOUD_API_URL env var is set, handleMissingConfig writes a stub
// config file with cloud_api_url == "https://api.vaultmtg.app/api/v1".
// This is the regression test for Issue #2125 where the missing /api/v1 suffix
// caused POST /daemon/register to 404 on every fresh install.
func TestHandleMissingConfig_DefaultCloudAPIURL(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "daemon.json")

	// Ensure the env var is unset so we exercise the hardcoded default.
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", "")
	// Run in headless mode so no browser is opened during the test.
	t.Setenv("MTGA_DAEMON_HEADLESS", "1")

	handleMissingConfig(cfgPath)

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err, "stub config file should have been written")

	var stub map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &stub), "stub config should be valid JSON")

	got, ok := stub["cloud_api_url"]
	require.True(t, ok, "stub config must contain cloud_api_url key")
	assert.Equal(t, "https://api.vaultmtg.app/api/v1", got,
		"default cloud_api_url must include /api/v1 prefix so registerWithBFF resolves to the correct BFF path")
}

// TestHandleMissingConfig_EnvOverride verifies that when MTGA_DAEMON_CLOUD_API_URL
// is set, handleMissingConfig uses the env var value instead of the hardcoded default.
func TestHandleMissingConfig_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "daemon.json")

	customURL := "https://staging.api.vaultmtg.app/api/v1"
	t.Setenv("MTGA_DAEMON_CLOUD_API_URL", customURL)
	t.Setenv("MTGA_DAEMON_HEADLESS", "1")

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
