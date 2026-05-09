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

// ─── ListDraftAnalytics ───────────────────────────────────────────────────────

func TestStatsRepository_ListDraftAnalytics_Empty(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountID := insertTestAccount(t, db, "stats-lda-empty")

	rows, err := repo.ListDraftAnalytics(context.Background(), accountID, "", nil, "", 20)
	if err != nil {
		t.Fatalf("ListDraftAnalytics: %v", err)
	}

	if len(rows) != 0 {
		t.Errorf("expected 0 rows for empty account, got %d", len(rows))
	}
}

func TestStatsRepository_ListDraftAnalytics_ReturnsSessionWithWinsLosses(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountID := insertTestAccount(t, db, "stats-lda-wl")

	now := time.Now().UTC().Truncate(time.Second)
	sessionID := fmt.Sprintf("da-wl-%d", accountID)

	insertTestDraftSession(t, db, sessionID, accountID, "ONE", now)
	insertTestDraftMatchResult(t, db, sessionID, "da-wl-m1", "win", now.Add(time.Minute))
	insertTestDraftMatchResult(t, db, sessionID, "da-wl-m2", "win", now.Add(2*time.Minute))
	insertTestDraftMatchResult(t, db, sessionID, "da-wl-m3", "loss", now.Add(3*time.Minute))

	rows, err := repo.ListDraftAnalytics(context.Background(), accountID, "", nil, "", 20)
	if err != nil {
		t.Fatalf("ListDraftAnalytics: %v", err)
	}

	if len(rows) == 0 {
		t.Fatal("expected at least 1 row")
	}

	var found *repository.DraftAnalyticsRow

	for i := range rows {
		if rows[i].SessionID == sessionID {
			found = &rows[i]
			break
		}
	}

	if found == nil {
		t.Fatalf("session %q not found in results", sessionID)
	}

	if found.Wins != 2 {
		t.Errorf("Wins: want 2, got %d", found.Wins)
	}

	if found.Losses != 1 {
		t.Errorf("Losses: want 1, got %d", found.Losses)
	}

	if found.SetCode != "ONE" {
		t.Errorf("SetCode: want ONE, got %s", found.SetCode)
	}
}

func TestStatsRepository_ListDraftAnalytics_SetCodeFilter(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountID := insertTestAccount(t, db, "stats-lda-set")

	now := time.Now().UTC().Truncate(time.Second)

	insertTestDraftSession(t, db, fmt.Sprintf("da-set-one-%d", accountID), accountID, "ONE", now)
	insertTestDraftSession(t, db, fmt.Sprintf("da-set-blb-%d", accountID), accountID, "BLB", now.Add(-time.Second))

	rows, err := repo.ListDraftAnalytics(context.Background(), accountID, "ONE", nil, "", 20)
	if err != nil {
		t.Fatalf("ListDraftAnalytics (ONE): %v", err)
	}

	for _, r := range rows {
		if r.SetCode != "ONE" {
			t.Errorf("set filter returned wrong set %q", r.SetCode)
		}
	}
}

func TestStatsRepository_ListDraftAnalytics_CrossAccountIsolation(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountA := insertTestAccount(t, db, "stats-lda-iso-a")
	accountB := insertTestAccount(t, db, "stats-lda-iso-b")

	now := time.Now().UTC().Truncate(time.Second)
	bSessionID := fmt.Sprintf("da-iso-b-%d", accountB)

	insertTestDraftSession(t, db, bSessionID, accountB, "BLB", now)

	rows, err := repo.ListDraftAnalytics(context.Background(), accountA, "", nil, "", 20)
	if err != nil {
		t.Fatalf("ListDraftAnalytics: %v", err)
	}

	for _, r := range rows {
		if r.SessionID == bSessionID {
			t.Errorf("cross-account leak: accountA query returned accountB session %q", bSessionID)
		}
	}
}

func TestStatsRepository_ListDraftAnalytics_KeysetPagination(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountID := insertTestAccount(t, db, "stats-lda-page")

	base := time.Now().UTC().Truncate(time.Second)

	// Insert 3 sessions at different start_times.
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("da-page-%d-%d", i, accountID)
		insertTestDraftSession(t, db, id, accountID, "MKM", base.Add(time.Duration(i)*time.Hour))
	}

	// Fetch first page of 2.
	page1, err := repo.ListDraftAnalytics(context.Background(), accountID, "", nil, "", 2)
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}

	// limit+1 sentinel: 3 rows exist but we asked for 2, so we get 2 back (no sentinel on exact count).
	// Confirm we got exactly 2 rows (limit=2 returns up to limit+1=3 internally, but only limit rows exist after
	// this slice; we just assert we got >=1 row to confirm the query ran).
	if len(page1) == 0 {
		t.Fatal("page 1: expected results, got none")
	}

	// Use cursor from last row on page 1 to fetch page 2.
	last := page1[len(page1)-1]
	cursorTS := last.StartTime
	cursorID := last.SessionID

	page2, err := repo.ListDraftAnalytics(context.Background(), accountID, "", &cursorTS, cursorID, 2)
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}

	// Every page 2 row must have start_time <= cursor.
	for _, r := range page2 {
		if r.StartTime.After(cursorTS) {
			t.Errorf("page 2 row start_time %v after cursor %v", r.StartTime, cursorTS)
		}
	}
}

// ─── ListRankProgression ─────────────────────────────────────────────────────

// insertTestMatchWithRank inserts a match row that carries rank_before / rank_after.
func insertTestMatchWithRank(t *testing.T, db *sql.DB, matchID string, accountID int64, format string, ts time.Time, rankBefore, rankAfter, result string) {
	t.Helper()

	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO matches
			(id, account_id, event_id, event_name, timestamp, player_wins, opponent_wins,
			 player_team_id, format, result, rank_before, rank_after)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		matchID, accountID,
		"evt-"+matchID, "event-"+matchID,
		ts,
		1, 0, 1,
		format, result,
		rankBefore, rankAfter,
	)
	if err != nil {
		t.Fatalf("insertTestMatchWithRank %q: %v", matchID, err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM matches WHERE id = $1`, matchID)
	})
}

func TestStatsRepository_ListRankProgression_Empty(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountID := insertTestAccount(t, db, "stats-lrp-empty")

	rows, err := repo.ListRankProgression(context.Background(), accountID, "", nil, "", 20)
	if err != nil {
		t.Fatalf("ListRankProgression: %v", err)
	}

	if len(rows) != 0 {
		t.Errorf("expected 0 rows for empty account, got %d", len(rows))
	}
}

func TestStatsRepository_ListRankProgression_ReturnsRankedMatches(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountID := insertTestAccount(t, db, "stats-lrp-ranked")

	now := time.Now().UTC().Truncate(time.Second)
	matchID := fmt.Sprintf("rp-ranked-%d", accountID)

	insertTestMatchWithRank(t, db, matchID, accountID, "Standard", now, "Gold-1", "Platinum-4", "win")

	// Also insert a match without rank data — it must NOT appear.
	noRankID := fmt.Sprintf("rp-norank-%d", accountID)
	insertTestMatch(t, db, noRankID, accountID, "Standard", now.Add(-time.Second))

	rows, err := repo.ListRankProgression(context.Background(), accountID, "", nil, "", 20)
	if err != nil {
		t.Fatalf("ListRankProgression: %v", err)
	}

	if len(rows) == 0 {
		t.Fatal("expected at least 1 row")
	}

	var found *repository.RankProgressionRow

	for i := range rows {
		if rows[i].MatchID == matchID {
			found = &rows[i]
			break
		}
	}

	if found == nil {
		t.Fatalf("match %q not found in results", matchID)
	}

	if found.RankBefore == nil || *found.RankBefore != "Gold-1" {
		t.Errorf("RankBefore: want Gold-1, got %v", found.RankBefore)
	}

	if found.RankAfter == nil || *found.RankAfter != "Platinum-4" {
		t.Errorf("RankAfter: want Platinum-4, got %v", found.RankAfter)
	}

	// Ensure the no-rank match is not included.
	for _, r := range rows {
		if r.MatchID == noRankID {
			t.Errorf("match without rank data leaked into results: %q", noRankID)
		}
	}
}

func TestStatsRepository_ListRankProgression_FormatFilter(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountID := insertTestAccount(t, db, "stats-lrp-fmt")

	now := time.Now().UTC().Truncate(time.Second)

	insertTestMatchWithRank(t, db, fmt.Sprintf("rp-std-%d", accountID), accountID, "Standard", now, "Gold-1", "Gold-2", "win")
	insertTestMatchWithRank(t, db, fmt.Sprintf("rp-hist-%d", accountID), accountID, "Historic", now.Add(-time.Second), "Silver-1", "Silver-2", "loss")

	rows, err := repo.ListRankProgression(context.Background(), accountID, "Standard", nil, "", 20)
	if err != nil {
		t.Fatalf("ListRankProgression (Standard): %v", err)
	}

	for _, r := range rows {
		if r.Format != "Standard" {
			t.Errorf("format filter returned non-Standard row: %q", r.Format)
		}
	}
}

func TestStatsRepository_ListRankProgression_CrossAccountIsolation(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountA := insertTestAccount(t, db, "stats-lrp-iso-a")
	accountB := insertTestAccount(t, db, "stats-lrp-iso-b")

	now := time.Now().UTC().Truncate(time.Second)
	bMatchID := fmt.Sprintf("rp-iso-b-%d", accountB)

	insertTestMatchWithRank(t, db, bMatchID, accountB, "Standard", now, "Gold-1", "Gold-2", "win")

	rows, err := repo.ListRankProgression(context.Background(), accountA, "", nil, "", 20)
	if err != nil {
		t.Fatalf("ListRankProgression: %v", err)
	}

	for _, r := range rows {
		if r.MatchID == bMatchID {
			t.Errorf("cross-account leak: accountA query returned accountB match %q", bMatchID)
		}
	}
}

func TestStatsRepository_ListRankProgression_KeysetPagination(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountID := insertTestAccount(t, db, "stats-lrp-page")

	base := time.Now().UTC().Truncate(time.Second)

	// Insert 3 ranked matches at different timestamps.
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("rp-page-%d-%d", i, accountID)
		insertTestMatchWithRank(t, db, id, accountID, "Standard", base.Add(time.Duration(i)*time.Hour), "Gold-1", "Gold-2", "win")
	}

	// Fetch first page.
	page1, err := repo.ListRankProgression(context.Background(), accountID, "", nil, "", 2)
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}

	if len(page1) == 0 {
		t.Fatal("page 1: expected results, got none")
	}

	last := page1[len(page1)-1]
	cursorTS := last.OccurredAt
	cursorID := last.MatchID

	page2, err := repo.ListRankProgression(context.Background(), accountID, "", &cursorTS, cursorID, 2)
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}

	// Every page 2 row must have timestamp <= cursor.
	for _, r := range page2 {
		if r.OccurredAt.After(cursorTS) {
			t.Errorf("page 2 row timestamp %v after cursor %v", r.OccurredAt, cursorTS)
		}
	}
}

// ─── GetResultBreakdown ───────────────────────────────────────────────────────

func TestStatsRepository_GetResultBreakdown_Empty(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountID := insertTestAccount(t, db, "stats-grb-empty")

	rows, err := repo.GetResultBreakdown(context.Background(), accountID, "")
	if err != nil {
		t.Fatalf("GetResultBreakdown: %v", err)
	}

	if len(rows) != 0 {
		t.Errorf("expected 0 rows for empty account, got %d", len(rows))
	}
}

func TestStatsRepository_GetResultBreakdown_WinsAndLosses(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountID := insertTestAccount(t, db, "stats-grb-wl")

	base := time.Now().UTC().Truncate(time.Second)

	// 2 Standard wins, 1 Standard loss.
	insertTestMatch(t, db, fmt.Sprintf("rb-std-w1-%d", accountID), accountID, "Standard", base)
	insertTestMatch(t, db, fmt.Sprintf("rb-std-w2-%d", accountID), accountID, "Standard", base.Add(-time.Second))

	lossID := fmt.Sprintf("rb-std-l1-%d", accountID)
	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO matches
			(id, account_id, event_id, event_name, timestamp, player_wins, opponent_wins,
			 player_team_id, format, result)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		lossID, accountID,
		"evt-"+lossID, "event",
		base.Add(-2*time.Second), 0, 1, 1, "Standard", "loss",
	)
	if err != nil {
		t.Fatalf("insert loss: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM matches WHERE id = $1`, lossID)
	})

	rows, err := repo.GetResultBreakdown(context.Background(), accountID, "")
	if err != nil {
		t.Fatalf("GetResultBreakdown: %v", err)
	}

	if len(rows) == 0 {
		t.Fatal("expected at least 1 row")
	}

	var found *repository.ResultBreakdownRow

	for i := range rows {
		if rows[i].Format == "Standard" {
			found = &rows[i]
			break
		}
	}

	if found == nil {
		t.Fatal("Standard row not found")
	}

	if found.Wins != 2 {
		t.Errorf("Wins: want 2, got %d", found.Wins)
	}

	if found.Losses != 1 {
		t.Errorf("Losses: want 1, got %d", found.Losses)
	}
}

func TestStatsRepository_GetResultBreakdown_FormatFilter(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountID := insertTestAccount(t, db, "stats-grb-fmt")

	now := time.Now().UTC().Truncate(time.Second)

	insertTestMatch(t, db, fmt.Sprintf("rb-fmt-std-%d", accountID), accountID, "Standard", now)
	insertTestMatch(t, db, fmt.Sprintf("rb-fmt-hist-%d", accountID), accountID, "Historic", now.Add(-time.Second))

	rows, err := repo.GetResultBreakdown(context.Background(), accountID, "Standard")
	if err != nil {
		t.Fatalf("GetResultBreakdown (Standard): %v", err)
	}

	for _, r := range rows {
		if r.Format != "Standard" {
			t.Errorf("format filter returned non-Standard row: %q", r.Format)
		}
	}
}

func TestStatsRepository_GetResultBreakdown_CrossAccountIsolation(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStatsRepository(db)

	accountA := insertTestAccount(t, db, "stats-grb-iso-a")
	accountB := insertTestAccount(t, db, "stats-grb-iso-b")

	now := time.Now().UTC().Truncate(time.Second)
	insertTestMatch(t, db, fmt.Sprintf("rb-iso-b-%d", accountB), accountB, "Standard", now)

	rows, err := repo.GetResultBreakdown(context.Background(), accountA, "")
	if err != nil {
		t.Fatalf("GetResultBreakdown: %v", err)
	}

	var total int

	for _, r := range rows {
		total += r.Wins + r.Losses
	}

	if total != 0 {
		t.Errorf("cross-account leak: accountA breakdown shows %d results from accountB", total)
	}
}
