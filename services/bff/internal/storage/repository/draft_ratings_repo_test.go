package repository_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// openTestDB opens a real PostgreSQL connection using TEST_DATABASE_URL.
// The test is skipped when that variable is not set.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping integration test")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	return db
}

// seedCardRating inserts a single draft_card_ratings row and returns its
// cached_at value as stored in the DB.  The row is cleaned up via t.Cleanup.
func seedCardRating(t *testing.T, db *sql.DB, setCode, format, name string, cachedAt time.Time) {
	t.Helper()

	_, err := db.ExecContext(context.Background(), `
		INSERT INTO draft_card_ratings
			(set_code, draft_format, arena_id, name, cached_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (set_code, draft_format, arena_id) DO UPDATE
			SET name = EXCLUDED.name, cached_at = EXCLUDED.cached_at`,
		setCode, format, 99901, name, cachedAt,
	)
	if err != nil {
		t.Fatalf("seedCardRating: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM draft_card_ratings WHERE set_code = $1 AND draft_format = $2 AND arena_id = 99901`,
			setCode, format,
		)
	})
}

func TestDraftRatingsRepository_GetRatings_ReturnsRows(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDraftRatingsRepository(db)

	const setCode = "TST"
	const format = "PremierDraft"
	now := time.Now().UTC().Truncate(time.Second)

	seedCardRating(t, db, setCode, format, "Test Card", now)

	result, err := repo.GetRatings(context.Background(), setCode, format)
	if err != nil {
		t.Fatalf("GetRatings: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result, got nil")
	}

	if len(result.CardRatings) == 0 {
		t.Fatal("expected at least one card rating")
	}

	// CachedAt must equal what was written (within 1-second tolerance for
	// timestamp truncation differences between Go and PostgreSQL).
	diff := result.CachedAt.Sub(now)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("CachedAt mismatch: got %v, want %v (diff %v)", result.CachedAt, now, diff)
	}
}

func TestDraftRatingsRepository_GetRatings_EmptyResultReturnsNil(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDraftRatingsRepository(db)

	// Use a set code that should never exist in the test DB.
	result, err := repo.GetRatings(context.Background(), "ZZZNONE", "PremierDraft")
	if err != nil {
		t.Fatalf("GetRatings: %v", err)
	}

	if result != nil {
		t.Errorf("expected nil result for missing set, got %+v", result)
	}
}

func TestDraftRatingsRepository_GetRatings_CachedAtIsMaxAcrossRows(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDraftRatingsRepository(db)

	const setCode = "TST2"
	const format = "QuickDraft"
	older := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Second)
	newer := time.Now().UTC().Truncate(time.Second)

	// Seed two rows with different arena_ids and cached_at values.
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO draft_card_ratings (set_code, draft_format, arena_id, name, cached_at)
		VALUES ($1, $2, 99902, 'Old Card', $3), ($1, $2, 99903, 'New Card', $4)
		ON CONFLICT (set_code, draft_format, arena_id) DO UPDATE
			SET name = EXCLUDED.name, cached_at = EXCLUDED.cached_at`,
		setCode, format, older, newer,
	)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM draft_card_ratings WHERE set_code = $1 AND draft_format = $2 AND arena_id IN (99902, 99903)`,
			setCode, format,
		)
	})

	result, err := repo.GetRatings(context.Background(), setCode, format)
	if err != nil {
		t.Fatalf("GetRatings: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	diff := result.CachedAt.Sub(newer)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("CachedAt should equal MAX(cached_at)=%v, got %v (diff %v)", newer, result.CachedAt, diff)
	}
}

func TestDraftRatingsRepository_GetMaxCachedAt_ReturnsZeroForMissing(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDraftRatingsRepository(db)

	ts, err := repo.GetMaxCachedAt(context.Background(), "ZZZNONE2", "PremierDraft")
	if err != nil {
		t.Fatalf("GetMaxCachedAt: %v", err)
	}

	if !ts.IsZero() {
		t.Errorf("expected zero time for missing set, got %v", ts)
	}
}
