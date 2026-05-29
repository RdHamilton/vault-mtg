package scryfall_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RdHamilton/vault-mtg/services/sync/internal/scryfall"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixtureResponse(sets []scryfall.ScryfallSet) []byte {
	type envelope struct {
		Data []scryfall.ScryfallSet `json:"data"`
	}
	b, _ := json.Marshal(envelope{Data: sets})
	return b
}

func TestFetchSets_FiltersCorrectly(t *testing.T) {
	allSets := []scryfall.ScryfallSet{
		// Should be included: digital expansion
		{Code: "fdn", Name: "Foundations", SetType: "expansion", Digital: true, CardCount: 276, ReleasedAt: "2024-11-15"},
		// Should be included: digital core
		{Code: "m21", Name: "Core Set 2021", SetType: "core", Digital: true, CardCount: 274, ReleasedAt: "2020-07-03"},
		// Should be included: digital masters (e.g. Khans of Tarkir Masters on Arena)
		{Code: "2x2", Name: "Double Masters 2022", SetType: "masters", Digital: true, CardCount: 332, ReleasedAt: "2022-07-08"},
		// Should be included: digital draft_innovation
		{Code: "di1", Name: "Chaos Draft Innovation", SetType: "draft_innovation", Digital: true, CardCount: 50, ReleasedAt: "2024-03-01"},
		// Should be included: digital alchemy set
		{Code: "ya1", Name: "Alchemy: Dominaria", SetType: "alchemy", Digital: true, CardCount: 60, ReleasedAt: "2023-01-10"},
		// Excluded: not digital
		{Code: "dsk", Name: "Duskmourn", SetType: "expansion", Digital: false, CardCount: 261, ReleasedAt: "2024-09-27"},
		// Excluded: non-digital, non-expansion
		{Code: "lea", Name: "Limited Edition Alpha", SetType: "core", Digital: false, CardCount: 295, ReleasedAt: "1993-08-05"},
		// Excluded: digital token set
		{Code: "tfdn", Name: "Foundations Tokens", SetType: "token", Digital: true, CardCount: 30, ReleasedAt: "2024-11-15"},
		// Excluded: digital promo set
		{Code: "spg", Name: "Special Guests", SetType: "promo", Digital: true, CardCount: 20, ReleasedAt: "2023-11-17"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/sets", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixtureResponse(allSets))
	}))
	defer srv.Close()

	client := scryfall.NewClientWithBase(srv.URL, srv.Client())
	got, err := client.FetchSets(context.Background())

	require.NoError(t, err)
	require.Len(t, got, 5, "digital expansion/core/masters/draft_innovation/alchemy sets should be returned")

	codes := make([]string, len(got))
	for i, s := range got {
		codes[i] = s.Code
	}
	assert.ElementsMatch(t, []string{"fdn", "m21", "2x2", "di1", "ya1"}, codes)
}

func TestFetchSets_DateFieldPresent(t *testing.T) {
	sets := []scryfall.ScryfallSet{
		{Code: "blb", Name: "Bloomburrow", SetType: "expansion", Digital: true, CardCount: 266, ReleasedAt: "2024-08-02"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixtureResponse(sets))
	}))
	defer srv.Close()

	client := scryfall.NewClientWithBase(srv.URL, srv.Client())
	got, err := client.FetchSets(context.Background())

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "2024-08-02", got[0].ReleasedAt)
}

func TestFetchSets_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixtureResponse(nil))
	}))
	defer srv.Close()

	client := scryfall.NewClientWithBase(srv.URL, srv.Client())
	got, err := client.FetchSets(context.Background())

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestFetchSets_NonDigitalExcluded(t *testing.T) {
	sets := []scryfall.ScryfallSet{
		{Code: "lea", Name: "Alpha", SetType: "expansion", Digital: false, CardCount: 295, ReleasedAt: "1993-08-05"},
		{Code: "leb", Name: "Beta", SetType: "expansion", Digital: false, CardCount: 302, ReleasedAt: "1993-10-04"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixtureResponse(sets))
	}))
	defer srv.Close()

	client := scryfall.NewClientWithBase(srv.URL, srv.Client())
	got, err := client.FetchSets(context.Background())

	require.NoError(t, err)
	assert.Empty(t, got, "non-digital sets must be excluded")
}

func TestFetchSets_NonDraftableTypesExcluded(t *testing.T) {
	sets := []scryfall.ScryfallSet{
		// masters is now included — should be returned
		{Code: "mh3", Name: "Modern Horizons 3", SetType: "masters", Digital: true, CardCount: 300, ReleasedAt: "2024-06-14"},
		// promo is never draftable on Arena — must be excluded
		{Code: "spg", Name: "Special Guests", SetType: "promo", Digital: true, CardCount: 20, ReleasedAt: "2023-11-17"},
		// token set — excluded
		{Code: "tfdn", Name: "Foundations Tokens", SetType: "token", Digital: true, CardCount: 30, ReleasedAt: "2024-11-15"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixtureResponse(sets))
	}))
	defer srv.Close()

	client := scryfall.NewClientWithBase(srv.URL, srv.Client())
	got, err := client.FetchSets(context.Background())

	require.NoError(t, err)
	require.Len(t, got, 1, "only masters type should pass the filter")
	assert.Equal(t, "mh3", got[0].Code)
}

func TestFetchSets_AlchemyAndDraftInnovationIncluded(t *testing.T) {
	sets := []scryfall.ScryfallSet{
		{Code: "ya1", Name: "Alchemy: Dominaria", SetType: "alchemy", Digital: true, CardCount: 60, ReleasedAt: "2023-01-10"},
		{Code: "di1", Name: "Chaos Draft Innovation", SetType: "draft_innovation", Digital: true, CardCount: 50, ReleasedAt: "2024-03-01"},
		// non-digital alchemy — must be excluded
		{Code: "ya2", Name: "Alchemy: Ixalan Paper", SetType: "alchemy", Digital: false, CardCount: 60, ReleasedAt: "2024-01-10"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixtureResponse(sets))
	}))
	defer srv.Close()

	client := scryfall.NewClientWithBase(srv.URL, srv.Client())
	got, err := client.FetchSets(context.Background())

	require.NoError(t, err)
	require.Len(t, got, 2, "digital alchemy and draft_innovation must be included")
	codes := make([]string, len(got))
	for i, s := range got {
		codes[i] = s.Code
	}
	assert.ElementsMatch(t, []string{"ya1", "di1"}, codes)
}

func TestFetchSets_ErrorOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := scryfall.NewClientWithBase(srv.URL, srv.Client())
	_, err := client.FetchSets(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestFetchSets_ErrorOnInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	client := scryfall.NewClientWithBase(srv.URL, srv.Client())
	_, err := client.FetchSets(context.Background())

	require.Error(t, err)
}

// bulkMetadataBody builds the JSON metadata object that Scryfall returns for
// GET /bulk-data/default-cards. The downloadURL must be an absolute URL
// pointing at the JSONL download server.
func bulkMetadataBody(downloadURL string) []byte {
	b, _ := json.Marshal(map[string]any{
		"object":           "bulk_data",
		"id":               "27bf3214-1271-490b-bdfe-c0be6c23d02f",
		"type":             "default_cards",
		"download_uri":     downloadURL,
		"content_encoding": "gzip",
	})
	return b
}

// writeBulkJSON writes a slice of ScryfallCard objects as a plain JSON array
// directly to the response writer, mirroring the actual Scryfall bulk-data
// download format.
func writeBulkJSON(w http.ResponseWriter, cards []scryfall.ScryfallCard) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cards)
}

// writeBulkJSONGzip writes a slice of ScryfallCard objects as a gzip-compressed
// JSON array, used in tests that set DisableCompression on the transport so
// the explicit decompression path in client.go is exercised.
func writeBulkJSONGzip(w http.ResponseWriter, cards []scryfall.ScryfallCard) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Encoding", "gzip")

	gz := gzip.NewWriter(w)
	_ = json.NewEncoder(gz).Encode(cards)
	_ = gz.Close()
}

// twoHopServer starts two httptest servers: a metadata server that returns the
// bulk-data metadata object pointing at a download server, and the download
// server itself that serves the JSON array payload via writeBody. It returns
// the metadata server URL (which is the base URL for NewClientWithBase), and
// a cleanup function that closes both servers.
//
// writeBody is called on the download server and is responsible for setting
// headers and writing the response body.
func twoHopServer(t *testing.T, writeBody func(w http.ResponseWriter)) (metaURL string, cleanup func()) {
	t.Helper()

	// Start the download server first so we know its URL.
	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeBody(w)
	}))

	metaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/bulk-data/default-cards", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write(bulkMetadataBody(dlSrv.URL + "/bulk/default-cards.json"))
	}))

	return metaSrv.URL, func() {
		metaSrv.Close()
		dlSrv.Close()
	}
}

// intPtr is a test helper that returns a pointer to an int literal.
func intPtr(v int) *int { return &v }

// --- FetchBulkDefaultCards tests ---

// TestFetchBulkDefaultCards_TwoHopPlain verifies the two-hop flow with
// plain (uncompressed) JSON array on the download server.
func TestFetchBulkDefaultCards_TwoHopPlain(t *testing.T) {
	cards := []scryfall.ScryfallCard{
		{ScryfallID: "aaa", ArenaID: intPtr(12345), Name: "Lightning Bolt", SetCode: "fdn"},
		{ScryfallID: "bbb", ArenaID: nil, Name: "Black Lotus", SetCode: "lea"},
		{ScryfallID: "ccc", ArenaID: intPtr(67890), Name: "Counterspell", SetCode: "fdn"},
	}

	metaURL, cleanup := twoHopServer(t, func(w http.ResponseWriter) {
		writeBulkJSON(w, cards)
	})
	defer cleanup()

	client := scryfall.NewClientWithBase(metaURL, http.DefaultClient)
	got, err := client.FetchBulkDefaultCards(context.Background())

	require.NoError(t, err)
	require.Len(t, got, 2, "only Arena-tagged cards (non-null arena_id) must be returned")
	assert.Equal(t, 12345, *got[0].ArenaID)
	assert.Equal(t, 67890, *got[1].ArenaID)
}

// TestFetchBulkDefaultCards_TwoHopGzip verifies the two-hop flow when the
// download server responds with gzip-compressed JSON array and
// DisableCompression is set on the transport (explicit decompression path).
func TestFetchBulkDefaultCards_TwoHopGzip(t *testing.T) {
	cards := []scryfall.ScryfallCard{
		{ScryfallID: "ddd", ArenaID: intPtr(11111), Name: "Shock", SetCode: "m21"},
		{ScryfallID: "eee", ArenaID: nil, Name: "Ancestral Recall", SetCode: "lea"},
		{ScryfallID: "fff", ArenaID: intPtr(22222), Name: "Giant Growth", SetCode: "m21"},
	}

	metaURL, cleanup := twoHopServer(t, func(w http.ResponseWriter) {
		writeBulkJSONGzip(w, cards)
	})
	defer cleanup()

	// Use a transport with DisableCompression so Go does not transparently
	// decompress and strip the Content-Encoding header — our explicit
	// gzip.NewReader path is exercised.
	transport := &http.Transport{DisableCompression: true}
	httpClient := &http.Client{Transport: transport}
	client := scryfall.NewClientWithBase(metaURL, httpClient)
	got, err := client.FetchBulkDefaultCards(context.Background())

	require.NoError(t, err)
	require.Len(t, got, 2, "only Arena-tagged cards must be returned from gzip stream")
	ids := []int{*got[0].ArenaID, *got[1].ArenaID}
	assert.ElementsMatch(t, []int{11111, 22222}, ids)
}

// TestFetchBulkDefaultCards_ArenaFilterApplied verifies that paper-only cards
// (nil arena_id) are excluded across a larger fixture set.
func TestFetchBulkDefaultCards_ArenaFilterApplied(t *testing.T) {
	cards := []scryfall.ScryfallCard{
		{ScryfallID: "a1", ArenaID: intPtr(1), Name: "A", SetCode: "fdn"},
		{ScryfallID: "a2", ArenaID: nil, Name: "B", SetCode: "lea"},
		{ScryfallID: "a3", ArenaID: nil, Name: "C", SetCode: "lea"},
		{ScryfallID: "a4", ArenaID: intPtr(4), Name: "D", SetCode: "fdn"},
		{ScryfallID: "a5", ArenaID: intPtr(5), Name: "E", SetCode: "fdn"},
	}

	metaURL, cleanup := twoHopServer(t, func(w http.ResponseWriter) {
		writeBulkJSON(w, cards)
	})
	defer cleanup()

	client := scryfall.NewClientWithBase(metaURL, http.DefaultClient)
	got, err := client.FetchBulkDefaultCards(context.Background())

	require.NoError(t, err)
	require.Len(t, got, 3)
}

// TestFetchBulkDefaultCards_MetadataNon200 verifies that a non-200 on the
// metadata endpoint is propagated as an error.
func TestFetchBulkDefaultCards_MetadataNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := scryfall.NewClientWithBase(srv.URL, http.DefaultClient)
	_, err := client.FetchBulkDefaultCards(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

// TestFetchBulkDefaultCards_MetadataInvalidJSON verifies that unparseable
// metadata JSON returns an error.
func TestFetchBulkDefaultCards_MetadataInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	client := scryfall.NewClientWithBase(srv.URL, http.DefaultClient)
	_, err := client.FetchBulkDefaultCards(context.Background())

	require.Error(t, err)
}

// TestFetchBulkDefaultCards_MetadataMissingDownloadURI verifies that a
// metadata response with an empty download_uri returns an error.
func TestFetchBulkDefaultCards_MetadataMissingDownloadURI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Valid JSON but download_uri is absent.
		_, _ = w.Write([]byte(`{"object":"bulk_data","type":"default_cards"}`))
	}))
	defer srv.Close()

	client := scryfall.NewClientWithBase(srv.URL, http.DefaultClient)
	_, err := client.FetchBulkDefaultCards(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "download_uri")
}

// TestFetchBulkDefaultCards_DownloadNon200 verifies that a non-200 on the
// download endpoint is propagated as an error.
func TestFetchBulkDefaultCards_DownloadNon200(t *testing.T) {
	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer dlSrv.Close()

	metaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(bulkMetadataBody(dlSrv.URL + "/file.json.gz"))
	}))
	defer metaSrv.Close()

	client := scryfall.NewClientWithBase(metaSrv.URL, http.DefaultClient)
	_, err := client.FetchBulkDefaultCards(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

// TestFetchBulkDefaultCards_DownloadInvalidJSON verifies that a malformed
// body on the download endpoint returns an error.
func TestFetchBulkDefaultCards_DownloadInvalidJSON(t *testing.T) {
	metaURL, cleanup := twoHopServer(t, func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not-json"))
	})
	defer cleanup()

	client := scryfall.NewClientWithBase(metaURL, http.DefaultClient)
	_, err := client.FetchBulkDefaultCards(context.Background())

	require.Error(t, err)
}

// TestFetchBulkDefaultCards_DownloadEmpty verifies that an empty download body
// returns an error (empty body does not start with '[').
func TestFetchBulkDefaultCards_DownloadEmpty(t *testing.T) {
	metaURL, cleanup := twoHopServer(t, func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		// Empty body — not a valid JSON array.
	})
	defer cleanup()

	client := scryfall.NewClientWithBase(metaURL, http.DefaultClient)
	_, err := client.FetchBulkDefaultCards(context.Background())

	// An empty body does not produce a valid '[' token — expect an error.
	require.Error(t, err)
}

// TestFetchBulkDefaultCards_DownloadEmptyArray verifies that a valid empty
// JSON array "[]" returns an empty slice with no error.
func TestFetchBulkDefaultCards_DownloadEmptyArray(t *testing.T) {
	metaURL, cleanup := twoHopServer(t, func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	})
	defer cleanup()

	client := scryfall.NewClientWithBase(metaURL, http.DefaultClient)
	got, err := client.FetchBulkDefaultCards(context.Background())

	require.NoError(t, err)
	assert.Empty(t, got)
}

// TestFetchBulkDefaultCards_TableDriven covers the metadata->download->filter
// flow with multiple scenarios in a single table.
func TestFetchBulkDefaultCards_TableDriven(t *testing.T) {
	arenaCard := scryfall.ScryfallCard{ScryfallID: "x1", ArenaID: intPtr(999), Name: "Opt", SetCode: "fdn"}
	paperCard := scryfall.ScryfallCard{ScryfallID: "x2", ArenaID: nil, Name: "Mox Pearl", SetCode: "lea"}

	tests := []struct {
		name       string
		inputCards []scryfall.ScryfallCard
		wantCount  int
		useGzip    bool
	}{
		{
			name:       "all arena — plain",
			inputCards: []scryfall.ScryfallCard{arenaCard, arenaCard},
			wantCount:  2,
			useGzip:    false,
		},
		{
			name:       "all arena — gzip",
			inputCards: []scryfall.ScryfallCard{arenaCard, arenaCard},
			wantCount:  2,
			useGzip:    true,
		},
		{
			name:       "mixed — only arena returned",
			inputCards: []scryfall.ScryfallCard{arenaCard, paperCard, arenaCard},
			wantCount:  2,
			useGzip:    true,
		},
		{
			name:       "all paper — zero returned",
			inputCards: []scryfall.ScryfallCard{paperCard, paperCard},
			wantCount:  0,
			useGzip:    false,
		},
		{
			name:       "empty array — zero returned",
			inputCards: []scryfall.ScryfallCard{},
			wantCount:  0,
			useGzip:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var httpClient *http.Client
			if tc.useGzip {
				// DisableCompression so our explicit gzip.NewReader path is hit.
				httpClient = &http.Client{Transport: &http.Transport{DisableCompression: true}}
			} else {
				httpClient = http.DefaultClient
			}

			metaURL, cleanup := twoHopServer(t, func(w http.ResponseWriter) {
				if tc.useGzip {
					writeBulkJSONGzip(w, tc.inputCards)
				} else {
					writeBulkJSON(w, tc.inputCards)
				}
			})
			defer cleanup()

			client := scryfall.NewClientWithBase(metaURL, httpClient)
			got, err := client.FetchBulkDefaultCards(context.Background())

			require.NoError(t, err)
			assert.Len(t, got, tc.wantCount)
		})
	}
}

// TestFetchBulkDefaultCards_TransparentGzipHandledByTransport verifies the
// explicit gzip decompression path: when DisableCompression is set on the
// transport and the server sends Content-Encoding: gzip, our gzip.NewReader
// path decompresses the JSON array correctly.
func TestFetchBulkDefaultCards_TransparentGzipHandledByTransport(t *testing.T) {
	cards := []scryfall.ScryfallCard{
		{ScryfallID: "t1", ArenaID: intPtr(777), Name: "Terror", SetCode: "m21"},
	}

	// Build gzip-compressed JSON array in memory.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_ = json.NewEncoder(gz).Encode(cards)
	_ = gz.Close()
	compressed := buf.Bytes()

	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/json")
		w.Write(compressed)
	}))
	defer dlSrv.Close()

	metaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(bulkMetadataBody(dlSrv.URL + "/file.json"))
	}))
	defer metaSrv.Close()

	// DisableCompression so Go does NOT transparently strip Content-Encoding
	// and decompress — our explicit gzip.NewReader path is exercised.
	transport := &http.Transport{DisableCompression: true}
	httpClient := &http.Client{Transport: transport}

	client := scryfall.NewClientWithBase(metaSrv.URL, httpClient)
	got, err := client.FetchBulkDefaultCards(context.Background())

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 777, *got[0].ArenaID)
}

// TestNewClient_BulkClientHasNoTimeout asserts that the bulk-download HTTP client
// created by NewClient has Timeout == 0. This catches any future edit that
// re-introduces a transport-level timeout on the bulk-data stream, which would
// race the 900 s context deadline and kill the download in ~30 s from Lambda.
func TestNewClient_BulkClientHasNoTimeout(t *testing.T) {
	c := scryfall.NewClientForTest()
	assert.Equal(t, 0, int(c.BulkTimeout()),
		"bulk HTTP client must have Timeout==0; a non-zero transport timeout races the context deadline and kills the 150 MB stream")
}

// TestFetchBulkDefaultCards_RealScryfall is a real-network integration test
// that calls the live Scryfall bulk-data endpoint and verifies that the
// Arena-tagged card count meets the expected minimum. It is skipped by default
// and must be enabled with -run=TestFetchBulkDefaultCards_RealScryfall.
//
// Local Verification harness: run this test to populate the PR's Local
// Verification section with the real card count.
func TestFetchBulkDefaultCards_RealScryfall(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-network test in short mode")
	}

	client := scryfall.NewClient()
	cards, err := client.FetchBulkDefaultCards(context.Background())

	require.NoError(t, err, "FetchBulkDefaultCards must not return an error against the real Scryfall API")

	count := len(cards)
	t.Logf("Arena-tagged card count from live Scryfall bulk-data: %d", count)

	// The live Scryfall default-cards file contains ~18,900 Arena-tagged cards
	// as of 2026-05-29. The threshold is set to 15,000 to allow for seasonal
	// fluctuation without being so low it fails to catch a broken fetch.
	// (The dispatch's "≥30k" estimate referred to total cards, not
	// Arena-filtered cards — the default-cards file has ~114k total.)
	const minArenaCards = 15_000
	assert.GreaterOrEqual(t, count, minArenaCards,
		fmt.Sprintf("expected >= %d Arena-tagged cards, got %d", minArenaCards, count))
}
