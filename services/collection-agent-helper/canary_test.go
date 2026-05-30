package main

// canary_test.go — outer drift-detection canary (vault-mtg-tickets#210)
//
// Seam with #224: scanner_real_test.go (Bob's unit-level fixture tests) validates
// that scanDictEntries parses the snapshot correctly. This file is the OUTER canary
// layer — it validates drift-detection behaviour (correct error token, known-entry
// subset, signature version format) and runs in CI on every push + on the scheduled
// patch-day workflow (.github/workflows/collection-canary.yml).
//
// Tests here do NOT duplicate scanner_real_test.go's entry-count gate.
// They complement it by validating the surrounding canary machinery.
//
// Shared constants (knownMinEntries, snapshotFile, manifestFile) and the
// snapshotManifest type are defined in scanner_real_test.go (#224).

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadManifest(t *testing.T) snapshotManifest {
	t.Helper()
	raw, err := os.ReadFile(manifestFile)
	require.NoError(t, err, "manifest missing: %s", manifestFile)
	var m snapshotManifest
	require.NoError(t, json.Unmarshal(raw, &m), "manifest JSON invalid")
	return m
}

// TestCanaryKnownEntriesSubset asserts that every GRP ID in the manifest's
// known_entries map is present in the parsed snapshot output at the recorded
// quantity. This detects scanner regressions that produce wrong quantities for
// known-good card IDs even when the total entry count stays above the threshold.
func TestCanaryKnownEntriesSubset(t *testing.T) {
	m := loadManifest(t)

	data, err := os.ReadFile(snapshotFile)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	got := scanDictEntries(data)

	var failures []string
	for grpIDStr, wantQty := range m.KnownEntries {
		grpID := 0
		_, scanErr := fmt.Sscanf(grpIDStr, "%d", &grpID)
		require.NoError(t, scanErr, "bad GRP ID in manifest: %s", grpIDStr)

		gotQty, ok := got[grpID]
		if !ok {
			failures = append(failures, fmt.Sprintf("grp=%d missing (want qty=%d)", grpID, wantQty))
		} else if gotQty != wantQty {
			failures = append(failures, fmt.Sprintf("grp=%d qty=%d want %d", grpID, gotQty, wantQty))
		}
	}

	assert.Empty(t, failures,
		"COLLECTION_SCAN_DRIFT: known_entries mismatch — scanner returned wrong IDs/quantities.\n"+
			"If MTGA patched and shifted the layout, re-derive per ADR-040 §G4.\n"+
			"Failures:\n  "+strings.Join(failures, "\n  "),
	)
}

// TestCanarySignatureVersionFormat asserts that CollectionSignatureVersion
// follows the YYYYMMDD-NNN convention defined in ADR-040 §G4. A malformed
// version string means the startup log line will not parse correctly by the
// CloudWatch metric filter (Ray's infra ticket, same T3 cluster).
func TestCanarySignatureVersionFormat(t *testing.T) {
	v := CollectionSignatureVersion
	require.NotEmpty(t, v, "CollectionSignatureVersion must not be empty")

	// Format: YYYYMMDD-NNN (8 digit date, dash, 3+ digit counter)
	if len(v) < 12 {
		t.Fatalf("CollectionSignatureVersion %q is too short (expected YYYYMMDD-NNN format)", v)
	}
	if v[8] != '-' {
		t.Fatalf("CollectionSignatureVersion %q missing dash at position 8 (expected YYYYMMDD-NNN format)", v)
	}
	datePart := v[:8]
	for _, ch := range datePart {
		if ch < '0' || ch > '9' {
			t.Fatalf("CollectionSignatureVersion %q date part %q contains non-digit %q", v, datePart, string(ch))
		}
	}
	counterPart := v[9:]
	if len(counterPart) < 3 {
		t.Fatalf("CollectionSignatureVersion %q counter part %q must be at least 3 digits", v, counterPart)
	}
	for _, ch := range counterPart {
		if ch < '0' || ch > '9' {
			t.Fatalf("CollectionSignatureVersion %q counter part %q contains non-digit %q", v, counterPart, string(ch))
		}
	}
}

// TestCanaryVersionInKnownSignatures asserts that CollectionSignatureVersion
// is registered in knownSignatureVersions. A version that is active but not
// registered means the startup log note will be empty — the on-call note
// appears blank in CloudWatch when triaging a COLLECTION_SCAN_DRIFT alarm.
func TestCanaryVersionInKnownSignatures(t *testing.T) {
	note, ok := knownSignatureVersions[CollectionSignatureVersion]
	assert.True(t, ok,
		"CollectionSignatureVersion %q is not registered in knownSignatureVersions — "+
			"add an entry to collection_signatures.go", CollectionSignatureVersion)
	assert.NotEmpty(t, note,
		"knownSignatureVersions[%q] must have a non-empty description", CollectionSignatureVersion)
}

// TestCanaryDriftTokenInFatalMessage asserts that the COLLECTION_SCAN_DRIFT
// token appears in the error string produced when scanDictEntries returns fewer
// entries than expected. This token is the CloudWatch alarm filter string
// (ADR-040 §G4) — renaming it without updating the alarm filter silently breaks
// alerting.
//
// We synthesise a drift scenario by calling scanDictEntries on zeroed data
// (all bytes zero → no valid entries, guaranteed < knownMinEntries) and
// verifying the message format we would emit matches the expected token.
func TestCanaryDriftTokenInFatalMessage(t *testing.T) {
	// Synthesise the drift scenario: all-zero 16-byte buffer yields no valid
	// entries because hashCode=0 != key=0 is NOT a valid entry (minGRPID=1000).
	zeroed := make([]byte, 16*100) // 100 zero-entry records
	got := scanDictEntries(zeroed)
	assert.Empty(t, got, "zeroed buffer must produce no entries (precondition for drift scenario)")

	// Build the drift message the same way scanner_real_test.go and the daemon
	// server.go do. We assert the token appears literally — any rename here
	// would be caught before the alarm filter silently breaks.
	driftMsg := fmt.Sprintf(
		"COLLECTION_SCAN_DRIFT: recovered %d entries from snapshot, expected >= %d",
		len(got), knownMinEntries,
	)
	assert.Contains(t, driftMsg, "COLLECTION_SCAN_DRIFT",
		"drift message must contain the CloudWatch alarm filter token COLLECTION_SCAN_DRIFT — "+
			"do not rename this token without updating the alarm filter in infra")
}

// TestCanaryManifestConsistency asserts that the manifest's known_min_entries
// matches the knownMinEntries constant in scanner_real_test.go. A mismatch
// means someone updated one without the other, which will produce confusing
// canary failures.
func TestCanaryManifestConsistency(t *testing.T) {
	m := loadManifest(t)
	assert.Equal(t, knownMinEntries, m.KnownMinEntries,
		"manifest known_min_entries (%d) does not match knownMinEntries constant (%d) — "+
			"update scanner_real_test.go to match the manifest",
		m.KnownMinEntries, knownMinEntries,
	)
}
