package daemonstate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/RdHamilton/vault-mtg/services/daemon/internal/daemonstate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_MissingFileReturnsZeroState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon-state.json")
	s, err := daemonstate.Load(path)
	require.NoError(t, err)
	assert.False(t, s.AuthPaused)
	assert.Equal(t, 0, s.AuthAttempts)
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon-state.json")

	want := daemonstate.State{
		AuthPaused:   true,
		AuthAttempts: 3,
	}
	require.NoError(t, daemonstate.Save(path, want))

	got, err := daemonstate.Load(path)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestSave_IsAtomic(t *testing.T) {
	// Verify that the file exists at the expected path after Save and that the
	// directory was created when absent (the temp dir already exists, but the
	// sub-dir does not).
	dir := filepath.Join(t.TempDir(), "subdir")
	path := filepath.Join(dir, "daemon-state.json")

	require.NoError(t, daemonstate.Save(path, daemonstate.State{AuthPaused: true}))

	// The target file must exist; no leftover .tmp files.
	_, err := os.Stat(path)
	require.NoError(t, err)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp", "no leftover temp files")
	}
}

func TestLoad_CorruptFileReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon-state.json")
	require.NoError(t, os.WriteFile(path, []byte("not-json{{{"), 0o600))

	_, err := daemonstate.Load(path)
	assert.Error(t, err)
}

func TestStateFilePath(t *testing.T) {
	got := daemonstate.StateFilePath("/home/user/.vaultmtg/daemon.json")
	assert.Equal(t, "/home/user/.vaultmtg/daemon-state.json", got)
}

func TestStateFilePath_Windows(t *testing.T) {
	// Use filepath.Join for cross-platform correctness.
	dir := filepath.Join("C:", "Users", "user", "AppData", "Roaming", "vaultmtg")
	got := daemonstate.StateFilePath(filepath.Join(dir, "daemon.json"))
	assert.Equal(t, filepath.Join(dir, "daemon-state.json"), got)
}

func TestLoad_UnknownFieldsIgnored(t *testing.T) {
	// Future-compatibility: new fields added to daemon-state.json by a newer
	// daemon must not break the current decoder.
	path := filepath.Join(t.TempDir(), "daemon-state.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"auth_paused":true,"auth_attempts":2,"future_field":42}`), 0o600))

	s, err := daemonstate.Load(path)
	require.NoError(t, err)
	assert.True(t, s.AuthPaused)
	assert.Equal(t, 2, s.AuthAttempts)
}
