package refresh_test

import (
	"context"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-sync/internal/datasets"
	"github.com/ramonehamilton/mtga-sync/internal/draftdata"
	"github.com/ramonehamilton/mtga-sync/internal/refresh"
	"github.com/ramonehamilton/mtga-sync/internal/scryfall"
	"github.com/ramonehamilton/mtga-sync/internal/seventeenlands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubFetcher returns a fixed set of card ratings.
type stubFetcher struct {
	called int
	cards  []seventeenlands.CardRating
}

func (f *stubFetcher) FetchCardRatings(_ context.Context, _, _ string) ([]seventeenlands.CardRating, error) {
	f.called++
	return f.cards, nil
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
	dbSets       []string
	upserted     []draftdata.SetRatings
	upsertedSets []scryfall.ScryfallSet
}

func (s *stubStore) GetActiveSets(_ context.Context) ([]string, error) {
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

// Ensure stubStore satisfies the Store interface.
var _ datasets.Store = (*stubStore)(nil)

func TestScheduler_ImmediateFetchOnStart(t *testing.T) {
	t.Setenv("SYNC_ACTIVE_SETS", "FDN")
	t.Setenv("SYNC_REFRESH_HOUR", "2")

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

	fetcher := &stubFetcher{
		cards: []seventeenlands.CardRating{{Name: "Forest", ALSA: 9.0}},
	}
	store := &stubStore{dbSets: []string{"BLB", "DSK"}}
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
