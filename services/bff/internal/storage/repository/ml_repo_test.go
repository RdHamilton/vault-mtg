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

// insertTestMatchWithDeck is defined in stats_repo_test.go (same package).
// Signature: (t, db, matchID, accountID, format, ts, deckID, result).
// The helper below wraps it and resets processed_for_ml=FALSE explicitly so
// the ML compute tests can control the unprocessed-match window precisely.

// ----------------------------------------------------------------------------
// MLRepository.ComputeAndWritePairStats
// ----------------------------------------------------------------------------

// TestMLRepository_ComputeAndWritePairStats_WritesRows is the primary
// acceptance test (AC1 / AC2 / AC4 / AC5): insert matches + deck_cards,
// call ComputeAndWritePairStats, assert card_combination_stats rows exist
// with correct synergy and confidence values, then assert SynergyReport
// returns non-zero pairs.
func TestMLRepository_ComputeAndWritePairStats_WritesRows(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMLRepository(db)

	const card1ID = 88001
	const card2ID = 88002
	const format = "standard"

	accountID := insertTestAccount(t, db, "ml-compute-write")
	deckID := insertTestStandardDeck(t, db, accountID, "ml-compute-write")
	insertTestDeckCard(t, db, deckID, card1ID, false)
	insertTestDeckCard(t, db, deckID, card2ID, false)

	now := time.Now().UTC()
	// 3 wins, 1 loss = 4 games together.
	// Signature: (t, db, matchID, accountID, format, ts, deckID, result)
	insertTestMatchWithDeck(t, db, fmt.Sprintf("ml-cw-win1-%d", accountID), accountID, format, now, deckID, "win")
	insertTestMatchWithDeck(t, db, fmt.Sprintf("ml-cw-win2-%d", accountID), accountID, format, now.Add(-time.Second), deckID, "win")
	insertTestMatchWithDeck(t, db, fmt.Sprintf("ml-cw-win3-%d", accountID), accountID, format, now.Add(-2*time.Second), deckID, "win")
	insertTestMatchWithDeck(t, db, fmt.Sprintf("ml-cw-loss1-%d", accountID), accountID, format, now.Add(-3*time.Second), deckID, "loss")

	result, err := repo.ComputeAndWritePairStats(context.Background(), accountID, format, 30, 1000)
	if err != nil {
		t.Fatalf("ComputeAndWritePairStats: %v", err)
	}
	if result.PairsWritten == 0 {
		t.Fatal("expected at least one pair written")
	}
	if result.MatchesProcessed != 4 {
		t.Errorf("matches_processed: got %d, want 4", result.MatchesProcessed)
	}
	if result.Truncated {
		t.Error("truncated: expected false for 4 matches with cap 1000")
	}

	// Verify the row in card_combination_stats.
	c, err := repo.CombinationStats(context.Background(), card1ID, card2ID, format)
	if err != nil {
		t.Fatalf("CombinationStats: %v", err)
	}
	if c == nil {
		t.Fatal("expected a card_combination_stats row after ComputeAndWritePairStats")
	}
	if c.GamesTogether != 4 {
		t.Errorf("games_together: got %d, want 4", c.GamesTogether)
	}
	if c.WinsTogether != 3 {
		t.Errorf("wins_together: got %d, want 3", c.WinsTogether)
	}
	// synergy_score = wins_together / games_together = 3/4 = 0.75
	wantSynergy := float64(3) / float64(4)
	if diff := c.SynergyScore - wantSynergy; diff > 0.001 || diff < -0.001 {
		t.Errorf("synergy_score: got %f, want %f", c.SynergyScore, wantSynergy)
	}
	// confidence_score = 1 - 1/(4+1) = 0.8
	wantConf := 1.0 - 1.0/float64(4+1)
	if diff := c.ConfidenceScore - wantConf; diff > 0.001 || diff < -0.001 {
		t.Errorf("confidence_score: got %f, want %f", c.ConfidenceScore, wantConf)
	}

	// AC5: SynergyReport returns non-zero pairs after processing.
	report, err := repo.SynergyReport(context.Background(), accountID, deckID)
	if err != nil {
		t.Fatalf("SynergyReport after compute: %v", err)
	}
	if report == nil {
		t.Fatal("expected non-nil SynergyReport after compute")
	}
	if report.TotalPairs == 0 {
		t.Error("SynergyReport.TotalPairs: expected > 0 after ComputeAndWritePairStats")
	}

	// Clean up card_combination_stats row created by the upsert.
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM card_combination_stats
			  WHERE card_id_1 = $1 AND card_id_2 = $2 AND format = $3 AND deck_id IS NULL`,
			card1ID, card2ID, format)
	})
}

// TestMLRepository_ComputeAndWritePairStats_IdempotentUpsert verifies that
// calling ComputeAndWritePairStats twice (after resetting processed_for_ml)
// accumulates counts rather than inserting duplicate rows — proving that the
// partial unique index idx_combo_stats_global deduplicates on re-run.
func TestMLRepository_ComputeAndWritePairStats_IdempotentUpsert(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMLRepository(db)

	const card1ID = 88003
	const card2ID = 88004
	const format = "standard"

	accountID := insertTestAccount(t, db, "ml-idem-upsert")
	deckID := insertTestStandardDeck(t, db, accountID, "ml-idem-upsert")
	insertTestDeckCard(t, db, deckID, card1ID, false)
	insertTestDeckCard(t, db, deckID, card2ID, false)

	now := time.Now().UTC()
	matchID := fmt.Sprintf("ml-idem-win-%d", accountID)
	// Signature: (t, db, matchID, accountID, format, ts, deckID, result)
	insertTestMatchWithDeck(t, db, matchID, accountID, format, now, deckID, "win")

	// First call.
	if _, err := repo.ComputeAndWritePairStats(context.Background(), accountID, format, 30, 1000); err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Reset processed_for_ml to simulate a re-run.
	if _, err := db.ExecContext(context.Background(),
		`UPDATE matches SET processed_for_ml = FALSE WHERE id = $1`, matchID); err != nil {
		t.Fatalf("reset processed_for_ml: %v", err)
	}

	// Second call — should upsert (accumulate), not insert a second row.
	if _, err := repo.ComputeAndWritePairStats(context.Background(), accountID, format, 30, 1000); err != nil {
		t.Fatalf("second call: %v", err)
	}

	// Assert exactly one row in card_combination_stats for this pair+format.
	var count int
	err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM card_combination_stats
		  WHERE card_id_1 = $1 AND card_id_2 = $2 AND format = $3 AND deck_id IS NULL`,
		card1ID, card2ID, format,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 1 {
		t.Errorf("row count: got %d, want exactly 1 (partial index must dedup)", count)
	}

	// Counts must be accumulated (2 games, 2 wins).
	c, err := repo.CombinationStats(context.Background(), card1ID, card2ID, format)
	if err != nil {
		t.Fatalf("CombinationStats: %v", err)
	}
	if c == nil {
		t.Fatal("expected row")
	}
	if c.GamesTogether != 2 {
		t.Errorf("games_together after 2 calls: got %d, want 2", c.GamesTogether)
	}
	if c.WinsTogether != 2 {
		t.Errorf("wins_together after 2 calls: got %d, want 2", c.WinsTogether)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM card_combination_stats
			  WHERE card_id_1 = $1 AND card_id_2 = $2 AND format = $3 AND deck_id IS NULL`,
			card1ID, card2ID, format)
	})
}

// TestMLRepository_ComputeAndWritePairStats_CapTruncates verifies that the
// 1000-match hard cap is enforced and Truncated is true when there are more
// matches than the cap.
func TestMLRepository_ComputeAndWritePairStats_CapTruncates(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMLRepository(db)

	const card1ID = 88005
	const card2ID = 88006
	const format = "standard"
	const smallCap = 2

	accountID := insertTestAccount(t, db, "ml-cap-truncate")
	deckID := insertTestStandardDeck(t, db, accountID, "ml-cap-truncate")
	insertTestDeckCard(t, db, deckID, card1ID, false)
	insertTestDeckCard(t, db, deckID, card2ID, false)

	now := time.Now().UTC()
	// Insert 3 matches but cap at 2 — truncated must be true.
	// Signature: (t, db, matchID, accountID, format, ts, deckID, result)
	for i := 0; i < 3; i++ {
		insertTestMatchWithDeck(t, db,
			fmt.Sprintf("ml-cap-%d-%d", accountID, i), accountID, format,
			now.Add(-time.Duration(i)*time.Second), deckID, "win",
		)
	}

	result, err := repo.ComputeAndWritePairStats(context.Background(), accountID, format, 30, smallCap)
	if err != nil {
		t.Fatalf("ComputeAndWritePairStats: %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated: expected true when matches > cap")
	}
	if result.MatchesProcessed != smallCap {
		t.Errorf("MatchesProcessed: got %d, want %d", result.MatchesProcessed, smallCap)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM card_combination_stats
			  WHERE card_id_1 = $1 AND card_id_2 = $2 AND format = $3 AND deck_id IS NULL`,
			card1ID, card2ID, format)
	})
}

// TestMLRepository_ComputeAndWritePairStats_MarksProcessedForML verifies that
// matches are marked processed_for_ml = TRUE after a successful compute call,
// and that a second call with the same cap finds zero unprocessed matches.
func TestMLRepository_ComputeAndWritePairStats_MarksProcessedForML(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMLRepository(db)

	const card1ID = 88007
	const card2ID = 88008
	const format = "standard"

	accountID := insertTestAccount(t, db, "ml-mark-processed")
	deckID := insertTestStandardDeck(t, db, accountID, "ml-mark-processed")
	insertTestDeckCard(t, db, deckID, card1ID, false)
	insertTestDeckCard(t, db, deckID, card2ID, false)

	now := time.Now().UTC()
	// Signature: (t, db, matchID, accountID, format, ts, deckID, result)
	insertTestMatchWithDeck(t, db, fmt.Sprintf("ml-mark-%d", accountID), accountID, format, now, deckID, "win")

	// First call — processes the match.
	first, err := repo.ComputeAndWritePairStats(context.Background(), accountID, format, 30, 1000)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if first.MatchesProcessed != 1 {
		t.Fatalf("first call: expected 1 match processed, got %d", first.MatchesProcessed)
	}

	// Second call — the match is already marked processed, so zero matches.
	second, err := repo.ComputeAndWritePairStats(context.Background(), accountID, format, 30, 1000)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if second.MatchesProcessed != 0 {
		t.Errorf("second call: expected 0 matches processed (already marked), got %d", second.MatchesProcessed)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM card_combination_stats
			  WHERE card_id_1 = $1 AND card_id_2 = $2 AND format = $3 AND deck_id IS NULL`,
			card1ID, card2ID, format)
	})
}

// TestMLRepository_CardSynergies_ReturnsWrittenRows verifies that CardSynergies
// returns rows written by ComputeAndWritePairStats.
func TestMLRepository_CardSynergies_ReturnsWrittenRows(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewMLRepository(db)

	const card1ID = 88009
	const card2ID = 88010
	const format = "standard"

	accountID := insertTestAccount(t, db, "ml-synergy-read")
	deckID := insertTestStandardDeck(t, db, accountID, "ml-synergy-read")
	insertTestDeckCard(t, db, deckID, card1ID, false)
	insertTestDeckCard(t, db, deckID, card2ID, false)

	now := time.Now().UTC()
	// Signature: (t, db, matchID, accountID, format, ts, deckID, result)
	insertTestMatchWithDeck(t, db, fmt.Sprintf("ml-sr-%d", accountID), accountID, format, now, deckID, "win")

	if _, err := repo.ComputeAndWritePairStats(context.Background(), accountID, format, 30, 1000); err != nil {
		t.Fatalf("ComputeAndWritePairStats: %v", err)
	}

	synergies, err := repo.CardSynergies(context.Background(), card1ID, format, 10)
	if err != nil {
		t.Fatalf("CardSynergies: %v", err)
	}
	if len(synergies) == 0 {
		t.Error("CardSynergies: expected non-empty result after ComputeAndWritePairStats")
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM card_combination_stats
			  WHERE card_id_1 = $1 AND card_id_2 = $2 AND format = $3 AND deck_id IS NULL`,
			card1ID, card2ID, format)
	})
}
