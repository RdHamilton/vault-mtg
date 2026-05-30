package logreader

// drift_canary_test.go — parser drift canary for real MTGA Player.log fixtures.
//
// This canary runs every fixture in testdata/real/ through all registered
// classifiers and asserts that the recognized-event match rate stays above the
// threshold defined in driftThreshold.  If it falls below threshold, MTGA has
// likely changed its log format and the fixtures need refreshing per the
// ADR-041 G3 procedure documented in testdata/real/MANIFEST.md.
//
// MTGA client version covered: 2026.59.20 (build 2026.59.20.4846.1277160).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// driftThreshold is the minimum fraction of candidate lines that must be
// recognized by at least one classifier.  90% matches the AC3 threshold.
const driftThreshold = 0.90

// realFixtureDir is the directory containing real captured fixtures.
const realFixtureDir = "testdata/real"

// isRecognized reports whether entry matches any known event classifier.
// Only the classification step is exercised here — we do not re-parse, since
// the golden tests in real_fixture_golden_test.go already cover full parsing.
func isRecognized(entry *LogEntry) bool {
	return IsInventoryEntry(entry) ||
		IsQuestProgressEntry(entry) ||
		IsMatchCompletedEntry(entry) ||
		IsCollectionEntry(entry) ||
		isAuthenticateEntry(entry) ||
		isDraftPackEntry(entry) ||
		isDraftPickEntry(entry)
}

// isAuthenticateEntry returns true when the entry carries an
// "authenticateResponse" key — the player_authenticated event.
func isAuthenticateEntry(entry *LogEntry) bool {
	if entry == nil || !entry.IsJSON {
		return false
	}
	_, ok := entry.JSON["authenticateResponse"]
	return ok
}

// isDraftPackEntry returns true when the entry carries a "draftPack" key.
func isDraftPackEntry(entry *LogEntry) bool {
	if entry == nil || !entry.IsJSON {
		return false
	}
	_, ok := entry.JSON["draftPack"]
	return ok
}

// isDraftPickEntry returns true when the entry carries a "pickedCards" key.
func isDraftPickEntry(entry *LogEntry) bool {
	if entry == nil || !entry.IsJSON {
		return false
	}
	_, ok := entry.JSON["pickedCards"]
	return ok
}

// TestParserDriftCanary_RealFixture_2026_59_20 parses every fixture file in
// testdata/real/ and asserts that the recognised-event rate is at or above
// driftThreshold.
//
// On failure the test emits:
//
//	PARSER DRIFT DETECTED: match rate X% below threshold Y% — MTGA may have
//	changed its log format.  Refresh fixtures per ADR-041 G3.
//
// The MANIFEST.md in testdata/real/ describes the refresh procedure.
func TestParserDriftCanary_RealFixture_2026_59_20(t *testing.T) {
	entries, err := os.ReadDir(realFixtureDir)
	if err != nil {
		t.Fatalf("open real fixture dir %s: %v", realFixtureDir, err)
	}

	var total, recognized int

	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".log") {
			continue
		}

		path := filepath.Join(realFixtureDir, de.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read fixture %s: %v", de.Name(), err)
			continue
		}

		entry := &LogEntry{Raw: string(raw)}
		entry.parseJSON()

		if !entry.IsJSON {
			// A non-JSON fixture line is not a candidate for classification.
			continue
		}

		total++
		if isRecognized(entry) {
			recognized++
		} else {
			t.Logf("unrecognized fixture: %s", de.Name())
		}
	}

	if total == 0 {
		t.Fatal("no JSON fixture files found in " + realFixtureDir + " — fixture directory may be empty")
	}

	rate := float64(recognized) / float64(total)
	if rate < driftThreshold {
		t.Fatalf(
			"PARSER DRIFT DETECTED: match rate %.0f%% (%d/%d) below %.0f%% threshold — "+
				"MTGA may have changed its log format. Refresh fixtures per ADR-041 G3.",
			rate*100, recognized, total, driftThreshold*100,
		)
	}
}
