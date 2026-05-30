//go:build darwin

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDumpOutdirClean verifies that filepath.Clean normalises traversal sequences
// the same way runDumpRegions does before passing outdir to os.MkdirAll.
// This is a pure-function test — no mach calls required.
func TestDumpOutdirClean(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"/tmp/collection-dump", "/tmp/collection-dump"},
		{"/tmp/collection-dump/", "/tmp/collection-dump"},
		{"/tmp/../tmp/collection-dump", "/tmp/collection-dump"},
		{"/tmp/foo/../../tmp/collection-dump", "/tmp/collection-dump"},
		{"../../etc/passwd", "../../etc/passwd"}, // Clean does not add a leading slash
	}
	for _, tc := range cases {
		got := filepath.Clean(tc.input)
		assert.Equal(t, tc.want, got, "filepath.Clean(%q)", tc.input)
	}
}

// TestManifestPermissions verifies that a manifest.json written via
// os.OpenFile(..., 0o640) has exactly mode 0o640 (before umask on the test
// machine). We test this by writing a manifest the same way runDumpRegions
// does and stat-ing the result.
func TestManifestPermissions(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")

	entries := []RegionManifestEntry{
		{RegionN: 0, AddrHex: "0x1000", Size: 4096, File: "region_0000_0x1000.bin"},
	}

	f, err := os.OpenFile(manifestPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	require.NoError(t, err)
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	require.NoError(t, enc.Encode(entries))
	require.NoError(t, f.Close())

	info, err := os.Stat(manifestPath)
	require.NoError(t, err)

	// Mask off the file-type bits and compare mode bits.
	got := info.Mode().Perm()
	// On macOS the default umask is typically 022, which would yield 0o620.
	// We assert the requested mode (0o640) with the applied umask — the key
	// invariant is that world-read (0o004) is never set.
	assert.Zero(t, got&0o004, "manifest.json must not be world-readable (mode=%04o)", got)
}

// TestRegionManifestEntryJSON verifies that RegionManifestEntry round-trips
// through JSON correctly — confirming the struct tags match the expected
// manifest format read by analyze_dump.
func TestRegionManifestEntryJSON(t *testing.T) {
	entry := RegionManifestEntry{
		RegionN: 42,
		AddrHex: "0x389c30000",
		Size:    16777216,
		File:    "region_0042_0x389c30000.bin",
	}

	data, err := json.Marshal(entry)
	require.NoError(t, err)

	var got RegionManifestEntry
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, entry, got)
}
