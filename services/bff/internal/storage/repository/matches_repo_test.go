package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// insertTestAccount inserts a minimal accounts row and returns its auto-assigned id.
// The row is removed via t.Cleanup.
// Defined here (matches_repo_test.go) and shared with draft_sessions_repo_test.go
// within the same package repository_test.
func insertTestAccount(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()

	var id int64

	err := db.QueryRowContext(
		context.Background(),
		`INSERT INTO accounts (name) VALUES ($1) RETURNING id`,
		name,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insertTestAccount %q: %v", name, err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM accounts WHERE id = $1`, id)
	})

	return id
}

// insertTestMatch inserts a minimal matches row for the given account.
// The row is removed via t.Cleanup.
func insertTestMatch(t *testing.T, db *sql.DB, matchID string, accountID int64, format string, ts time.Time) {
	t.Helper()

	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO matches
			(id, account_id, event_id, event_name, timestamp, player_wins, opponent_wins,
			 player_team_id, format, result)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		matchID, accountID, "evt-"+matchID, "event-"+matchID, ts,
		1, 0, 1, format, "win",
	)
	if err != nil {
		t.Fatalf("insertTestMatch %q: %v", matchID, err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM matches WHERE id = $1`, matchID)
	})
}

// TestMatchesRepository_ListByAccountID_ReturnsOnlyOwnRows verifies cross-account
// isolation: account A cannot see account B's matches.
func TestMatchesRepository_ListByAccountID_ReturnsOnlyOwnRows(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMatchesRepository(db)

	accountA := insertTestAccount(t, db, "match-test-account-a")
	accountB := insertTestAccount(t, db, "match-test-account-b")

	now := time.Now().UTC().Truncate(time.Second)

	insertTestMatch(t, db, fmt.Sprintf("match-iso-a-%d", accountA), accountA, "Standard", now)
	insertTestMatch(t, db, fmt.Sprintf("match-iso-b-%d", accountB), accountB, "Standard", now)

	rows, _, err := repo.ListByAccountID(context.Background(), accountA, "", 1, 100)
	if err != nil {
		t.Fatalf("ListByAccountID: %v", err)
	}

	bID := fmt.Sprintf("match-iso-b-%d", accountB)

	for _, r := range rows {
		if r.ID == bID {
			t.Errorf("cross-account leak: accountA query returned accountB row %q", r.ID)
		}
	}
}

// TestMatchesRepository_ListByAccountID_EmptyAccount verifies that an account
// with no matches returns an empty slice (not an error).
func TestMatchesRepository_ListByAccountID_EmptyAccount(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMatchesRepository(db)

	accountID := insertTestAccount(t, db, "match-test-empty")

	rows, total, err := repo.ListByAccountID(context.Background(), accountID, "", 1, 10)
	if err != nil {
		t.Fatalf("ListByAccountID: %v", err)
	}

	if len(rows) != 0 {
		t.Errorf("expected 0 rows for empty account, got %d", len(rows))
	}

	if total != 0 {
		t.Errorf("expected total=0 for empty account, got %d", total)
	}
}

// TestMatchesRepository_ListByAccountID_Pagination verifies offset/limit paging
// and ordering (timestamp DESC).
func TestMatchesRepository_ListByAccountID_Pagination(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMatchesRepository(db)

	accountID := insertTestAccount(t, db, "match-test-pagination")

	base := time.Now().UTC().Truncate(time.Second)

	// Insert 3 matches at different timestamps.
	for i := 0; i < 3; i++ {
		ts := base.Add(time.Duration(i) * time.Minute)
		insertTestMatch(t, db, fmt.Sprintf("match-page-%d-%d", accountID, i), accountID, "Standard", ts)
	}

	// Page 1, limit 2 — expect 2 rows, newest first.
	page1, total, err := repo.ListByAccountID(context.Background(), accountID, "", 1, 2)
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}

	if total != 3 {
		t.Errorf("total: want 3, got %d", total)
	}

	if len(page1) != 2 {
		t.Fatalf("page 1 length: want 2, got %d", len(page1))
	}

	// Newest first: page1[0].Timestamp >= page1[1].Timestamp.
	if page1[0].Timestamp.Before(page1[1].Timestamp) {
		t.Errorf("expected DESC order: page1[0]=%v < page1[1]=%v", page1[0].Timestamp, page1[1].Timestamp)
	}

	// Page 2, limit 2 — expect 1 row.
	page2, _, err := repo.ListByAccountID(context.Background(), accountID, "", 2, 2)
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}

	if len(page2) != 1 {
		t.Errorf("page 2 length: want 1, got %d", len(page2))
	}
}

// TestMatchesRepository_ListByAccountID_FormatFilter verifies that the optional
// format filter restricts results correctly.
func TestMatchesRepository_ListByAccountID_FormatFilter(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMatchesRepository(db)

	accountID := insertTestAccount(t, db, "match-test-format-filter")

	now := time.Now().UTC().Truncate(time.Second)

	insertTestMatch(t, db, fmt.Sprintf("match-fmt-std-%d", accountID), accountID, "Standard", now)
	insertTestMatch(t, db, fmt.Sprintf("match-fmt-draft-%d", accountID), accountID, "PremierDraft", now.Add(-time.Second))

	stdRows, stdTotal, err := repo.ListByAccountID(context.Background(), accountID, "Standard", 1, 10)
	if err != nil {
		t.Fatalf("ListByAccountID (Standard): %v", err)
	}

	if stdTotal != 1 {
		t.Errorf("format filter total: want 1, got %d", stdTotal)
	}

	for _, r := range stdRows {
		if r.Format != "Standard" {
			t.Errorf("format filter returned non-Standard row: %q", r.Format)
		}
	}
}

// TestMatchesRepository_UpsertMatch_InsertAndUpdate verifies that UpsertMatch
// creates a new row on first call and updates it on second call with the same id.
func TestMatchesRepository_UpsertMatch_InsertAndUpdate(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMatchesRepository(db)

	accountID := insertTestAccount(t, db, "match-test-upsert")

	matchID := fmt.Sprintf("match-upsert-%d", accountID)
	now := time.Now().UTC().Truncate(time.Second)

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM matches WHERE id = $1`, matchID)
	})

	m := repository.MatchUpsert{
		ID:           matchID,
		AccountID:    accountID,
		EventID:      "evt-upsert",
		EventName:    "TestEvent",
		Timestamp:    now,
		Format:       "Standard",
		Result:       "win",
		PlayerWins:   2,
		OpponentWins: 0,
		PlayerTeamID: 1,
	}

	if err := repo.UpsertMatch(context.Background(), m); err != nil {
		t.Fatalf("first UpsertMatch: %v", err)
	}

	// Update result to "loss".
	m.Result = "loss"
	m.PlayerWins = 0
	m.OpponentWins = 2

	if err := repo.UpsertMatch(context.Background(), m); err != nil {
		t.Fatalf("second UpsertMatch: %v", err)
	}

	// Verify the row reflects the updated result.
	var result string

	err := db.QueryRowContext(
		context.Background(),
		`SELECT result FROM matches WHERE id = $1`,
		matchID,
	).Scan(&result)
	if err != nil {
		t.Fatalf("select after upsert: %v", err)
	}

	if result != "loss" {
		t.Errorf("expected result=loss after update upsert, got %q", result)
	}
}

// TestMatchesRepository_Interface is a compile-time check.
func TestMatchesRepository_Interface(t *testing.T) {
	var db repository.DB = &fakeDB{}
	repo := repository.NewMatchesRepository(db)

	if repo == nil {
		t.Fatal("NewMatchesRepository returned nil")
	}
}
