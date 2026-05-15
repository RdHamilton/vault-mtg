package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// insertTestQuest inserts a minimal quests row for the given account using the
// repo's own UpsertQuestProgress method. The row is removed via t.Cleanup.
func insertTestQuest(t *testing.T, db *sql.DB, repo *repository.QuestRepository, accountID int64, questID string, seenAt time.Time) {
	t.Helper()

	err := repo.UpsertQuestProgress(context.Background(), repository.QuestProgressUpsert{
		AccountID: accountID,
		QuestID:   questID,
		QuestName: "test-quest",
		Progress:  0,
		Goal:      15,
		CanSwap:   true,
		SeenAt:    seenAt,
	})
	if err != nil {
		t.Fatalf("insertTestQuest %q: %v", questID, err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM quests WHERE quest_id = $1 AND account_id = $2`, questID, accountID)
	})
}

// TestQuestRepository_LastQuestSeenAt_NoQuests verifies that an account with
// no quest rows returns (zero, false, nil) — no panic, no scan error.
// This is the NULL-scan path that the **time.Time bug caused to panic.
func TestQuestRepository_LastQuestSeenAt_NoQuests(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewQuestRepository(db)

	accountID := insertTestAccount(t, db, fmt.Sprintf("quest-test-empty-%d", time.Now().UnixNano()))

	got, ok, err := repo.LastQuestSeenAt(context.Background(), accountID)
	if err != nil {
		t.Fatalf("LastQuestSeenAt: %v", err)
	}
	if ok {
		t.Errorf("expected ok=false for account with no quests, got true (ts=%v)", got)
	}
	if !got.IsZero() {
		t.Errorf("expected zero time for account with no quests, got %v", got)
	}
}

// TestQuestRepository_LastQuestSeenAt_NonNullTimestamp verifies that when a
// quest row exists, LastQuestSeenAt returns ok=true and the correct timestamp.
func TestQuestRepository_LastQuestSeenAt_NonNullTimestamp(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewQuestRepository(db)

	accountID := insertTestAccount(t, db, fmt.Sprintf("quest-test-ts-%d", time.Now().UnixNano()))

	seenAt := time.Now().UTC().Truncate(time.Second)
	insertTestQuest(t, db, repo, accountID, fmt.Sprintf("quest-ts-%d", accountID), seenAt)

	got, ok, err := repo.LastQuestSeenAt(context.Background(), accountID)
	if err != nil {
		t.Fatalf("LastQuestSeenAt: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true for account with a quest row, got false")
	}
	if !got.Equal(seenAt) {
		t.Errorf("timestamp mismatch: want %v, got %v", seenAt, got)
	}
}

// TestQuestRepository_LastQuestSeenAt_NullLastSeenAtFallsBackToAssignedAt
// verifies that when last_seen_at is NULL, the COALESCE falls back to
// assigned_at and still returns ok=true.
func TestQuestRepository_LastQuestSeenAt_NullLastSeenAtFallsBackToAssignedAt(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewQuestRepository(db)

	accountID := insertTestAccount(t, db, fmt.Sprintf("quest-test-null-lsa-%d", time.Now().UnixNano()))

	assignedAt := time.Now().UTC().Truncate(time.Second)
	questID := fmt.Sprintf("quest-null-lsa-%d", accountID)

	insertTestQuest(t, db, repo, accountID, questID, assignedAt)

	// NULL out last_seen_at to exercise the COALESCE(MAX(last_seen_at), MAX(assigned_at)) fallback.
	_, err := db.ExecContext(
		context.Background(),
		`UPDATE quests SET last_seen_at = NULL WHERE quest_id = $1 AND account_id = $2`,
		questID, accountID,
	)
	if err != nil {
		t.Fatalf("nullify last_seen_at: %v", err)
	}

	got, ok, err := repo.LastQuestSeenAt(context.Background(), accountID)
	if err != nil {
		t.Fatalf("LastQuestSeenAt: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true when last_seen_at is NULL but assigned_at exists, got false")
	}
	if !got.Equal(assignedAt) {
		t.Errorf("fallback timestamp mismatch: want %v, got %v", assignedAt, got)
	}
}

// ─── Issue #1924: account-scoped unique constraint tests ──────────────────────

// TestQuestRepository_UpsertQuestProgress_CrossTenantCollisionSucceeds is the
// primary AC1 regression test for issue #1924.
//
// Two different accounts are assigned the same quest_id at the exact same
// assigned_at timestamp.  Prior to migration 000078 the old constraint
// UNIQUE(quest_id, assigned_at) would cause the second INSERT to fail with a
// constraint violation.  After the migration the constraint is
// UNIQUE(account_id, quest_id, assigned_at) and both inserts must succeed.
func TestQuestRepository_UpsertQuestProgress_CrossTenantCollisionSucceeds(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewQuestRepository(db)
	ctx := context.Background()

	accountA := insertTestAccount(t, db, fmt.Sprintf("quest-collision-a-%d", time.Now().UnixNano()))
	accountB := insertTestAccount(t, db, fmt.Sprintf("quest-collision-b-%d", time.Now().UnixNano()))

	// Use the exact same quest_id and assigned_at for both accounts —
	// this is precisely the collision that issue #1924 describes.
	questID := "quest-shared-daily-001"
	assignedAt := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM quests WHERE account_id = $1 AND quest_id = $2`, accountA, questID)
		_, _ = db.ExecContext(ctx, `DELETE FROM quests WHERE account_id = $1 AND quest_id = $2`, accountB, questID)
	})

	if err := repo.UpsertQuestProgress(ctx, repository.QuestProgressUpsert{
		AccountID: accountA,
		QuestID:   questID,
		QuestName: "Win 3 Games",
		Progress:  1,
		Goal:      3,
		CanSwap:   true,
		SeenAt:    assignedAt,
	}); err != nil {
		t.Fatalf("UpsertQuestProgress account A: %v", err)
	}

	// This second insert must NOT fail with a unique constraint violation.
	if err := repo.UpsertQuestProgress(ctx, repository.QuestProgressUpsert{
		AccountID: accountB,
		QuestID:   questID,
		QuestName: "Win 3 Games",
		Progress:  2,
		Goal:      3,
		CanSwap:   true,
		SeenAt:    assignedAt,
	}); err != nil {
		t.Fatalf("UpsertQuestProgress account B (cross-tenant collision): %v", err)
	}

	// Both rows must exist independently.
	var countA, countB int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM quests WHERE account_id = $1 AND quest_id = $2 AND assigned_at = $3`,
		accountA, questID, assignedAt,
	).Scan(&countA); err != nil {
		t.Fatalf("count query account A: %v", err)
	}
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM quests WHERE account_id = $1 AND quest_id = $2 AND assigned_at = $3`,
		accountB, questID, assignedAt,
	).Scan(&countB); err != nil {
		t.Fatalf("count query account B: %v", err)
	}

	if countA != 1 {
		t.Errorf("account A: expected 1 quest row, got %d", countA)
	}
	if countB != 1 {
		t.Errorf("account B: expected 1 quest row, got %d", countB)
	}
}

// TestQuestRepository_UpsertQuestProgress_SameAccountIdempotent verifies that
// two upserts for the same (account_id, quest_id, assigned_at) correctly
// update in place — the ON CONFLICT clause targets the new account-scoped
// constraint after migration 000078.
func TestQuestRepository_UpsertQuestProgress_SameAccountIdempotent(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewQuestRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, fmt.Sprintf("quest-idempotent-%d", time.Now().UnixNano()))

	questID := fmt.Sprintf("quest-idem-%d", accountID)
	assignedAt := time.Date(2026, 5, 14, 9, 0, 0, 0, time.UTC)

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM quests WHERE account_id = $1 AND quest_id = $2`, accountID, questID)
	})

	// First upsert — progress = 1.
	if err := repo.UpsertQuestProgress(ctx, repository.QuestProgressUpsert{
		AccountID: accountID,
		QuestID:   questID,
		QuestName: "Cast 5 Spells",
		Progress:  1,
		Goal:      5,
		CanSwap:   true,
		SeenAt:    assignedAt,
	}); err != nil {
		t.Fatalf("first UpsertQuestProgress: %v", err)
	}

	// Second upsert — same key, updated progress.
	if err := repo.UpsertQuestProgress(ctx, repository.QuestProgressUpsert{
		AccountID: accountID,
		QuestID:   questID,
		QuestName: "Cast 5 Spells",
		Progress:  3,
		Goal:      5,
		CanSwap:   true,
		SeenAt:    assignedAt,
	}); err != nil {
		t.Fatalf("second UpsertQuestProgress: %v", err)
	}

	// Exactly one row must exist; ending_progress must reflect the update.
	var count, endingProgress int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*), MAX(ending_progress) FROM quests WHERE account_id = $1 AND quest_id = $2`,
		accountID, questID,
	).Scan(&count, &endingProgress); err != nil {
		t.Fatalf("count/progress query: %v", err)
	}

	if count != 1 {
		t.Errorf("expected exactly 1 quest row after idempotent upsert, got %d", count)
	}
	if endingProgress != 3 {
		t.Errorf("expected ending_progress=3 after update, got %d", endingProgress)
	}
}

// TestQuestRepository_UpsertQuestProgress_CrossTenantDataIsolation verifies
// that updating one account's quest row does not mutate the other account's
// row, even when they share the same quest_id and assigned_at.
func TestQuestRepository_UpsertQuestProgress_CrossTenantDataIsolation(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewQuestRepository(db)
	ctx := context.Background()

	accountA := insertTestAccount(t, db, fmt.Sprintf("quest-isolation-a-%d", time.Now().UnixNano()))
	accountB := insertTestAccount(t, db, fmt.Sprintf("quest-isolation-b-%d", time.Now().UnixNano()))

	questID := fmt.Sprintf("quest-iso-%d", time.Now().UnixNano())
	assignedAt := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM quests WHERE quest_id = $1`, questID)
	})

	// Seed both accounts with progress = 1.
	for _, id := range []int64{accountA, accountB} {
		if err := repo.UpsertQuestProgress(ctx, repository.QuestProgressUpsert{
			AccountID: id,
			QuestID:   questID,
			QuestName: "Win 5 Games",
			Progress:  1,
			Goal:      5,
			CanSwap:   false,
			SeenAt:    assignedAt,
		}); err != nil {
			t.Fatalf("seed UpsertQuestProgress account %d: %v", id, err)
		}
	}

	// Advance account A to progress = 4.
	if err := repo.UpsertQuestProgress(ctx, repository.QuestProgressUpsert{
		AccountID: accountA,
		QuestID:   questID,
		QuestName: "Win 5 Games",
		Progress:  4,
		Goal:      5,
		CanSwap:   false,
		SeenAt:    assignedAt,
	}); err != nil {
		t.Fatalf("update UpsertQuestProgress account A: %v", err)
	}

	// Account A must be at 4; account B must remain at 1.
	var progressA, progressB int
	if err := db.QueryRowContext(
		ctx,
		`SELECT ending_progress FROM quests WHERE account_id = $1 AND quest_id = $2`,
		accountA, questID,
	).Scan(&progressA); err != nil {
		t.Fatalf("query account A ending_progress: %v", err)
	}
	if err := db.QueryRowContext(
		ctx,
		`SELECT ending_progress FROM quests WHERE account_id = $1 AND quest_id = $2`,
		accountB, questID,
	).Scan(&progressB); err != nil {
		t.Fatalf("query account B ending_progress: %v", err)
	}

	if progressA != 4 {
		t.Errorf("account A ending_progress: want 4, got %d", progressA)
	}
	if progressB != 1 {
		t.Errorf("account B ending_progress: want 1 (unchanged), got %d", progressB)
	}
}
