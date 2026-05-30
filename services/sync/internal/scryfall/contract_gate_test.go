// contract_gate_test.go — ADR-042 Layer 2 contract gate: Scryfall ingest path.
//
// These tests assert structural invariants of the Scryfall HTTP client seam
// that, if broken, would silently corrupt the Layer 3b integration test (#246)
// and the live sync Lambda. They run inside the existing package scryfall_test
// build and share the twoHopServer / bulkMetadataBody / writeBulkJSON helpers
// defined in client_test.go.
//
// All three tests are tagged [contract-gate] in their names so they can be
// filtered individually with -run=contract-gate.
package scryfall_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/RdHamilton/vault-mtg/services/sync/internal/scryfall"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// corpusFixturePath returns the absolute path to the sync-local corpus fixture
// for the Scryfall set-cards bulk response. The fixture lives at:
//
//	services/sync/testdata/corpus/api-expected/set-cards-response.json
//
// Using runtime.Caller ensures the path resolves correctly regardless of the
// working directory the test binary is invoked from.
func corpusFixturePath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile = .../services/sync/internal/scryfall/contract_gate_test.go
	// Navigate up two levels (internal/scryfall → internal → services/sync)
	// then into testdata/corpus/api-expected/.
	return filepath.Join(
		filepath.Dir(thisFile),
		"..", "..",
		"testdata", "corpus", "api-expected", "set-cards-response.json",
	)
}

// loadCorpusFixture reads the sync-local Scryfall fixture and returns the
// parsed card slice. Fails the test immediately if the file is missing or
// malformed — a missing fixture is a contract violation, not a skip.
func loadCorpusFixture(t *testing.T) []scryfall.ScryfallCard {
	t.Helper()

	fixturePath := corpusFixturePath()
	data, err := os.ReadFile(fixturePath)
	require.NoError(t, err,
		"[contract-gate] corpus fixture missing at %s — copy from daemon corpus or regenerate; "+
			"absence breaks the Layer 2 Scryfall ingest-path gate", fixturePath)

	var cards []scryfall.ScryfallCard
	require.NoError(t, json.Unmarshal(data, &cards),
		"[contract-gate] corpus fixture is not valid Scryfall bulk JSON array at %s", fixturePath)

	return cards
}

// TestContractGate_ScryfallClientSeamIsInjectable verifies that the Scryfall
// client accepts a custom base URL via NewClientWithBase, routing HTTP requests
// to the injected server rather than the hard-coded api.scryfall.com endpoint.
//
// Regression class: if the base URL is ever hard-coded in FetchBulkDefaultCards
// instead of read from c.baseURL, all stub-based tests (Layer 2 and 3b) will
// silently attempt to call the real Scryfall API or time out.
func TestContractGate_ScryfallClientSeamIsInjectable(t *testing.T) {
	cards := loadCorpusFixture(t)
	require.NotEmpty(t, cards, "[contract-gate] corpus fixture must contain at least one card")

	// Collect all arena_id values from the fixture for assertion.
	var wantArenaIDs []int
	for _, c := range cards {
		if c.ArenaID != nil {
			wantArenaIDs = append(wantArenaIDs, *c.ArenaID)
		}
	}
	require.NotEmpty(t, wantArenaIDs,
		"[contract-gate] corpus fixture must have at least one card with a non-null arena_id")

	// Wire up a two-hop stub that serves the corpus fixture cards.
	metaURL, cleanup := twoHopServer(t, func(w http.ResponseWriter) {
		writeBulkJSON(w, cards)
	})
	defer cleanup()

	client := scryfall.NewClientWithBase(metaURL, http.DefaultClient)
	got, err := client.FetchBulkDefaultCards(context.Background())

	require.NoError(t, err, "[contract-gate] FetchBulkDefaultCards must succeed against injected stub")

	gotArenaIDs := make([]int, len(got))
	for i, c := range got {
		require.NotNil(t, c.ArenaID,
			"[contract-gate] client must not return nil ArenaID after filtering")
		gotArenaIDs[i] = *c.ArenaID
	}

	assert.ElementsMatch(t, wantArenaIDs, gotArenaIDs,
		"[contract-gate] FetchBulkDefaultCards must return exactly the arena-tagged cards from the corpus fixture; "+
			"if this fails the base URL seam is broken or the arena_id filter is wrong")
}

// TestContractGate_BulkClientHasNoTimeout asserts that the bulk-download HTTP
// client created by NewClient has Timeout == 0 (no transport-level deadline).
//
// Regression class: #218-class transport-deadline regression — a non-zero
// Timeout on the bulk client races the 900 s context deadline and kills the
// 150 MB stream at ~30 s from Lambda in us-east-1. This gate catches any
// future edit that re-introduces such a timeout.
func TestContractGate_BulkClientHasNoTimeout(t *testing.T) {
	c := scryfall.NewClientForTest()
	assert.Equal(t, 0, int(c.BulkTimeout()),
		"[contract-gate] bulk HTTP client Timeout must be 0; "+
			"a non-zero transport timeout races the 900 s context deadline and kills the 150 MB Scryfall stream — "+
			"see #218-class regression")
}

// TestContractGate_TwoHopFlowEnforced asserts that the Scryfall client follows
// the two-hop download flow: (1) fetch /bulk-data/default-cards metadata to
// obtain the download_uri, then (2) fetch that URI. A single-hop implementation
// that embeds the download URL directly would fail here because the metadata
// server and the download server are distinct.
//
// Regression class: if the two-hop flow collapses to a single hop (e.g., a
// developer hard-codes the bulk download URL), the client bypasses the
// metadata endpoint and the download_uri field becomes dead code. This gate
// ensures the indirection cannot silently disappear.
func TestContractGate_TwoHopFlowEnforced(t *testing.T) {
	cards := loadCorpusFixture(t)

	// metaHit and dlHit track whether each server was contacted.
	var metaHit, dlHit bool

	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dlHit = true
		writeBulkJSON(w, cards)
	}))
	defer dlSrv.Close()

	metaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metaHit = true
		assert.Equal(t, "/bulk-data/default-cards", r.URL.Path,
			"[contract-gate] metadata hop must target /bulk-data/default-cards")
		w.Header().Set("Content-Type", "application/json")
		w.Write(bulkMetadataBody(dlSrv.URL + "/bulk/default-cards.json")) //nolint:errcheck
	}))
	defer metaSrv.Close()

	client := scryfall.NewClientWithBase(metaSrv.URL, http.DefaultClient)
	got, err := client.FetchBulkDefaultCards(context.Background())

	require.NoError(t, err, "[contract-gate] two-hop flow must complete without error")

	assert.True(t, metaHit,
		"[contract-gate] metadata server was never contacted — "+
			"FetchBulkDefaultCards must issue a GET to /bulk-data/default-cards first")
	assert.True(t, dlHit,
		"[contract-gate] download server was never contacted — "+
			"FetchBulkDefaultCards must follow the download_uri returned by the metadata endpoint")

	// Confirm round-trip data integrity: all arena-tagged corpus cards returned.
	var wantArenaIDs []int
	for _, c := range cards {
		if c.ArenaID != nil {
			wantArenaIDs = append(wantArenaIDs, *c.ArenaID)
		}
	}
	gotArenaIDs := make([]int, len(got))
	for i, c := range got {
		gotArenaIDs[i] = *c.ArenaID
	}
	assert.ElementsMatch(t, wantArenaIDs, gotArenaIDs,
		"[contract-gate] two-hop flow must return corpus arena_ids unmodified")
}
