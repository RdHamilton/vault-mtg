package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
)

// insertTestStandardConfig inserts (or replaces) the singleton standard_config row
// and restores the original via t.Cleanup.
func insertTestStandardConfig(t *testing.T, db *sql.DB, nextRotation string) {
	t.Helper()
	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO standard_config (id, next_rotation_date, rotation_enabled)
		 VALUES (1, $1, TRUE)
		 ON CONFLICT (id) DO UPDATE SET next_rotation_date = EXCLUDED.next_rotation_date`,
		nextRotation,
	)
	if err != nil {
		t.Fatalf("insertTestStandardConfig: %v", err)
	}
}

// insertTestSetCardWithLegalities inserts a set_cards row with a legalities JSON
// blob. arena_id is TEXT in set_cards (migration 000014).
func insertTestSetCardWithLegalities(t *testing.T, db *sql.DB, setCode, arenaIDText, name, legalities string) {
	t.Helper()
	_, err := db.ExecContext(
		context.Background(), `
		INSERT INTO set_cards (set_code, arena_id, scryfall_id, name, legalities)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (set_code, arena_id) DO UPDATE
			SET name       = EXCLUDED.name,
			    legalities = EXCLUDED.legalities`,
		setCode, arenaIDText, fmt.Sprintf("scryfall-%s-%s", arenaIDText, setCode), name, legalities,
	)
	if err != nil {
		t.Fatalf("insertTestSetCardWithLegalities set=%q arena_id=%q: %v", setCode, arenaIDText, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			`DELETE FROM set_cards WHERE set_code = $1 AND arena_id = $2`,
			setCode, arenaIDText,
		)
	})
}

// insertTestStandardDeck inserts a minimal Standard deck row and returns its id.
func insertTestStandardDeck(t *testing.T, db *sql.DB, accountID int64, suffix string) string {
	t.Helper()
	id := fmt.Sprintf("test-std-deck-%s", suffix)
	now := time.Now().UTC()
	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO decks
			(id, account_id, name, format, source, is_app_created, created_method, created_at, modified_at)
		 VALUES ($1, $2, $3, 'standard', 'constructed', FALSE, 'imported', $4, $5)`,
		id, accountID, "Std Deck "+suffix, now, now,
	)
	if err != nil {
		t.Fatalf("insertTestStandardDeck %q: %v", id, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM decks WHERE id = $1`, id)
	})
	return id
}

// ----------------------------------------------------------------------------
// StandardRepository.CardByArenaID
// ----------------------------------------------------------------------------

// TestStandardRepository_CardByArenaID_ReturnsFromSetCards verifies that
// CardByArenaID reads from set_cards (not the retired cards table) and returns
// the correct name, set_code, and legalities.
func TestStandardRepository_CardByArenaID_ReturnsFromSetCards(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStandardRepository(db)

	const arenaID = 88001
	const setCode = "SRTST"
	const wantName = "Standard Repo Test Card"
	const wantLegalities = `{"standard":"legal"}`

	insertTestSet(t, db, setCode, "Standard Repo Test Set")
	insertTestSetCardWithLegalities(t, db, setCode, "88001", wantName, wantLegalities)

	got, err := repo.CardByArenaID(context.Background(), arenaID)
	if err != nil {
		t.Fatalf("CardByArenaID: %v", err)
	}

	if got.ArenaID != arenaID {
		t.Errorf("ArenaID: got %d, want %d", got.ArenaID, arenaID)
	}
	if got.Name != wantName {
		t.Errorf("Name: got %q, want %q", got.Name, wantName)
	}
	if got.SetCode != setCode {
		t.Errorf("SetCode: got %q, want %q", got.SetCode, setCode)
	}
	if got.Legalities != wantLegalities {
		t.Errorf("Legalities: got %q, want %q", got.Legalities, wantLegalities)
	}
}

// TestStandardRepository_CardByArenaID_NotFound verifies that CardByArenaID
// returns sql.ErrNoRows when no set_cards row exists for the arena_id.
func TestStandardRepository_CardByArenaID_NotFound(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStandardRepository(db)

	_, err := repo.CardByArenaID(context.Background(), 999999999)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

// TestStandardRepository_CardByArenaID_LegalitiesFallback verifies that
// COALESCE returns '{}' when legalities is NULL.
func TestStandardRepository_CardByArenaID_LegalitiesFallback(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStandardRepository(db)

	const arenaID = 88002
	const setCode = "SRTST"

	// Insert without legalities (NULL).
	insertTestSet(t, db, setCode, "Standard Repo Test Set")
	_, err := db.ExecContext(
		context.Background(), `
		INSERT INTO set_cards (set_code, arena_id, scryfall_id, name)
		VALUES ($1, '88002', $2, 'No Legalities Card')
		ON CONFLICT (set_code, arena_id) DO NOTHING`,
		setCode, "scryfall-88002-"+setCode,
	)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM set_cards WHERE set_code = $1 AND arena_id = '88002'`, setCode)
	})

	got, err := repo.CardByArenaID(context.Background(), arenaID)
	if err != nil {
		t.Fatalf("CardByArenaID: %v", err)
	}
	if got.Legalities != "{}" {
		t.Errorf("Legalities COALESCE: got %q, want %q", got.Legalities, "{}")
	}
}

// ----------------------------------------------------------------------------
// StandardRepository.DeckCardsForValidation
// ----------------------------------------------------------------------------

// TestStandardRepository_DeckCardsForValidation_ReturnsFromSetCards verifies
// that DeckCardsForValidation resolves card name + legalities from set_cards
// and joins the set row for rotation metadata.
func TestStandardRepository_DeckCardsForValidation_ReturnsFromSetCards(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStandardRepository(db)

	const setCode = "SRTST"
	const arenaID = 88003
	const wantName = "Validation Test Card"
	const wantLegalities = `{"standard":"legal"}`

	insertTestSet(t, db, setCode, "Standard Repo Test Set")
	insertTestSetCardWithLegalities(t, db, setCode, "88003", wantName, wantLegalities)

	accountID := insertTestAccount(t, db, "std-repo-dcfv-account")
	deckID := insertTestStandardDeck(t, db, accountID, "dcfv")
	insertTestDeckCard(t, db, deckID, arenaID, false)

	rows, err := repo.DeckCardsForValidation(context.Background(), deckID)
	if err != nil {
		t.Fatalf("DeckCardsForValidation: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected at least one row")
	}

	row := rows[0]
	if row.CardID != arenaID {
		t.Errorf("CardID: got %d, want %d", row.CardID, arenaID)
	}
	if row.Name != wantName {
		t.Errorf("Name: got %q, want %q", row.Name, wantName)
	}
	if row.SetCode != setCode {
		t.Errorf("SetCode: got %q, want %q", row.SetCode, setCode)
	}
	if row.Legalities != wantLegalities {
		t.Errorf("Legalities: got %q, want %q", row.Legalities, wantLegalities)
	}
}

// TestStandardRepository_DeckCardsForValidation_NullCardMetadata verifies
// that rows for deck cards with no matching set_cards row still scan cleanly
// with empty-string fallbacks (COALESCE) — no scan error.
func TestStandardRepository_DeckCardsForValidation_NullCardMetadata(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStandardRepository(db)

	const arenaID = 88004 // no matching set_cards row

	accountID := insertTestAccount(t, db, "std-repo-dcfv-null")
	deckID := insertTestStandardDeck(t, db, accountID, "dcfv-null")
	insertTestDeckCard(t, db, deckID, arenaID, false)

	rows, err := repo.DeckCardsForValidation(context.Background(), deckID)
	if err != nil {
		t.Fatalf("DeckCardsForValidation: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected one row even without card metadata")
	}
	row := rows[0]
	// Name and SetCode should fall back to empty string via COALESCE.
	if row.Name != "" {
		t.Errorf("Name: got %q, want empty string for missing card metadata", row.Name)
	}
	if row.Legalities != "{}" {
		t.Errorf("Legalities: got %q, want %q", row.Legalities, "{}")
	}
}
