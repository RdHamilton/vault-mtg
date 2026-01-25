package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
	"github.com/ramonehamilton/MTGA-Companion/internal/storage/repository"
)

// setupPerformanceTestDB creates an in-memory database with all tables needed for performance tests.
func setupPerformanceTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	schema := `
		CREATE TABLE accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			is_default INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);

		CREATE TABLE decks (
			id TEXT PRIMARY KEY,
			account_id INTEGER NOT NULL DEFAULT 1,
			name TEXT NOT NULL,
			format TEXT NOT NULL,
			description TEXT,
			color_identity TEXT,
			source TEXT NOT NULL DEFAULT 'constructed',
			draft_event_id TEXT,
			matches_played INTEGER NOT NULL DEFAULT 0,
			matches_won INTEGER NOT NULL DEFAULT 0,
			games_played INTEGER NOT NULL DEFAULT 0,
			games_won INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			modified_at DATETIME NOT NULL,
			last_played DATETIME,
			is_app_created BOOLEAN DEFAULT FALSE,
			created_method TEXT DEFAULT 'imported',
			seed_card_id INTEGER,
			current_permutation_id INTEGER,
			FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
			CHECK(source IN ('draft', 'constructed', 'imported', 'arena'))
		);

		CREATE TABLE deck_cards (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			deck_id TEXT NOT NULL,
			card_id INTEGER NOT NULL,
			quantity INTEGER NOT NULL,
			board TEXT NOT NULL,
			from_draft_pick INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (deck_id) REFERENCES decks(id) ON DELETE CASCADE,
			UNIQUE(deck_id, card_id, board)
		);

		CREATE TABLE deck_permutations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			deck_id TEXT NOT NULL,
			parent_permutation_id INTEGER,
			cards TEXT NOT NULL,
			card_hash TEXT NOT NULL,
			version_number INTEGER NOT NULL DEFAULT 1,
			version_name TEXT,
			change_summary TEXT,
			matches_played INTEGER NOT NULL DEFAULT 0,
			matches_won INTEGER NOT NULL DEFAULT 0,
			games_played INTEGER NOT NULL DEFAULT 0,
			games_won INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_played_at DATETIME,
			FOREIGN KEY (deck_id) REFERENCES decks(id) ON DELETE CASCADE,
			FOREIGN KEY (parent_permutation_id) REFERENCES deck_permutations(id) ON DELETE SET NULL
		);

		CREATE TABLE matches (
			id TEXT PRIMARY KEY,
			account_id INTEGER NOT NULL DEFAULT 1,
			event_id TEXT NOT NULL,
			event_name TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			duration_seconds INTEGER,
			player_wins INTEGER NOT NULL DEFAULT 0,
			opponent_wins INTEGER NOT NULL DEFAULT 0,
			player_team_id INTEGER NOT NULL DEFAULT 0,
			deck_id TEXT,
			rank_before TEXT,
			rank_after TEXT,
			format TEXT NOT NULL,
			result TEXT NOT NULL,
			result_reason TEXT,
			opponent_name TEXT,
			opponent_id TEXT,
			created_at DATETIME NOT NULL,
			FOREIGN KEY (deck_id) REFERENCES decks(id) ON DELETE SET NULL
		);

		CREATE INDEX idx_deck_permutations_deck_id ON deck_permutations(deck_id);
		CREATE INDEX idx_deck_permutations_created ON deck_permutations(deck_id, created_at DESC);
		CREATE UNIQUE INDEX idx_deck_permutations_hash ON deck_permutations(deck_id, card_hash);
		CREATE INDEX idx_deck_permutations_version ON deck_permutations(deck_id, version_number);
		CREATE INDEX idx_decks_current_permutation ON decks(current_permutation_id);
		CREATE INDEX idx_matches_deck_id ON matches(deck_id);
		CREATE INDEX idx_matches_timestamp ON matches(timestamp);

		-- Insert a default test account
		INSERT INTO accounts (id, name, is_default, created_at, updated_at)
		VALUES (1, 'Test Account', 1, '2024-01-01 00:00:00', '2024-01-01 00:00:00');
	`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return db
}

// TestRecalculateDeckPerformance_TimestampBasedAttribution tests that matches are attributed
// to the correct permutation based on when the match was played vs when permutations were created.
func TestRecalculateDeckPerformance_TimestampBasedAttribution(t *testing.T) {
	db := setupPerformanceTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close db: %v", err)
		}
	}()

	ctx := context.Background()

	// Set up test data
	deckID := "test-deck-1"
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create a deck
	deckCreatedAt := baseTime.Format("2006-01-02 15:04:05.999999")
	_, err := db.Exec(`
		INSERT INTO decks (id, account_id, name, format, source, created_at, modified_at)
		VALUES (?, 1, 'Test Deck', 'Standard', 'constructed', ?, ?)
	`, deckID, deckCreatedAt, deckCreatedAt)
	if err != nil {
		t.Fatalf("failed to create deck: %v", err)
	}

	// Create 3 permutations with different creation times:
	// - Permutation 1: created at baseTime (Jan 1)
	// - Permutation 2: created at baseTime + 7 days (Jan 8)
	// - Permutation 3: created at baseTime + 14 days (Jan 15)
	perm1Time := baseTime
	perm2Time := baseTime.Add(7 * 24 * time.Hour)
	perm3Time := baseTime.Add(14 * 24 * time.Hour)

	cards := []models.DeckPermutationCard{
		{CardID: 100, Quantity: 4, Board: "main"},
	}
	cardsJSON, _ := json.Marshal(cards)

	// Insert permutation 1
	result, err := db.Exec(`
		INSERT INTO deck_permutations (deck_id, cards, card_hash, version_number, created_at)
		VALUES (?, ?, 'hash1', 1, ?)
	`, deckID, string(cardsJSON), perm1Time.Format("2006-01-02 15:04:05.999999"))
	if err != nil {
		t.Fatalf("failed to create permutation 1: %v", err)
	}
	perm1ID, _ := result.LastInsertId()

	// Insert permutation 2
	result, err = db.Exec(`
		INSERT INTO deck_permutations (deck_id, cards, card_hash, version_number, parent_permutation_id, created_at)
		VALUES (?, ?, 'hash2', 2, ?, ?)
	`, deckID, string(cardsJSON), perm1ID, perm2Time.Format("2006-01-02 15:04:05.999999"))
	if err != nil {
		t.Fatalf("failed to create permutation 2: %v", err)
	}
	perm2ID, _ := result.LastInsertId()

	// Insert permutation 3
	result, err = db.Exec(`
		INSERT INTO deck_permutations (deck_id, cards, card_hash, version_number, parent_permutation_id, created_at)
		VALUES (?, ?, 'hash3', 3, ?, ?)
	`, deckID, string(cardsJSON), perm2ID, perm3Time.Format("2006-01-02 15:04:05.999999"))
	if err != nil {
		t.Fatalf("failed to create permutation 3: %v", err)
	}
	perm3ID, _ := result.LastInsertId()

	// Set permutation 3 as current
	_, err = db.Exec(`UPDATE decks SET current_permutation_id = ? WHERE id = ?`, perm3ID, deckID)
	if err != nil {
		t.Fatalf("failed to set current permutation: %v", err)
	}

	// Create matches at different times:
	// - Match 1: played on Jan 3 (should be attributed to perm1)
	// - Match 2: played on Jan 5 (should be attributed to perm1)
	// - Match 3: played on Jan 10 (should be attributed to perm2)
	// - Match 4: played on Jan 12 (should be attributed to perm2)
	// - Match 5: played on Jan 17 (should be attributed to perm3)
	// - Match 6: played on Jan 20 (should be attributed to perm3)
	matches := []struct {
		id           string
		timestamp    time.Time
		result       string
		playerWins   int
		opponentWins int
	}{
		{"match-1", baseTime.Add(2 * 24 * time.Hour), "win", 2, 0},   // Jan 3 -> perm1
		{"match-2", baseTime.Add(4 * 24 * time.Hour), "loss", 1, 2},  // Jan 5 -> perm1
		{"match-3", baseTime.Add(9 * 24 * time.Hour), "win", 2, 1},   // Jan 10 -> perm2
		{"match-4", baseTime.Add(11 * 24 * time.Hour), "win", 2, 0},  // Jan 12 -> perm2
		{"match-5", baseTime.Add(16 * 24 * time.Hour), "loss", 0, 2}, // Jan 17 -> perm3
		{"match-6", baseTime.Add(19 * 24 * time.Hour), "win", 2, 1},  // Jan 20 -> perm3
	}

	for _, m := range matches {
		_, err := db.Exec(`
			INSERT INTO matches (id, account_id, event_id, event_name, timestamp, player_wins, opponent_wins, deck_id, format, result, created_at)
			VALUES (?, 1, 'event-1', 'Test Event', ?, ?, ?, ?, 'Standard', ?, ?)
		`, m.id, m.timestamp.Format("2006-01-02 15:04:05.999999"), m.playerWins, m.opponentWins, deckID, m.result, m.timestamp.Format("2006-01-02 15:04:05.999999"))
		if err != nil {
			t.Fatalf("failed to create match %s: %v", m.id, err)
		}
	}

	// Create a Service with the test database
	testDB := &DB{conn: db}
	service := NewServiceWithConfig(testDB, &ServiceConfig{
		Decks:   repository.NewDeckRepository(db),
		Matches: repository.NewMatchRepository(db),
	})
	// The service should automatically detect the default account with is_default=1

	// Run recalculation
	result2, err := service.RecalculateDeckPerformance(ctx)
	if err != nil {
		t.Fatalf("RecalculateDeckPerformance failed: %v", err)
	}

	// Verify result counts
	if result2.DecksProcessed != 1 {
		t.Errorf("expected 1 deck processed, got %d", result2.DecksProcessed)
	}
	if result2.MatchesProcessed != 6 {
		t.Errorf("expected 6 matches processed, got %d", result2.MatchesProcessed)
	}

	// Verify permutation 1 stats (2 matches: 1 win, 1 loss)
	var perm1MatchesPlayed, perm1MatchesWon, perm1GamesPlayed, perm1GamesWon int
	err = db.QueryRow(`SELECT matches_played, matches_won, games_played, games_won FROM deck_permutations WHERE id = ?`, perm1ID).
		Scan(&perm1MatchesPlayed, &perm1MatchesWon, &perm1GamesPlayed, &perm1GamesWon)
	if err != nil {
		t.Fatalf("failed to query perm1 stats: %v", err)
	}
	if perm1MatchesPlayed != 2 {
		t.Errorf("perm1: expected 2 matches_played, got %d", perm1MatchesPlayed)
	}
	if perm1MatchesWon != 1 {
		t.Errorf("perm1: expected 1 matches_won, got %d", perm1MatchesWon)
	}
	if perm1GamesPlayed != 5 { // 2+0 + 1+2 = 5
		t.Errorf("perm1: expected 5 games_played, got %d", perm1GamesPlayed)
	}
	if perm1GamesWon != 3 { // 2 + 1 = 3
		t.Errorf("perm1: expected 3 games_won, got %d", perm1GamesWon)
	}

	// Verify permutation 2 stats (2 matches: 2 wins, 0 losses)
	var perm2MatchesPlayed, perm2MatchesWon, perm2GamesPlayed, perm2GamesWon int
	err = db.QueryRow(`SELECT matches_played, matches_won, games_played, games_won FROM deck_permutations WHERE id = ?`, perm2ID).
		Scan(&perm2MatchesPlayed, &perm2MatchesWon, &perm2GamesPlayed, &perm2GamesWon)
	if err != nil {
		t.Fatalf("failed to query perm2 stats: %v", err)
	}
	if perm2MatchesPlayed != 2 {
		t.Errorf("perm2: expected 2 matches_played, got %d", perm2MatchesPlayed)
	}
	if perm2MatchesWon != 2 {
		t.Errorf("perm2: expected 2 matches_won, got %d", perm2MatchesWon)
	}
	if perm2GamesPlayed != 5 { // 2+1 + 2+0 = 5
		t.Errorf("perm2: expected 5 games_played, got %d", perm2GamesPlayed)
	}
	if perm2GamesWon != 4 { // 2 + 2 = 4
		t.Errorf("perm2: expected 4 games_won, got %d", perm2GamesWon)
	}

	// Verify permutation 3 stats (2 matches: 1 win, 1 loss)
	var perm3MatchesPlayed, perm3MatchesWon, perm3GamesPlayed, perm3GamesWon int
	err = db.QueryRow(`SELECT matches_played, matches_won, games_played, games_won FROM deck_permutations WHERE id = ?`, perm3ID).
		Scan(&perm3MatchesPlayed, &perm3MatchesWon, &perm3GamesPlayed, &perm3GamesWon)
	if err != nil {
		t.Fatalf("failed to query perm3 stats: %v", err)
	}
	if perm3MatchesPlayed != 2 {
		t.Errorf("perm3: expected 2 matches_played, got %d", perm3MatchesPlayed)
	}
	if perm3MatchesWon != 1 {
		t.Errorf("perm3: expected 1 matches_won, got %d", perm3MatchesWon)
	}
	if perm3GamesPlayed != 5 { // 0+2 + 2+1 = 5
		t.Errorf("perm3: expected 5 games_played, got %d", perm3GamesPlayed)
	}
	if perm3GamesWon != 2 { // 0 + 2 = 2
		t.Errorf("perm3: expected 2 games_won, got %d", perm3GamesWon)
	}

	// Verify deck totals (6 matches: 4 wins, 2 losses)
	var deckMatchesPlayed, deckMatchesWon, deckGamesPlayed, deckGamesWon int
	err = db.QueryRow(`SELECT matches_played, matches_won, games_played, games_won FROM decks WHERE id = ?`, deckID).
		Scan(&deckMatchesPlayed, &deckMatchesWon, &deckGamesPlayed, &deckGamesWon)
	if err != nil {
		t.Fatalf("failed to query deck stats: %v", err)
	}
	if deckMatchesPlayed != 6 {
		t.Errorf("deck: expected 6 matches_played, got %d", deckMatchesPlayed)
	}
	if deckMatchesWon != 4 {
		t.Errorf("deck: expected 4 matches_won, got %d", deckMatchesWon)
	}
	if deckGamesPlayed != 15 { // 5 + 5 + 5 = 15
		t.Errorf("deck: expected 15 games_played, got %d", deckGamesPlayed)
	}
	if deckGamesWon != 9 { // 3 + 4 + 2 = 9
		t.Errorf("deck: expected 9 games_won, got %d", deckGamesWon)
	}
}

// TestRecalculateDeckPerformance_MatchBeforeAnyPermutation tests that matches played
// before any permutation was created are attributed to the first permutation (v1) as a fallback.
func TestRecalculateDeckPerformance_MatchBeforeAnyPermutation(t *testing.T) {
	db := setupPerformanceTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close db: %v", err)
		}
	}()

	ctx := context.Background()

	deckID := "test-deck-2"
	baseTime := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	// Create deck
	deckCreatedAt := baseTime.Format("2006-01-02 15:04:05.999999")
	_, err := db.Exec(`
		INSERT INTO decks (id, account_id, name, format, source, created_at, modified_at)
		VALUES (?, 1, 'Test Deck 2', 'Standard', 'constructed', ?, ?)
	`, deckID, deckCreatedAt, deckCreatedAt)
	if err != nil {
		t.Fatalf("failed to create deck: %v", err)
	}

	// Create permutation on Jan 15
	cards := []models.DeckPermutationCard{{CardID: 100, Quantity: 4, Board: "main"}}
	cardsJSON, _ := json.Marshal(cards)

	result, err := db.Exec(`
		INSERT INTO deck_permutations (deck_id, cards, card_hash, version_number, created_at)
		VALUES (?, ?, 'hash1', 1, ?)
	`, deckID, string(cardsJSON), baseTime.Format("2006-01-02 15:04:05.999999"))
	if err != nil {
		t.Fatalf("failed to create permutation: %v", err)
	}
	permID, _ := result.LastInsertId()

	// Set as current
	_, err = db.Exec(`UPDATE decks SET current_permutation_id = ? WHERE id = ?`, permID, deckID)
	if err != nil {
		t.Fatalf("failed to set current permutation: %v", err)
	}

	// Create a match on Jan 10 (before the permutation was created)
	matchTime := baseTime.Add(-5 * 24 * time.Hour) // 5 days before
	_, err = db.Exec(`
		INSERT INTO matches (id, account_id, event_id, event_name, timestamp, player_wins, opponent_wins, deck_id, format, result, created_at)
		VALUES ('match-early', 1, 'event-1', 'Test Event', ?, 2, 0, ?, 'Standard', 'win', ?)
	`, matchTime.Format("2006-01-02 15:04:05.999999"), deckID, matchTime.Format("2006-01-02 15:04:05.999999"))
	if err != nil {
		t.Fatalf("failed to create early match: %v", err)
	}

	// Create a match on Jan 20 (after the permutation was created)
	matchTime2 := baseTime.Add(5 * 24 * time.Hour)
	_, err = db.Exec(`
		INSERT INTO matches (id, account_id, event_id, event_name, timestamp, player_wins, opponent_wins, deck_id, format, result, created_at)
		VALUES ('match-after', 1, 'event-1', 'Test Event', ?, 2, 1, ?, 'Standard', 'win', ?)
	`, matchTime2.Format("2006-01-02 15:04:05.999999"), deckID, matchTime2.Format("2006-01-02 15:04:05.999999"))
	if err != nil {
		t.Fatalf("failed to create after match: %v", err)
	}

	// Create service and run recalculation
	testDB := &DB{conn: db}
	service := NewServiceWithConfig(testDB, &ServiceConfig{
		Decks:   repository.NewDeckRepository(db),
		Matches: repository.NewMatchRepository(db),
	})

	_, err = service.RecalculateDeckPerformance(ctx)
	if err != nil {
		t.Fatalf("RecalculateDeckPerformance failed: %v", err)
	}

	// Verify permutation stats - should have both matches (early match falls back to v1)
	var matchesPlayed, matchesWon int
	err = db.QueryRow(`SELECT matches_played, matches_won FROM deck_permutations WHERE id = ?`, permID).
		Scan(&matchesPlayed, &matchesWon)
	if err != nil {
		t.Fatalf("failed to query perm stats: %v", err)
	}
	if matchesPlayed != 2 {
		t.Errorf("expected 2 matches_played (early match falls back to v1), got %d", matchesPlayed)
	}
	if matchesWon != 2 {
		t.Errorf("expected 2 matches_won, got %d", matchesWon)
	}

	// Verify deck stats - should have both matches
	var deckMatchesPlayed, deckMatchesWon int
	err = db.QueryRow(`SELECT matches_played, matches_won FROM decks WHERE id = ?`, deckID).
		Scan(&deckMatchesPlayed, &deckMatchesWon)
	if err != nil {
		t.Fatalf("failed to query deck stats: %v", err)
	}
	if deckMatchesPlayed != 2 {
		t.Errorf("expected 2 deck matches_played, got %d", deckMatchesPlayed)
	}
	if deckMatchesWon != 2 {
		t.Errorf("expected 2 deck matches_won, got %d", deckMatchesWon)
	}
}

// TestDeckPermutation_CurrentPermutationID tests that current_permutation_id is correctly tracked.
func TestDeckPermutation_CurrentPermutationID(t *testing.T) {
	db := setupPerformanceTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close db: %v", err)
		}
	}()

	deckID := "test-deck-current"
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create deck
	deckCreatedAt := baseTime.Format("2006-01-02 15:04:05.999999")
	_, err := db.Exec(`
		INSERT INTO decks (id, account_id, name, format, source, created_at, modified_at)
		VALUES (?, 1, 'Test Deck Current', 'Standard', 'constructed', ?, ?)
	`, deckID, deckCreatedAt, deckCreatedAt)
	if err != nil {
		t.Fatalf("failed to create deck: %v", err)
	}

	// Create 3 permutations
	cards := []models.DeckPermutationCard{{CardID: 100, Quantity: 4, Board: "main"}}
	cardsJSON, _ := json.Marshal(cards)

	// Permutation 1
	result, err := db.Exec(`
		INSERT INTO deck_permutations (deck_id, cards, card_hash, version_number, created_at)
		VALUES (?, ?, 'hash1', 1, ?)
	`, deckID, string(cardsJSON), baseTime.Format("2006-01-02 15:04:05.999999"))
	if err != nil {
		t.Fatalf("failed to create permutation 1: %v", err)
	}
	perm1ID, _ := result.LastInsertId()

	// Permutation 2
	result, err = db.Exec(`
		INSERT INTO deck_permutations (deck_id, cards, card_hash, version_number, parent_permutation_id, created_at)
		VALUES (?, ?, 'hash2', 2, ?, ?)
	`, deckID, string(cardsJSON), perm1ID, baseTime.Add(7*24*time.Hour).Format("2006-01-02 15:04:05.999999"))
	if err != nil {
		t.Fatalf("failed to create permutation 2: %v", err)
	}
	perm2ID, _ := result.LastInsertId()

	// Permutation 3
	result, err = db.Exec(`
		INSERT INTO deck_permutations (deck_id, cards, card_hash, version_number, parent_permutation_id, created_at)
		VALUES (?, ?, 'hash3', 3, ?, ?)
	`, deckID, string(cardsJSON), perm2ID, baseTime.Add(14*24*time.Hour).Format("2006-01-02 15:04:05.999999"))
	if err != nil {
		t.Fatalf("failed to create permutation 3: %v", err)
	}
	perm3ID, _ := result.LastInsertId()

	// Set permutation 2 as current
	_, err = db.Exec(`UPDATE decks SET current_permutation_id = ? WHERE id = ?`, perm2ID, deckID)
	if err != nil {
		t.Fatalf("failed to set current permutation: %v", err)
	}

	// Verify current_permutation_id is correctly set
	var currentPermID *int64
	err = db.QueryRow(`SELECT current_permutation_id FROM decks WHERE id = ?`, deckID).Scan(&currentPermID)
	if err != nil {
		t.Fatalf("failed to query current_permutation_id: %v", err)
	}

	if currentPermID == nil {
		t.Fatal("expected current_permutation_id to be set, got nil")
	}
	if *currentPermID != perm2ID {
		t.Errorf("expected current_permutation_id %d, got %d", perm2ID, *currentPermID)
	}

	// Change current to permutation 3
	_, err = db.Exec(`UPDATE decks SET current_permutation_id = ? WHERE id = ?`, perm3ID, deckID)
	if err != nil {
		t.Fatalf("failed to update current permutation: %v", err)
	}

	// Verify update
	err = db.QueryRow(`SELECT current_permutation_id FROM decks WHERE id = ?`, deckID).Scan(&currentPermID)
	if err != nil {
		t.Fatalf("failed to query updated current_permutation_id: %v", err)
	}

	if *currentPermID != perm3ID {
		t.Errorf("expected updated current_permutation_id %d, got %d", perm3ID, *currentPermID)
	}
}

// TestDeckPermutation_RestoreUpdatesCurrentID tests that restoring a permutation updates current_permutation_id.
func TestDeckPermutation_RestoreUpdatesCurrentID(t *testing.T) {
	db := setupPerformanceTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close db: %v", err)
		}
	}()

	deckID := "test-deck-restore"
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create deck
	deckCreatedAt := baseTime.Format("2006-01-02 15:04:05.999999")
	_, err := db.Exec(`
		INSERT INTO decks (id, account_id, name, format, source, created_at, modified_at)
		VALUES (?, 1, 'Test Deck Restore', 'Standard', 'constructed', ?, ?)
	`, deckID, deckCreatedAt, deckCreatedAt)
	if err != nil {
		t.Fatalf("failed to create deck: %v", err)
	}

	// Create 2 permutations
	cards := []models.DeckPermutationCard{{CardID: 100, Quantity: 4, Board: "main"}}
	cardsJSON, _ := json.Marshal(cards)

	// Permutation 1
	result, err := db.Exec(`
		INSERT INTO deck_permutations (deck_id, cards, card_hash, version_number, created_at)
		VALUES (?, ?, 'hash1', 1, ?)
	`, deckID, string(cardsJSON), baseTime.Format("2006-01-02 15:04:05.999999"))
	if err != nil {
		t.Fatalf("failed to create permutation 1: %v", err)
	}
	perm1ID, _ := result.LastInsertId()

	// Permutation 2 (current)
	result, err = db.Exec(`
		INSERT INTO deck_permutations (deck_id, cards, card_hash, version_number, parent_permutation_id, created_at)
		VALUES (?, ?, 'hash2', 2, ?, ?)
	`, deckID, string(cardsJSON), perm1ID, baseTime.Add(7*24*time.Hour).Format("2006-01-02 15:04:05.999999"))
	if err != nil {
		t.Fatalf("failed to create permutation 2: %v", err)
	}
	perm2ID, _ := result.LastInsertId()

	// Set permutation 2 as current
	_, err = db.Exec(`UPDATE decks SET current_permutation_id = ? WHERE id = ?`, perm2ID, deckID)
	if err != nil {
		t.Fatalf("failed to set current permutation: %v", err)
	}

	// Simulate restore: update current_permutation_id to perm1
	_, err = db.Exec(`UPDATE decks SET current_permutation_id = ? WHERE id = ?`, perm1ID, deckID)
	if err != nil {
		t.Fatalf("failed to restore permutation: %v", err)
	}

	// Verify the restore
	var currentPermID int64
	err = db.QueryRow(`SELECT current_permutation_id FROM decks WHERE id = ?`, deckID).Scan(&currentPermID)
	if err != nil {
		t.Fatalf("failed to query current_permutation_id after restore: %v", err)
	}

	if currentPermID != perm1ID {
		t.Errorf("expected current_permutation_id %d after restore, got %d", perm1ID, currentPermID)
	}
}
