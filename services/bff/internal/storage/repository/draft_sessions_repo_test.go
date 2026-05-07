package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// insertTestDraftSession inserts a minimal draft_sessions row for the given account.
// The row (and any associated draft_match_results) is removed via t.Cleanup.
func insertTestDraftSession(t *testing.T, db *sql.DB, sessionID string, accountID int64, setCode string, startTime time.Time) {
	t.Helper()

	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO draft_sessions
			(id, account_id, event_name, set_code, draft_type, start_time, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		sessionID, accountID, "event-"+sessionID, setCode, "PremierDraft", startTime, "completed",
	)
	if err != nil {
		t.Fatalf("insertTestDraftSession %q: %v", sessionID, err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM draft_match_results WHERE session_id = $1`, sessionID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM draft_sessions WHERE id = $1`, sessionID)
	})
}

// insertTestDraftMatchResult inserts a draft_match_results row for the given session.
// The session cleanup (registered by insertTestDraftSession) handles cascade deletion.
func insertTestDraftMatchResult(t *testing.T, db *sql.DB, sessionID, matchID, result string, ts time.Time) {
	t.Helper()

	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO draft_match_results
			(session_id, match_id, result, match_timestamp)
		 VALUES ($1, $2, $3, $4)`,
		sessionID, matchID, result, ts,
	)
	if err != nil {
		t.Fatalf("insertTestDraftMatchResult session=%q match=%q: %v", sessionID, matchID, err)
	}
}

// TestDraftSessionsRepository_ListByAccountID_ReturnsOnlyOwnRows verifies
// cross-account isolation: account A cannot see account B's draft sessions.
func TestDraftSessionsRepository_ListByAccountID_ReturnsOnlyOwnRows(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDraftSessionsRepository(db)

	accountA := insertTestAccount(t, db, "draft-test-account-a")
	accountB := insertTestAccount(t, db, "draft-test-account-b")

	now := time.Now().UTC().Truncate(time.Second)

	insertTestDraftSession(t, db, fmt.Sprintf("ds-iso-a-%d", accountA), accountA, "ONE", now)
	insertTestDraftSession(t, db, fmt.Sprintf("ds-iso-b-%d", accountB), accountB, "ONE", now)

	rows, _, err := repo.ListByAccountID(context.Background(), accountA, "", 1, 100)
	if err != nil {
		t.Fatalf("ListByAccountID: %v", err)
	}

	bID := fmt.Sprintf("ds-iso-b-%d", accountB)

	for _, r := range rows {
		if r.ID == bID {
			t.Errorf("cross-account leak: accountA query returned accountB session %q", r.ID)
		}
	}
}

// TestDraftSessionsRepository_ListByAccountID_EmptyAccount verifies that an
// account with no sessions returns an empty slice (not an error).
func TestDraftSessionsRepository_ListByAccountID_EmptyAccount(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDraftSessionsRepository(db)

	accountID := insertTestAccount(t, db, "draft-test-empty")

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

// TestDraftSessionsRepository_ListByAccountID_Pagination verifies offset/limit
// paging and ordering (start_time DESC).
func TestDraftSessionsRepository_ListByAccountID_Pagination(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDraftSessionsRepository(db)

	accountID := insertTestAccount(t, db, "draft-test-pagination")

	base := time.Now().UTC().Truncate(time.Second)

	// Insert 3 sessions at different start_times.
	for i := 0; i < 3; i++ {
		ts := base.Add(time.Duration(i) * time.Hour)
		insertTestDraftSession(t, db, fmt.Sprintf("ds-page-%d-%d", accountID, i), accountID, "BLB", ts)
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

	// Newest first: page1[0].StartTime >= page1[1].StartTime.
	if page1[0].StartTime.Before(page1[1].StartTime) {
		t.Errorf("expected DESC order: page1[0]=%v < page1[1]=%v", page1[0].StartTime, page1[1].StartTime)
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

// TestDraftSessionsRepository_ListByAccountID_SetCodeFilter verifies that the
// optional setCode filter restricts results correctly.
func TestDraftSessionsRepository_ListByAccountID_SetCodeFilter(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDraftSessionsRepository(db)

	accountID := insertTestAccount(t, db, "draft-test-setcode-filter")

	now := time.Now().UTC().Truncate(time.Second)

	insertTestDraftSession(t, db, fmt.Sprintf("ds-set-one-%d", accountID), accountID, "ONE", now)
	insertTestDraftSession(t, db, fmt.Sprintf("ds-set-blb-%d", accountID), accountID, "BLB", now.Add(-time.Second))

	oneRows, oneTotal, err := repo.ListByAccountID(context.Background(), accountID, "ONE", 1, 10)
	if err != nil {
		t.Fatalf("ListByAccountID (ONE): %v", err)
	}

	if oneTotal != 1 {
		t.Errorf("setCode filter total: want 1, got %d", oneTotal)
	}

	for _, r := range oneRows {
		if r.SetCode != "ONE" {
			t.Errorf("setCode filter returned wrong set %q", r.SetCode)
		}
	}
}

// TestDraftSessionsRepository_ListByAccountID_WinsLossesAggregated verifies that
// the wins/losses columns are computed correctly via the LEFT JOIN on
// draft_match_results.
func TestDraftSessionsRepository_ListByAccountID_WinsLossesAggregated(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDraftSessionsRepository(db)

	accountID := insertTestAccount(t, db, "draft-test-wl-agg")

	now := time.Now().UTC().Truncate(time.Second)
	sessionID := fmt.Sprintf("ds-wl-%d", accountID)

	insertTestDraftSession(t, db, sessionID, accountID, "MKM", now)

	// Seed 2 wins and 1 loss.
	insertTestDraftMatchResult(t, db, sessionID, "match-w1", "win", now.Add(time.Minute))
	insertTestDraftMatchResult(t, db, sessionID, "match-w2", "win", now.Add(2*time.Minute))
	insertTestDraftMatchResult(t, db, sessionID, "match-l1", "loss", now.Add(3*time.Minute))

	rows, _, err := repo.ListByAccountID(context.Background(), accountID, "", 1, 10)
	if err != nil {
		t.Fatalf("ListByAccountID: %v", err)
	}

	var found *repository.DraftSessionRow

	for i := range rows {
		if rows[i].ID == sessionID {
			found = &rows[i]
			break
		}
	}

	if found == nil {
		t.Fatalf("seeded session %q not found in results", sessionID)
	}

	if found.Wins != 2 {
		t.Errorf("wins: want 2, got %d", found.Wins)
	}

	if found.Losses != 1 {
		t.Errorf("losses: want 1, got %d", found.Losses)
	}
}

// TestDraftSessionsRepository_UpsertDraftSession_InsertAndUpdate verifies that
// UpsertDraftSession creates a new row on first call and updates it on second
// call with the same id.
func TestDraftSessionsRepository_UpsertDraftSession_InsertAndUpdate(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDraftSessionsRepository(db)

	accountID := insertTestAccount(t, db, "draft-test-upsert")

	sessionID := fmt.Sprintf("ds-upsert-%d", accountID)
	now := time.Now().UTC().Truncate(time.Second)

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM draft_sessions WHERE id = $1`, sessionID)
	})

	s := repository.DraftSessionUpsert{
		ID:        sessionID,
		AccountID: accountID,
		EventName: "TestEvent",
		SetCode:   "ONE",
		DraftType: "PremierDraft",
		StartTime: now,
		Status:    "in_progress",
	}

	if err := repo.UpsertDraftSession(context.Background(), s); err != nil {
		t.Fatalf("first UpsertDraftSession: %v", err)
	}

	// Update status to completed.
	s.Status = "completed"
	s.TotalPicks = 42

	if err := repo.UpsertDraftSession(context.Background(), s); err != nil {
		t.Fatalf("second UpsertDraftSession: %v", err)
	}

	// Verify status updated and total_picks is GREATEST(42, 0) = 42.
	var status string
	var picks int

	err := db.QueryRowContext(
		context.Background(),
		`SELECT status, total_picks FROM draft_sessions WHERE id = $1`,
		sessionID,
	).Scan(&status, &picks)
	if err != nil {
		t.Fatalf("select after upsert: %v", err)
	}

	if status != "completed" {
		t.Errorf("status: want completed, got %q", status)
	}

	if picks != 42 {
		t.Errorf("total_picks: want 42, got %d", picks)
	}
}

// TestDraftSessionsRepository_Interface is a compile-time check.
func TestDraftSessionsRepository_Interface(t *testing.T) {
	var db repository.DB = &fakeDB{}
	repo := repository.NewDraftSessionsRepository(db)

	if repo == nil {
		t.Fatal("NewDraftSessionsRepository returned nil")
	}
}
