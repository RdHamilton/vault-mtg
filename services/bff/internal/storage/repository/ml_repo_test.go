package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
)

// insertTestSetCardForML inserts a minimal set_cards row used by ML synergy
// tests. arena_id is TEXT in set_cards (migration 000014).
func insertTestSetCardForML(t *testing.T, db *sql.DB, setCode, arenaIDText, name string) {
	t.Helper()
	_, err := db.ExecContext(
		context.Background(), `
		INSERT INTO set_cards (set_code, arena_id, scryfall_id, name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (set_code, arena_id) DO UPDATE SET name = EXCLUDED.name`,
		setCode, arenaIDText, fmt.Sprintf("scryfall-ml-%s-%s", arenaIDText, setCode), name,
	)
	if err != nil {
		t.Fatalf("insertTestSetCardForML arena_id=%q: %v", arenaIDText, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM set_cards WHERE set_code = $1 AND arena_id = $2`, setCode, arenaIDText)
	})
}

// insertTestCardCombinationStat inserts a card_combination_stats row.
// card_id_1 < card_id_2 is enforced by the table CHECK constraint.
// The row is cleaned up via t.Cleanup.
func insertTestCardCombinationStat(t *testing.T, db *sql.DB, card1, card2 int, format string, synergyScore float64) int64 {
	t.Helper()
	if card1 > card2 {
		card1, card2 = card2, card1
	}
	now := time.Now().UTC()
	var id int64
	err := db.QueryRowContext(
		context.Background(), `
		INSERT INTO card_combination_stats
			(card_id_1, card_id_2, format,
			 games_together, games_card1_only, games_card2_only,
			 wins_together, wins_card1_only, wins_card2_only,
			 synergy_score, confidence_score, created_at, updated_at)
		VALUES ($1, $2, $3, 10, 5, 5, 6, 3, 3, $4, 0.5, $5, $5)
		ON CONFLICT DO NOTHING
		RETURNING id`,
		card1, card2, format, synergyScore, now,
	).Scan(&id)
	if err == sql.ErrNoRows {
		// ON CONFLICT — row already exists; skip cleanup for this call.
		return 0
	}
	if err != nil {
		t.Fatalf("insertTestCardCombinationStat (%d,%d) %q: %v", card1, card2, format, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM card_combination_stats WHERE id = $1`, id)
	})
	return id
}

// ----------------------------------------------------------------------------
// MLRepository.SynergyReport — set_cards name join
// ----------------------------------------------------------------------------

// TestMLRepository_SynergyReport_CardNamesFromSetCards verifies that the
// synergy report resolves card names from set_cards (not the retired cards
// table).
func TestMLRepository_SynergyReport_CardNamesFromSetCards(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMLRepository(db)

	const setCode = "MLTST"
	const card1ID = 87001
	const card2ID = 87002
	const format = "standard"
	const wantName1 = "ML Test Card Alpha"
	const wantName2 = "ML Test Card Beta"

	insertTestSet(t, db, setCode, "ML Test Set")
	insertTestSetCardForML(t, db, setCode, "87001", wantName1)
	insertTestSetCardForML(t, db, setCode, "87002", wantName2)

	insertTestCardCombinationStat(t, db, card1ID, card2ID, format, 0.75)

	accountID := insertTestAccount(t, db, "ml-synergy-owner")
	deckID := insertTestStandardDeck(t, db, accountID, "ml-synergy")
	insertTestDeckCard(t, db, deckID, card1ID, false)
	insertTestDeckCard(t, db, deckID, card2ID, false)

	report, err := repo.SynergyReport(context.Background(), accountID, deckID)
	if err != nil {
		t.Fatalf("SynergyReport: %v", err)
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if report.TotalPairs == 0 {
		t.Fatal("expected at least one synergy pair")
	}

	pair := report.Synergies[0]
	if pair.Card1Name == nil || *pair.Card1Name == "" {
		t.Error("Card1Name must not be empty when set_cards row exists")
	}
	if pair.Card2Name == nil || *pair.Card2Name == "" {
		t.Error("Card2Name must not be empty when set_cards row exists")
	}
}

// TestMLRepository_SynergyReport_NullNamesWhenNoSetCard verifies that the
// synergy report returns nil card names (not an error) when no set_cards row
// exists for the card id — the LEFT JOIN must not hard-fail.
func TestMLRepository_SynergyReport_NullNamesWhenNoSetCard(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMLRepository(db)

	const card1ID = 87003
	const card2ID = 87004
	const format = "standard"

	// No set_cards rows for these IDs.
	insertTestCardCombinationStat(t, db, card1ID, card2ID, format, 0.5)

	accountID := insertTestAccount(t, db, "ml-synergy-null-meta")
	deckID := insertTestStandardDeck(t, db, accountID, "ml-synergy-null")
	insertTestDeckCard(t, db, deckID, card1ID, false)
	insertTestDeckCard(t, db, deckID, card2ID, false)

	report, err := repo.SynergyReport(context.Background(), accountID, deckID)
	if err != nil {
		t.Fatalf("SynergyReport: %v", err)
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	// Card names should be nil (not empty string) — the LEFT JOIN produced NULLs.
	if report.TotalPairs > 0 {
		pair := report.Synergies[0]
		if pair.Card1Name != nil && *pair.Card1Name != "" {
			t.Errorf("Card1Name: got %q, want nil or empty when no set_cards row", *pair.Card1Name)
		}
		if pair.Card2Name != nil && *pair.Card2Name != "" {
			t.Errorf("Card2Name: got %q, want nil or empty when no set_cards row", *pair.Card2Name)
		}
	}
}

// TestMLRepository_SynergyReport_EmptyDeckReturnsNoSynergies verifies that a
// deck with fewer than 2 cards returns a report with zero synergies without error.
func TestMLRepository_SynergyReport_EmptyDeckReturnsNoSynergies(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMLRepository(db)

	accountID := insertTestAccount(t, db, "ml-synergy-empty")
	deckID := insertTestStandardDeck(t, db, accountID, "ml-synergy-empty")
	// No deck cards — deck has 0 cards.

	report, err := repo.SynergyReport(context.Background(), accountID, deckID)
	if err != nil {
		t.Fatalf("SynergyReport: %v", err)
	}
	if report == nil {
		t.Fatal("expected non-nil report for owned deck with no cards")
	}
	if report.TotalPairs != 0 {
		t.Errorf("TotalPairs: got %d, want 0 for empty deck", report.TotalPairs)
	}
}

// TestMLRepository_SynergyReport_WrongOwnerReturnsNil verifies that the
// account-ownership check returns nil for a deck owned by a different account.
func TestMLRepository_SynergyReport_WrongOwnerReturnsNil(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMLRepository(db)

	ownerID := insertTestAccount(t, db, "ml-synergy-owner2")
	otherID := insertTestAccount(t, db, "ml-synergy-other2")
	deckID := insertTestStandardDeck(t, db, ownerID, "ml-wrong-owner")

	report, err := repo.SynergyReport(context.Background(), otherID, deckID)
	if err != nil {
		t.Fatalf("SynergyReport: %v", err)
	}
	if report != nil {
		t.Errorf("expected nil report for non-owner account, got %+v", report)
	}
}
