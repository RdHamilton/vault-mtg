package datasets_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/sync/internal/datasets"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/draftdata"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/scryfall"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/seventeenlands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStore is an in-memory Store implementation used in unit tests.
type mockStore struct {
	data         map[string]*draftdata.SetRatings
	colorRatings map[string][]seventeenlands.ColorRating
	hashes       map[string]string
}

func newMockStore() *mockStore {
	return &mockStore{
		data:         make(map[string]*draftdata.SetRatings),
		colorRatings: make(map[string][]seventeenlands.ColorRating),
		hashes:       make(map[string]string),
	}
}

func (m *mockStore) GetActiveSets(_ context.Context) ([]datasets.SyncSet, error) {
	seen := make(map[string]bool)
	var sets []datasets.SyncSet
	for k := range m.data {
		code := strings.SplitN(k, "/", 2)[0]
		if !seen[code] {
			seen[code] = true
			sets = append(sets, datasets.SyncSet{Code: code, ExpansionCode: code})
		}
	}
	return sets, nil
}

func (m *mockStore) UpsertRatings(_ context.Context, ratings draftdata.SetRatings) error {
	key := ratings.SetCode + "/" + ratings.DraftFormat
	m.data[key] = &ratings
	return nil
}

func (m *mockStore) GetRatings(_ context.Context, setCode, draftFormat string) (*draftdata.SetRatings, error) {
	key := setCode + "/" + draftFormat
	return m.data[key], nil
}

func (m *mockStore) UpsertSets(_ context.Context, _ []scryfall.ScryfallSet) error {
	return nil
}

func (m *mockStore) UpsertColorRatings(_ context.Context, setCode, draftFormat string, ratings []seventeenlands.ColorRating) error {
	key := setCode + "/" + draftFormat
	m.colorRatings[key] = ratings
	return nil
}

func (m *mockStore) GetHash(_ context.Context, key string) (string, error) {
	return m.hashes[key], nil
}

func (m *mockStore) SetHash(_ context.Context, key string, hash string) error {
	m.hashes[key] = hash
	return nil
}

func (m *mockStore) UpsertSetCards(_ context.Context, _ []scryfall.ScryfallCard) error {
	return nil
}

func TestMockStore_RoundTrip(t *testing.T) {
	store := newMockStore()
	ctx := context.Background()

	input := draftdata.SetRatings{
		SetCode:     "FDN",
		DraftFormat: "PremierDraft",
		FetchedAt:   time.Now().UTC().Truncate(time.Second),
		Cards: []seventeenlands.CardRating{
			{MtgaID: 101, Name: "Lightning Bolt", ALSA: 1.5, ATA: 1.8, GIHWR: 0.62, SeenCount: 1000},
		},
	}

	require.NoError(t, store.UpsertRatings(ctx, input))

	got, err := store.GetRatings(ctx, "FDN", "PremierDraft")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "FDN", got.SetCode)
	assert.Len(t, got.Cards, 1)
	assert.Equal(t, "Lightning Bolt", got.Cards[0].Name)
}

// TestMockStore_SecondUpsertReplacesAllCards verifies the DELETE+INSERT semantics:
// after a second UpsertRatings call for the same set/format, the store contains
// exactly the cards from the second call — not a merged/partial set.
func TestMockStore_SecondUpsertReplacesAllCards(t *testing.T) {
	store := newMockStore()
	ctx := context.Background()

	first := draftdata.SetRatings{
		SetCode:     "BLB",
		DraftFormat: "PremierDraft",
		FetchedAt:   time.Now().UTC().Truncate(time.Second),
		Cards: []seventeenlands.CardRating{
			{Name: "Card A", ALSA: 1.0, ATA: 1.1, GIHWR: 0.55, SeenCount: 500},
			{Name: "Card B", ALSA: 2.0, ATA: 2.1, GIHWR: 0.45, SeenCount: 400},
			{Name: "Card C", ALSA: 3.0, ATA: 3.1, GIHWR: 0.40, SeenCount: 300},
		},
	}
	require.NoError(t, store.UpsertRatings(ctx, first))

	second := draftdata.SetRatings{
		SetCode:     "BLB",
		DraftFormat: "PremierDraft",
		FetchedAt:   time.Now().UTC().Truncate(time.Second),
		Cards: []seventeenlands.CardRating{
			{Name: "Card D", ALSA: 4.0, ATA: 4.1, GIHWR: 0.60, SeenCount: 600},
			{Name: "Card E", ALSA: 5.0, ATA: 5.1, GIHWR: 0.50, SeenCount: 700},
		},
	}
	require.NoError(t, store.UpsertRatings(ctx, second))

	got, err := store.GetRatings(ctx, "BLB", "PremierDraft")
	require.NoError(t, err)
	require.NotNil(t, got)

	// Must have exactly the cards from the second call — the first batch is gone.
	assert.Len(t, got.Cards, 2, "second upsert must replace all cards, not append")

	names := make([]string, 0, len(got.Cards))
	for _, c := range got.Cards {
		names = append(names, c.Name)
	}
	assert.Contains(t, names, "Card D")
	assert.Contains(t, names, "Card E")
	assert.NotContains(t, names, "Card A")
	assert.NotContains(t, names, "Card B")
	assert.NotContains(t, names, "Card C")
}

// TestMockStore_UpsertColorRatings verifies that color ratings can be stored and
// replaced via UpsertColorRatings (DELETE+INSERT semantics on the mock).
func TestMockStore_UpsertColorRatings(t *testing.T) {
	store := newMockStore()
	ctx := context.Background()

	first := []seventeenlands.ColorRating{
		{ShortName: "WU", ColorName: "Azorius", Wins: 2900, Games: 5000, IsSummary: false},
		{ShortName: "BG", ColorName: "Golgari", Wins: 1664, Games: 3200, IsSummary: false},
	}
	require.NoError(t, store.UpsertColorRatings(ctx, "FDN", "PremierDraft", first))

	got := store.colorRatings["FDN/PremierDraft"]
	require.Len(t, got, 2)
	assert.Equal(t, "WU", got[0].ShortName)
	assert.InDelta(t, 0.58, got[0].WinRate(), 0.001)

	// Second upsert must replace the first batch.
	second := []seventeenlands.ColorRating{
		{ShortName: "RG", ColorName: "Gruul", Wins: 3660, Games: 6000, IsSummary: false},
	}
	require.NoError(t, store.UpsertColorRatings(ctx, "FDN", "PremierDraft", second))

	got2 := store.colorRatings["FDN/PremierDraft"]
	require.Len(t, got2, 1, "second upsert must replace all color ratings")
	assert.Equal(t, "RG", got2[0].ShortName)
}

// TestMockStore_ZeroFetchedAt_AcceptedByMock documents that the mock store accepts a
// zero FetchedAt without error. The real defensive default (substituting time.Now()) lives
// in PostgresStore.UpsertRatings and is covered by the integration test.
func TestMockStore_ZeroFetchedAt_AcceptedByMock(t *testing.T) {
	store := newMockStore()
	ctx := context.Background()

	// Deliberately leave FetchedAt as zero to simulate a caller that forgot to set it.
	input := draftdata.SetRatings{
		SetCode:     "TST",
		DraftFormat: "PremierDraft",
		// FetchedAt intentionally zero
		Cards: []seventeenlands.CardRating{
			{MtgaID: 42, Name: "Test Creature", ALSA: 2.5},
		},
	}

	require.NoError(t, store.UpsertRatings(ctx, input))

	got, err := store.GetRatings(ctx, "TST", "PremierDraft")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Len(t, got.Cards, 1)
	// FetchedAt in the mock is stored as-is (zero) — the real defensive default
	// lives in PostgresStore.UpsertRatings and is covered by integration tests.
	assert.True(t, got.FetchedAt.IsZero(), "mock store stores FetchedAt as provided (zero)")
}

// TestMockStore_GetHash_MissingKey verifies that GetHash returns ("", nil) for
// a key that has never been stored — callers treat the empty string as "no hash".
func TestMockStore_GetHash_MissingKey(t *testing.T) {
	store := newMockStore()
	ctx := context.Background()

	hash, err := store.GetHash(ctx, "FDN")
	require.NoError(t, err)
	assert.Empty(t, hash, "missing key must return empty string")
}

// TestMockStore_SetHash_ThenGetHash verifies a round-trip: store a hash and
// read it back unchanged.
func TestMockStore_SetHash_ThenGetHash(t *testing.T) {
	store := newMockStore()
	ctx := context.Background()

	const key = "BLB"
	const want = "sha256:abc123"

	require.NoError(t, store.SetHash(ctx, key, want))

	got, err := store.GetHash(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// TestMockStore_SetHash_OverwritesPreviousHash verifies that a second SetHash
// call for the same key replaces the stored value, not appends.
func TestMockStore_SetHash_OverwritesPreviousHash(t *testing.T) {
	store := newMockStore()
	ctx := context.Background()

	const key = "TST"

	require.NoError(t, store.SetHash(ctx, key, "hash-v1"))
	require.NoError(t, store.SetHash(ctx, key, "hash-v2"))

	got, err := store.GetHash(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, "hash-v2", got, "second SetHash must overwrite the first")
}

// TestMockStore_SetHash_IndependentKeys verifies that two different keys store
// independent hashes and do not interfere with each other.
func TestMockStore_SetHash_IndependentKeys(t *testing.T) {
	store := newMockStore()
	ctx := context.Background()

	require.NoError(t, store.SetHash(ctx, "FDN", "hash-fdn"))
	require.NoError(t, store.SetHash(ctx, "BLB", "hash-blb"))

	fdn, err := store.GetHash(ctx, "FDN")
	require.NoError(t, err)
	assert.Equal(t, "hash-fdn", fdn)

	blb, err := store.GetHash(ctx, "BLB")
	require.NoError(t, err)
	assert.Equal(t, "hash-blb", blb)
}

// Compile-time assertion that PostgresStore satisfies the Store interface.
var _ datasets.Store = (*datasets.PostgresStore)(nil)
