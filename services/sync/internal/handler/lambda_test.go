package handler_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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

	// Hash control fields.
	// storedHashes maps hash key -> hash value returned by GetHash.
	// setHashCalls records every (key, hash) pair passed to SetHash.
	storedHashes map[string]string
	getHashErr   error
	setHashErr   error
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
	if s.getHashErr != nil {
		return "", s.getHashErr
	}
	if s.storedHashes != nil {
		return s.storedHashes[key], nil
	}
	return "", nil
}

func (s *stubStore) SetHash(_ context.Context, key, hash string) error {
	s.setHashCalls = append(s.setHashCalls, stubSetHashCall{key, hash})
	return s.setHashErr
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

// --- hash-delta-skip helpers ---

// computeExpectedHash mirrors the production computeRatingsHash so tests can
// build the exact hash value the handler will produce for a given slice.
func computeExpectedHash(ratings []seventeenlands.CardRating) string {
	sorted := make([]seventeenlands.CardRating, len(ratings))
	copy(sorted, ratings)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].MtgaID < sorted[j].MtgaID
	})

	b, _ := json.Marshal(sorted)
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum)
}

// --- hash-delta-skip tests (ADR-005, #1100) ---

// TestHandle_HashMatch_SkipsUpsert verifies that when the stored hash equals the
// computed hash of the fetched payload, UpsertRatings is not called.
func TestHandle_HashMatch_SkipsUpsert(t *testing.T) {
	cards := []seventeenlands.CardRating{
		{MtgaID: 101, Name: "Lightning Bolt", ALSA: 1.5},
		{MtgaID: 202, Name: "Plains", ALSA: 9.0},
	}
	existingHash := computeExpectedHash(cards)

	fetcher := &stubFetcher{cards: cards}
	store := &stubStore{
		storedHashes: map[string]string{
			"FDN/PremierDraft": existingHash,
		},
	}

	h := handler.NewWithFormats(fetcher, store, []string{"FDN"}, []string{"PremierDraft"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	// Fetch was called (always fetch to compute the hash).
	assert.Equal(t, 1, fetcher.called)
	// Upsert must NOT have been called -- payload is unchanged.
	assert.Empty(t, store.upserted, "UpsertRatings must be skipped when hash matches")
	// SetHash must NOT have been called -- nothing changed.
	assert.Empty(t, store.setHashCalls, "SetHash must not be called when hash matches")
}

// TestHandle_HashMismatch_Upserts verifies that when the stored hash differs from
// the computed hash, UpsertRatings is called and the new hash is persisted via SetHash.
func TestHandle_HashMismatch_Upserts(t *testing.T) {
	cards := []seventeenlands.CardRating{
		{MtgaID: 101, Name: "Lightning Bolt", ALSA: 1.5},
	}

	fetcher := &stubFetcher{cards: cards}
	store := &stubStore{
		storedHashes: map[string]string{
			"FDN/PremierDraft": "stale-hash-from-previous-run",
		},
	}

	h := handler.NewWithFormats(fetcher, store, []string{"FDN"}, []string{"PremierDraft"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	// Upsert must have been called -- hash differs.
	require.Len(t, store.upserted, 1, "UpsertRatings must be called when hash differs")
	assert.Equal(t, "FDN", store.upserted[0].SetCode)
	// SetHash must have been called with the new hash.
	require.Len(t, store.setHashCalls, 1, "SetHash must be called once after successful upsert")
	assert.Equal(t, "FDN/PremierDraft", store.setHashCalls[0].key)
	assert.Equal(t, computeExpectedHash(cards), store.setHashCalls[0].hash)
}

// TestHandle_FirstRun_NoPriorHash_Upserts verifies that when no hash has been
// stored (GetHash returns ""), the handler proceeds with the upsert and stores
// the new hash.
func TestHandle_FirstRun_NoPriorHash_Upserts(t *testing.T) {
	cards := []seventeenlands.CardRating{
		{MtgaID: 303, Name: "Island", ALSA: 8.0},
	}

	fetcher := &stubFetcher{cards: cards}
	store := &stubStore{} // storedHashes is nil -- GetHash returns "" for any key

	h := handler.NewWithFormats(fetcher, store, []string{"BLB"}, []string{"PremierDraft"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	// Upsert must have proceeded -- no prior hash.
	require.Len(t, store.upserted, 1, "UpsertRatings must be called on first run (no prior hash)")
	assert.Equal(t, "BLB", store.upserted[0].SetCode)
	// SetHash must have been called to store the new hash.
	require.Len(t, store.setHashCalls, 1, "SetHash must be called after first-run upsert")
	assert.Equal(t, "BLB/PremierDraft", store.setHashCalls[0].key)
	assert.Equal(t, computeExpectedHash(cards), store.setHashCalls[0].hash)
}

// TestHandle_HashSortOrder_Deterministic verifies that reordering the same cards
// in the fetched slice produces the same hash (ordering differences in the 17Lands
// response must not cause unnecessary upserts).
func TestHandle_HashSortOrder_Deterministic(t *testing.T) {
	// Cards provided in unsorted order (MtgaID 202 before 101).
	cards := []seventeenlands.CardRating{
		{MtgaID: 202, Name: "Plains", ALSA: 9.0},
		{MtgaID: 101, Name: "Lightning Bolt", ALSA: 1.5},
	}
	// computeExpectedHash always sorts, so it produces the canonical hash.
	existingHash := computeExpectedHash(cards)

	fetcher := &stubFetcher{cards: cards}
	store := &stubStore{
		storedHashes: map[string]string{
			"DSK/PremierDraft": existingHash,
		},
	}

	h := handler.NewWithFormats(fetcher, store, []string{"DSK"}, []string{"PremierDraft"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	// Hash must match regardless of original slice order -- upsert must be skipped.
	assert.Empty(t, store.upserted, "upsert must be skipped when sorted hash matches")
}

// TestHandle_GetHashError_FallsThrough verifies that a GetHash error is non-fatal:
// the handler logs and falls through to perform the upsert rather than skipping it.
func TestHandle_GetHashError_FallsThrough(t *testing.T) {
	cards := []seventeenlands.CardRating{
		{MtgaID: 404, Name: "Mountain", ALSA: 9.0},
	}

	fetcher := &stubFetcher{cards: cards}
	store := &stubStore{
		getHashErr: errors.New("db connection error"),
	}

	h := handler.NewWithFormats(fetcher, store, []string{"FDN"}, []string{"PremierDraft"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	// GetHash failed but upsert must still proceed (non-fatal).
	require.Len(t, store.upserted, 1, "upsert must proceed when GetHash errors")
}

// TestHandle_SetHashError_NonFatal verifies that a SetHash error after a successful
// upsert does not cause Handle to return an error -- the upsert data is preserved
// and the next invocation will simply re-upsert.
func TestHandle_SetHashError_NonFatal(t *testing.T) {
	cards := []seventeenlands.CardRating{
		{MtgaID: 505, Name: "Swamp", ALSA: 9.0},
	}

	fetcher := &stubFetcher{cards: cards}
	store := &stubStore{
		setHashErr: errors.New("hash write failed"),
	}

	h := handler.NewWithFormats(fetcher, store, []string{"BLB"}, []string{"PremierDraft"})
	err := h.Handle(context.Background(), nil)

	// SetHash failed but Handle must still return nil -- data is preserved.
	require.NoError(t, err, "SetHash failure must not bubble up as a fatal error")
	// Upsert must have succeeded.
	require.Len(t, store.upserted, 1, "UpsertRatings must succeed even if SetHash fails")
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
