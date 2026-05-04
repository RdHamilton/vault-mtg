package scryfall_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ramonehamilton/mtga-sync/internal/scryfall"
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
		// Excluded: not digital
		{Code: "dsk", Name: "Duskmourn", SetType: "expansion", Digital: false, CardCount: 261, ReleasedAt: "2024-09-27"},
		// Excluded: digital but wrong type (masters)
		{Code: "2x2", Name: "Double Masters 2022", SetType: "masters", Digital: true, CardCount: 332, ReleasedAt: "2022-07-08"},
		// Excluded: non-digital, non-expansion
		{Code: "lea", Name: "Limited Edition Alpha", SetType: "core", Digital: false, CardCount: 295, ReleasedAt: "1993-08-05"},
		// Excluded: digital token set
		{Code: "tfdn", Name: "Foundations Tokens", SetType: "token", Digital: true, CardCount: 30, ReleasedAt: "2024-11-15"},
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
	require.Len(t, got, 2, "only digital expansion/core sets should be returned")

	codes := make([]string, len(got))
	for i, s := range got {
		codes[i] = s.Code
	}
	assert.ElementsMatch(t, []string{"fdn", "m21"}, codes)
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

func TestFetchSets_NonExpansionCoreExcluded(t *testing.T) {
	sets := []scryfall.ScryfallSet{
		{Code: "mh3", Name: "Modern Horizons 3", SetType: "masters", Digital: true, CardCount: 300, ReleasedAt: "2024-06-14"},
		{Code: "spg", Name: "Special Guests", SetType: "promo", Digital: true, CardCount: 20, ReleasedAt: "2023-11-17"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixtureResponse(sets))
	}))
	defer srv.Close()

	client := scryfall.NewClientWithBase(srv.URL, srv.Client())
	got, err := client.FetchSets(context.Background())

	require.NoError(t, err)
	assert.Empty(t, got, "non-expansion/core types must be excluded")
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
