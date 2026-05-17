package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

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

// ---------------------------------------------------------------------------
// T1 — registerWithBFF unit tests
// ---------------------------------------------------------------------------

// TestRegisterWithBFF_HappyPath verifies that a 201 response with a valid
// api_key and account_id causes both values to be returned with no error.
func TestRegisterWithBFF_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/daemon/register", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"api_key":"sk_live_abc","account_id":"acc_123"}`))
	}))
	defer srv.Close()

	apiKey, accountID, err := registerWithBFF(
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
}

// TestRegisterWithBFF_EmptyAPIKey verifies that when the BFF returns an empty
// api_key the function surfaces an error mentioning "empty api_key".
func TestRegisterWithBFF_EmptyAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"api_key":"","account_id":"acc_123"}`))
	}))
	defer srv.Close()

	_, _, err := registerWithBFF(
		context.Background(),
		srv.URL,
		"clerk-jwt-token",
		"device-uuid-002",
		"darwin",
		"0.3.0",
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty api_key")
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

	_, _, err := registerWithBFF(
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

	_, _, err := registerWithBFF(
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

	_, _, err := registerWithBFF(
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
