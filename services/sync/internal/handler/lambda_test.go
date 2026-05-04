package handler_test

import (
	"context"
	"errors"
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
	called int
	cards  []seventeenlands.CardRating
	err    error
}

func (f *stubFetcher) FetchCardRatings(_ context.Context, _, _ string) ([]seventeenlands.CardRating, error) {
	f.called++
	return f.cards, f.err
}

// stubStore is a test double for the datasets.Store interface.
type stubStore struct {
	dbSets   []string
	dbErr    error
	upserted []draftdata.SetRatings
	upsertFn func(setCode string) error
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

// Compile-time check that stubStore satisfies datasets.Store.
var _ datasets.Store = (*stubStore)(nil)

// TestHandle_WithOverrideSets verifies that override sets bypass the DB.
func TestHandle_WithOverrideSets(t *testing.T) {
	fetcher := &stubFetcher{
		cards: []seventeenlands.CardRating{{Name: "Lightning Bolt", ALSA: 1.5}},
	}
	store := &stubStore{}

	h := handler.New(fetcher, store, []string{"FDN", "BLB"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, 2, fetcher.called)
	require.Len(t, store.upserted, 2)
	setCodes := []string{store.upserted[0].SetCode, store.upserted[1].SetCode}
	assert.ElementsMatch(t, []string{"FDN", "BLB"}, setCodes)
}

// TestHandle_WithDBSets verifies that active sets are read from the store when
// no override is provided.
func TestHandle_WithDBSets(t *testing.T) {
	fetcher := &stubFetcher{
		cards: []seventeenlands.CardRating{{Name: "Forest", ALSA: 9.0}},
	}
	store := &stubStore{dbSets: []string{"DSK"}}

	h := handler.New(fetcher, store, nil)
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, 1, fetcher.called)
	require.Len(t, store.upserted, 1)
	assert.Equal(t, "DSK", store.upserted[0].SetCode)
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
	callCount := 0
	fetcher := &stubFetcher{}
	fetcher.err = nil
	// Return an error only on the first call using a custom stubFetcher that
	// counts calls.
	custom := &countingFetcher{
		results: map[string]fetchResult{
			"SET1": {err: errors.New("upstream timeout")},
			"SET2": {cards: []seventeenlands.CardRating{{Name: "Island", ALSA: 8.0}}},
		},
	}
	_ = callCount

	store := &stubStore{}
	h := handler.New(custom, store, []string{"SET1", "SET2"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, 2, custom.called)
	// Only SET2 should have been upserted.
	require.Len(t, store.upserted, 1)
	assert.Equal(t, "SET2", store.upserted[0].SetCode)
}

// TestHandle_EmptyCardsSkipsUpsert verifies that a 0-card response does not call UpsertRatings.
func TestHandle_EmptyCardsSkipsUpsert(t *testing.T) {
	fetcher := &stubFetcher{cards: []seventeenlands.CardRating{}} // 0 cards
	store := &stubStore{}

	h := handler.New(fetcher, store, []string{"FDN"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, 1, fetcher.called)
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

	h := handler.New(fetcher, store, []string{"SET1", "SET2"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, 2, upsertCalls)
	require.Len(t, upserted, 1)
	assert.Equal(t, "SET2", upserted[0].SetCode)
}

// TestHandle_ContextCancelled verifies early exit when context is cancelled.
func TestHandle_ContextCancelled(t *testing.T) {
	fetcher := &stubFetcher{
		cards: []seventeenlands.CardRating{{Name: "Mountain", ALSA: 9.0}},
	}
	store := &stubStore{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	h := handler.New(fetcher, store, []string{"SET1", "SET2"})
	err := h.Handle(ctx, nil)

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 0, fetcher.called)
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

	h := handler.New(fetcher, store, []string{"BLB"})
	err := h.Handle(context.Background(), nil)

	require.NoError(t, err)
	require.Len(t, store.upserted, 1)

	sr := store.upserted[0]
	assert.False(t, sr.FetchedAt.IsZero(), "FetchedAt must not be zero — cached_at would be 0001-01-01 in Postgres")
	assert.True(t, sr.FetchedAt.After(before) || sr.FetchedAt.Equal(before),
		"FetchedAt should be >= time before Handle was called")
}

// --- helpers ---

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
