package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// knownMinEntries is the expected minimum number of collection entries recovered
// from the static MTGA memory snapshot. This MUST be updated to the actual entry
// count from the snapshot manifest after Tim's #224 fixture PR lands.
//
// WARNING: a value of 10 is a placeholder that provides no real regression
// guarantee — Lee blocks Phase 2 if this constant is still 10 at the Phase 2
// merge. Update it to the actual count from testdata manifest.
const knownMinEntries = 10

// TestScanDictEntriesRealSnapshot asserts that the heuristic recovers known GRP
// IDs from the static anonymized MTGA memory snapshot committed in testdata/.
//
// Phase 1 (this PR): skipped — snapshot is a placeholder empty file until Tim's
// #224 fixture PR lands.
//
// Phase 2 (after Tim's #224): replace testdata/mtga_collection_snapshot_20260529.bin
// with the real snapshot; update knownMinEntries to the manifest entry count;
// remove the t.Skip call. This test must be GREEN before the Phase 2 commit merges.
func TestScanDictEntriesRealSnapshot(t *testing.T) {
	t.Skip("Phase 2: requires #224 captured MTGA snapshot — remove t.Skip after Tim's fixture PR lands")
	data, err := os.ReadFile("testdata/mtga_collection_snapshot_20260529.bin")
	require.NoError(t, err)
	got := scanDictEntries(data)
	assert.GreaterOrEqual(t, len(got), 1,
		"must recover at least one collection entry from snapshot — "+
			"if this fails with an empty snapshot, the MTGA memory layout derivation "+
			"has not been completed yet (see ADR-040 G4 procedure)")
}

// TestScanDictEntriesDriftCanary is the CI regression gate for signature drift.
// If this fails on a future PR, the MTGA memory layout has changed — refresh the
// snapshot and re-derive the signature per ADR-040 §G4.
//
// Phase 1 (this PR): skipped — snapshot is a placeholder empty file until Tim's
// #224 fixture PR lands.
//
// IMPORTANT: knownMinEntries MUST NOT remain 10 (placeholder) after Phase 2 merges.
// Lee blocks any PR where knownMinEntries == 10 unless the snapshot is also a placeholder.
func TestScanDictEntriesDriftCanary(t *testing.T) {
	t.Skip("Phase 2: requires #224 captured MTGA snapshot — remove t.Skip after Tim's fixture PR lands")
	data, err := os.ReadFile("testdata/mtga_collection_snapshot_20260529.bin")
	require.NoError(t, err)
	got := scanDictEntries(data)
	if len(got) < knownMinEntries {
		t.Fatalf(
			"COLLECTION SCAN DRIFT: recovered %d entries, expected >= %d — "+
				"MTGA memory layout may have changed. Re-derive signature per ADR-040 G4.",
			len(got), knownMinEntries,
		)
	}
}
