package scryfall_test

import (
	"context"
	"encoding/json"
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

// writeBulkJSONL writes a slice of ScryfallCard objects as JSONL (one JSON
// object per line) directly to the response writer. This mirrors the actual
// Scryfall bulk-data file format.
func writeBulkJSONL(w http.ResponseWriter, cards []scryfall.ScryfallCard) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	enc := json.NewEncoder(w)
	for _, c := range cards {
		_ = enc.Encode(c)
	}
}

func TestFetchBulkDefaultCards_ReturnsArenaCards(t *testing.T) {
	cards := []scryfall.ScryfallCard{
		// Arena card — must be returned.
		{ScryfallID: "aaa", ArenaID: intPtr(12345), Name: "Lightning Bolt", SetCode: "fdn"},
		// No arena_id — must be skipped.
		{ScryfallID: "bbb", ArenaID: nil, Name: "Black Lotus", SetCode: "lea"},
		// Another arena card.
		{ScryfallID: "ccc", ArenaID: intPtr(67890), Name: "Counterspell", SetCode: "fdn"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/bulk-data/default-cards", r.URL.Path)
		writeBulkJSONL(w, cards)
	}))
	defer srv.Close()

	client := scryfall.NewClientWithBase(srv.URL, srv.Client())
	got, err := client.FetchBulkDefaultCards(context.Background())

	require.NoError(t, err)
	require.Len(t, got, 2, "only Arena-tagged cards (non-null arena_id) must be returned")
	assert.Equal(t, 12345, *got[0].ArenaID)
	assert.Equal(t, 67890, *got[1].ArenaID)
}

func TestFetchBulkDefaultCards_ErrorOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := scryfall.NewClientWithBase(srv.URL, srv.Client())
	_, err := client.FetchBulkDefaultCards(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestFetchBulkDefaultCards_ErrorOnInvalidJSONL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte("not-json\n"))
	}))
	defer srv.Close()

	client := scryfall.NewClientWithBase(srv.URL, srv.Client())
	_, err := client.FetchBulkDefaultCards(context.Background())

	require.Error(t, err)
}

func TestFetchBulkDefaultCards_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		// Empty body — no lines written.
	}))
	defer srv.Close()

	client := scryfall.NewClientWithBase(srv.URL, srv.Client())
	got, err := client.FetchBulkDefaultCards(context.Background())

	require.NoError(t, err)
	assert.Empty(t, got)
}

// intPtr is a test helper that returns a pointer to an int literal.
func intPtr(v int) *int { return &v }
