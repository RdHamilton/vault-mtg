package main

// scanner_real_test.go — PR-gate canary for the scanDictEntries heuristic against
// a real anonymized MTGA memory snapshot.
//
// Tim captured the snapshot per ADR-041 G4 from a live MTGA macOS session
// (version 2026.59.20) and committed it alongside a manifest recording all 500
// known GRP-ID→quantity pairs.
//
// These tests complement the unit tests in scanner_test.go which exercise
// the heuristic against hand-crafted byte sequences.  This file validates that
// the heuristic works against the real Unity in-memory layout.
//
// #210 coordination seam: this file tests scanDictEntries at the unit level
// against a static fixture.  The scheduled nightly/patch-day operational canary
// (COLLECTION_SCAN_DRIFT log token) is implemented in ticket #210 — do not
// add cron or live-process code here.

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// knownMinEntries is the minimum number of collection entries the scanner
	// must recover from the static fixture.  Derived from
	// testdata/mtga_collection_snapshot_2026.59.20.manifest.json
	// (known_min_entries = 500).
	//
	// Lee blocks any PR where this value is 10 (placeholder) unless the
	// corresponding snapshot is also a placeholder.
	knownMinEntries = 500

	// snapshotFile is the anonymized MTGA memory snapshot captured from a
	// 2026.59.20 session on darwin_arm64.
	snapshotFile = "testdata/mtga_collection_snapshot_2026.59.20.bin"

	// manifestFile records the known GRP-ID→quantity map for the snapshot.
	manifestFile = "testdata/mtga_collection_snapshot_2026.59.20.manifest.json"
)

// snapshotManifest is the JSON structure persisted alongside the binary snapshot.
// Only the fields relevant to test validation are decoded here.
type snapshotManifest struct {
	KnownMinEntries int            `json:"known_min_entries"`
	KnownEntries    map[string]int `json:"known_entries"`
}

// TestScanDictEntriesRealSnapshot asserts that scanDictEntries recovers every
// GRP-ID→quantity pair listed in the snapshot manifest from the static binary
// fixture.
//
// On failure: either the heuristic has regressed or the snapshot is corrupted.
// Refresh the snapshot per ADR-041 G4.
func TestScanDictEntriesRealSnapshot(t *testing.T) {
	data, err := os.ReadFile(snapshotFile)
	require.NoError(t, err, "read snapshot fixture — verify testdata/ is committed")

	manifestData, err := os.ReadFile(manifestFile)
	require.NoError(t, err, "read snapshot manifest")

	var manifest snapshotManifest
	require.NoError(t, json.Unmarshal(manifestData, &manifest))
	require.NotEmpty(t, manifest.KnownEntries, "manifest must list known GRP entries")

	got := scanDictEntries(data)
	require.NotEmpty(t, got, "scanner must recover at least one entry from real snapshot")

	// Validate every entry in the manifest is present with the correct quantity.
	// String keys in the manifest JSON are the GRP IDs (MTGA card identifiers).
	var mismatches []string
	for grpStr, wantQty := range manifest.KnownEntries {
		var grpID int
		_, err := parseGRPID(grpStr, &grpID)
		if err != nil {
			t.Errorf("manifest has non-integer GRP key %q: %v", grpStr, err)
			continue
		}
		gotQty, ok := got[grpID]
		if !ok {
			mismatches = append(mismatches, grpStr+" missing")
		} else if gotQty != wantQty {
			mismatches = append(mismatches,
				grpStr+": got qty "+itoa(gotQty)+" want "+itoa(wantQty))
		}
	}
	assert.Empty(t, mismatches,
		"scanner returned unexpected results for %d manifest entries", len(mismatches))
}

// TestScanDictEntriesDriftCanary is the CI regression gate for MTGA memory
// layout drift.  It asserts that the total number of recovered entries is at
// or above the known minimum baseline.
//
// On failure the test emits the COLLECTION SCAN DRIFT sentinel so that log
// scanners (ticket #210) can detect the regression token:
//
//	COLLECTION SCAN DRIFT: recovered N entries, expected >= 500 — MTGA memory
//	layout may have changed. Refresh snapshot per ADR-041 G4.
//
// Do not change the wording of that message — #210's alert rules key on the
// COLLECTION_SCAN_DRIFT token prefix.
func TestScanDictEntriesDriftCanary(t *testing.T) {
	data, err := os.ReadFile(snapshotFile)
	require.NoError(t, err, "read snapshot fixture — verify testdata/ is committed")

	got := scanDictEntries(data)
	if len(got) < knownMinEntries {
		t.Fatalf(
			"COLLECTION SCAN DRIFT: recovered %d entries, expected >= %d — "+
				"MTGA memory layout may have changed. Refresh snapshot per ADR-041 G4.",
			len(got), knownMinEntries,
		)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// parseGRPID converts a decimal string s to an int via *out, returning s for
// chaining and an error on non-integer input.
func parseGRPID(s string, out *int) (string, error) {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return s, &badGRPKey{s}
		}
		n = n*10 + int(ch-'0')
	}
	*out = n
	return s, nil
}

type badGRPKey struct{ key string }

func (e *badGRPKey) Error() string { return "non-integer GRP key: " + e.key }

// itoa converts an int to a decimal string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	b := [20]byte{}
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
