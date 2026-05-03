package datasets_test

import (
	"context"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-sync/internal/datasets"
	"github.com/ramonehamilton/mtga-sync/internal/draftdata"
	"github.com/ramonehamilton/mtga-sync/internal/seventeenlands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStore is an in-memory Store implementation used in unit tests.
type mockStore struct {
	data map[string]*draftdata.SetRatings
}

func newMockStore() *mockStore {
	return &mockStore{data: make(map[string]*draftdata.SetRatings)}
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

func TestMockStore_RoundTrip(t *testing.T) {
	store := newMockStore()
	ctx := context.Background()

	input := draftdata.SetRatings{
		SetCode:     "FDN",
		DraftFormat: "PremierDraft",
		FetchedAt:   time.Now().UTC().Truncate(time.Second),
		Cards: []seventeenlands.CardRating{
			{Name: "Lightning Bolt", ALSA: 1.5, ATA: 1.8, GIHWR: 0.62, SeenCount: 1000},
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

// Compile-time assertion that PostgresStore satisfies the Store interface.
var _ datasets.Store = (*datasets.PostgresStore)(nil)
