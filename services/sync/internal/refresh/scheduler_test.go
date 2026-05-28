package refresh_test

import (
	"context"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/sync/internal/datasets"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/draftdata"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/refresh"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/scryfall"
	"github.com/RdHamilton/vault-mtg/services/sync/internal/seventeenlands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubFetcher returns a fixed set of card and color ratings.
type stubFetcher struct {
	called       int
	cards        []seventeenlands.CardRating
	colorsCalled int
	colors       []seventeenlands.ColorRating
}

func (f *stubFetcher) FetchCardRatings(_ context.Context, _, _ string) ([]seventeenlands.CardRating, error) {
	f.called++
	return f.cards, nil
}

func (f *stubFetcher) FetchColorRatings(_ context.Context, _, _, _, _ string) ([]seventeenlands.ColorRating, error) {
	f.colorsCalled++
	return f.colors, nil
}

// stubSetFetcher returns a fixed list of Scryfall sets.
type stubSetFetcher struct {
	sets []scryfall.ScryfallSet
}

func (f *stubSetFetcher) FetchSets(_ context.Context) ([]scryfall.ScryfallSet, error) {
	return f.sets, nil
}

// Ensure stubSetFetcher satisfies the SetFetcher interface.
var _ refresh.SetFetcher = (*stubSetFetcher)(nil)

// stubStore records upserted ratings and returns a configurable set list from the DB.
type stubStore struct {
	dbSets               []datasets.SyncSet
	upserted             []draftdata.SetRatings
	upsertedSets         []scryfall.ScryfallSet
	upsertedColorRatings []stubColorUpsert
}

type stubColorUpsert struct {
	setCode     string
	draftFormat string
	ratings     []seventeenlands.ColorRating
}

func (s *stubStore) GetActiveSets(_ context.Context) ([]datasets.SyncSet, error) {
	return s.dbSets, nil
}

func (s *stubStore) UpsertRatings(_ context.Context, r draftdata.SetRatings) error {
	s.upserted = append(s.upserted, r)
	return nil
}

func (s *stubStore) GetRatings(_ context.Context, _, _ string) (*draftdata.SetRatings, error) {
	return nil, nil
}

func (s *stubStore) UpsertSets(_ context.Context, sets []scryfall.ScryfallSet) error {
	s.upsertedSets = append(s.upsertedSets, sets...)
	return nil
}

func (s *stubStore) UpsertColorRatings(_ context.Context, setCode, draftFormat string, ratings []seventeenlands.ColorRating) error {
	s.upsertedColorRatings = append(s.upsertedColorRatings, stubColorUpsert{setCode, draftFormat, ratings})
	return nil
}

func (s *stubStore) GetHash(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (s *stubStore) SetHash(_ context.Context, _ string, _ string) error {
	return nil
}

// Ensure stubStore satisfies the Store interface.
var _ datasets.Store = (*stubStore)(nil)

func TestScheduler_ImmediateFetchOnStart(t *testing.T) {
	t.Setenv("SYNC_ACTIVE_SETS", "FDN")
	t.Setenv("SYNC_REFRESH_HOUR", "2")
	// Pin to a single format so upserted count is deterministic.
	t.Setenv("SYNC_FORMATS", "PremierDraft")

	fetcher := &stubFetcher{
		cards: []seventeenlands.CardRating{
			{Name: "Lightning Bolt", ALSA: 1.5},
		},
	}
	store := &stubStore{}
	setFetcher := &stubSetFetcher{}

	sched := refresh.New(setFetcher, fetcher, store)

	// Cancel immediately after startup — the scheduler should have run one fetch.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	sched.Start(ctx)

	require.GreaterOrEqual(t, fetcher.called, 1, "expected at least one fetch on startup")
	require.Len(t, store.upserted, 1)
	assert.Equal(t, "FDN", store.upserted[0].SetCode)
	assert.Len(t, store.upserted[0].Cards, 1)
}

func TestScheduler_FetchesFromDB_WhenNoOverride(t *testing.T) {
	t.Setenv("SYNC_ACTIVE_SETS", "") // no override — should fall through to DB
	t.Setenv("SYNC_REFRESH_HOUR", "2")
	// Pin to a single format so call count is deterministic: 2 sets × 1 format = 2.
	t.Setenv("SYNC_FORMATS", "PremierDraft")

	fetcher := &stubFetcher{
		cards: []seventeenlands.CardRating{{Name: "Forest", ALSA: 9.0}},
	}
	store := &stubStore{dbSets: []datasets.SyncSet{{Code: "BLB", ExpansionCode: "BLB"}, {Code: "DSK", ExpansionCode: "DSK"}}}
	setFetcher := &stubSetFetcher{}

	sched := refresh.New(setFetcher, fetcher, store)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	sched.Start(ctx)

	require.Equal(t, 2, fetcher.called, "expected one fetch per active set from DB")
	require.Len(t, store.upserted, 2)
	codes := []string{store.upserted[0].SetCode, store.upserted[1].SetCode}
	assert.ElementsMatch(t, []string{"BLB", "DSK"}, codes)
}

func TestScheduler_NoSetsSkipsFetch(t *testing.T) {
	t.Setenv("SYNC_ACTIVE_SETS", "")

	fetcher := &stubFetcher{}
	store := &stubStore{} // empty dbSets
	setFetcher := &stubSetFetcher{}

	sched := refresh.New(setFetcher, fetcher, store)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	sched.Start(ctx)

	assert.Equal(t, 0, fetcher.called)
	assert.Empty(t, store.upserted)
}

func TestScheduler_ScryfallSetsUpsertedBeforeRatings(t *testing.T) {
	t.Setenv("SYNC_ACTIVE_SETS", "FDN")
	t.Setenv("SYNC_REFRESH_HOUR", "2")
	t.Setenv("SYNC_FORMATS", "PremierDraft")

	scryfallSet := scryfall.ScryfallSet{Code: "fdn", Name: "Foundations", SetType: "expansion", Digital: true, CardCount: 276}
	fetcher := &stubFetcher{
		cards: []seventeenlands.CardRating{{Name: "Lightning Bolt", ALSA: 1.5}},
	}
	store := &stubStore{}
	setFetcher := &stubSetFetcher{sets: []scryfall.ScryfallSet{scryfallSet}}

	sched := refresh.New(setFetcher, fetcher, store)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	sched.Start(ctx)

	require.Len(t, store.upsertedSets, 1, "Scryfall sets must be upserted")
	assert.Equal(t, "fdn", store.upsertedSets[0].Code)
	require.GreaterOrEqual(t, fetcher.called, 1, "card ratings must be fetched after set sync")
}

// TestScheduler_MultiFormat_FetchesAllFormatsPerSet verifies that the scheduler
// iterates over every (set, format) combination — the core fix for #1123.
func TestScheduler_MultiFormat_FetchesAllFormatsPerSet(t *testing.T) {
	t.Setenv("SYNC_ACTIVE_SETS", "FDN,BLB")
	t.Setenv("SYNC_FORMATS", "PremierDraft,QuickDraft")

	fetcher := &stubFetcher{
		cards: []seventeenlands.CardRating{{Name: "Forest", ALSA: 9.0}},
	}
	store := &stubStore{}
	setFetcher := &stubSetFetcher{}

	sched := refresh.New(setFetcher, fetcher, store)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	sched.Start(ctx)

	// 2 sets × 2 formats = 4 fetch calls.
	require.Equal(t, 4, fetcher.called)
	require.Len(t, store.upserted, 4)

	type pair struct{ set, format string }
	got := make(map[pair]bool)
	for _, sr := range store.upserted {
		got[pair{sr.SetCode, sr.DraftFormat}] = true
	}
	for _, s := range []string{"FDN", "BLB"} {
		for _, f := range []string{"PremierDraft", "QuickDraft"} {
			assert.True(t, got[pair{s, f}], "expected upsert for set=%s format=%s", s, f)
		}
	}
}

// TestScheduler_DefaultFormats verifies that when SYNC_FORMATS is unset the
// scheduler defaults to PremierDraft and QuickDraft (two formats).
func TestScheduler_DefaultFormats(t *testing.T) {
	t.Setenv("SYNC_ACTIVE_SETS", "FDN")
	t.Setenv("SYNC_FORMATS", "") // unset — use defaults

	fetcher := &stubFetcher{
		cards: []seventeenlands.CardRating{{Name: "Mountain", ALSA: 2.0}},
	}
	store := &stubStore{}
	setFetcher := &stubSetFetcher{}

	sched := refresh.New(setFetcher, fetcher, store)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	sched.Start(ctx)

	// 1 set × 2 default formats (PremierDraft, QuickDraft) = 2 calls.
	require.Equal(t, 2, fetcher.called)
	require.Len(t, store.upserted, 2)
	gotFormats := []string{store.upserted[0].DraftFormat, store.upserted[1].DraftFormat}
	assert.ElementsMatch(t, []string{"PremierDraft", "QuickDraft"}, gotFormats)
}

// TestScheduler_ColorRatingsFetchedAfterCardRatings verifies that color ratings
// are fetched and persisted immediately after card ratings for each set/format.
func TestScheduler_ColorRatingsFetchedAfterCardRatings(t *testing.T) {
	t.Setenv("SYNC_ACTIVE_SETS", "FDN")
	t.Setenv("SYNC_FORMATS", "PremierDraft")

	fetcher := &stubFetcher{
		cards: []seventeenlands.CardRating{{Name: "Lightning Bolt", ALSA: 1.5}},
		colors: []seventeenlands.ColorRating{
			{ShortName: "WU", ColorName: "Azorius", Wins: 2900, Games: 5000, IsSummary: false},
			{ShortName: "BG", ColorName: "Golgari", Wins: 1664, Games: 3200, IsSummary: false},
		},
	}
	store := &stubStore{}
	setFetcher := &stubSetFetcher{}

	sched := refresh.New(setFetcher, fetcher, store)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	sched.Start(ctx)

	// Card ratings fetched once.
	require.Equal(t, 1, fetcher.called, "card ratings must be fetched")
	// Color ratings fetched once.
	require.Equal(t, 1, fetcher.colorsCalled, "color ratings must be fetched")

	// Card ratings stored.
	require.Len(t, store.upserted, 1)
	assert.Equal(t, "FDN", store.upserted[0].SetCode)

	// Color ratings stored.
	require.Len(t, store.upsertedColorRatings, 1)
	cr := store.upsertedColorRatings[0]
	assert.Equal(t, "FDN", cr.setCode)
	assert.Equal(t, "PremierDraft", cr.draftFormat)
	require.Len(t, cr.ratings, 2)
	assert.Equal(t, "WU", cr.ratings[0].ShortName)
	assert.InDelta(t, 0.58, cr.ratings[0].WinRate(), 0.001)
}

// TestScheduler_ColorRatingsSkippedWhenEmpty verifies that when the fetcher
// returns no color ratings (e.g. 17Lands has no data for that set/format),
// UpsertColorRatings is not called.
func TestScheduler_ColorRatingsSkippedWhenEmpty(t *testing.T) {
	t.Setenv("SYNC_ACTIVE_SETS", "DSK")
	t.Setenv("SYNC_FORMATS", "PremierDraft")

	fetcher := &stubFetcher{
		cards:  []seventeenlands.CardRating{{Name: "Swamp", ALSA: 9.0}},
		colors: nil, // no color data
	}
	store := &stubStore{}
	setFetcher := &stubSetFetcher{}

	sched := refresh.New(setFetcher, fetcher, store)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	sched.Start(ctx)

	require.Equal(t, 1, fetcher.called)
	require.Equal(t, 1, fetcher.colorsCalled, "color fetch is attempted even when result is empty")
	assert.Empty(t, store.upsertedColorRatings, "UpsertColorRatings must not be called when no data returned")
}
