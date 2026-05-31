// Package corpus_test validates that every expected corpus file is present,
// parses as valid JSON, and that the daemon-emit files deserialise into
// contract.DaemonEvent without error. It does not run the daemon or projection
// worker — it is a structural integrity check for the Layer 1 corpus.
//
// Run: go test ./services/daemon/testdata/corpus/...
package corpus_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/RdHamilton/vault-mtg/services/contract"
)

// corpusDir is the root of this corpus (the directory containing this file).
// Using os.ReadFile with relative paths keeps this portable and avoids the
// embed workaround complexity for files outside the package directory.
const corpusDir = "."

// expectedPlayerLog lists all player-log fixture files that must exist and
// must be non-empty single-line JSON.
var expectedPlayerLog = []string{
	"player-log/match-completed.log",
	"player-log/quest-progress.log",
	"player-log/draft-pack.log",
	"player-log/draft-pick.log",
	"player-log/collection-updated.log",
	"player-log/deck-updated.log",
	"player-log/inventory-updated.log",
	"player-log/player-authenticated.log",
}

// expectedDaemonEmit lists all daemon-emit fixture files that must exist,
// must parse as JSON, and must deserialise into contract.DaemonEvent.
var expectedDaemonEmit = []string{
	"daemon-emit/match-completed.json",
	"daemon-emit/match-completed-empty-format.json",
	"daemon-emit/match-completed-missing-id.json",
	"daemon-emit/quest-progress.json",
	"daemon-emit/quest-progress-duplicate.json",
	"daemon-emit/draft-pack.json",
	"daemon-emit/draft-pick.json",
	"daemon-emit/collection-updated.json",
	"daemon-emit/deck-updated.json",
	"daemon-emit/inventory-updated.json",
}

// expectedDBExpected lists all db-expected fixture files that must exist and
// parse as valid JSON objects.
var expectedDBExpected = []string{
	"db-expected/match-completed.json",
	"db-expected/match-completed-empty-format.json",
	"db-expected/quest-progress.json",
	"db-expected/quest-upsert-result.json",
	"db-expected/deck-updated.json",
}

// expectedAPIExpected lists all api-expected fixture files that must exist
// and parse as valid JSON objects.
var expectedAPIExpected = []string{
	"api-expected/match-history-response.json",
	"api-expected/quest-history-response.json",
	"api-expected/deck-response.json",
	"api-expected/meta-archetypes-response.json",
	"api-expected/set-cards-response.json",
}

// TestCorpusFilesLoad verifies each expected corpus file is present and
// contains valid JSON.
func TestCorpusFilesLoad(t *testing.T) {
	t.Run("player-log files present and valid JSON", func(t *testing.T) {
		for _, rel := range expectedPlayerLog {
			rel := rel
			t.Run(rel, func(t *testing.T) {
				data := mustRead(t, rel)
				assertValidJSON(t, rel, data)
			})
		}
	})

	t.Run("daemon-emit files present, valid JSON, deserialise to DaemonEvent", func(t *testing.T) {
		for _, rel := range expectedDaemonEmit {
			rel := rel
			t.Run(rel, func(t *testing.T) {
				data := mustRead(t, rel)
				assertValidJSON(t, rel, data)
				var evt contract.DaemonEvent
				if err := json.Unmarshal(data, &evt); err != nil {
					t.Fatalf("%s: json.Unmarshal into contract.DaemonEvent: %v", rel, err)
				}
				if evt.Type == "" {
					t.Errorf("%s: DaemonEvent.Type must be non-empty", rel)
				}
			})
		}
	})

	t.Run("db-expected files present and valid JSON", func(t *testing.T) {
		for _, rel := range expectedDBExpected {
			rel := rel
			t.Run(rel, func(t *testing.T) {
				data := mustRead(t, rel)
				assertValidJSON(t, rel, data)
			})
		}
	})

	t.Run("api-expected files present and valid JSON", func(t *testing.T) {
		for _, rel := range expectedAPIExpected {
			rel := rel
			t.Run(rel, func(t *testing.T) {
				data := mustRead(t, rel)
				assertValidJSON(t, rel, data)
			})
		}
	})
}

// TestDaemonEmitVariants asserts regression-class variant fixtures carry the
// expected field values.
func TestDaemonEmitVariants(t *testing.T) {
	t.Run("match-completed-empty-format has empty Format", func(t *testing.T) {
		data := mustRead(t, "daemon-emit/match-completed-empty-format.json")
		var evt contract.DaemonEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		var payload contract.MatchCompletedPayload
		if err := json.Unmarshal(evt.Payload, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		if payload.Format != "" {
			t.Errorf("expected Format == %q (empty), got %q", "", payload.Format)
		}
	})

	t.Run("match-completed-missing-id has empty MatchID", func(t *testing.T) {
		data := mustRead(t, "daemon-emit/match-completed-missing-id.json")
		var evt contract.DaemonEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		var payload contract.MatchCompletedPayload
		if err := json.Unmarshal(evt.Payload, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		if payload.MatchID != "" {
			t.Errorf("expected MatchID == %q (empty), got %q", "", payload.MatchID)
		}
	})

	t.Run("quest-progress-duplicate has same QuestID as quest-progress", func(t *testing.T) {
		orig := mustReadDaemonEvent(t, "daemon-emit/quest-progress.json")
		dup := mustReadDaemonEvent(t, "daemon-emit/quest-progress-duplicate.json")

		var origPayload, dupPayload contract.QuestProgressPayload
		if err := json.Unmarshal(orig.Payload, &origPayload); err != nil {
			t.Fatalf("unmarshal orig payload: %v", err)
		}
		if err := json.Unmarshal(dup.Payload, &dupPayload); err != nil {
			t.Fatalf("unmarshal dup payload: %v", err)
		}
		if len(origPayload.Quests) == 0 || len(dupPayload.Quests) == 0 {
			t.Fatal("quest payloads must have at least one quest entry")
		}
		if origPayload.Quests[0].QuestID != dupPayload.Quests[0].QuestID {
			t.Errorf("quest-progress and quest-progress-duplicate must share the same first QuestID: got %q vs %q",
				origPayload.Quests[0].QuestID, dupPayload.Quests[0].QuestID)
		}
		// The duplicate must have a higher or equal progress count.
		if dupPayload.Quests[0].Progress < origPayload.Quests[0].Progress {
			t.Errorf("duplicate quest progress (%d) should be >= original (%d)",
				dupPayload.Quests[0].Progress, origPayload.Quests[0].Progress)
		}
	})
}

// TestManifestExists verifies the MANIFEST and README files are present.
func TestManifestExists(t *testing.T) {
	for _, rel := range []string{"MANIFEST", "README.md", "mtga-version.txt"} {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			mustRead(t, rel)
		})
	}
}

// catalogEntry mirrors one row of catalog/catalog.json emitted by
// tools/fixture-extractor/extract.py --catalog (#262).
type catalogEntry struct {
	Axis       string `json:"axis"`
	Event      string `json:"event"`
	Count      int    `json:"count"`
	SampleFile string `json:"sample_file"`
}

// TestCatalogIntegrity verifies the event-discovery catalog (#262) is valid
// JSON, non-empty, and that every referenced sample file exists on disk.
func TestCatalogIntegrity(t *testing.T) {
	data := mustRead(t, "catalog/catalog.json")
	var entries []catalogEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("catalog/catalog.json: invalid JSON: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("catalog/catalog.json: must contain at least one event type")
	}
	mustRead(t, "catalog/catalog.md")

	for _, e := range entries {
		if e.Axis == "" || e.Event == "" {
			t.Errorf("catalog entry missing axis/event: %+v", e)
		}
		if e.Count <= 0 {
			t.Errorf("catalog entry %s/%s: count must be > 0, got %d", e.Axis, e.Event, e.Count)
		}
		if e.SampleFile == "" {
			continue // sample-less entries are permitted (no parseable body)
		}
		rel := filepath.Join("catalog", e.SampleFile)
		if _, err := os.ReadFile(filepath.Join(corpusDir, rel)); err != nil {
			t.Errorf("catalog entry %s/%s references missing sample %q: %v",
				e.Axis, e.Event, e.SampleFile, err)
		}
	}
}

// TestPlayerAuthenticatedShape pins the corrected player-authenticated fixture
// (#262 / Ray change #2): the real authenticateResponse is {clientId, sessionId,
// screenName} only — it must NOT reintroduce a userId/accountId key, and its
// clientId is the local player's account token (== reservedPlayers[].userId).
func TestPlayerAuthenticatedShape(t *testing.T) {
	data := mustRead(t, "player-log/player-authenticated.log")
	var wrapper struct {
		AuthenticateResponse map[string]json.RawMessage `json:"authenticateResponse"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		t.Fatalf("player-authenticated.log: invalid JSON: %v", err)
	}
	ar := wrapper.AuthenticateResponse
	if ar == nil {
		t.Fatal("player-authenticated.log: missing authenticateResponse")
	}
	for _, forbidden := range []string{"userId", "accountId"} {
		if _, ok := ar[forbidden]; ok {
			t.Errorf("authenticateResponse must not contain %q key (real shape is clientId-only)", forbidden)
		}
	}
	for _, required := range []string{"clientId", "sessionId", "screenName"} {
		if _, ok := ar[required]; !ok {
			t.Errorf("authenticateResponse missing required key %q", required)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustRead(t *testing.T, rel string) []byte {
	t.Helper()
	path := filepath.Join(corpusDir, rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("corpus file %s: %v", rel, err)
	}
	if len(data) == 0 {
		t.Fatalf("corpus file %s is empty", rel)
	}
	return data
}

func assertValidJSON(t *testing.T, name string, data []byte) {
	t.Helper()
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("%s: invalid JSON: %v", name, err)
	}
}

func mustReadDaemonEvent(t *testing.T, rel string) contract.DaemonEvent {
	t.Helper()
	data := mustRead(t, rel)
	var evt contract.DaemonEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		t.Fatalf("%s: json.Unmarshal into contract.DaemonEvent: %v", rel, err)
	}
	return evt
}
