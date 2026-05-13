package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// ----------------------------------------------------------------------------
// Seeding helpers
// ----------------------------------------------------------------------------

func insertTestSet(t *testing.T, db *sql.DB, code, name string) {
	t.Helper()
	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO sets (code, name) VALUES ($1, $2) ON CONFLICT (code) DO NOTHING`,
		code, name,
	)
	if err != nil {
		t.Fatalf("insertTestSet %q: %v", code, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM sets WHERE code = $1`, code)
	})
}

type setCardSeed struct {
	SetCode         string
	ArenaID         string
	Name            string
	Rarity          string
	Colors          string
	PriceUSD        *float64
	PriceEUR        *float64
	PriceUSDFoil    *float64
	PricesUpdatedAt *time.Time
}

func insertTestSetCard(t *testing.T, db *sql.DB, s setCardSeed) {
	t.Helper()
	updatedAt := sql.NullTime{}
	if s.PricesUpdatedAt != nil {
		updatedAt = sql.NullTime{Valid: true, Time: *s.PricesUpdatedAt}
	}
	_, err := db.ExecContext(
		context.Background(), `
		INSERT INTO set_cards
			(set_code, arena_id, scryfall_id, name, rarity, colors, price_usd, price_usd_foil, price_eur, prices_updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (set_code, arena_id) DO NOTHING`,
		s.SetCode, s.ArenaID, "scryfall-"+s.ArenaID+"-"+s.SetCode,
		s.Name, s.Rarity, s.Colors, s.PriceUSD, s.PriceUSDFoil, s.PriceEUR, updatedAt,
	)
	if err != nil {
		t.Fatalf("insertTestSetCard arena_id=%q set=%q: %v", s.ArenaID, s.SetCode, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			`DELETE FROM set_cards WHERE set_code = $1 AND arena_id = $2`, s.SetCode, s.ArenaID,
		)
	})
}

func insertTestInventory(t *testing.T, db *sql.DB, accountID int64, cardID int, count int) {
	t.Helper()
	hash := fmt.Sprintf("hash-%d-%d", cardID, count)
	_, err := db.ExecContext(
		context.Background(), `
		INSERT INTO card_inventory (account_id, card_id, count, snapshot_hash)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (account_id, card_id) DO UPDATE SET count = EXCLUDED.count, snapshot_hash = EXCLUDED.snapshot_hash`,
		accountID, cardID, count, hash,
	)
	if err != nil {
		t.Fatalf("insertTestInventory account=%d card=%d: %v", accountID, cardID, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			`DELETE FROM card_inventory WHERE account_id = $1 AND card_id = $2`, accountID, cardID,
		)
	})
}

func ptr[T any](v T) *T { return &v }

// ----------------------------------------------------------------------------
// CollectionRepository.ListCollection
// ----------------------------------------------------------------------------

func TestCollectionRepository_ListCollection_WithMetadata(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCollectionRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, "col-list-meta")
	insertTestSet(t, db, "TST", "Test Set")
	now := time.Now().UTC().Truncate(time.Second)
	insertTestSetCard(t, db, setCardSeed{
		SetCode:         "TST",
		ArenaID:         "99001",
		Name:            "Lightning Bolt",
		Rarity:          "uncommon",
		Colors:          `["R"]`,
		PriceUSD:        ptr(1.23),
		PriceEUR:        ptr(0.99),
		PriceUSDFoil:    ptr(2.50),
		PricesUpdatedAt: &now,
	})
	insertTestInventory(t, db, accountID, 99001, 3)

	items, err := repo.ListCollection(ctx, accountID, repository.CollectionFilter{})
	if err != nil {
		t.Fatalf("ListCollection: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	item := items[0]
	if item.Name != "Lightning Bolt" {
		t.Errorf("Name: got %q, want %q", item.Name, "Lightning Bolt")
	}
	if item.Quantity != 3 {
		t.Errorf("Quantity: got %d, want 3", item.Quantity)
	}
	if item.SetCode != "TST" {
		t.Errorf("SetCode: got %q, want TST", item.SetCode)
	}
	if item.SetName != "Test Set" {
		t.Errorf("SetName: got %q, want Test Set", item.SetName)
	}
	if item.Rarity != "uncommon" {
		t.Errorf("Rarity: got %q, want uncommon", item.Rarity)
	}
	if item.PriceUSD == nil || *item.PriceUSD != 1.23 {
		t.Errorf("PriceUSD: got %v, want 1.23", item.PriceUSD)
	}
	if item.PricesUpdated == nil {
		t.Error("PricesUpdated should be non-nil")
	}
}

func TestCollectionRepository_ListCollection_NoMetadata(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCollectionRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, "col-list-nometa")
	// No set_cards row — card_inventory exists but set_cards has no match.
	insertTestInventory(t, db, accountID, 88001, 1)

	items, err := repo.ListCollection(ctx, accountID, repository.CollectionFilter{})
	if err != nil {
		t.Fatalf("ListCollection: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item (unmatched card), got %d", len(items))
	}
	item := items[0]
	if item.Name != "" {
		t.Errorf("Name: expected empty string for unmatched card, got %q", item.Name)
	}
	if item.Quantity != 1 {
		t.Errorf("Quantity: got %d, want 1", item.Quantity)
	}
	if item.PriceUSD != nil {
		t.Errorf("PriceUSD: expected nil for unmatched card, got %v", item.PriceUSD)
	}
}

// TestCollectionRepository_ListCollection_NoDuplicatesForReprint verifies the
// DISTINCT ON dedup fix: a card whose arena_id appears in two set_cards rows
// must return exactly one ListCollection row (not two).
func TestCollectionRepository_ListCollection_NoDuplicatesForReprint(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCollectionRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, "col-list-dedup")
	insertTestSet(t, db, "S1", "Set One")
	insertTestSet(t, db, "S2", "Set Two")
	// Same arena_id in two different sets (reprint scenario).
	insertTestSetCard(t, db, setCardSeed{SetCode: "S1", ArenaID: "77001", Name: "Reprint Card", Rarity: "rare"})
	insertTestSetCard(t, db, setCardSeed{SetCode: "S2", ArenaID: "77001", Name: "Reprint Card", Rarity: "rare"})
	insertTestInventory(t, db, accountID, 77001, 2)

	items, err := repo.ListCollection(ctx, accountID, repository.CollectionFilter{})
	if err != nil {
		t.Fatalf("ListCollection: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("dedup: expected 1 item for reprinted card, got %d (duplicate rows indicate missing DISTINCT ON)", len(items))
	}
	if len(items) == 1 && items[0].Quantity != 2 {
		t.Errorf("Quantity: got %d, want 2", items[0].Quantity)
	}
}

// TestCollectionRepository_ListCollection_SetCodeFilterMatchesHigherIdPrinting
// verifies that SetCode filter picks the printing in the requested set even when
// a lower-id printing exists in a different set.  Before the CTE pre-filter fix,
// DISTINCT ON would collapse to the S1 row and the SetCode='S2' predicate would
// then miss the card entirely.
func TestCollectionRepository_ListCollection_SetCodeFilterMatchesHigherIdPrinting(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCollectionRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, "col-filter-setcode")
	insertTestSet(t, db, "F1", "Filter Set One")
	insertTestSet(t, db, "F2", "Filter Set Two")
	// S1 row is inserted first so it gets a lower id — the old bug would pick it.
	insertTestSetCard(t, db, setCardSeed{SetCode: "F1", ArenaID: "75001", Name: "Shared Card", Rarity: "rare"})
	insertTestSetCard(t, db, setCardSeed{SetCode: "F2", ArenaID: "75001", Name: "Shared Card", Rarity: "rare"})
	insertTestInventory(t, db, accountID, 75001, 1)

	items, err := repo.ListCollection(ctx, accountID, repository.CollectionFilter{SetCode: "F2"})
	if err != nil {
		t.Fatalf("ListCollection(SetCode=F2): %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("SetCode filter: expected 1 item, got %d (card in F2 missing from results)", len(items))
	}
	if items[0].SetCode != "F2" {
		t.Errorf("SetCode: got %q, want F2", items[0].SetCode)
	}
}

// ----------------------------------------------------------------------------
// CollectionRepository.CountCollection
// ----------------------------------------------------------------------------

func TestCollectionRepository_CountCollection(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCollectionRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, "col-count")
	insertTestInventory(t, db, accountID, 55001, 4)
	insertTestInventory(t, db, accountID, 55002, 2)

	counts, err := repo.CountCollection(ctx, accountID)
	if err != nil {
		t.Fatalf("CountCollection: %v", err)
	}
	if counts.UniqueCards != 2 {
		t.Errorf("UniqueCards: got %d, want 2", counts.UniqueCards)
	}
	if counts.TotalCards != 6 {
		t.Errorf("TotalCards: got %d, want 6", counts.TotalCards)
	}
}

// ----------------------------------------------------------------------------
// CollectionRepository.CountByRarity
// ----------------------------------------------------------------------------

func TestCollectionRepository_CountByRarity(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCollectionRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, "col-rarity")
	insertTestSet(t, db, "RAR", "Rarity Test Set")
	insertTestSetCard(t, db, setCardSeed{SetCode: "RAR", ArenaID: "66001", Name: "Rare Card", Rarity: "rare"})
	insertTestSetCard(t, db, setCardSeed{SetCode: "RAR", ArenaID: "66002", Name: "Common Card", Rarity: "common"})
	insertTestInventory(t, db, accountID, 66001, 1)
	insertTestInventory(t, db, accountID, 66002, 3)

	rows, err := repo.CountByRarity(ctx, accountID)
	if err != nil {
		t.Fatalf("CountByRarity: %v", err)
	}

	totals := map[string]int{}
	for _, r := range rows {
		totals[r.Rarity] = r.TotalCards
	}
	if totals["rare"] != 1 {
		t.Errorf("rare total: got %d, want 1", totals["rare"])
	}
	if totals["common"] != 3 {
		t.Errorf("common total: got %d, want 3", totals["common"])
	}
}

// TestCollectionRepository_CountByRarity_NoDuplicatesForReprint mirrors the
// ListCollection dedup test: rarity SUM must not be doubled for reprints.
func TestCollectionRepository_CountByRarity_NoDuplicatesForReprint(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCollectionRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, "col-rarity-dedup")
	insertTestSet(t, db, "R1", "Rarity One")
	insertTestSet(t, db, "R2", "Rarity Two")
	insertTestSetCard(t, db, setCardSeed{SetCode: "R1", ArenaID: "44001", Name: "Mythic Reprint", Rarity: "mythic"})
	insertTestSetCard(t, db, setCardSeed{SetCode: "R2", ArenaID: "44001", Name: "Mythic Reprint", Rarity: "mythic"})
	insertTestInventory(t, db, accountID, 44001, 2)

	rows, err := repo.CountByRarity(ctx, accountID)
	if err != nil {
		t.Fatalf("CountByRarity: %v", err)
	}
	for _, r := range rows {
		if r.Rarity == "mythic" && r.TotalCards != 2 {
			t.Errorf("mythic total: got %d, want 2 (dedup failure — reprint doubled the count)", r.TotalCards)
		}
	}
}

// ----------------------------------------------------------------------------
// CollectionRepository.ValueRows
// ----------------------------------------------------------------------------

func TestCollectionRepository_ValueRows_MixedPriced(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCollectionRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, "col-value")
	insertTestSet(t, db, "VAL", "Value Test Set")
	insertTestSetCard(t, db, setCardSeed{
		SetCode: "VAL", ArenaID: "33001", Name: "Priced Card", Rarity: "rare",
		PriceUSD: ptr(5.00), PriceEUR: ptr(4.00),
	})
	insertTestSetCard(t, db, setCardSeed{
		SetCode: "VAL", ArenaID: "33002", Name: "Unpriced Card", Rarity: "common",
	})
	insertTestInventory(t, db, accountID, 33001, 2)
	insertTestInventory(t, db, accountID, 33002, 4)

	rows, unpricedCount, err := repo.ValueRows(ctx, accountID)
	if err != nil {
		t.Fatalf("ValueRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 priced row, got %d", len(rows))
	}
	if rows[0].Name != "Priced Card" {
		t.Errorf("priced row Name: got %q, want Priced Card", rows[0].Name)
	}
	if rows[0].PriceUSD != 5.00 {
		t.Errorf("PriceUSD: got %v, want 5.00", rows[0].PriceUSD)
	}
	if rows[0].Quantity != 2 {
		t.Errorf("Quantity: got %d, want 2", rows[0].Quantity)
	}
	if unpricedCount != 1 {
		t.Errorf("unpricedCount: got %d, want 1", unpricedCount)
	}
}

// ----------------------------------------------------------------------------
// CollectionRepository.SetCompletion + LastPriceUpdate
// ----------------------------------------------------------------------------

func TestCollectionRepository_SetCompletion(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCollectionRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, "col-setcomp")
	insertTestSet(t, db, "SCP", "Set Completion Set")
	insertTestSetCard(t, db, setCardSeed{SetCode: "SCP", ArenaID: "22001", Name: "Card One", Rarity: "common"})
	insertTestSetCard(t, db, setCardSeed{SetCode: "SCP", ArenaID: "22002", Name: "Card Two", Rarity: "rare"})
	// Own only the first card.
	insertTestInventory(t, db, accountID, 22001, 1)

	rows, err := repo.SetCompletion(ctx, accountID)
	if err != nil {
		t.Fatalf("SetCompletion: %v", err)
	}

	var found bool
	for _, r := range rows {
		if r.SetCode == "SCP" {
			found = true
			if r.TotalCards != 2 {
				t.Errorf("TotalCards: got %d, want 2", r.TotalCards)
			}
			if r.OwnedCards != 1 {
				t.Errorf("OwnedCards: got %d, want 1", r.OwnedCards)
			}
			if r.SetName != "Set Completion Set" {
				t.Errorf("SetName: got %q, want Set Completion Set", r.SetName)
			}
		}
	}
	if !found {
		t.Error("SCP set not found in SetCompletion result")
	}
}

func TestCollectionRepository_LastPriceUpdate(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewCollectionRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, "col-price-ts")
	insertTestSet(t, db, "PTS", "Price Timestamp Set")
	now := time.Now().UTC().Truncate(time.Second)
	insertTestSetCard(t, db, setCardSeed{
		SetCode: "PTS", ArenaID: "11001", Name: "Timestamped Card", Rarity: "rare",
		PricesUpdatedAt: &now,
	})
	insertTestInventory(t, db, accountID, 11001, 1)

	ts, err := repo.LastPriceUpdate(ctx, accountID)
	if err != nil {
		t.Fatalf("LastPriceUpdate: %v", err)
	}
	if ts == 0 {
		t.Error("LastPriceUpdate: expected non-zero unix timestamp, got 0")
	}
	if ts != now.Unix() {
		t.Errorf("LastPriceUpdate: got %d, want %d", ts, now.Unix())
	}
}
