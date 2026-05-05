package handler_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-sync/internal/datasets"
	"github.com/ramonehamilton/mtga-sync/internal/draftdata"
	"github.com/ramonehamilton/mtga-sync/internal/handler"
	"github.com/ramonehamilton/mtga-sync/internal/scryfall"
	"github.com/ramonehamilton/mtga-sync/internal/seventeenlands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubFetcher is a test double for the Fetcher interface.
type stubFetcher struct {
	called       int
	cards        []seventeenlands.CardRating
	err          error
	colorsCalled int
	colors       []seventeenlands.ColorRating
	colorsErr    error
}

func (f *stubFetcher) FetchCardRatings(_ context.Context, _, _ string) ([]seventeenlands.CardRating, error) {
	f.called++
	return f.cards, f.err
}

func (f *stubFetcher) FetchColorRatings(_ context.Context, _, _ string) ([]seventeenlands.ColorRating, error) {
	f.colorsCalled++
	return f.colors, f.colorsErr
}

// stubStore is a test double for the datasets.Store interface.
type stubStore struct {
	dbSets               []string
	dbErr                error
	upserted             []draftdata.SetRatings
	upsertFn             func(setCode string) error
	upsertedColorRatings []stubColorUpsert

	// hash support -- populate storedHashes to simulate existing stored hashes.
	storedHashes []stubSetHashCall
	hashGetErr   error
	hashSetErr   error
	setHashCalls []stubSetHashCall
}

type stubColorUpsert struct {
	setCode     string
	draftFormat string
	ratings     []seventeenlands.ColorRating
}

type stubSetHashCall struct {
	key  string
	hash string
}

func (s *stubStore) GetActiveSets(_ context.Context) ([]string, error) {
	return s.dbSets, s.dbErr
}

func (s *stubStore) UpsertRatings(_ context.Context, r draftdata.SetRatings) error {
	if s.upsertFn != nil {
		return s.upsertFn(r.SetCode)
	}
	s.upserted = append(s.upserted, r)
	return nil
}

func (s *stubStore) GetRatings(_ context.Context, _, _ string) (*draftdata.SetRatings, error) {
	return nil, nil
}

func (s *stubStore) UpsertSets(_ context.Context, _ []scryfall.ScryfallSet) error {
	return nil
}

func (s *stubStore) UpsertColorRatings(_ context.Context, setCode, draftFormat string, ratings []seventeenlands.ColorRating) error {
	s.upsertedColorRatings = append(s.upsertedColorRatings, stubColorUpsert{setCode, draftFormat, ratings})
	return nil
}

func (s *stubStore) GetHash(_ context.Context, key string) (string, error) {
	if s.hashGetErr != nil {
		return "", s.hashGetErr
	}
	for _, h := range s.storedHashes {
		if h.key == key {
			return h.hash, nil
		}
	}
	return "", nil
}

func (s *stubStore) SetHash(_ context.Context, key, hash string) error {
	if s.hashSetErr != nil {
		return s.hashSetErr
	}
	s.setHashCalls = append(s.setHashCalls, stubSetHashCall{key, hash})
	return nil
}

// Compile-time check that stubStore satisfies datasets.Store.
var _ datasets.Store = (*stubStore)(nil)

// TestHandle_WithOverrideSets verifies that override sets bypass the DB.
// Uses a single explicit format so the assertion counts are deterministic.
func TestHandle_WithOverrideSets(t *testing.T) {
	fetcher := &stubFetcher{
		cards: []seventeenlands.CardRating{{Name: "Lightning Bolt", ALSA: 1.5}},
	}
	store := &stubStore{}

	h := handler.NewWithFormats(fetcher, store, []string{"FDN", "BLB"}, []string{"PremierDraft"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	// 2 sets x 1 format = 2 card-rating calls.
	assert.Equal(t, 2, fetcher.called)
	require.Len(t, store.upserted, 2)
	setCodes := map[string]bool{}
	for _, u := range store.upserted {
		setCodes[u.SetCode] = true
	}
	assert.True(t, setCodes["FDN"])
	assert.True(t, setCodes["BLB"])
}

// TestHandle_WithDBSets verifies that active sets are read from the store when
// no override is provided.
func TestHandle_WithDBSets(t *testing.T) {
	fetcher := &stubFetcher{
		cards: []seventeenlands.CardRating{{Name: "Forest", ALSA: 9.0}},
	}
	store := &stubStore{dbSets: []string{"DSK"}}

	h := handler.NewWithFormats(fetcher, store, nil, []string{"PremierDraft"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	// 1 set x 1 format = 1 call.
	assert.Equal(t, 1, fetcher.called)
	require.Len(t, store.upserted, 1)
	for _, u := range store.upserted {
		assert.Equal(t, "DSK", u.SetCode)
	}
}

// TestHandle_NoSets verifies that an empty active-sets list is a no-op (no error).
func TestHandle_NoSets(t *testing.T) {
	fetcher := &stubFetcher{}
	store := &stubStore{} // empty dbSets

	h := handler.New(fetcher, store, nil)
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, 0, fetcher.called)
	assert.Empty(t, store.upserted)
}

// TestHandle_GetActiveSetsError verifies that a DB error is propagated.
func TestHandle_GetActiveSetsError(t *testing.T) {
	fetcher := &stubFetcher{}
	store := &stubStore{dbErr: errors.New("connection refused")}

	h := handler.New(fetcher, store, nil)
	err := h.Handle(context.Background(), nil)

	require.Error(t, err)
	assert.Equal(t, 0, fetcher.called)
}

// TestHandle_FetchErrorContinues verifies that a fetch failure for one set does not
// abort the remaining sets.
func TestHandle_FetchErrorContinues(t *testing.T) {
	// Return an error only for SET1; SET2 returns valid cards.
	custom := &countingFetcher{
		results: map[string]fetchResult{
			"SET1": {err: errors.New("upstream timeout")},
			"SET2": {cards: []seventeenlands.CardRating{{Name: "Island", ALSA: 8.0}}},
		},
	}

	store := &stubStore{}
	// Use a single format so call counts are deterministic: 2 sets x 1 format = 2 calls.
	h := handler.NewWithFormats(custom, store, []string{"SET1", "SET2"}, []string{"PremierDraft"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	// 2 sets x 1 format = 2 card-fetch calls.
	assert.Equal(t, 2, custom.called)
	// Only SET2 should have been upserted (once -- single format).
	require.Len(t, store.upserted, 1)
	for _, u := range store.upserted {
		assert.Equal(t, "SET2", u.SetCode)
	}
}

// TestHandle_EmptyCardsSkipsUpsert verifies that a 0-card response does not call UpsertRatings.
func TestHandle_EmptyCardsSkipsUpsert(t *testing.T) {
	fetcher := &stubFetcher{cards: []seventeenlands.CardRating{}} // 0 cards
	store := &stubStore{}

	h := handler.NewWithFormats(fetcher, store, []string{"FDN"}, []string{"PremierDraft"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, 1, fetcher.called) // 1 set x 1 format
	assert.Empty(t, store.upserted)
}

// TestHandle_UpsertErrorContinues verifies that a upsert failure for one set does
// not abort the remaining sets.
func TestHandle_UpsertErrorContinues(t *testing.T) {
	fetcher := &stubFetcher{
		cards: []seventeenlands.CardRating{{Name: "Swamp", ALSA: 9.0}},
	}
	upsertCalls := 0
	var upserted []draftdata.SetRatings
	store := &stubStore{
		upsertFn: func(setCode string) error {
			upsertCalls++
			if setCode == "SET1" {
				return errors.New("write failed")
			}
			upserted = append(upserted, draftdata.SetRatings{SetCode: setCode})
			return nil
		},
	}

	// Single format so upsert count is deterministic: 2 sets x 1 format = 2 upsert calls.
	h := handler.NewWithFormats(fetcher, store, []string{"SET1", "SET2"}, []string{"PremierDraft"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	// 2 sets x 1 format = 2 upsert attempts.
	assert.Equal(t, 2, upsertCalls)
	// Only SET2 row succeeds (1 format).
	require.Len(t, upserted, 1)
	for _, u := range upserted {
		assert.Equal(t, "SET2", u.SetCode)
	}
}

// TestHandle_ContextCancelled verifies early exit when context is cancelled.
func TestHandle_ContextCancelled(t *testing.T) {
	fetcher := &stubFetcher{
		cards: []seventeenlands.CardRating{{Name: "Mountain", ALSA: 9.0}},
	}
	store := &stubStore{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	h := handler.NewWithFormats(fetcher, store, []string{"SET1", "SET2"}, []string{"PremierDraft"})
	err := h.Handle(ctx, nil)

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 0, fetcher.called)
}

// TestHandle_ColorRatingsFetchedAndStored verifies that color ratings are fetched
// and persisted after card ratings for each set/format combination.
func TestHandle_ColorRatingsFetchedAndStored(t *testing.T) {
	fetcher := &stubFetcher{
		cards: []seventeenlands.CardRating{{Name: "Lightning Bolt", ALSA: 1.5}},
		colors: []seventeenlands.ColorRating{
			{ColorCombination: "WU", WinRate: 0.58, GamesPlayed: 5000},
		},
	}
	store := &stubStore{}

	h := handler.New(fetcher, store, []string{"FDN"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	// Card ratings: 1 set x 2 formats = 2 fetches.
	assert.Equal(t, 2, fetcher.called)
	// Color ratings: 1 set x 2 formats = 2 fetches.
	assert.Equal(t, 2, fetcher.colorsCalled)
	// Both persisted.
	assert.Len(t, store.upserted, 2)
	assert.Len(t, store.upsertedColorRatings, 2)
	for _, cr := range store.upsertedColorRatings {
		assert.Equal(t, "FDN", cr.setCode)
		require.Len(t, cr.ratings, 1)
		assert.Equal(t, "WU", cr.ratings[0].ColorCombination)
	}
}

// TestHandle_ColorRatingsEmptySkipsUpsert verifies that when the fetcher returns
// no color ratings, UpsertColorRatings is not called.
func TestHandle_ColorRatingsEmptySkipsUpsert(t *testing.T) {
	fetcher := &stubFetcher{
		cards:  []seventeenlands.CardRating{{Name: "Island", ALSA: 8.0}},
		colors: nil, // no color data
	}
	store := &stubStore{}

	h := handler.New(fetcher, store, []string{"DSK"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, 2, fetcher.called)          // card ratings fetched
	assert.Equal(t, 2, fetcher.colorsCalled)    // color ratings attempted
	assert.Empty(t, store.upsertedColorRatings) // but not stored
}

// TestHandle_FetchedAtIsNonZero verifies that the SetRatings passed to UpsertRatings
// always has a non-zero FetchedAt, so cached_at is stored correctly in Postgres.
// A zero FetchedAt would result in cached_at = 0001-01-01, making the BFF staleness
// check always fire X-Cache-Degraded: true.
func TestHandle_FetchedAtIsNonZero(t *testing.T) {
	before := time.Now().UTC()

	fetcher := &stubFetcher{
		cards: []seventeenlands.CardRating{{Name: "Plains", MtgaID: 1, ALSA: 9.0}},
	}
	store := &stubStore{}

	h := handler.NewWithFormats(fetcher, store, []string{"BLB"}, []string{"PremierDraft"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	require.Len(t, store.upserted, 1)

	sr := store.upserted[0]
	assert.False(t, sr.FetchedAt.IsZero(), "FetchedAt must not be zero -- cached_at would be 0001-01-01 in Postgres")
	assert.True(t, sr.FetchedAt.After(before) || sr.FetchedAt.Equal(before),
		"FetchedAt should be >= time before Handle was called")
}

// TestHandle_MultiFormat_AllFormatsPerSet verifies that each (set, format) pair is
// fetched independently -- the core behaviour introduced by #1123.
func TestHandle_MultiFormat_AllFormatsPerSet(t *testing.T) {
	fetcher := &formatTrackingFetcher{
		cards: []seventeenlands.CardRating{{Name: "Lightning Bolt", ALSA: 1.5}},
	}
	store := &stubStore{}

	formats := []string{"PremierDraft", "QuickDraft"}
	h := handler.NewWithFormats(fetcher, store, []string{"FDN"}, formats)
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	// 1 set x 2 formats = 2 fetch calls.
	assert.Equal(t, 2, fetcher.called)
	// Both formats should be persisted.
	require.Len(t, store.upserted, 2)
	gotFormats := []string{store.upserted[0].DraftFormat, store.upserted[1].DraftFormat}
	assert.ElementsMatch(t, formats, gotFormats)
}

// TestHandle_MultiFormat_MultipleSetsCrossFormats verifies that every (set, format)
// combination is fetched when there are multiple sets and formats.
func TestHandle_MultiFormat_MultipleSetsCrossFormats(t *testing.T) {
	fetcher := &formatTrackingFetcher{
		cards: []seventeenlands.CardRating{{Name: "Island", ALSA: 7.5}},
	}
	store := &stubStore{}

	sets := []string{"FDN", "BLB", "DSK"}
	formats := []string{"PremierDraft", "QuickDraft"}
	h := handler.NewWithFormats(fetcher, store, sets, formats)
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	// 3 sets x 2 formats = 6 fetch calls.
	assert.Equal(t, 6, fetcher.called)
	require.Len(t, store.upserted, 6)

	// Verify every (set, format) pair appears exactly once.
	type pair struct{ set, format string }
	got := make(map[pair]bool)
	for _, sr := range store.upserted {
		got[pair{sr.SetCode, sr.DraftFormat}] = true
	}
	for _, s := range sets {
		for _, f := range formats {
			assert.True(t, got[pair{s, f}], "expected upsert for set=%s format=%s", s, f)
		}
	}
}

// TestHandle_MultiFormat_DraftFormatStoredCorrectly verifies that the DraftFormat
// field in SetRatings matches the format that was requested from 17Lands.
func TestHandle_MultiFormat_DraftFormatStoredCorrectly(t *testing.T) {
	fetcher := &formatTrackingFetcher{
		cards: []seventeenlands.CardRating{{Name: "Forest", ALSA: 9.0}},
	}
	store := &stubStore{}

	h := handler.NewWithFormats(fetcher, store, []string{"FDN"}, []string{"QuickDraft"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	require.Len(t, store.upserted, 1)
	assert.Equal(t, "FDN", store.upserted[0].SetCode)
	assert.Equal(t, "QuickDraft", store.upserted[0].DraftFormat)
}

// TestHandle_SyncFormatsEnvVar verifies that SYNC_FORMATS overrides the default
// format list when handler.New is used (not NewWithFormats).
func TestHandle_SyncFormatsEnvVar(t *testing.T) {
	t.Setenv("SYNC_FORMATS", "PremierDraft,QuickDraft,Sealed")

	fetcher := &formatTrackingFetcher{
		cards: []seventeenlands.CardRating{{Name: "Swamp", ALSA: 8.0}},
	}
	store := &stubStore{}

	h := handler.New(fetcher, store, []string{"FDN"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	// 1 set x 3 formats (from env) = 3 fetch calls.
	assert.Equal(t, 3, fetcher.called)
	require.Len(t, store.upserted, 3)
	gotFormats := make([]string, len(store.upserted))
	for i, sr := range store.upserted {
		gotFormats[i] = sr.DraftFormat
	}
	assert.ElementsMatch(t, []string{"PremierDraft", "QuickDraft", "Sealed"}, gotFormats)
}

// --- hash delta-skip tests ---

// TestHandle_HashMatch_SkipSync verifies that when the stored hash matches the
// computed hash of fetched card ratings, the upsert is skipped and SetHash is
// NOT called again.
func TestHandle_HashMatch_SkipSync(t *testing.T) {
	cards := []seventeenlands.CardRating{{Name: "Lightning Bolt", ALSA: 1.5}}
	rawBytes, err := json.Marshal(cards)
	require.NoError(t, err)
	existingHash := fmt.Sprintf("%x", sha256.Sum256(rawBytes))

	fetcher := &stubFetcher{cards: cards}
	store := &stubStore{
		storedHashes: []stubSetHashCall{
			{key: "FDN/PremierDraft", hash: existingHash},
		},
	}

	h := handler.NewWithFormats(fetcher, store, []string{"FDN"}, []string{"PremierDraft"})
	err = h.Handle(context.Background(), nil)

	require.NoError(t, err)
	// Fetch still happens (we need the data to compute the hash).
	assert.Equal(t, 1, fetcher.called)
	// Upsert must be skipped.
	assert.Empty(t, store.upserted, "UpsertRatings must not be called when hash matches")
	// SetHash must NOT be called.
	assert.Empty(t, store.setHashCalls, "SetHash must not be called when hash matches")
}

// TestHandle_HashDifferent_SyncProceeds verifies that when the stored hash differs
// from the newly computed hash, the sync proceeds and SetHash IS called with the
// new hash.
func TestHandle_HashDifferent_SyncProceeds(t *testing.T) {
	cards := []seventeenlands.CardRating{{Name: "Island", ALSA: 8.0}}
	rawBytes, err := json.Marshal(cards)
	require.NoError(t, err)
	newHash := fmt.Sprintf("%x", sha256.Sum256(rawBytes))

	fetcher := &stubFetcher{cards: cards}
	store := &stubStore{
		storedHashes: []stubSetHashCall{
			{key: "FDN/PremierDraft", hash: "stale-hash-from-previous-run"},
		},
	}

	h := handler.NewWithFormats(fetcher, store, []string{"FDN"}, []string{"PremierDraft"})
	err = h.Handle(context.Background(), nil)

	require.NoError(t, err)
	// Sync must proceed.
	require.Len(t, store.upserted, 1, "UpsertRatings must be called when hash differs")
	// SetHash must be called with the new hash.
	require.Len(t, store.setHashCalls, 1, "SetHash must be called after successful upsert")
	assert.Equal(t, "FDN/PremierDraft", store.setHashCalls[0].key)
	assert.Equal(t, newHash, store.setHashCalls[0].hash)
}

// TestHandle_NoStoredHash_SyncProceeds verifies that on first run (empty stored hash)
// the sync proceeds and SetHash is called to seed the initial hash.
func TestHandle_NoStoredHash_SyncProceeds(t *testing.T) {
	cards := []seventeenlands.CardRating{{Name: "Plains", ALSA: 9.0}}
	rawBytes, err := json.Marshal(cards)
	require.NoError(t, err)
	expectedHash := fmt.Sprintf("%x", sha256.Sum256(rawBytes))

	fetcher := &stubFetcher{cards: cards}
	store := &stubStore{} // no storedHashes -- simulates first run

	h := handler.NewWithFormats(fetcher, store, []string{"BLB"}, []string{"QuickDraft"})
	err = h.Handle(context.Background(), nil)

	require.NoError(t, err)
	// Sync must proceed.
	require.Len(t, store.upserted, 1, "UpsertRatings must be called on first run")
	// SetHash must seed the initial hash.
	require.Len(t, store.setHashCalls, 1, "SetHash must be called to seed hash on first run")
	assert.Equal(t, "BLB/QuickDraft", store.setHashCalls[0].key)
	assert.Equal(t, expectedHash, store.setHashCalls[0].hash)
}

// TestHandle_HashDeterministicAcrossOrdering verifies that two identical card sets
// in different order produce the same stored hash. This is AC #2 of #1100: the hash
// must be computed on a sorted (by MtgaID ascending) payload so that API response
// ordering does not affect whether a sync is skipped.
func TestHandle_HashDeterministicAcrossOrdering(t *testing.T) {
	// Two cards defined in ascending MtgaID order.
	cardA := seventeenlands.CardRating{MtgaID: 100, Name: "Lightning Bolt", ALSA: 1.5}
	cardB := seventeenlands.CardRating{MtgaID: 200, Name: "Island", ALSA: 8.0}

	// First run: cards arrive in ascending order.
	fetcherAsc := &stubFetcher{cards: []seventeenlands.CardRating{cardA, cardB}}
	storeAsc := &stubStore{}
	h1 := handler.NewWithFormats(fetcherAsc, storeAsc, []string{"FDN"}, []string{"PremierDraft"})
	require.NoError(t, h1.Handle(context.Background(), nil))
	require.Len(t, storeAsc.setHashCalls, 1, "SetHash must be called on first run")
	hashAsc := storeAsc.setHashCalls[0].hash

	// Second run: same cards but in descending order (reversed).
	fetcherDesc := &stubFetcher{cards: []seventeenlands.CardRating{cardB, cardA}}
	storeDesc := &stubStore{}
	h2 := handler.NewWithFormats(fetcherDesc, storeDesc, []string{"FDN"}, []string{"PremierDraft"})
	require.NoError(t, h2.Handle(context.Background(), nil))
	require.Len(t, storeDesc.setHashCalls, 1, "SetHash must be called on first run")
	hashDesc := storeDesc.setHashCalls[0].hash

	assert.Equal(t, hashAsc, hashDesc,
		"hash must be identical regardless of API response ordering (sorted by MtgaID before hashing)")
}

// --- helpers ---

// formatTrackingFetcher records the (setCode, format) pairs it was called with.
type formatTrackingFetcher struct {
	called int
	calls  []struct{ setCode, format string }
	cards  []seventeenlands.CardRating
}

func (f *formatTrackingFetcher) FetchCardRatings(_ context.Context, setCode, format string) ([]seventeenlands.CardRating, error) {
	f.called++
	f.calls = append(f.calls, struct{ setCode, format string }{setCode, format})
	return f.cards, nil
}

func (f *formatTrackingFetcher) FetchColorRatings(_ context.Context, _, _ string) ([]seventeenlands.ColorRating, error) {
	return nil, nil
}

type fetchResult struct {
	cards []seventeenlands.CardRating
	err   error
}

type countingFetcher struct {
	called  int
	results map[string]fetchResult
}

func (c *countingFetcher) FetchCardRatings(_ context.Context, setCode, _ string) ([]seventeenlands.CardRating, error) {
	c.called++
	if r, ok := c.results[setCode]; ok {
		return r.cards, r.err
	}
	return nil, nil
}

func (c *countingFetcher) FetchColorRatings(_ context.Context, _, _ string) ([]seventeenlands.ColorRating, error) {
	return nil, nil
}
