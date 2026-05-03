package refresh_test

import (
	"context"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-sync/internal/datasets"
	"github.com/ramonehamilton/mtga-sync/internal/draftdata"
	"github.com/ramonehamilton/mtga-sync/internal/refresh"
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

// stubStore records upserted ratings.
type stubStore struct {
	upserted []draftdata.SetRatings
}

func (s *stubStore) UpsertRatings(_ context.Context, r draftdata.SetRatings) error {
	s.upserted = append(s.upserted, r)
	return nil
}

func (s *stubStore) GetRatings(_ context.Context, _, _ string) (*draftdata.SetRatings, error) {
	return nil, nil
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

	sched := refresh.New(fetcher, store)

	// Cancel immediately after startup — the scheduler should have run one fetch.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	sched.Start(ctx)

	require.GreaterOrEqual(t, fetcher.called, 1, "expected at least one fetch on startup")
	require.Len(t, store.upserted, 1)
	assert.Equal(t, "FDN", store.upserted[0].SetCode)
	assert.Len(t, store.upserted[0].Cards, 1)
}

func TestScheduler_NoSetsSkipsFetch(t *testing.T) {
	t.Setenv("SYNC_ACTIVE_SETS", "")

	fetcher := &stubFetcher{}
	store := &stubStore{}

	sched := refresh.New(fetcher, store)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	sched.Start(ctx)

	assert.Equal(t, 0, fetcher.called)
	assert.Empty(t, store.upserted)
}
