//go:build integration

package store_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/RdHamilton/vault-mtg/services/meta-scrape/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// TestMetaStore_UpsertArchetypes_InsertsNew verifies that a fresh slice of
// archetypes is inserted with correct field values.
func TestMetaStore_UpsertArchetypes_InsertsNew(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	s := store.NewMetaStore(pool)

	archetypes := []store.Archetype{
		{
			Name:            "_test-esper-control",
			Format:          "_test-standard",
			Tier:            strPtr("1"),
			MetaShare:       float32Ptr(12.5),
			TournamentTop8s: intPtr(8),
			TournamentWins:  intPtr(2),
			ConfidenceScore: float32Ptr(0.85),
			TrendDirection:  strPtr("up"),
		},
	}

	require.NoError(t, s.UpsertArchetypes(ctx, archetypes))
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM mtgzone_archetypes WHERE name LIKE '\_test-%' AND format LIKE '\_test-%'`)
	})

	var tier string
	var metaShare float32
	var top8s, wins int
	var confidence float32
	var trend string
	err := pool.QueryRow(
		ctx,
		`SELECT COALESCE(tier,''), COALESCE(meta_share,0), COALESCE(tournament_top8s,0),
		        COALESCE(tournament_wins,0), COALESCE(confidence_score,0), COALESCE(trend_direction,'')
		   FROM mtgzone_archetypes WHERE name = $1 AND format = $2`,
		"_test-esper-control", "_test-standard",
	).Scan(&tier, &metaShare, &top8s, &wins, &confidence, &trend)
	require.NoError(t, err)
	assert.Equal(t, "1", tier)
	assert.InDelta(t, float32(12.5), metaShare, 0.01)
	assert.Equal(t, 8, top8s)
	assert.Equal(t, 2, wins)
	assert.InDelta(t, float32(0.85), confidence, 0.001)
	assert.Equal(t, "up", trend)
}

// TestMetaStore_UpsertArchetypes_UpdatesOnConflict verifies that upserting an
// archetype with the same (name, format) key updates the existing row without
// duplicating it.
func TestMetaStore_UpsertArchetypes_UpdatesOnConflict(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	s := store.NewMetaStore(pool)

	first := []store.Archetype{{
		Name:      "_test-update-arc",
		Format:    "_test-format",
		Tier:      strPtr("2"),
		MetaShare: float32Ptr(5.0),
	}}
	require.NoError(t, s.UpsertArchetypes(ctx, first))
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM mtgzone_archetypes WHERE name LIKE '\_test-%' AND format LIKE '\_test-%'`)
	})

	second := []store.Archetype{{
		Name:      "_test-update-arc",
		Format:    "_test-format",
		Tier:      strPtr("1"),
		MetaShare: float32Ptr(18.0),
	}}
	require.NoError(t, s.UpsertArchetypes(ctx, second))

	// Exactly one row must exist.
	var count int
	err := pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM mtgzone_archetypes WHERE name = $1 AND format = $2`,
		"_test-update-arc", "_test-format",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "ON CONFLICT must not duplicate the row")

	// Values must reflect the second upsert.
	var tier string
	var metaShare float32
	err = pool.QueryRow(
		ctx,
		`SELECT COALESCE(tier,''), COALESCE(meta_share,0)
		   FROM mtgzone_archetypes WHERE name = $1 AND format = $2`,
		"_test-update-arc", "_test-format",
	).Scan(&tier, &metaShare)
	require.NoError(t, err)
	assert.Equal(t, "1", tier)
	assert.InDelta(t, float32(18.0), metaShare, 0.01)
}

// TestMetaStore_UpsertArchetypes_NoopOnEmpty verifies that passing an empty
// slice is a no-op — no error and no rows inserted.
func TestMetaStore_UpsertArchetypes_NoopOnEmpty(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	s := store.NewMetaStore(pool)

	var before int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM mtgzone_archetypes").Scan(&before)

	require.NoError(t, s.UpsertArchetypes(ctx, nil))
	require.NoError(t, s.UpsertArchetypes(ctx, []store.Archetype{}))

	var after int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM mtgzone_archetypes").Scan(&after)
	assert.Equal(t, before, after, "empty slice must not insert rows")
}

// TestMetaStore_UpsertArchetypeCards_InsertsNew verifies that cards for a
// seeded archetype are inserted with correct values.
func TestMetaStore_UpsertArchetypeCards_InsertsNew(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	s := store.NewMetaStore(pool)

	// Seed the parent archetype.
	require.NoError(t, s.UpsertArchetypes(ctx, []store.Archetype{{
		Name:   "_test-arc-cards",
		Format: "_test-standard",
	}}))
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `
			DELETE FROM mtgzone_archetype_cards ac
			  USING mtgzone_archetypes a
			  WHERE ac.archetype_id = a.id
			    AND a.name LIKE '\_test-%'
			    AND a.format LIKE '\_test-%'`)
		_, _ = pool.Exec(ctx, `DELETE FROM mtgzone_archetypes WHERE name LIKE '\_test-%' AND format LIKE '\_test-%'`)
	})

	arcID, err := s.ArchetypeIDByKey(ctx, "_test-arc-cards", "_test-standard")
	require.NoError(t, err)
	require.NotZero(t, arcID)

	cards := []store.ArchetypeCard{
		{CardName: "Sheoldred, the Apocalypse", Role: "threat", Copies: 4, Importance: strPtr("high")},
		{CardName: "Memory Deluge", Role: "draw", Copies: 2},
	}
	require.NoError(t, s.UpsertArchetypeCards(ctx, arcID, cards))

	// Verify both rows exist.
	var count int
	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM mtgzone_archetype_cards WHERE archetype_id = $1`, arcID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Spot-check field values for the first card.
	var role string
	var copies int
	var importance string
	err = pool.QueryRow(
		ctx,
		`SELECT role, copies, COALESCE(importance,'')
		   FROM mtgzone_archetype_cards
		  WHERE archetype_id = $1 AND card_name = $2`,
		arcID, "Sheoldred, the Apocalypse",
	).Scan(&role, &copies, &importance)
	require.NoError(t, err)
	assert.Equal(t, "threat", role)
	assert.Equal(t, 4, copies)
	assert.Equal(t, "high", importance)
}

// TestMetaStore_UpsertArchetypeCards_UpdatesOnConflict verifies that a second
// upsert on the same (archetype_id, card_name) updates the existing row.
func TestMetaStore_UpsertArchetypeCards_UpdatesOnConflict(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	s := store.NewMetaStore(pool)

	require.NoError(t, s.UpsertArchetypes(ctx, []store.Archetype{{
		Name:   "_test-arc-conflict",
		Format: "_test-standard",
	}}))
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `
			DELETE FROM mtgzone_archetype_cards ac
			  USING mtgzone_archetypes a
			  WHERE ac.archetype_id = a.id
			    AND a.name LIKE '\_test-%'
			    AND a.format LIKE '\_test-%'`)
		_, _ = pool.Exec(ctx, `DELETE FROM mtgzone_archetypes WHERE name LIKE '\_test-%' AND format LIKE '\_test-%'`)
	})

	arcID, err := s.ArchetypeIDByKey(ctx, "_test-arc-conflict", "_test-standard")
	require.NoError(t, err)

	first := []store.ArchetypeCard{{CardName: "Consider", Role: "cantrip", Copies: 4}}
	require.NoError(t, s.UpsertArchetypeCards(ctx, arcID, first))

	second := []store.ArchetypeCard{{CardName: "Consider", Role: "filter", Copies: 2}}
	require.NoError(t, s.UpsertArchetypeCards(ctx, arcID, second))

	var count int
	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM mtgzone_archetype_cards
		  WHERE archetype_id = $1 AND card_name = 'Consider'`, arcID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "ON CONFLICT must not duplicate the card row")

	var role string
	var copies int
	err = pool.QueryRow(
		ctx,
		`SELECT role, copies FROM mtgzone_archetype_cards
		  WHERE archetype_id = $1 AND card_name = 'Consider'`, arcID,
	).Scan(&role, &copies)
	require.NoError(t, err)
	assert.Equal(t, "filter", role)
	assert.Equal(t, 2, copies)
}

// TestMetaStore_UpsertArchetypeCards_NoopOnEmpty verifies that passing an empty
// cards slice is a no-op — no error.
func TestMetaStore_UpsertArchetypeCards_NoopOnEmpty(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	s := store.NewMetaStore(pool)

	require.NoError(t, s.UpsertArchetypeCards(ctx, 99999, nil))
	require.NoError(t, s.UpsertArchetypeCards(ctx, 99999, []store.ArchetypeCard{}))
}

// TestMetaStore_PartialSourceFailure_PriorRowsPreserved verifies AC3: rows from
// a prior healthy run are untouched when a subsequent run only calls
// UpsertArchetypes for a different format.
func TestMetaStore_PartialSourceFailure_PriorRowsPreserved(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	s := store.NewMetaStore(pool)

	// "Run 1" seeds two archetypes across two formats.
	run1 := []store.Archetype{
		{Name: "_test-prior-row", Format: "_test-standard", Tier: strPtr("1")},
		{Name: "_test-other-row", Format: "_test-historic", Tier: strPtr("2")},
	}
	require.NoError(t, s.UpsertArchetypes(ctx, run1))
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM mtgzone_archetypes WHERE name LIKE '\_test-%' AND format LIKE '\_test-%'`)
	})

	// "Run 2" only touches the historic format (standard source "failed" and was
	// skipped by the caller — empty slice is never passed here; the caller simply
	// does not call UpsertArchetypes for the standard format at all).
	run2 := []store.Archetype{
		{Name: "_test-other-row", Format: "_test-historic", Tier: strPtr("1")},
	}
	require.NoError(t, s.UpsertArchetypes(ctx, run2))

	// The standard archetype from run 1 must still exist.
	var count int
	err := pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM mtgzone_archetypes WHERE name = $1 AND format = $2`,
		"_test-prior-row", "_test-standard",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "row from prior run must be preserved when a different source is upserted")
}

// TestMetaStore_ArchetypeIDByKey_ReturnsCorrectID verifies that ArchetypeIDByKey
// returns the correct PK after an upsert, and pgx.ErrNoRows on a missing key.
func TestMetaStore_ArchetypeIDByKey_ReturnsCorrectID(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	s := store.NewMetaStore(pool)

	require.NoError(t, s.UpsertArchetypes(ctx, []store.Archetype{{
		Name:   "_test-id-lookup",
		Format: "_test-standard",
	}}))
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM mtgzone_archetypes WHERE name LIKE '\_test-%' AND format LIKE '\_test-%'`)
	})

	id, err := s.ArchetypeIDByKey(ctx, "_test-id-lookup", "_test-standard")
	require.NoError(t, err)
	assert.NotZero(t, id, "ArchetypeIDByKey must return a non-zero PK")

	// Missing key must return pgx.ErrNoRows.
	_, err = s.ArchetypeIDByKey(ctx, "_test-does-not-exist", "_test-standard")
	assert.True(t, errors.Is(err, pgx.ErrNoRows), "expected pgx.ErrNoRows for missing key, got: %v", err)
}

// Pointer constructors used in test data.

func strPtr(s string) *string       { return &s }
func float32Ptr(f float32) *float32 { return &f }
func intPtr(i int) *int             { return &i }
