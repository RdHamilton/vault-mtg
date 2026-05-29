package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
)

// insertTestAccountForGamePlay inserts a minimal accounts row and returns its
// auto-assigned id. Removed via t.Cleanup.
func insertTestAccountForGamePlay(t *testing.T, db *sql.DB, suffix string) int64 {
	t.Helper()

	name := fmt.Sprintf("gp-test-account-%s", suffix)
	var id int64
	err := db.QueryRowContext(
		context.Background(),
		`INSERT INTO accounts (name) VALUES ($1) RETURNING id`,
		name,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insertTestAccountForGamePlay %q: %v", name, err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM accounts WHERE id = $1`, id)
	})

	return id
}

// cleanupGamePlays deletes game_plays (and cascaded life_change_tracking) rows
// for the given account.
func cleanupGamePlays(t *testing.T, db *sql.DB, accountID int64) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM game_plays WHERE account_id = $1`, accountID)
	})
}

func TestGamePlayRepository_SingleInsert(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewGamePlayRepository(db)
	ctx := context.Background()

	accountID := insertTestAccountForGamePlay(t, db, "single-insert")
	cleanupGamePlays(t, db, accountID)

	now := time.Now().UTC().Truncate(time.Microsecond)

	id, err := repo.InsertGamePlay(ctx, repository.GamePlayInsert{
		AccountID:     accountID,
		MatchID:       "match-si-001",
		GameNumber:    1,
		WinningTeamID: 1,
		TurnCount:     10,
		DurationSecs:  240,
		Sequence:      42,
		OccurredAt:    now,
	})
	if err != nil {
		t.Fatalf("InsertGamePlay: %v", err)
	}
	if id == 0 {
		t.Error("InsertGamePlay returned id=0")
	}

	row, err := repo.GetGamePlay(ctx, accountID, "match-si-001", 1)
	if err != nil {
		t.Fatalf("GetGamePlay: %v", err)
	}

	if row.MatchID != "match-si-001" {
		t.Errorf("match_id: want match-si-001, got %q", row.MatchID)
	}
	if row.GameNumber != 1 {
		t.Errorf("game_number: want 1, got %d", row.GameNumber)
	}
	if row.WinningTeamID != 1 {
		t.Errorf("winning_team_id: want 1, got %d", row.WinningTeamID)
	}
	if row.TurnCount != 10 {
		t.Errorf("turn_count: want 10, got %d", row.TurnCount)
	}
	if row.DurationSecs != 240 {
		t.Errorf("duration_secs: want 240, got %d", row.DurationSecs)
	}
	if row.Sequence != 42 {
		t.Errorf("sequence: want 42, got %d", row.Sequence)
	}
	if row.AccountID != accountID {
		t.Errorf("account_id: want %d, got %d", accountID, row.AccountID)
	}
}

func TestGamePlayRepository_MultiGameSession(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewGamePlayRepository(db)
	ctx := context.Background()

	accountID := insertTestAccountForGamePlay(t, db, "multi-game")
	cleanupGamePlays(t, db, accountID)

	base := time.Now().UTC().Truncate(time.Microsecond)

	for i := 1; i <= 3; i++ {
		_, err := repo.InsertGamePlay(ctx, repository.GamePlayInsert{
			AccountID:  accountID,
			MatchID:    "match-multi-001",
			GameNumber: i,
			TurnCount:  5 * i,
			Sequence:   uint64(i),
			OccurredAt: base.Add(time.Duration(i) * time.Minute),
		})
		if err != nil {
			t.Fatalf("InsertGamePlay game %d: %v", i, err)
		}
	}

	rows, err := repo.ListGamePlaysByMatch(ctx, accountID, "match-multi-001")
	if err != nil {
		t.Fatalf("ListGamePlaysByMatch: %v", err)
	}

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}

	// Verify ordering by (occurred_at, sequence).
	for i, r := range rows {
		wantGame := i + 1
		if r.GameNumber != wantGame {
			t.Errorf("row[%d] game_number: want %d, got %d", i, wantGame, r.GameNumber)
		}
	}
}

func TestGamePlayRepository_OutOfOrderSequenceReordering(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewGamePlayRepository(db)
	ctx := context.Background()

	accountID := insertTestAccountForGamePlay(t, db, "ooo-seq")
	cleanupGamePlays(t, db, accountID)

	base := time.Now().UTC().Truncate(time.Microsecond)

	// Insert game 1 and game 2 in-order first.
	for _, gn := range []int{1, 2} {
		_, err := repo.InsertGamePlay(ctx, repository.GamePlayInsert{
			AccountID:  accountID,
			MatchID:    "match-ooo-001",
			GameNumber: gn,
			TurnCount:  5,
			Sequence:   uint64(10 + gn),
			OccurredAt: base.Add(time.Duration(gn) * time.Minute),
		})
		if err != nil {
			t.Fatalf("InsertGamePlay game %d: %v", gn, err)
		}
	}

	// Re-send game 1 with a lower sequence (stale retransmit).
	// The DB WHERE guard must reject the update.
	_, err := repo.InsertGamePlay(ctx, repository.GamePlayInsert{
		AccountID:  accountID,
		MatchID:    "match-ooo-001",
		GameNumber: 1,
		TurnCount:  99, // stale value — should not overwrite
		Sequence:   5,  // lower than the stored 11
		OccurredAt: base,
	})
	if err != nil {
		t.Fatalf("InsertGamePlay stale retransmit: %v", err)
	}

	row, err := repo.GetGamePlay(ctx, accountID, "match-ooo-001", 1)
	if err != nil {
		t.Fatalf("GetGamePlay after stale retransmit: %v", err)
	}

	// TurnCount must still be 5 (original), not 99 (stale).
	if row.TurnCount != 5 {
		t.Errorf("turn_count after stale retransmit: want 5, got %d", row.TurnCount)
	}
	if row.Sequence != 11 {
		t.Errorf("sequence after stale retransmit: want 11, got %d", row.Sequence)
	}
}

func TestGamePlayRepository_LifeChanges_InsertAndCount(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewGamePlayRepository(db)
	ctx := context.Background()

	accountID := insertTestAccountForGamePlay(t, db, "life-changes")
	cleanupGamePlays(t, db, accountID)

	gamePlayID, err := repo.InsertGamePlay(ctx, repository.GamePlayInsert{
		AccountID:  accountID,
		MatchID:    "match-lc-001",
		GameNumber: 1,
		Sequence:   1,
		OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("InsertGamePlay: %v", err)
	}

	changes := []repository.LifeChangeInsert{
		{AccountID: accountID, GamePlayID: gamePlayID, TeamID: 1, LifeTotal: 20, Delta: 0, TurnNumber: 1},
		{AccountID: accountID, GamePlayID: gamePlayID, TeamID: 2, LifeTotal: 17, Delta: -3, TurnNumber: 2},
		{AccountID: accountID, GamePlayID: gamePlayID, TeamID: 1, LifeTotal: 23, Delta: 3, TurnNumber: 3},
	}

	if err := repo.InsertLifeChanges(ctx, changes); err != nil {
		t.Fatalf("InsertLifeChanges: %v", err)
	}

	n, err := repo.CountLifeChangesByGame(ctx, gamePlayID)
	if err != nil {
		t.Fatalf("CountLifeChangesByGame: %v", err)
	}
	if n != 3 {
		t.Errorf("life_change_tracking count: want 3, got %d", n)
	}
}

func TestGamePlayRepository_AccountIsolation(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewGamePlayRepository(db)
	ctx := context.Background()

	accountA := insertTestAccountForGamePlay(t, db, "isolation-a")
	accountB := insertTestAccountForGamePlay(t, db, "isolation-b")
	cleanupGamePlays(t, db, accountA)
	cleanupGamePlays(t, db, accountB)

	const matchID = "match-iso-001"
	now := time.Now().UTC()

	_, err := repo.InsertGamePlay(ctx, repository.GamePlayInsert{
		AccountID: accountA, MatchID: matchID, GameNumber: 1,
		TurnCount: 5, Sequence: 1, OccurredAt: now,
	})
	if err != nil {
		t.Fatalf("InsertGamePlay account A: %v", err)
	}

	_, err = repo.InsertGamePlay(ctx, repository.GamePlayInsert{
		AccountID: accountB, MatchID: matchID, GameNumber: 1,
		TurnCount: 99, Sequence: 1, OccurredAt: now,
	})
	if err != nil {
		t.Fatalf("InsertGamePlay account B: %v", err)
	}

	rowA, err := repo.GetGamePlay(ctx, accountA, matchID, 1)
	if err != nil {
		t.Fatalf("GetGamePlay account A: %v", err)
	}
	rowB, err := repo.GetGamePlay(ctx, accountB, matchID, 1)
	if err != nil {
		t.Fatalf("GetGamePlay account B: %v", err)
	}

	if rowA.TurnCount != 5 {
		t.Errorf("account A turn_count: want 5, got %d", rowA.TurnCount)
	}
	if rowB.TurnCount != 99 {
		t.Errorf("account B turn_count: want 99, got %d", rowB.TurnCount)
	}
}

func TestGamePlayRepository_ListGamePlaysByMatch_OrderedByOccurredAtSequence(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewGamePlayRepository(db)
	ctx := context.Background()

	accountID := insertTestAccountForGamePlay(t, db, "ordering")
	cleanupGamePlays(t, db, accountID)

	// Insert game 3, then game 1, then game 2 to verify ORDER BY works.
	base := time.Now().UTC().Truncate(time.Microsecond)

	type gameSeed struct {
		gameNumber int
		seq        uint64
		at         time.Time
	}
	seeds := []gameSeed{
		{3, 30, base.Add(3 * time.Minute)},
		{1, 10, base.Add(1 * time.Minute)},
		{2, 20, base.Add(2 * time.Minute)},
	}

	for _, s := range seeds {
		_, err := repo.InsertGamePlay(ctx, repository.GamePlayInsert{
			AccountID:  accountID,
			MatchID:    "match-order-001",
			GameNumber: s.gameNumber,
			Sequence:   s.seq,
			OccurredAt: s.at,
		})
		if err != nil {
			t.Fatalf("InsertGamePlay game %d: %v", s.gameNumber, err)
		}
	}

	rows, err := repo.ListGamePlaysByMatch(ctx, accountID, "match-order-001")
	if err != nil {
		t.Fatalf("ListGamePlaysByMatch: %v", err)
	}

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}

	wantOrder := []int{1, 2, 3}
	for i, r := range rows {
		if r.GameNumber != wantOrder[i] {
			t.Errorf("row[%d]: want game_number=%d, got %d", i, wantOrder[i], r.GameNumber)
		}
	}
}

func TestGamePlayRepository_GetGamePlay_NotFound(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewGamePlayRepository(db)
	ctx := context.Background()

	accountID := insertTestAccountForGamePlay(t, db, "not-found")

	_, err := repo.GetGamePlay(ctx, accountID, "match-nonexistent", 1)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

// --- partial flag integration tests ---

// TestGamePlayRepository_PartialTrue verifies that InsertGamePlay stores
// partial=true when the insert carries Partial:true, and that GetGamePlay
// returns sql.ErrNoRows — partial rows are excluded from read queries.
func TestGamePlayRepository_PartialTrue(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewGamePlayRepository(db)
	ctx := context.Background()

	accountID := insertTestAccountForGamePlay(t, db, "partial-true")
	cleanupGamePlays(t, db, accountID)

	now := time.Now().UTC().Truncate(time.Microsecond)

	_, err := repo.InsertGamePlay(ctx, repository.GamePlayInsert{
		AccountID:  accountID,
		MatchID:    "match-partial-001",
		GameNumber: 1,
		Sequence:   1,
		OccurredAt: now,
		Partial:    true,
	})
	if err != nil {
		t.Fatalf("InsertGamePlay: %v", err)
	}

	// After the AND partial = false filter, GetGamePlay on a partial row must
	// return sql.ErrNoRows — partial rows are invisible to callers.
	_, err = repo.GetGamePlay(ctx, accountID, "match-partial-001", 1)
	if err != sql.ErrNoRows {
		t.Errorf("GetGamePlay on partial row: want sql.ErrNoRows, got %v", err)
	}
}

// TestGamePlayRepository_GetGamePlay_ExcludesPartial verifies that GetGamePlay
// does not return a row that was inserted with Partial:true.
func TestGamePlayRepository_GetGamePlay_ExcludesPartial(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewGamePlayRepository(db)
	ctx := context.Background()

	accountID := insertTestAccountForGamePlay(t, db, "gp-excl-partial")
	cleanupGamePlays(t, db, accountID)

	now := time.Now().UTC().Truncate(time.Microsecond)

	_, err := repo.InsertGamePlay(ctx, repository.GamePlayInsert{
		AccountID:  accountID,
		MatchID:    "match-excl-partial-001",
		GameNumber: 1,
		Sequence:   1,
		OccurredAt: now,
		Partial:    true,
	})
	if err != nil {
		t.Fatalf("InsertGamePlay partial=true: %v", err)
	}

	_, err = repo.GetGamePlay(ctx, accountID, "match-excl-partial-001", 1)
	if err != sql.ErrNoRows {
		t.Errorf("GetGamePlay on partial row: want sql.ErrNoRows, got %v", err)
	}
}

// TestGamePlayRepository_ListGamePlaysByMatch_ExcludesPartial verifies that
// ListGamePlaysByMatch omits rows inserted with Partial:true.
func TestGamePlayRepository_ListGamePlaysByMatch_ExcludesPartial(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewGamePlayRepository(db)
	ctx := context.Background()

	accountID := insertTestAccountForGamePlay(t, db, "list-excl-partial")
	cleanupGamePlays(t, db, accountID)

	base := time.Now().UTC().Truncate(time.Microsecond)

	type gameSeed struct {
		gameNumber int
		partial    bool
		seq        uint64
		at         time.Time
	}
	seeds := []gameSeed{
		{1, false, 10, base.Add(1 * time.Minute)},
		{2, true, 20, base.Add(2 * time.Minute)},
		{3, false, 30, base.Add(3 * time.Minute)},
	}

	for _, s := range seeds {
		_, err := repo.InsertGamePlay(ctx, repository.GamePlayInsert{
			AccountID:  accountID,
			MatchID:    "match-list-excl-001",
			GameNumber: s.gameNumber,
			Sequence:   s.seq,
			OccurredAt: s.at,
			Partial:    s.partial,
		})
		if err != nil {
			t.Fatalf("InsertGamePlay game %d: %v", s.gameNumber, err)
		}
	}

	rows, err := repo.ListGamePlaysByMatch(ctx, accountID, "match-list-excl-001")
	if err != nil {
		t.Fatalf("ListGamePlaysByMatch: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (partial row excluded), got %d", len(rows))
	}

	// Rows must be game_number 1 and 3 — never game_number 2 (partial).
	wantGameNumbers := []int{1, 3}
	for i, r := range rows {
		if r.GameNumber != wantGameNumbers[i] {
			t.Errorf("row[%d]: want game_number=%d, got %d", i, wantGameNumbers[i], r.GameNumber)
		}
		if r.Partial {
			t.Errorf("row[%d] game_number=%d: partial must be false in read results, got true", i, r.GameNumber)
		}
	}
}

// TestGamePlayRepository_PartialFalse verifies that InsertGamePlay stores
// partial=false (the default) when Partial is not set.
func TestGamePlayRepository_PartialFalse(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewGamePlayRepository(db)
	ctx := context.Background()

	accountID := insertTestAccountForGamePlay(t, db, "partial-false")
	cleanupGamePlays(t, db, accountID)

	now := time.Now().UTC().Truncate(time.Microsecond)

	_, err := repo.InsertGamePlay(ctx, repository.GamePlayInsert{
		AccountID:  accountID,
		MatchID:    "match-nopartial-001",
		GameNumber: 1,
		Sequence:   1,
		OccurredAt: now,
		Partial:    false,
	})
	if err != nil {
		t.Fatalf("InsertGamePlay: %v", err)
	}

	row, err := repo.GetGamePlay(ctx, accountID, "match-nopartial-001", 1)
	if err != nil {
		t.Fatalf("GetGamePlay: %v", err)
	}
	if row.Partial {
		t.Errorf("Partial: want false, got true")
	}
}
