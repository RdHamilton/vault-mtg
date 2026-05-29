package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
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

// countQuestRows returns the number of quests rows for the given
// (account_id, quest_id) pair.
func countQuestRows(t *testing.T, db *sql.DB, accountID int64, questID string) int {
	t.Helper()

	var n int
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT COUNT(*) FROM quests WHERE account_id = $1 AND quest_id = $2`,
		accountID, questID,
	).Scan(&n); err != nil {
		t.Fatalf("countQuestRows account=%d quest=%q: %v", accountID, questID, err)
	}

	return n
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

// ─── Issue #1924 / #204: account-scoped unique constraint tests ───────────────

// TestQuestRepository_UpsertQuestProgress_CrossTenantCollisionSucceeds is the
// primary AC1 regression test for issue #1924.
//
// Two different accounts are assigned the same quest_id at the exact same
// seen_at timestamp.  The unique constraint UNIQUE(account_id, quest_id) (added
// by migration 000096, replacing the 3-column constraint from migration 000078)
// scopes each row to a single account, so both inserts must succeed independently.
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
// two upserts for the same (account_id, quest_id) correctly update in place —
// the ON CONFLICT clause targets the 2-column constraint (account_id, quest_id)
// introduced by migration 000096.
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

// ─── Issue #204: 2-column constraint dedup tests ──────────────────────────────

// TestQuestRepository_UpsertQuestProgress_NoDupOnResighting verifies AC1/AC2/AC5:
// multiple sync events for the same (account_id, quest_id) at different seen_at
// timestamps produce exactly one row and update progress in place.
//
// This is the primary regression test for issue #204.  With the old 3-column
// constraint (account_id, quest_id, assigned_at), each distinct seen_at caused
// a fresh INSERT, accumulating one duplicate row per sync cycle.  With the new
// 2-column constraint (account_id, quest_id) all events collapse onto a single
// row via ON CONFLICT DO UPDATE.
func TestQuestRepository_UpsertQuestProgress_NoDupOnResighting(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewQuestRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, fmt.Sprintf("quest-nodup-%d", time.Now().UnixNano()))
	questID := fmt.Sprintf("quest-nodup-%d", time.Now().UnixNano())

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM quests WHERE account_id = $1 AND quest_id = $2`, accountID, questID)
	})

	base := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)

	// Simulate three separate sync cycles at distinct timestamps — the old bug
	// would insert three rows; after the fix exactly one row must exist.
	for i, progress := range []int{0, 2, 5} {
		seenAt := base.Add(time.Duration(i) * time.Minute)
		if err := repo.UpsertQuestProgress(ctx, repository.QuestProgressUpsert{
			AccountID: accountID,
			QuestID:   questID,
			QuestName: "Win 5 Games",
			Progress:  progress,
			Goal:      5,
			CanSwap:   true,
			SeenAt:    seenAt,
		}); err != nil {
			t.Fatalf("UpsertQuestProgress cycle %d: %v", i, err)
		}
	}

	count := countQuestRows(t, db, accountID, questID)
	if count != 1 {
		t.Errorf("expected exactly 1 row after 3 sync events, got %d (duplicate rows detected — issue #204 regression)", count)
	}

	// The surviving row must carry the final progress value.
	var endingProgress int
	if err := db.QueryRowContext(
		ctx,
		`SELECT ending_progress FROM quests WHERE account_id = $1 AND quest_id = $2`,
		accountID, questID,
	).Scan(&endingProgress); err != nil {
		t.Fatalf("query ending_progress: %v", err)
	}
	if endingProgress != 5 {
		t.Errorf("ending_progress: want 5 (latest sync), got %d", endingProgress)
	}
}

// TestQuestRepository_UpsertQuestProgress_DifferentQuestsDistinct verifies AC3:
// two different quest_ids for the same account produce two separate rows and are
// not collapsed by the ON CONFLICT clause.
func TestQuestRepository_UpsertQuestProgress_DifferentQuestsDistinct(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewQuestRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, fmt.Sprintf("quest-distinct-%d", time.Now().UnixNano()))
	questA := fmt.Sprintf("quest-distinct-a-%d", time.Now().UnixNano())
	questB := fmt.Sprintf("quest-distinct-b-%d", time.Now().UnixNano())

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM quests WHERE account_id = $1 AND quest_id IN ($2, $3)`, accountID, questA, questB)
	})

	seenAt := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)

	for _, qid := range []string{questA, questB} {
		if err := repo.UpsertQuestProgress(ctx, repository.QuestProgressUpsert{
			AccountID: accountID,
			QuestID:   qid,
			QuestName: "Win 3 Games",
			Progress:  1,
			Goal:      3,
			CanSwap:   false,
			SeenAt:    seenAt,
		}); err != nil {
			t.Fatalf("UpsertQuestProgress %q: %v", qid, err)
		}
	}

	if n := countQuestRows(t, db, accountID, questA); n != 1 {
		t.Errorf("quest A: expected 1 row, got %d", n)
	}
	if n := countQuestRows(t, db, accountID, questB); n != 1 {
		t.Errorf("quest B: expected 1 row, got %d", n)
	}
}

// TestQuestRepository_UpsertQuestProgress_CrossAccountDistinct verifies AC3:
// the same quest_id for two different accounts produces two rows — each account
// owns its own row independently under the (account_id, quest_id) constraint.
func TestQuestRepository_UpsertQuestProgress_CrossAccountDistinct(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewQuestRepository(db)
	ctx := context.Background()

	accountA := insertTestAccount(t, db, fmt.Sprintf("quest-xacct-a-%d", time.Now().UnixNano()))
	accountB := insertTestAccount(t, db, fmt.Sprintf("quest-xacct-b-%d", time.Now().UnixNano()))
	questID := fmt.Sprintf("quest-xacct-%d", time.Now().UnixNano())

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM quests WHERE quest_id = $1`, questID)
	})

	seenAt := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)

	for _, id := range []int64{accountA, accountB} {
		if err := repo.UpsertQuestProgress(ctx, repository.QuestProgressUpsert{
			AccountID: id,
			QuestID:   questID,
			QuestName: "Cast 10 Spells",
			Progress:  3,
			Goal:      10,
			CanSwap:   true,
			SeenAt:    seenAt,
		}); err != nil {
			t.Fatalf("UpsertQuestProgress account %d: %v", id, err)
		}
	}

	if n := countQuestRows(t, db, accountA, questID); n != 1 {
		t.Errorf("account A: expected 1 row, got %d", n)
	}
	if n := countQuestRows(t, db, accountB, questID); n != 1 {
		t.Errorf("account B: expected 1 row, got %d", n)
	}
}
