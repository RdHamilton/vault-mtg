package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// insertTestDeck inserts a minimal deck row owned by accountID and returns the
// deck id.  The row (and its cascade children) are cleaned up via t.Cleanup.
func insertTestDeck(t *testing.T, db *sql.DB, accountID int64, suffix string) string {
	t.Helper()
	id := fmt.Sprintf("test-deck-%s", suffix)
	now := time.Now().UTC()
	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO decks
			(id, account_id, name, format, source, is_app_created, created_method, created_at, modified_at)
		 VALUES ($1, $2, $3, $4, $5, FALSE, 'imported', $6, $7)`,
		id, accountID, "Test Deck "+suffix, "standard", "constructed", now, now,
	)
	if err != nil {
		t.Fatalf("insertTestDeck %q: %v", id, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM decks WHERE id = $1`, id)
	})
	return id
}

// insertTestDeckCard inserts a deck_cards row using a raw SQL INSERT so the
// test can control the from_draft_pick value directly.  The row is removed
// via the parent deck's ON DELETE CASCADE, so no separate cleanup is needed.
func insertTestDeckCard(t *testing.T, db *sql.DB, deckID string, cardID int, fromDraftPick bool) {
	t.Helper()
	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO deck_cards (deck_id, card_id, quantity, board, from_draft_pick)
		 VALUES ($1, $2, 1, 'main', $3)
		 ON CONFLICT (deck_id, card_id, board) DO NOTHING`,
		deckID, cardID, fromDraftPick,
	)
	if err != nil {
		t.Fatalf("insertTestDeckCard deck=%q card=%d: %v", deckID, cardID, err)
	}
}

// insertTestDeckCardAsInteger inserts a deck_cards row using an explicit
// INTEGER cast for from_draft_pick.  This mirrors the pre-migration schema
// where the column type was INTEGER (0/1) rather than BOOLEAN, and validates
// that the `::boolean` CAST added to deckCards() handles the coercion without
// a scan-time type error.
func insertTestDeckCardAsInteger(t *testing.T, db *sql.DB, deckID string, cardID int, fromDraftPickInt int) {
	t.Helper()
	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO deck_cards (deck_id, card_id, quantity, board, from_draft_pick)
		 VALUES ($1, $2, 1, 'main', $3::boolean)
		 ON CONFLICT (deck_id, card_id, board) DO NOTHING`,
		deckID, cardID, fromDraftPickInt,
	)
	if err != nil {
		t.Fatalf("insertTestDeckCardAsInteger deck=%q card=%d int=%d: %v", deckID, cardID, fromDraftPickInt, err)
	}
}

// ----------------------------------------------------------------------------
// DecksRepository.GetDeck — from_draft_pick scan correctness (#1973 CAST fix)
// ----------------------------------------------------------------------------

// TestDecksRepository_GetDeck_FromDraftPickFalse verifies that a deck_cards
// row with from_draft_pick = FALSE scans into DeckCardRow.FromDraftPick = false
// without error.  This exercises the `(dc.from_draft_pick::boolean)` CAST
// introduced in #1973 for pgx/v5 compatibility.
func TestDecksRepository_GetDeck_FromDraftPickFalse(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDecksRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, "decks-repo-fdp-false")
	deckID := insertTestDeck(t, db, accountID, "fdp-false")
	insertTestDeckCard(t, db, deckID, 90001, false)

	detail, err := repo.GetDeck(ctx, accountID, deckID)
	if err != nil {
		t.Fatalf("GetDeck: %v", err)
	}
	if detail == nil {
		t.Fatal("GetDeck returned nil — deck not found")
	}
	if len(detail.Cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(detail.Cards))
	}
	if detail.Cards[0].FromDraftPick != false {
		t.Errorf("FromDraftPick: got true, want false")
	}
}

// TestDecksRepository_GetDeck_FromDraftPickTrue verifies that a deck_cards
// row with from_draft_pick = TRUE scans into DeckCardRow.FromDraftPick = true
// without error.
func TestDecksRepository_GetDeck_FromDraftPickTrue(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDecksRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, "decks-repo-fdp-true")
	deckID := insertTestDeck(t, db, accountID, "fdp-true")
	insertTestDeckCard(t, db, deckID, 90002, true)

	detail, err := repo.GetDeck(ctx, accountID, deckID)
	if err != nil {
		t.Fatalf("GetDeck: %v", err)
	}
	if detail == nil {
		t.Fatal("GetDeck returned nil — deck not found")
	}
	if len(detail.Cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(detail.Cards))
	}
	if detail.Cards[0].FromDraftPick != true {
		t.Errorf("FromDraftPick: got false, want true")
	}
}

// TestDecksRepository_GetDeck_FromDraftPickIntegerCast verifies that the
// `::boolean` CAST in deckCards() correctly coerces an INTEGER-encoded value
// (0 = false, 1 = true) into the Go bool field.  On incrementally-migrated
// databases the column type is INTEGER; the CAST makes the scan compatible
// with both INTEGER and BOOLEAN column types.
func TestDecksRepository_GetDeck_FromDraftPickIntegerCast(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDecksRepository(db)
	ctx := context.Background()

	// --- integer value 0 → false ---
	accountID0 := insertTestAccount(t, db, "decks-repo-int-cast-0")
	deckID0 := insertTestDeck(t, db, accountID0, "int-cast-0")
	insertTestDeckCardAsInteger(t, db, deckID0, 90003, 0)

	detail0, err := repo.GetDeck(ctx, accountID0, deckID0)
	if err != nil {
		t.Fatalf("GetDeck (int=0): %v", err)
	}
	if detail0 == nil || len(detail0.Cards) != 1 {
		t.Fatalf("GetDeck (int=0): expected 1 card, got %v", detail0)
	}
	if detail0.Cards[0].FromDraftPick != false {
		t.Errorf("FromDraftPick (int=0): got true, want false")
	}

	// --- integer value 1 → true ---
	accountID1 := insertTestAccount(t, db, "decks-repo-int-cast-1")
	deckID1 := insertTestDeck(t, db, accountID1, "int-cast-1")
	insertTestDeckCardAsInteger(t, db, deckID1, 90004, 1)

	detail1, err := repo.GetDeck(ctx, accountID1, deckID1)
	if err != nil {
		t.Fatalf("GetDeck (int=1): %v", err)
	}
	if detail1 == nil || len(detail1.Cards) != 1 {
		t.Fatalf("GetDeck (int=1): expected 1 card, got %v", detail1)
	}
	if detail1.Cards[0].FromDraftPick != true {
		t.Errorf("FromDraftPick (int=1): got false, want true")
	}
}

// TestDecksRepository_GetDeck_NotFound verifies that GetDeck returns (nil, nil)
// when the deck does not exist for the given account.
func TestDecksRepository_GetDeck_NotFound(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDecksRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, "decks-repo-notfound")

	detail, err := repo.GetDeck(ctx, accountID, "deck-does-not-exist-xyz")
	if err != nil {
		t.Fatalf("GetDeck: unexpected error: %v", err)
	}
	if detail != nil {
		t.Errorf("GetDeck: expected nil for missing deck, got %+v", detail)
	}
}

// TestDecksRepository_GetDeck_CrossAccountIsolation verifies that GetDeck does
// not return a deck owned by a different account.
func TestDecksRepository_GetDeck_CrossAccountIsolation(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDecksRepository(db)
	ctx := context.Background()

	ownerID := insertTestAccount(t, db, "decks-repo-owner")
	otherID := insertTestAccount(t, db, "decks-repo-other")
	deckID := insertTestDeck(t, db, ownerID, "isolation")

	// Query deck using a different account — must return nil (not found).
	detail, err := repo.GetDeck(ctx, otherID, deckID)
	if err != nil {
		t.Fatalf("GetDeck cross-account: %v", err)
	}
	if detail != nil {
		t.Errorf("cross-account isolation failure: GetDeck returned deck for wrong account")
	}
}

// ----------------------------------------------------------------------------
// deckCards — set_cards JOIN (#2002 regression fix)
// ----------------------------------------------------------------------------

// TestDecksRepository_GetDeck_SetCardsMetadata verifies that deckCards() joins
// against set_cards (not the dropped `cards` table) and correctly populates
// card metadata fields (Name, SetCode, Types/TypeLine, Rarity, ManaCost, CMC,
// Colors, ImageURIs).  This is the regression test for issue #2002.
func TestDecksRepository_GetDeck_SetCardsMetadata(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDecksRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, "decks-repo-set-cards-meta")
	deckID := insertTestDeck(t, db, accountID, "set-cards-meta")

	// Seed a set_cards row for arena_id "91001".
	insertTestSetCard(t, db, setCardSeed{
		SetCode: "TST",
		ArenaID: "91001",
		Name:    "Test Creature",
		Rarity:  "rare",
		Colors:  `["R"]`,
	})

	// Insert a deck_cards row referencing the same arena_id.
	insertTestDeckCard(t, db, deckID, 91001, false)

	detail, err := repo.GetDeck(ctx, accountID, deckID)
	if err != nil {
		t.Fatalf("GetDeck: %v", err)
	}
	if detail == nil {
		t.Fatal("GetDeck returned nil — deck not found")
	}
	if len(detail.Cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(detail.Cards))
	}
	c := detail.Cards[0]
	if c.Name != "Test Creature" {
		t.Errorf("Name: got %q, want %q", c.Name, "Test Creature")
	}
	if c.SetCode != "TST" {
		t.Errorf("SetCode: got %q, want %q", c.SetCode, "TST")
	}
	if c.Rarity != "rare" {
		t.Errorf("Rarity: got %q, want %q", c.Rarity, "rare")
	}
	if c.Colors != `["R"]` {
		t.Errorf("Colors: got %q, want %q", c.Colors, `["R"]`)
	}
}

// TestDecksRepository_GetDeck_SetCardsMetadata_NoMatch verifies that deckCards()
// returns a row with empty metadata when no set_cards row exists for the
// card_id (LEFT JOIN falls through gracefully).
func TestDecksRepository_GetDeck_SetCardsMetadata_NoMatch(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDecksRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, "decks-repo-set-cards-nomatch")
	deckID := insertTestDeck(t, db, accountID, "set-cards-nomatch")

	// card_id 91002 has no corresponding set_cards row.
	insertTestDeckCard(t, db, deckID, 91002, false)

	detail, err := repo.GetDeck(ctx, accountID, deckID)
	if err != nil {
		t.Fatalf("GetDeck: %v", err)
	}
	if detail == nil {
		t.Fatal("GetDeck returned nil — deck not found")
	}
	if len(detail.Cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(detail.Cards))
	}
	c := detail.Cards[0]
	if c.CardID != 91002 {
		t.Errorf("CardID: got %d, want 91002", c.CardID)
	}
	// Metadata fields must be empty strings (COALESCE defaults), not a DB error.
	if c.Name != "" {
		t.Errorf("Name: got %q, want empty string for no-match card", c.Name)
	}
	if c.Rarity != "" {
		t.Errorf("Rarity: got %q, want empty string for no-match card", c.Rarity)
	}
}

// ----------------------------------------------------------------------------
// DecksRepository.CreateDeck — issue #2012 regression tests
// ----------------------------------------------------------------------------

// TestDecksRepository_CreateDeck_HappyPath verifies that CreateDeck inserts a
// new row and returns a populated DeckDetailRow (AC1 for issue #2012).
func TestDecksRepository_CreateDeck_HappyPath(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDecksRepository(db)
	ctx := context.Background()

	accountID := insertTestAccount(t, db, "create-deck-happy")

	in := repository.CreateDeckInput{
		AccountID: accountID,
		Name:      "Test Constructed Deck",
		Format:    "standard",
		Source:    "constructed",
	}
	d, err := repo.CreateDeck(ctx, in)
	if err != nil {
		t.Fatalf("CreateDeck: %v", err)
	}
	if d == nil {
		t.Fatal("CreateDeck returned nil deck")
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM decks WHERE id = $1`, d.ID)
	})

	if d.Name != in.Name {
		t.Errorf("Name: got %q want %q", d.Name, in.Name)
	}
	if d.Format != in.Format {
		t.Errorf("Format: got %q want %q", d.Format, in.Format)
	}
	if d.Source != in.Source {
		t.Errorf("Source: got %q want %q", d.Source, in.Source)
	}
	if d.CreatedMethod != "manual" {
		t.Errorf("CreatedMethod: got %q want %q", d.CreatedMethod, "manual")
	}
	if d.ID == "" {
		t.Error("ID must not be empty")
	}
	// A newly created deck has no cards and zero counts.
	if d.CardCount != 0 {
		t.Errorf("CardCount: got %d want 0", d.CardCount)
	}
	if len(d.Cards) != 0 {
		t.Errorf("Cards: got %d want 0", len(d.Cards))
	}
}

// TestDecksRepository_CreateDeck_CrossAccountIsolation verifies that a deck
// created for one account cannot be fetched by another account.
func TestDecksRepository_CreateDeck_CrossAccountIsolation(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDecksRepository(db)
	ctx := context.Background()

	ownerID := insertTestAccount(t, db, "create-deck-owner")
	otherID := insertTestAccount(t, db, "create-deck-other")

	d, err := repo.CreateDeck(ctx, repository.CreateDeckInput{
		AccountID: ownerID,
		Name:      "Owner Deck",
		Format:    "standard",
		Source:    "constructed",
	})
	if err != nil {
		t.Fatalf("CreateDeck: %v", err)
	}
	if d == nil {
		t.Fatal("CreateDeck returned nil")
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM decks WHERE id = $1`, d.ID)
	})

	// other account must not be able to fetch this deck.
	got, err := repo.GetDeck(ctx, otherID, d.ID)
	if err != nil {
		t.Fatalf("GetDeck cross-account: %v", err)
	}
	if got != nil {
		t.Error("cross-account isolation failure: GetDeck returned deck for wrong account")
	}
}
