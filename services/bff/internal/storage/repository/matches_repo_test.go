package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
// isolation: account A cannot see account B's matches (legacy offset method).
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
// with no matches returns an empty slice (not an error) — legacy offset method.
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
// and ordering (timestamp DESC) — deprecated method, retained for regression.
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
// format filter restricts results correctly — deprecated method.
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

// ── Cursor-based tests (ListByAccountIDCursorFiltered) ────────────────────────

// TestMatchesRepository_CursorFiltered_EmptyAccount verifies that cursor query
// returns empty slice for an account with no matches.
func TestMatchesRepository_CursorFiltered_EmptyAccount(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMatchesRepository(db)

	accountID := insertTestAccount(t, db, "cursor-test-empty")

	rows, err := repo.ListByAccountIDCursorFiltered(context.Background(), accountID, repository.MatchFilter{}, nil, "", 50)
	if err != nil {
		t.Fatalf("ListByAccountIDCursorFiltered: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows for empty account, got %d", len(rows))
	}
}

// TestMatchesRepository_CursorFiltered_CrossAccountIsolation verifies account
// scoping: accountA cursor query must not return accountB rows.
func TestMatchesRepository_CursorFiltered_CrossAccountIsolation(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMatchesRepository(db)

	accountA := insertTestAccount(t, db, "cursor-iso-a")
	accountB := insertTestAccount(t, db, "cursor-iso-b")

	now := time.Now().UTC().Truncate(time.Second)
	insertTestMatch(t, db, fmt.Sprintf("cursor-iso-match-a-%d", accountA), accountA, "Standard", now)
	insertTestMatch(t, db, fmt.Sprintf("cursor-iso-match-b-%d", accountB), accountB, "Standard", now)

	rows, err := repo.ListByAccountIDCursorFiltered(context.Background(), accountA, repository.MatchFilter{}, nil, "", 100)
	if err != nil {
		t.Fatalf("ListByAccountIDCursorFiltered: %v", err)
	}

	bID := fmt.Sprintf("cursor-iso-match-b-%d", accountB)
	for _, r := range rows {
		if r.ID == bID {
			t.Errorf("cross-account leak: accountA cursor query returned accountB row %q", r.ID)
		}
	}
}

// TestMatchesRepository_CursorFiltered_OrderAndHasMore verifies DESC ordering
// and the limit+1 has_more probe pattern.
func TestMatchesRepository_CursorFiltered_OrderAndHasMore(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMatchesRepository(db)

	accountID := insertTestAccount(t, db, "cursor-test-order")
	base := time.Now().UTC().Truncate(time.Second)

	// Insert 3 matches with distinct timestamps.
	for i := 0; i < 3; i++ {
		ts := base.Add(time.Duration(i) * time.Minute)
		insertTestMatch(t, db, fmt.Sprintf("cursor-order-%d-%d", accountID, i), accountID, "Standard", ts)
	}

	// Fetch with limit=2 — should get 3 rows (limit+1 probe), signalling has_more.
	rows, err := repo.ListByAccountIDCursorFiltered(context.Background(), accountID, repository.MatchFilter{}, nil, "", 2)
	if err != nil {
		t.Fatalf("ListByAccountIDCursorFiltered: %v", err)
	}

	// Expect 3 rows (limit+1) because has_more probe.
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows (limit+1 probe), got %d", len(rows))
	}

	// Newest first.
	if rows[0].Timestamp.Before(rows[1].Timestamp) {
		t.Errorf("expected DESC order: rows[0]=%v < rows[1]=%v", rows[0].Timestamp, rows[1].Timestamp)
	}
}

// TestMatchesRepository_CursorFiltered_KeysetCursor verifies that a cursor
// correctly restricts results to rows older than the cursor row.
func TestMatchesRepository_CursorFiltered_KeysetCursor(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMatchesRepository(db)

	accountID := insertTestAccount(t, db, "cursor-test-keyset")
	base := time.Now().UTC().Truncate(time.Second)

	ids := []string{
		fmt.Sprintf("cursor-ks-0-%d", accountID),
		fmt.Sprintf("cursor-ks-1-%d", accountID),
		fmt.Sprintf("cursor-ks-2-%d", accountID),
	}
	for i, id := range ids {
		insertTestMatch(t, db, id, accountID, "Standard", base.Add(time.Duration(2-i)*time.Minute))
	}
	// Timestamps: ids[0] newest (base+2m), ids[1] middle (base+1m), ids[2] oldest (base).

	// First page: limit=2, no cursor → get ids[0] and ids[1] (+ probe=ids[2]).
	firstPage, err := repo.ListByAccountIDCursorFiltered(context.Background(), accountID, repository.MatchFilter{}, nil, "", 2)
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(firstPage) < 2 {
		t.Fatalf("first page: expected ≥2 rows, got %d", len(firstPage))
	}

	// Use the second row as the cursor to fetch the next page.
	cursorRow := firstPage[1] // ids[1]
	cursorTS := cursorRow.Timestamp
	cursorID := cursorRow.ID

	secondPage, err := repo.ListByAccountIDCursorFiltered(context.Background(), accountID, repository.MatchFilter{}, &cursorTS, cursorID, 10)
	if err != nil {
		t.Fatalf("second page: %v", err)
	}

	// Should only contain ids[2] (the oldest row).
	for _, r := range secondPage {
		if r.Timestamp.After(cursorTS) || (r.Timestamp.Equal(cursorTS) && r.ID >= cursorID) {
			t.Errorf("cursor leak: row %q ts=%v is not before cursor ts=%v id=%q", r.ID, r.Timestamp, cursorTS, cursorID)
		}
	}
}

// TestMatchesRepository_CursorFiltered_FilterDimensions verifies that the full
// MatchFilter (date range, deck, result, format) is applied correctly.
func TestMatchesRepository_CursorFiltered_FilterDimensions(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMatchesRepository(db)

	accountID := insertTestAccount(t, db, "cursor-test-filter")
	now := time.Now().UTC().Truncate(time.Second)

	insertTestMatch(t, db, fmt.Sprintf("cursor-flt-std-%d", accountID), accountID, "Standard", now)
	insertTestMatch(t, db, fmt.Sprintf("cursor-flt-draft-%d", accountID), accountID, "PremierDraft", now.Add(-time.Second))

	// Filter by format=Standard — should only return the Standard match.
	f := repository.MatchFilter{Format: "Standard"}
	rows, err := repo.ListByAccountIDCursorFiltered(context.Background(), accountID, f, nil, "", 50)
	if err != nil {
		t.Fatalf("filtered query: %v", err)
	}

	for _, r := range rows {
		if strings.ToLower(r.Format) != "standard" {
			t.Errorf("format filter leaked non-Standard row: format=%q id=%q", r.Format, r.ID)
		}
	}

	draftID := fmt.Sprintf("cursor-flt-draft-%d", accountID)
	for _, r := range rows {
		if r.ID == draftID {
			t.Errorf("format filter should have excluded PremierDraft row %q", draftID)
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
