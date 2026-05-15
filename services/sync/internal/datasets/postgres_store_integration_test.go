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

	// Verify rows were inserted with is_draft_active = TRUE.
	// Note: UpsertSets sets is_draft_active (not is_standard_legal). Standard
	// legality is managed separately by BFF migrations and is not written by
	// the sync service.
	for _, s := range sets {
		var name string
		var isDraftActive bool
		var cardCount int
		err := pool.QueryRow(
			ctx,
			`SELECT name, is_draft_active, card_count FROM sets WHERE code = $1`,
			s.Code,
		).Scan(&name, &isDraftActive, &cardCount)
		require.NoError(t, err, "set %q not found", s.Code)
		assert.Equal(t, s.Name, name)
		assert.True(t, isDraftActive, "is_draft_active must be TRUE for %q", s.Code)
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

// TestPostgresStore_UpsertRatings_ZeroFetchedAt_Integration verifies the defensive fallback:
// when FetchedAt is zero, UpsertRatings must substitute time.Now() so that cached_at in
// Postgres is never 0001-01-01 (which would make the BFF staleness check always fire
// X-Cache-Degraded: true).
func TestPostgresStore_UpsertRatings_ZeroFetchedAt_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()

	store := datasets.NewPostgresStore(pool)

	before := time.Now().UTC().Add(-time.Second)

	// Intentionally omit FetchedAt (zero value) — store must substitute time.Now().
	ratings := draftdata.SetRatings{
		SetCode:     "ZFT",
		DraftFormat: "PremierDraft",
		// FetchedAt intentionally zero
		Cards: []seventeenlands.CardRating{
			{MtgaID: 88801, Name: "Zero Fetch Card", ALSA: 5.0, GIHWR: 0.50, SeenCount: 100},
		},
	}

	require.NoError(t, store.UpsertRatings(ctx, ratings))

	var cachedAt time.Time
	err = pool.QueryRow(
		ctx,
		`SELECT cached_at FROM draft_card_ratings WHERE set_code = 'ZFT' AND draft_format = 'PremierDraft' AND arena_id = 88801`,
	).Scan(&cachedAt)
	require.NoError(t, err, "row must exist after upsert")

	// cached_at must be a real timestamp — not the zero value 0001-01-01.
	assert.False(t, cachedAt.IsZero(), "cached_at must not be zero — defensive fallback must have fired")
	assert.True(t, cachedAt.After(before),
		"cached_at (%v) must be after the time before the upsert (%v)", cachedAt, before)

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM draft_card_ratings WHERE set_code = 'ZFT'")
}
