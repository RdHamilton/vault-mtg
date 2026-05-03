package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-daemon/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServiceRunPreservation verifies that Service.Run performs log preservation
// on startup: it must snapshot Player.log into the archive dir and prune any
// archive files older than LogArchiveMaxAge before proceeding to poll.
func TestServiceRunPreservation(t *testing.T) {
	// Stand up a fake BFF so the dispatcher does not make real network calls.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Write a Player.log with known JSON content.
	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "Player.log")
	logContent := []byte(`{"type":"test.event"}` + "\n")
	require.NoError(t, os.WriteFile(logPath, logContent, 0o600))

	// Prepare an archive dir with one "old" archive file (mtime > 7 days ago).
	archiveDir := t.TempDir()
	oldArchive := filepath.Join(archiveDir, "Player_20200101T000000Z.log")
	require.NoError(t, os.WriteFile(oldArchive, []byte("stale"), 0o600))
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour)
	require.NoError(t, os.Chtimes(oldArchive, tenDaysAgo, tenDaysAgo))

	cfg := &config.Config{
		CloudAPIURL:        server.URL,
		APIKey:             "test-key",
		SyncEnabled:        false, // avoid real dispatch
		LogPath:            logPath,
		PollInterval:       50 * time.Millisecond,
		UseFSNotify:        false,
		IngestPath:         "/v1/ingest/events",
		LogArchiveDir:      archiveDir,
		LogArchiveMaxAge:   7 * 24 * time.Hour,
		LogPreserveOnStart: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	svc := New(cfg)
	// Run returns nil on context cancellation — that is the expected exit path.
	err := svc.Run(ctx)
	assert.NoError(t, err)

	// Assert: exactly one Player_*.log archive exists with matching content.
	entries, err := os.ReadDir(archiveDir)
	require.NoError(t, err)

	var snapshots []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "Player_") && strings.HasSuffix(name, ".log") {
			snapshots = append(snapshots, filepath.Join(archiveDir, name))
		}
	}

	require.Len(t, snapshots, 1, "expected exactly one snapshot after Run; old archive should be pruned")

	got, err := os.ReadFile(snapshots[0])
	require.NoError(t, err)
	assert.Equal(t, logContent, got, "snapshot content must match original Player.log")

	// Assert: the old archive was pruned.
	_, statErr := os.Stat(oldArchive)
	assert.True(t, os.IsNotExist(statErr), "old archive file should have been pruned by Run")
}
