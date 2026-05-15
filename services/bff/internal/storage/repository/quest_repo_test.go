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
