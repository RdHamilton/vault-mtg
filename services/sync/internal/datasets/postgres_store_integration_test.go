//go:build integration

package datasets_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ramonehamilton/mtga-sync/internal/datasets"
	"github.com/ramonehamilton/mtga-sync/internal/draftdata"
	"github.com/ramonehamilton/mtga-sync/internal/scryfall"
	"github.com/ramonehamilton/mtga-sync/internal/seventeenlands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresStore_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()

	store := datasets.NewPostgresStore(pool)

	ratings := draftdata.SetRatings{
		SetCode:     "INT",
		DraftFormat: "PremierDraft",
		FetchedAt:   time.Now().UTC().Truncate(time.Second),
		Cards: []seventeenlands.CardRating{
			{MtgaID: 99901, Name: "Test Card A", ALSA: 1.5, GIHWR: 0.60, SeenCount: 500},
			{MtgaID: 99902, Name: "Test Card B", ALSA: 3.0, GIHWR: 0.45, SeenCount: 300},
		},
	}

	require.NoError(t, store.UpsertRatings(ctx, ratings))

	got, err := store.GetRatings(ctx, "INT", "PremierDraft")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Len(t, got.Cards, 2)

	names := make([]string, len(got.Cards))
	for i, c := range got.Cards {
		names[i] = c.Name
	}
	assert.ElementsMatch(t, []string{"Test Card A", "Test Card B"}, names)

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM draft_card_ratings WHERE set_code = 'INT'")
}

func TestPostgresStore_UpsertSets_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()

	store := datasets.NewPostgresStore(pool)

	sets := []scryfall.ScryfallSet{
		{Code: "tst", Name: "Test Set Alpha", SetType: "expansion", Digital: true, CardCount: 100, ReleasedAt: "2024-01-01"},
		{Code: "ts2", Name: "Test Set Beta", SetType: "core", Digital: true, CardCount: 200, ReleasedAt: "2024-06-01"},
	}

	require.NoError(t, store.UpsertSets(ctx, sets))

	// Verify rows were inserted with is_standard_legal = TRUE.
	for _, s := range sets {
		var name string
		var isStandardLegal bool
		var cardCount int
		err := pool.QueryRow(
			ctx,
			`SELECT name, is_standard_legal, card_count FROM sets WHERE code = $1`,
			s.Code,
		).Scan(&name, &isStandardLegal, &cardCount)
		require.NoError(t, err, "set %q not found", s.Code)
		assert.Equal(t, s.Name, name)
		assert.True(t, isStandardLegal, "is_standard_legal must be TRUE for %q", s.Code)
		assert.Equal(t, s.CardCount, cardCount)
	}

	// Verify upsert updates an existing row.
	updated := []scryfall.ScryfallSet{
		{Code: "tst", Name: "Test Set Alpha Updated", SetType: "expansion", Digital: true, CardCount: 150, ReleasedAt: "2024-01-01"},
	}
	require.NoError(t, store.UpsertSets(ctx, updated))

	var name string
	err = pool.QueryRow(ctx, `SELECT name FROM sets WHERE code = 'tst'`).Scan(&name)
	require.NoError(t, err)
	assert.Equal(t, "Test Set Alpha Updated", name)

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM sets WHERE code IN ('tst', 'ts2')")
}
