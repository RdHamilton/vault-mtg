package repository_test

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
	_ "github.com/jackc/pgx/v5/stdlib"
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

	_, err := db.ExecContext(
		context.Background(), `
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
		_, _ = db.ExecContext(
			context.Background(),
			`DELETE FROM draft_card_ratings WHERE set_code = $1 AND draft_format = $2 AND arena_id = 99901`,
			setCode, format,
		)
	})
}

// seedCard inserts a minimal set_cards row for testing the color/rarity JOIN.
// set_cards.arena_id is TEXT (migration 000014); arenaID is converted to string.
// The row is cleaned up via t.Cleanup.
func seedCard(t *testing.T, db *sql.DB, arenaID int, colors, rarity string) {
	t.Helper()

	arenaIDText := strconv.Itoa(arenaID)
	_, err := db.ExecContext(
		context.Background(), `
		INSERT INTO set_cards (set_code, arena_id, scryfall_id, name, colors, rarity)
		VALUES ('TST', $1, $2, 'Test Card', $3, $4)
		ON CONFLICT (set_code, arena_id) DO UPDATE
			SET colors  = EXCLUDED.colors,
			    rarity  = EXCLUDED.rarity`,
		arenaIDText, "test-scryfall-id-"+arenaIDText, colors, rarity,
	)
	if err != nil {
		t.Fatalf("seedCard: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			`DELETE FROM set_cards WHERE set_code = 'TST' AND arena_id = $1`,
			arenaIDText,
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
	_, err := db.ExecContext(
		context.Background(), `
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
		_, _ = db.ExecContext(
			context.Background(),
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

func TestDraftRatingsRepository_GetRatings_ColorRarityFromCardsJoin(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDraftRatingsRepository(db)

	const setCode = "TST3"
	const format = "PremierDraft"
	now := time.Now().UTC().Truncate(time.Second)

	// Seed a card rating row (arena_id 99901).
	seedCardRating(t, db, setCode, format, "Test Card", now)

	// Seed a matching cards row so the JOIN can resolve color and rarity.
	seedCard(t, db, 99901, `["R","G"]`, "rare")

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

	card := result.CardRatings[0]

	if card.Color == "" {
		t.Error("Color must not be empty when cards row exists")
	}

	if card.Rarity == "" {
		t.Error("Rarity must not be empty when cards row exists")
	}

	if card.Rarity != "rare" {
		t.Errorf("Rarity: got %q, want %q", card.Rarity, "rare")
	}
}

func TestDraftRatingsRepository_GetRatings_ColorRarityEmptyWhenNoCard(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDraftRatingsRepository(db)

	const setCode = "TST4"
	const format = "PremierDraft"
	now := time.Now().UTC().Truncate(time.Second)

	// Seed only the rating row — no matching cards row.
	_, err := db.ExecContext(
		context.Background(), `
		INSERT INTO draft_card_ratings
			(set_code, draft_format, arena_id, name, cached_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (set_code, draft_format, arena_id) DO UPDATE
			SET name = EXCLUDED.name, cached_at = EXCLUDED.cached_at`,
		setCode, format, 99904, "No Metadata Card", now,
	)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			`DELETE FROM draft_card_ratings WHERE set_code = $1 AND draft_format = $2 AND arena_id = 99904`,
			setCode, format,
		)
	})

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

	// Color and rarity must be empty strings (COALESCE fallback), not an error.
	card := result.CardRatings[0]
	if card.Color != "" {
		t.Errorf("Color: got %q, want empty string when no cards row", card.Color)
	}

	if card.Rarity != "" {
		t.Errorf("Rarity: got %q, want empty string when no cards row", card.Rarity)
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
