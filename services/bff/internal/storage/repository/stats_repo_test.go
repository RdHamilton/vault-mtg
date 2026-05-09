package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// insertTestMatchWithDeck inserts a match row with a non-nil deck_id.
// Cleaned up via t.Cleanup.
func insertTestMatchWithDeck(t *testing.T, db *sql.DB, matchID string, accountID int64, format string, ts time.Time, deckID, result string) {
	t.Helper()

	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO matches
			(id, account_id, event_id, event_name, timestamp, player_wins, opponent_wins,
			 player_team_id, format, result, deck_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		matchID, accountID,
		"evt-"+matchID, "event-"+matchID,
		ts,
		1, 0, 1,
		format, result,
		deckID,
	)
	if err != nil {
		t.Fatalf("insertTestMatchWithDeck %q: %v", matchID, err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM matches WHERE id = $1`, matchID)
	})
}

// ─── GetDeckPerformance ───────────────────────────────────────────────────────

func TestStatsRepository_GetDeckPerformance_Empty(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountID := insertTestAccount(t, db, "stats-dp-empty")

	rows, err := repo.GetDeckPerformance(context.Background(), accountID)
	if err != nil {
		t.Fatalf("GetDeckPerformance: %v", err)
	}

	if len(rows) != 0 {
		t.Errorf("expected 0 rows for empty account, got %d", len(rows))
	}
}

func TestStatsRepository_GetDeckPerformance_WinsLossesDraws(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountID := insertTestAccount(t, db, "stats-dp-wld")

	base := time.Now().UTC().Truncate(time.Second)
	deckID := fmt.Sprintf("deck-wld-%d", accountID)

	// 2 wins, 1 loss
	insertTestMatchWithDeck(t, db, fmt.Sprintf("m-wld-1-%d", accountID), accountID, "Standard", base, deckID, "win")
	insertTestMatchWithDeck(t, db, fmt.Sprintf("m-wld-2-%d", accountID), accountID, "Standard", base.Add(-time.Second), deckID, "win")
	insertTestMatchWithDeck(t, db, fmt.Sprintf("m-wld-3-%d", accountID), accountID, "Standard", base.Add(-2*time.Second), deckID, "loss")

	rows, err := repo.GetDeckPerformance(context.Background(), accountID)
	if err != nil {
		t.Fatalf("GetDeckPerformance: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("want 1 deck row, got %d", len(rows))
	}

	r := rows[0]
	if r.DeckID != deckID {
		t.Errorf("DeckID: want %s, got %s", deckID, r.DeckID)
	}

	if r.Wins != 2 {
		t.Errorf("Wins: want 2, got %d", r.Wins)
	}

	if r.Losses != 1 {
		t.Errorf("Losses: want 1, got %d", r.Losses)
	}

	if r.Draws != 0 {
		t.Errorf("Draws: want 0, got %d", r.Draws)
	}

	if r.TotalGames != 3 {
		t.Errorf("TotalGames: want 3, got %d", r.TotalGames)
	}
}

func TestStatsRepository_GetDeckPerformance_CrossAccountIsolation(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountA := insertTestAccount(t, db, "stats-dp-iso-a")
	accountB := insertTestAccount(t, db, "stats-dp-iso-b")

	base := time.Now().UTC().Truncate(time.Second)
	deckA := fmt.Sprintf("deck-iso-a-%d", accountA)
	deckB := fmt.Sprintf("deck-iso-b-%d", accountB)

	insertTestMatchWithDeck(t, db, fmt.Sprintf("m-iso-a-%d", accountA), accountA, "Standard", base, deckA, "win")
	insertTestMatchWithDeck(t, db, fmt.Sprintf("m-iso-b-%d", accountB), accountB, "Standard", base, deckB, "win")

	rows, err := repo.GetDeckPerformance(context.Background(), accountA)
	if err != nil {
		t.Fatalf("GetDeckPerformance: %v", err)
	}

	for _, r := range rows {
		if r.DeckID == deckB {
			t.Errorf("cross-account leak: accountA query returned accountB deck %q", deckB)
		}
	}
}

// ─── GetWinRateTrend ──────────────────────────────────────────────────────────

func TestStatsRepository_GetWinRateTrend_Empty(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountID := insertTestAccount(t, db, "stats-wrt-empty")

	buckets, err := repo.GetWinRateTrend(context.Background(), accountID, "daily")
	if err != nil {
		t.Fatalf("GetWinRateTrend: %v", err)
	}

	if len(buckets) != 0 {
		t.Errorf("expected 0 buckets for empty account, got %d", len(buckets))
	}
}

func TestStatsRepository_GetWinRateTrend_Daily(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountID := insertTestAccount(t, db, "stats-wrt-daily")

	// Insert 2 wins and 1 loss on the same day within the 90-day window.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	insertTestMatch(t, db, fmt.Sprintf("m-wrt-d1-%d", accountID), accountID, "Standard", today.Add(time.Hour))
	insertTestMatch(t, db, fmt.Sprintf("m-wrt-d2-%d", accountID), accountID, "Standard", today.Add(2*time.Hour))

	// insertTestMatch inserts a 'win'; add a loss separately.
	lossID := fmt.Sprintf("m-wrt-d3-%d", accountID)
	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO matches
			(id, account_id, event_id, event_name, timestamp, player_wins, opponent_wins,
			 player_team_id, format, result)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		lossID, accountID,
		"evt-wrt-d3", "event",
		today.Add(3*time.Hour), 0, 1, 1, "Standard", "loss",
	)
	if err != nil {
		t.Fatalf("insert loss: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM matches WHERE id = $1`, lossID)
	})

	buckets, err := repo.GetWinRateTrend(context.Background(), accountID, "daily")
	if err != nil {
		t.Fatalf("GetWinRateTrend: %v", err)
	}

	if len(buckets) == 0 {
		t.Fatal("expected at least 1 bucket")
	}

	// Find today's bucket (date_trunc('day') returns UTC midnight).
	var todayBucket *repository.WinRateBucket
	for i := range buckets {
		bs := buckets[i].BucketStart.UTC()
		if bs.Year() == today.Year() && bs.Month() == today.Month() && bs.Day() == today.Day() {
			todayBucket = &buckets[i]
			break
		}
	}

	if todayBucket == nil {
		t.Fatal("no bucket found for today")
	}

	if todayBucket.TotalGames != 3 {
		t.Errorf("TotalGames: want 3, got %d", todayBucket.TotalGames)
	}

	if todayBucket.Wins != 2 {
		t.Errorf("Wins: want 2, got %d", todayBucket.Wins)
	}

	wantWinRate := 2.0 / 3.0
	if todayBucket.WinRate < wantWinRate-0.001 || todayBucket.WinRate > wantWinRate+0.001 {
		t.Errorf("WinRate: want ~%.4f, got %.4f", wantWinRate, todayBucket.WinRate)
	}
}

func TestStatsRepository_GetWinRateTrend_CrossAccountIsolation(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountA := insertTestAccount(t, db, "stats-wrt-iso-a")
	accountB := insertTestAccount(t, db, "stats-wrt-iso-b")

	now := time.Now().UTC().Truncate(time.Second)
	insertTestMatch(t, db, fmt.Sprintf("m-wrt-iso-b-%d", accountB), accountB, "Standard", now)

	buckets, err := repo.GetWinRateTrend(context.Background(), accountA, "daily")
	if err != nil {
		t.Fatalf("GetWinRateTrend: %v", err)
	}

	var total int
	for _, b := range buckets {
		total += b.TotalGames
	}

	if total != 0 {
		t.Errorf("cross-account leak: accountA trend shows %d games from accountB", total)
	}
}

// ─── GetFormatDistribution ────────────────────────────────────────────────────

func TestStatsRepository_GetFormatDistribution_Empty(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountID := insertTestAccount(t, db, "stats-fd-empty")

	rows, err := repo.GetFormatDistribution(context.Background(), accountID)
	if err != nil {
		t.Fatalf("GetFormatDistribution: %v", err)
	}

	if len(rows) != 0 {
		t.Errorf("expected 0 rows for empty account, got %d", len(rows))
	}
}

func TestStatsRepository_GetFormatDistribution_MultipleFormats(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountID := insertTestAccount(t, db, "stats-fd-multi")

	base := time.Now().UTC().Truncate(time.Second)

	// 3 Standard, 1 Historic.
	for i := 0; i < 3; i++ {
		insertTestMatch(
			t, db,
			fmt.Sprintf("m-fd-std-%d-%d", accountID, i),
			accountID, "Standard",
			base.Add(time.Duration(i)*time.Second),
		)
	}

	insertTestMatch(t, db, fmt.Sprintf("m-fd-hist-%d", accountID), accountID, "Historic", base.Add(-time.Second))

	rows, err := repo.GetFormatDistribution(context.Background(), accountID)
	if err != nil {
		t.Fatalf("GetFormatDistribution: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("want 2 format rows, got %d", len(rows))
	}

	// Standard should be first (3 > 1).
	if rows[0].Format != "Standard" {
		t.Errorf("row[0].Format: want Standard, got %s", rows[0].Format)
	}

	if rows[0].GameCount != 3 {
		t.Errorf("row[0].GameCount: want 3, got %d", rows[0].GameCount)
	}

	if rows[1].Format != "Historic" {
		t.Errorf("row[1].Format: want Historic, got %s", rows[1].Format)
	}
}

func TestStatsRepository_GetFormatDistribution_CrossAccountIsolation(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountA := insertTestAccount(t, db, "stats-fd-iso-a")
	accountB := insertTestAccount(t, db, "stats-fd-iso-b")

	now := time.Now().UTC().Truncate(time.Second)
	insertTestMatch(t, db, fmt.Sprintf("m-fd-iso-b-%d", accountB), accountB, "Standard", now)

	rows, err := repo.GetFormatDistribution(context.Background(), accountA)
	if err != nil {
		t.Fatalf("GetFormatDistribution: %v", err)
	}

	if len(rows) != 0 {
		t.Errorf("cross-account leak: accountA sees %d format rows from accountB", len(rows))
	}
}
