package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// insertTestAccountForInventory inserts a minimal accounts row and returns its
// auto-assigned id.  Removed via t.Cleanup so it does not conflict with
// insertTestAccount defined in matches_repo_test.go.
func insertTestAccountForInventory(t *testing.T, db *sql.DB, suffix string) int64 {
	t.Helper()

	name := fmt.Sprintf("inv-test-account-%s", suffix)
	var id int64
	err := db.QueryRowContext(
		context.Background(),
		`INSERT INTO accounts (name) VALUES ($1) RETURNING id`,
		name,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insertTestAccountForInventory %q: %v", name, err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM accounts WHERE id = $1`, id)
	})

	return id
}

func TestCardInventoryRepository_FirstInsert(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCardInventoryRepository(db)
	ctx := context.Background()

	accountID := insertTestAccountForInventory(t, db, "first-insert")

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM card_inventory WHERE account_id = $1`, accountID)
	})

	err := repo.UpsertDelta(ctx, repository.CardInventoryUpsert{
		AccountID:    accountID,
		CardID:       100001,
		Count:        4,
		SnapshotHash: "abc123",
	})
	if err != nil {
		t.Fatalf("UpsertDelta first insert: %v", err)
	}

	row, err := repo.GetByAccountAndCard(ctx, accountID, 100001)
	if err != nil {
		t.Fatalf("GetByAccountAndCard: %v", err)
	}

	if row.Count != 4 {
		t.Errorf("expected count=4, got %d", row.Count)
	}
	if row.SnapshotHash != "abc123" {
		t.Errorf("expected snapshot_hash=abc123, got %q", row.SnapshotHash)
	}
	if row.AccountID != accountID {
		t.Errorf("expected account_id=%d, got %d", accountID, row.AccountID)
	}
	if row.CardID != 100001 {
		t.Errorf("expected card_id=100001, got %d", row.CardID)
	}
}

func TestCardInventoryRepository_Update(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCardInventoryRepository(db)
	ctx := context.Background()

	accountID := insertTestAccountForInventory(t, db, "update")

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM card_inventory WHERE account_id = $1`, accountID)
	})

	// First insert.
	if err := repo.UpsertDelta(ctx, repository.CardInventoryUpsert{
		AccountID:    accountID,
		CardID:       200001,
		Count:        2,
		SnapshotHash: "hash-v1",
	}); err != nil {
		t.Fatalf("first UpsertDelta: %v", err)
	}

	// Update with a new snapshot hash — count and hash must change.
	if err := repo.UpsertDelta(ctx, repository.CardInventoryUpsert{
		AccountID:    accountID,
		CardID:       200001,
		Count:        5,
		SnapshotHash: "hash-v2",
	}); err != nil {
		t.Fatalf("second UpsertDelta: %v", err)
	}

	row, err := repo.GetByAccountAndCard(ctx, accountID, 200001)
	if err != nil {
		t.Fatalf("GetByAccountAndCard after update: %v", err)
	}

	if row.Count != 5 {
		t.Errorf("expected updated count=5, got %d", row.Count)
	}
	if row.SnapshotHash != "hash-v2" {
		t.Errorf("expected updated snapshot_hash=hash-v2, got %q", row.SnapshotHash)
	}
}

func TestCardInventoryRepository_IdempotentReapply(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCardInventoryRepository(db)
	ctx := context.Background()

	accountID := insertTestAccountForInventory(t, db, "idempotent")

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM card_inventory WHERE account_id = $1`, accountID)
	})

	upsert := repository.CardInventoryUpsert{
		AccountID:    accountID,
		CardID:       300001,
		Count:        3,
		SnapshotHash: "fixed-hash",
	}

	// Apply three times — must be idempotent.
	for i := 0; i < 3; i++ {
		if err := repo.UpsertDelta(ctx, upsert); err != nil {
			t.Fatalf("UpsertDelta attempt %d: %v", i+1, err)
		}
	}

	row, err := repo.GetByAccountAndCard(ctx, accountID, 300001)
	if err != nil {
		t.Fatalf("GetByAccountAndCard after idempotent applies: %v", err)
	}

	if row.Count != 3 {
		t.Errorf("expected count=3 after idempotent applies, got %d", row.Count)
	}
	if row.SnapshotHash != "fixed-hash" {
		t.Errorf("expected snapshot_hash=fixed-hash, got %q", row.SnapshotHash)
	}
}

func TestCardInventoryRepository_CrossAccountIsolation(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCardInventoryRepository(db)
	ctx := context.Background()

	accountA := insertTestAccountForInventory(t, db, "isolation-a")
	accountB := insertTestAccountForInventory(t, db, "isolation-b")

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM card_inventory WHERE account_id IN ($1, $2)`, accountA, accountB)
	})

	const cardID = 400001

	if err := repo.UpsertDelta(ctx, repository.CardInventoryUpsert{
		AccountID:    accountA,
		CardID:       cardID,
		Count:        1,
		SnapshotHash: "hash-a",
	}); err != nil {
		t.Fatalf("UpsertDelta account A: %v", err)
	}

	if err := repo.UpsertDelta(ctx, repository.CardInventoryUpsert{
		AccountID:    accountB,
		CardID:       cardID,
		Count:        10,
		SnapshotHash: "hash-b",
	}); err != nil {
		t.Fatalf("UpsertDelta account B: %v", err)
	}

	rowA, err := repo.GetByAccountAndCard(ctx, accountA, cardID)
	if err != nil {
		t.Fatalf("GetByAccountAndCard account A: %v", err)
	}
	rowB, err := repo.GetByAccountAndCard(ctx, accountB, cardID)
	if err != nil {
		t.Fatalf("GetByAccountAndCard account B: %v", err)
	}

	if rowA.Count != 1 {
		t.Errorf("account A count: expected 1, got %d", rowA.Count)
	}
	if rowB.Count != 10 {
		t.Errorf("account B count: expected 10, got %d", rowB.Count)
	}
}
