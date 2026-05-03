package logreader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshot_SourceMissing(t *testing.T) {
	archiveDir := t.TempDir()
	dst, err := Snapshot("/nonexistent/Player.log", archiveDir)
	assert.NoError(t, err)
	assert.Empty(t, dst)
}

func TestSnapshot_Success(t *testing.T) {
	srcDir := t.TempDir()
	archiveDir := t.TempDir()
	src := filepath.Join(srcDir, "Player.log")
	content := []byte(`{"type":"test.event"}` + "\n")
	require.NoError(t, os.WriteFile(src, content, 0o600))

	dst, err := Snapshot(src, archiveDir)
	require.NoError(t, err)
	assert.NotEmpty(t, dst)

	// Filename must match Player_<timestamp>.log
	base := filepath.Base(dst)
	assert.True(t, strings.HasPrefix(base, "Player_"), "filename should start with Player_")
	assert.True(t, strings.HasSuffix(base, ".log"), "filename should end with .log")

	// Content must match
	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestSnapshot_CreatesArchiveDir(t *testing.T) {
	srcDir := t.TempDir()
	// Use a subdirectory that doesn't exist yet
	archiveDir := filepath.Join(t.TempDir(), "nested", "archives")
	src := filepath.Join(srcDir, "Player.log")
	require.NoError(t, os.WriteFile(src, []byte("data"), 0o600))

	dst, err := Snapshot(src, archiveDir)
	require.NoError(t, err)
	assert.NotEmpty(t, dst)
}

func TestListSnapshots_MissingDir(t *testing.T) {
	paths, err := ListSnapshots("/nonexistent/archive/dir")
	assert.NoError(t, err)
	assert.Nil(t, paths)
}

func TestListSnapshots_Success(t *testing.T) {
	archiveDir := t.TempDir()

	// Create matching files
	names := []string{
		"Player_20240101T120000Z.log",
		"Player_20240102T120000Z.log",
	}
	for _, n := range names {
		require.NoError(t, os.WriteFile(filepath.Join(archiveDir, n), []byte("x"), 0o600))
	}

	// Create non-matching files that should be filtered out
	require.NoError(t, os.WriteFile(filepath.Join(archiveDir, "other.txt"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(archiveDir, "NotPlayer_20240101T120000Z.log"), []byte("x"), 0o600))

	paths, err := ListSnapshots(archiveDir)
	require.NoError(t, err)
	assert.Len(t, paths, 2)

	// Oldest-first (sorted by filename)
	assert.True(t, strings.HasSuffix(paths[0], "Player_20240101T120000Z.log"))
	assert.True(t, strings.HasSuffix(paths[1], "Player_20240102T120000Z.log"))
}

func TestPruneSnapshots_MissingDir(t *testing.T) {
	err := PruneSnapshots("/nonexistent/archive/dir", 7*24*time.Hour)
	assert.NoError(t, err)
}

func TestPruneSnapshots_RemovesOld(t *testing.T) {
	archiveDir := t.TempDir()

	// Create an old file
	oldFile := filepath.Join(archiveDir, "Player_20200101T000000Z.log")
	require.NoError(t, os.WriteFile(oldFile, []byte("old"), 0o600))

	// Set its mod time to 10 days ago
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour)
	require.NoError(t, os.Chtimes(oldFile, tenDaysAgo, tenDaysAgo))

	err := PruneSnapshots(archiveDir, 7*24*time.Hour)
	require.NoError(t, err)

	_, statErr := os.Stat(oldFile)
	assert.True(t, os.IsNotExist(statErr), "old snapshot should have been removed")
}

func TestPruneSnapshots_KeepsRecent(t *testing.T) {
	archiveDir := t.TempDir()

	// Create a recent file
	recentFile := filepath.Join(archiveDir, "Player_20260501T000000Z.log")
	require.NoError(t, os.WriteFile(recentFile, []byte("recent"), 0o600))

	// mod time defaults to now — well within 7 days

	err := PruneSnapshots(archiveDir, 7*24*time.Hour)
	require.NoError(t, err)

	_, statErr := os.Stat(recentFile)
	assert.NoError(t, statErr, "recent snapshot should NOT have been removed")
}
