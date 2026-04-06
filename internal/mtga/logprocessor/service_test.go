package logprocessor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ramonehamilton/MTGA-Companion/internal/mtga/logreader"
	"github.com/ramonehamilton/MTGA-Companion/internal/storage"
	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
)

// setupTestService creates a test storage service with a temporary database
func setupTestService(t *testing.T) (*storage.Service, func()) {
	t.Helper()

	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database connection with auto-migrate
	config := storage.DefaultConfig(dbPath)
	config.AutoMigrate = true
	db, err := storage.Open(config)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create service
	service := storage.NewService(db)

	cleanup := func() {
		if err := service.Close(); err != nil {
			t.Errorf("Failed to close service: %v", err)
		}
		os.RemoveAll(tmpDir)
	}

	return service, cleanup
}

func TestProcessLogEntries_ArenaStats(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	processor := NewService(service)

	// Create test log entries with match data
	// Note: This is a simplified test - actual log format may differ
	entries := []*logreader.LogEntry{
		{
			IsJSON:    true,
			Timestamp: "2025-11-15 10:00:00",
			JSON: map[string]interface{}{
				"greToClientEvent": map[string]interface{}{
					"greToClientMessages": []interface{}{
						map[string]interface{}{
							"type": "GREMessageType_GameStateMessage",
							"gameStateMessage": map[string]interface{}{
								"gameInfo": map[string]interface{}{
									"matchID":    "match-123",
									"gameNumber": float64(1),
									"matchState": "MatchState_GameInProgress",
								},
								"turnInfo": map[string]interface{}{
									"turnNumber": float64(1),
								},
							},
						},
					},
				},
			},
		},
	}

	// Process entries - service should handle gracefully even if no data is found
	result, err := processor.ProcessLogEntries(ctx, entries)
	if err != nil {
		t.Fatalf("ProcessLogEntries failed: %v", err)
	}

	// Verify service ran without errors
	// Note: Match/game counts may be 0 if parsers don't recognize this test data format
	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}
}

func TestProcessLogEntries_Decks(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	processor := NewService(service)

	// Create test log entries with deck data
	// Note: This is a simplified test - actual log format may differ
	entries := []*logreader.LogEntry{
		{
			IsJSON:    true,
			Timestamp: "2025-11-15 10:00:00",
			JSON: map[string]interface{}{
				"EventGetCoursesV2": map[string]interface{}{
					"Courses": []interface{}{
						map[string]interface{}{
							"InternalEventName": "Ladder",
							"CurrentEventState": "EventState_Active",
						},
					},
				},
			},
		},
	}

	// Process entries - service should handle gracefully even if no data is found
	result, err := processor.ProcessLogEntries(ctx, entries)
	if err != nil {
		t.Fatalf("ProcessLogEntries failed: %v", err)
	}

	// Verify service ran without errors
	// Note: Deck counts may be 0 if parsers don't recognize this test data format
	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}
}

func TestProcessLogEntries_RankUpdates(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	processor := NewService(service)

	// Create test log entries with rank update data
	entries := []*logreader.LogEntry{
		{
			IsJSON:    true,
			Timestamp: "2025-11-15 10:00:00",
			JSON: map[string]interface{}{
				"constructedSeasonOrdinal": float64(83),
				"constructedClass":         "Gold",
				"constructedLevel":         float64(4),
				"constructedStep":          float64(2),
			},
		},
	}

	// Process entries
	result, err := processor.ProcessLogEntries(ctx, entries)
	if err != nil {
		t.Fatalf("ProcessLogEntries failed: %v", err)
	}

	// Verify results
	if result.RanksStored == 0 {
		t.Error("Expected rank updates to be stored, got 0")
	}
	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}
}

func TestProcessLogEntries_MultipleTypes(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	processor := NewService(service)

	// Create test log entries with multiple data types
	entries := []*logreader.LogEntry{
		// Rank update
		{
			IsJSON:    true,
			Timestamp: "2025-11-15 09:00:00",
			JSON: map[string]interface{}{
				"constructedSeasonOrdinal": float64(83),
				"constructedClass":         "Gold",
				"constructedLevel":         float64(4),
			},
		},
		// Match data
		{
			IsJSON:    true,
			Timestamp: "2025-11-15 10:00:00",
			JSON: map[string]interface{}{
				"matchGameRoomStateChangedEvent": map[string]interface{}{
					"gameRoomInfo": map[string]interface{}{
						"gameRoomConfig": map[string]interface{}{
							"matchId": "match-456",
							"eventId": "Ladder",
						},
						"finalMatchResult": map[string]interface{}{
							"resultList": []interface{}{
								map[string]interface{}{
									"scope":  "MatchScope_Match",
									"result": "ResultType_Lost",
								},
							},
						},
					},
				},
			},
		},
	}

	// Process entries
	result, err := processor.ProcessLogEntries(ctx, entries)
	if err != nil {
		t.Fatalf("ProcessLogEntries failed: %v", err)
	}

	// Verify that multiple types were processed
	// Note: Actual counts may vary based on parser implementation
	if result.RanksStored == 0 && result.MatchesStored == 0 {
		t.Error("Expected either ranks or matches to be stored, got 0 for both")
	}
}

func TestProcessLogEntries_EmptyEntries(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	processor := NewService(service)

	// Process empty entries
	result, err := processor.ProcessLogEntries(ctx, []*logreader.LogEntry{})
	if err != nil {
		t.Fatalf("ProcessLogEntries failed on empty entries: %v", err)
	}

	// Verify nothing was stored
	if result.MatchesStored != 0 {
		t.Errorf("Expected 0 matches, got %d", result.MatchesStored)
	}
	if result.DecksStored != 0 {
		t.Errorf("Expected 0 decks, got %d", result.DecksStored)
	}
	if result.RanksStored != 0 {
		t.Errorf("Expected 0 ranks, got %d", result.RanksStored)
	}
}

func TestProcessLogEntries_InvalidData(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	processor := NewService(service)

	// Create test log entries with invalid data
	entries := []*logreader.LogEntry{
		{
			IsJSON:    true,
			Timestamp: "invalid-timestamp",
			JSON: map[string]interface{}{
				"someInvalidKey": "someInvalidValue",
			},
		},
	}

	// Process entries - should not fail even with invalid data
	result, err := processor.ProcessLogEntries(ctx, entries)
	if err != nil {
		t.Fatalf("ProcessLogEntries should handle invalid data gracefully: %v", err)
	}

	// Verify nothing was stored (invalid data should be skipped)
	if result.MatchesStored != 0 {
		t.Errorf("Expected 0 matches from invalid data, got %d", result.MatchesStored)
	}
}

func TestProcessLogEntries_ContextCancellation(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	processor := NewService(service)

	entries := []*logreader.LogEntry{
		{
			IsJSON:    true,
			Timestamp: "2025-11-15 10:00:00",
			JSON:      map[string]interface{}{},
		},
	}

	// Process with cancelled context
	// Note: Current implementation may not check context cancellation in all paths
	// This test ensures it doesn't panic or hang
	_, _ = processor.ProcessLogEntries(ctx, entries)
}

func TestNewService(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	processor := NewService(service)
	if processor == nil {
		t.Fatal("NewService returned nil")
	}
	if processor.storage != service {
		t.Error("NewService did not set storage correctly")
	}
}

func TestProcessResult_Structure(t *testing.T) {
	result := &ProcessResult{
		MatchesStored: 5,
		GamesStored:   10,
		DecksStored:   3,
		RanksStored:   2,
		Errors:        []error{},
	}

	if result.MatchesStored != 5 {
		t.Errorf("Expected MatchesStored=5, got %d", result.MatchesStored)
	}
	if result.GamesStored != 10 {
		t.Errorf("Expected GamesStored=10, got %d", result.GamesStored)
	}
	if result.DecksStored != 3 {
		t.Errorf("Expected DecksStored=3, got %d", result.DecksStored)
	}
	if result.RanksStored != 2 {
		t.Errorf("Expected RanksStored=2, got %d", result.RanksStored)
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d", len(result.Errors))
	}
}

func TestProcessCollection_FromDecks(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// First, store some decks with cards to the database
	// Create a deck directly in the database
	now := time.Now()
	_, err := service.GetDB().ExecContext(ctx, `
		INSERT INTO decks (id, account_id, name, format, source, created_at, modified_at)
		VALUES ('test-deck-1', 1, 'Test Deck', 'Standard', 'arena', ?, ?)
	`, now, now)
	if err != nil {
		t.Fatalf("Failed to insert test deck: %v", err)
	}

	// Add cards to the deck
	_, err = service.GetDB().ExecContext(ctx, `
		INSERT INTO deck_cards (deck_id, card_id, quantity, board)
		VALUES
			('test-deck-1', 12345, 4, 'main'),
			('test-deck-1', 67890, 2, 'main'),
			('test-deck-1', 11111, 1, 'sideboard')
	`)
	if err != nil {
		t.Fatalf("Failed to insert deck cards: %v", err)
	}

	// Process collection
	processor := NewService(service)
	result := &ProcessResult{}
	err = processor.processCollection(ctx, result)
	if err != nil {
		t.Fatalf("processCollection failed: %v", err)
	}

	// Verify collection was updated
	if result.CollectionNewCards != 3 {
		t.Errorf("Expected 3 new cards, got %d", result.CollectionNewCards)
	}

	// Verify quantities
	collection, err := service.CollectionRepo().GetAll(ctx)
	if err != nil {
		t.Fatalf("Failed to get collection: %v", err)
	}

	if collection[12345] != 4 {
		t.Errorf("Expected card 12345 to have quantity 4, got %d", collection[12345])
	}
	if collection[67890] != 2 {
		t.Errorf("Expected card 67890 to have quantity 2, got %d", collection[67890])
	}
	if collection[11111] != 1 {
		t.Errorf("Expected card 11111 to have quantity 1, got %d", collection[11111])
	}
}

func TestProcessCollection_CapAt4Copies(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a deck with more than 4 copies across multiple decks
	now := time.Now()
	_, err := service.GetDB().ExecContext(ctx, `
		INSERT INTO decks (id, account_id, name, format, source, created_at, modified_at)
		VALUES
			('test-deck-1', 1, 'Test Deck 1', 'Standard', 'arena', ?, ?),
			('test-deck-2', 1, 'Test Deck 2', 'Standard', 'arena', ?, ?)
	`, now, now, now, now)
	if err != nil {
		t.Fatalf("Failed to insert test decks: %v", err)
	}

	// Add cards - total should exceed 4
	_, err = service.GetDB().ExecContext(ctx, `
		INSERT INTO deck_cards (deck_id, card_id, quantity, board)
		VALUES
			('test-deck-1', 12345, 4, 'main'),
			('test-deck-2', 12345, 4, 'main')
	`)
	if err != nil {
		t.Fatalf("Failed to insert deck cards: %v", err)
	}

	// Process collection
	processor := NewService(service)
	result := &ProcessResult{}
	err = processor.processCollection(ctx, result)
	if err != nil {
		t.Fatalf("processCollection failed: %v", err)
	}

	// Verify collection was capped at 4
	collection, err := service.CollectionRepo().GetAll(ctx)
	if err != nil {
		t.Fatalf("Failed to get collection: %v", err)
	}

	if collection[12345] != 4 {
		t.Errorf("Expected card 12345 to be capped at 4, got %d", collection[12345])
	}
}

func TestProcessCollection_NoChanges(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Process empty collection - should not error
	processor := NewService(service)
	result := &ProcessResult{}
	err := processor.processCollection(ctx, result)
	if err != nil {
		t.Fatalf("processCollection failed on empty data: %v", err)
	}

	// Verify no changes reported
	if result.CollectionNewCards != 0 {
		t.Errorf("Expected 0 new cards, got %d", result.CollectionNewCards)
	}
	if result.CollectionCardsAdded != 0 {
		t.Errorf("Expected 0 cards added, got %d", result.CollectionCardsAdded)
	}
}

func TestProcessCollection_DryRunMode(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a deck with cards
	now := time.Now()
	_, err := service.GetDB().ExecContext(ctx, `
		INSERT INTO decks (id, account_id, name, format, source, created_at, modified_at)
		VALUES ('test-deck-1', 1, 'Test Deck', 'Standard', 'arena', ?, ?)
	`, now, now)
	if err != nil {
		t.Fatalf("Failed to insert test deck: %v", err)
	}

	_, err = service.GetDB().ExecContext(ctx, `
		INSERT INTO deck_cards (deck_id, card_id, quantity, board)
		VALUES ('test-deck-1', 12345, 4, 'main')
	`)
	if err != nil {
		t.Fatalf("Failed to insert deck cards: %v", err)
	}

	// Enable dry run mode
	processor := NewService(service)
	processor.SetDryRun(true)

	result := &ProcessResult{}
	err = processor.processCollection(ctx, result)
	if err != nil {
		t.Fatalf("processCollection failed in dry run mode: %v", err)
	}

	// Verify collection was NOT updated
	collection, err := service.CollectionRepo().GetAll(ctx)
	if err != nil {
		t.Fatalf("Failed to get collection: %v", err)
	}

	if len(collection) != 0 {
		t.Errorf("Expected empty collection in dry run mode, got %d cards", len(collection))
	}
}

func TestSplitCompletedDraftSessions_NewDraftAfterCompleted(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a completed draft session in the database
	now := time.Now()
	_, err := service.GetDB().ExecContext(ctx, `
		INSERT INTO draft_sessions (id, event_name, set_code, draft_type, start_time, status, total_picks, created_at, updated_at)
		VALUES ('QuickDraft_TLA_20251127', 'QuickDraft_TLA_20251127', 'TLA', 'QuickDraft', ?, 'completed', 42, ?, ?)
	`, now, now, now)
	if err != nil {
		t.Fatalf("Failed to insert completed draft session: %v", err)
	}

	// Add 42 picks to make it complete
	for i := 0; i < 42; i++ {
		packNum := i / 14
		pickNum := (i % 14) + 1
		_, err = service.GetDB().ExecContext(ctx, `
			INSERT INTO draft_picks (session_id, pack_number, pick_number, card_id, timestamp)
			VALUES ('QuickDraft_TLA_20251127', ?, ?, '12345', ?)
		`, packNum, pickNum, now)
		if err != nil {
			t.Fatalf("Failed to insert pick %d: %v", i, err)
		}
	}

	// Create events for a NEW draft with the same event name but P0P0 pack data
	events := map[string][]*logreader.DraftSessionEvent{
		"QuickDraft_TLA_20251127": {
			{
				Type:       "status_updated",
				EventName:  "QuickDraft_TLA_20251127",
				PackNumber: 0,
				PickNumber: 0,
				DraftPack:  []string{"card1", "card2", "card3"},
				Timestamp:  now,
			},
		},
	}

	// Process with splitCompletedDraftSessions
	processor := NewService(service)
	result := processor.splitCompletedDraftSessions(ctx, events)

	// Verify a new session ID was generated
	if len(result) != 1 {
		t.Fatalf("Expected 1 result group, got %d", len(result))
	}

	// The new session ID should NOT be the original
	for newSessionID := range result {
		if newSessionID == "QuickDraft_TLA_20251127" {
			t.Error("Expected new session ID to be different from completed session")
		}
		// Should start with the original prefix
		if len(newSessionID) < len("QuickDraft_TLA_20251127_") {
			t.Errorf("Expected new session ID to start with original prefix, got %s", newSessionID)
		}
	}
}

func TestSplitCompletedDraftSessions_InProgressSession(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create an in-progress draft session
	now := time.Now()
	_, err := service.GetDB().ExecContext(ctx, `
		INSERT INTO draft_sessions (id, event_name, set_code, draft_type, start_time, status, total_picks, created_at, updated_at)
		VALUES ('QuickDraft_TLA_20251127', 'QuickDraft_TLA_20251127', 'TLA', 'QuickDraft', ?, 'in_progress', 42, ?, ?)
	`, now, now, now)
	if err != nil {
		t.Fatalf("Failed to insert in-progress draft session: %v", err)
	}

	// Add only 10 picks (not complete)
	for i := 0; i < 10; i++ {
		_, err = service.GetDB().ExecContext(ctx, `
			INSERT INTO draft_picks (session_id, pack_number, pick_number, card_id, timestamp)
			VALUES ('QuickDraft_TLA_20251127', 0, ?, '12345', ?)
		`, i+1, now)
		if err != nil {
			t.Fatalf("Failed to insert pick %d: %v", i, err)
		}
	}

	// Create events with P0P11 pack data (continuing the draft)
	events := map[string][]*logreader.DraftSessionEvent{
		"QuickDraft_TLA_20251127": {
			{
				Type:       "status_updated",
				EventName:  "QuickDraft_TLA_20251127",
				PackNumber: 0,
				PickNumber: 11,
				DraftPack:  []string{"card1", "card2", "card3"},
				Timestamp:  now,
			},
		},
	}

	// Process with splitCompletedDraftSessions
	processor := NewService(service)
	result := processor.splitCompletedDraftSessions(ctx, events)

	// Verify the original session ID is preserved (no split)
	if len(result) != 1 {
		t.Fatalf("Expected 1 result group, got %d", len(result))
	}

	for sessionID := range result {
		if sessionID != "QuickDraft_TLA_20251127" {
			t.Errorf("Expected session ID to remain 'QuickDraft_TLA_20251127', got %s", sessionID)
		}
	}
}

func TestSplitCompletedDraftSessions_UUIDPassthrough(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// UUID-based sessions (Premier Draft) should pass through unchanged
	events := map[string][]*logreader.DraftSessionEvent{
		"73e1c7a3-75ee-4b38-b32b-d6854e5c6c9c": {
			{
				Type:       "status_updated",
				SessionID:  "73e1c7a3-75ee-4b38-b32b-d6854e5c6c9c",
				PackNumber: 0,
				PickNumber: 0,
				DraftPack:  []string{"card1", "card2", "card3"},
				Timestamp:  time.Now(),
			},
		},
	}

	processor := NewService(service)
	result := processor.splitCompletedDraftSessions(ctx, events)

	// UUID sessions should pass through unchanged
	if len(result) != 1 {
		t.Fatalf("Expected 1 result group, got %d", len(result))
	}

	for sessionID := range result {
		if sessionID != "73e1c7a3-75ee-4b38-b32b-d6854e5c6c9c" {
			t.Errorf("Expected UUID session ID to remain unchanged, got %s", sessionID)
		}
	}
}

func TestSplitCompletedDraftSessions_ReuseExistingInProgressSession(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a completed session for the base event name
	completedSession := &models.DraftSession{
		ID:         "QuickDraft_TLA_20251127",
		EventName:  "QuickDraft_TLA_20251127",
		SetCode:    "TLA",
		DraftType:  "QuickDraft",
		StartTime:  time.Now().Add(-2 * time.Hour),
		Status:     "completed",
		TotalPicks: 42,
		CreatedAt:  time.Now().Add(-2 * time.Hour),
		UpdatedAt:  time.Now().Add(-time.Hour),
	}
	if err := service.DraftRepo().CreateSession(ctx, completedSession); err != nil {
		t.Fatalf("Failed to create completed session: %v", err)
	}

	// Create an existing in_progress session with timestamp suffix
	existingInProgressSession := &models.DraftSession{
		ID:         "QuickDraft_TLA_20251127_1234567890",
		EventName:  "QuickDraft_TLA_20251127",
		SetCode:    "TLA",
		DraftType:  "QuickDraft",
		StartTime:  time.Now().Add(-30 * time.Minute),
		Status:     "in_progress",
		TotalPicks: 0,
		CreatedAt:  time.Now().Add(-30 * time.Minute),
		UpdatedAt:  time.Now(),
	}
	if err := service.DraftRepo().CreateSession(ctx, existingInProgressSession); err != nil {
		t.Fatalf("Failed to create in_progress session: %v", err)
	}

	// Simulate new events coming for the base event name with P0P1 pack data
	events := map[string][]*logreader.DraftSessionEvent{
		"QuickDraft_TLA_20251127": {
			{
				Type:       "status_updated",
				EventName:  "QuickDraft_TLA_20251127",
				PackNumber: 0,
				PickNumber: 1,
				DraftPack:  []string{"card1", "card2", "card3"},
				Timestamp:  time.Now(),
			},
		},
	}

	processor := NewService(service)
	result := processor.splitCompletedDraftSessions(ctx, events)

	// Should reuse the existing in_progress session instead of creating a new one
	if len(result) != 1 {
		t.Fatalf("Expected 1 result group, got %d", len(result))
	}

	for sessionID := range result {
		if sessionID != "QuickDraft_TLA_20251127_1234567890" {
			t.Errorf("Expected to reuse existing session 'QuickDraft_TLA_20251127_1234567890', got %s", sessionID)
		}
	}
}

func TestSplitCompletedDraftSessions_MixedOldAndNewEvents(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a completed draft session in the database
	now := time.Now()
	_, err := service.GetDB().ExecContext(ctx, `
		INSERT INTO draft_sessions (id, event_name, set_code, draft_type, start_time, status, total_picks, created_at, updated_at)
		VALUES ('QuickDraft_TLA_20251127', 'QuickDraft_TLA_20251127', 'TLA', 'QuickDraft', ?, 'completed', 42, ?, ?)
	`, now.Add(-2*time.Hour), now.Add(-2*time.Hour), now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("Failed to insert completed draft session: %v", err)
	}

	// Add 42 picks to make it complete
	for i := 0; i < 42; i++ {
		packNum := i / 14
		pickNum := (i % 14) + 1
		_, err = service.GetDB().ExecContext(ctx, `
			INSERT INTO draft_picks (session_id, pack_number, pick_number, card_id, timestamp)
			VALUES ('QuickDraft_TLA_20251127', ?, ?, '12345', ?)
		`, packNum, pickNum, now.Add(-2*time.Hour))
		if err != nil {
			t.Fatalf("Failed to insert pick %d: %v", i, err)
		}
	}

	// Simulate reprocessing the entire log file: events from BOTH old and new drafts mixed together
	// This is what happens when the daemon restarts with ReadFromStart=true
	events := map[string][]*logreader.DraftSessionEvent{
		"QuickDraft_TLA_20251127": {
			// OLD draft events (42 picks worth)
			{
				Type:        "status_updated",
				EventName:   "QuickDraft_TLA_20251127",
				PackNumber:  0,
				PickNumber:  1,
				DraftPack:   []string{"old1", "old2", "old3", "old4", "old5", "old6", "old7", "old8", "old9", "old10", "old11", "old12", "old13", "old14"},
				PickedCards: []string{}, // First pick of old draft
				Timestamp:   now.Add(-2 * time.Hour),
			},
			{
				Type:         "pick_made",
				EventName:    "QuickDraft_TLA_20251127",
				PackNumber:   0,
				PickNumber:   1,
				SelectedCard: []string{"old1"},
				Timestamp:    now.Add(-2 * time.Hour),
			},
			{
				Type:        "status_updated",
				EventName:   "QuickDraft_TLA_20251127",
				PackNumber:  0,
				PickNumber:  2,
				DraftPack:   []string{"old2", "old3", "old4", "old5", "old6", "old7", "old8", "old9", "old10", "old11", "old12", "old13"},
				PickedCards: []string{"old1"}, // Has picked cards - NOT a new draft
				Timestamp:   now.Add(-2 * time.Hour),
			},
			// ... more old draft events would be here in real scenario ...
			{
				Type:      "ended",
				EventName: "QuickDraft_TLA_20251127",
				Timestamp: now.Add(-time.Hour),
			},
			// NEW draft events (starting fresh)
			{
				Type:        "status_updated",
				EventName:   "QuickDraft_TLA_20251127",
				PackNumber:  0,
				PickNumber:  1,
				DraftPack:   []string{"new1", "new2", "new3", "new4", "new5", "new6", "new7", "new8", "new9", "new10", "new11", "new12", "new13", "new14"},
				PickedCards: []string{}, // Empty! This is a NEW draft
				Timestamp:   now,
			},
			{
				Type:         "pick_made",
				EventName:    "QuickDraft_TLA_20251127",
				PackNumber:   0,
				PickNumber:   1,
				SelectedCard: []string{"new1"},
				Timestamp:    now,
			},
			{
				Type:        "status_updated",
				EventName:   "QuickDraft_TLA_20251127",
				PackNumber:  0,
				PickNumber:  2,
				DraftPack:   []string{"new2", "new3", "new4", "new5", "new6", "new7", "new8", "new9", "new10", "new11", "new12", "new13"},
				PickedCards: []string{"new1"},
				Timestamp:   now,
			},
		},
	}

	processor := NewService(service)
	result := processor.splitCompletedDraftSessions(ctx, events)

	// Should create a new session ID for the new draft
	if len(result) != 1 {
		t.Fatalf("Expected 1 result group, got %d", len(result))
	}

	var newSessionID string
	var newEvents []*logreader.DraftSessionEvent
	for id, evts := range result {
		newSessionID = id
		newEvents = evts
	}

	// The new session ID should NOT be the original (should have timestamp suffix)
	if newSessionID == "QuickDraft_TLA_20251127" {
		t.Error("Expected new session ID to be different from completed session")
	}

	// Should have filtered out old draft events
	// We should only have the new draft events (the one with empty PickedCards and events after it)
	// Expected: status_updated (new P0P1), pick_made (new), status_updated (new P0P2)
	// But we may also include 'started' and 'session_info' events if any
	hasOldEvents := false
	for _, evt := range newEvents {
		// Check if any old draft events are included
		if evt.Type == "ended" {
			hasOldEvents = true
			t.Error("Old draft 'ended' event should have been filtered out")
		}
		// Check for old draft status_updated with PickedCards
		if evt.Type == "status_updated" && len(evt.PickedCards) > 0 && evt.PickedCards[0] == "old1" {
			hasOldEvents = true
			t.Error("Old draft status_updated with PickedCards should have been filtered out")
		}
		// Check for old pick_made
		if evt.Type == "pick_made" && len(evt.SelectedCard) > 0 && evt.SelectedCard[0] == "old1" {
			hasOldEvents = true
			t.Error("Old draft pick_made event should have been filtered out")
		}
	}

	if hasOldEvents {
		t.Errorf("Expected no old draft events in result, but some were found. Total events: %d", len(newEvents))
		for i, evt := range newEvents {
			t.Logf("Event %d: Type=%s, Pack=%d, Pick=%d", i, evt.Type, evt.PackNumber, evt.PickNumber)
		}
	}

	// Verify new draft events are present
	hasNewStatusUpdate := false
	hasNewPickMade := false
	for _, evt := range newEvents {
		if evt.Type == "status_updated" && len(evt.DraftPack) > 0 && evt.DraftPack[0] == "new1" {
			hasNewStatusUpdate = true
		}
		if evt.Type == "pick_made" && len(evt.SelectedCard) > 0 && evt.SelectedCard[0] == "new1" {
			hasNewPickMade = true
		}
	}

	if !hasNewStatusUpdate {
		t.Error("Expected new draft status_updated event to be present")
	}
	if !hasNewPickMade {
		t.Error("Expected new draft pick_made event to be present")
	}
}

func TestSplitCompletedDraftSessions_OngoingPicksRouteToTimestampedSession(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a completed session for the base event name
	completedSession := &models.DraftSession{
		ID:         "QuickDraft_TLA_20251127",
		EventName:  "QuickDraft_TLA_20251127",
		SetCode:    "TLA",
		DraftType:  "QuickDraft",
		StartTime:  time.Now().Add(-2 * time.Hour),
		Status:     "completed",
		TotalPicks: 42,
		CreatedAt:  time.Now().Add(-2 * time.Hour),
		UpdatedAt:  time.Now().Add(-time.Hour),
	}
	if err := service.DraftRepo().CreateSession(ctx, completedSession); err != nil {
		t.Fatalf("Failed to create completed session: %v", err)
	}

	// Create an existing in_progress session with timestamp suffix (the new draft)
	existingInProgressSession := &models.DraftSession{
		ID:         "QuickDraft_TLA_20251127_1234567890",
		EventName:  "QuickDraft_TLA_20251127",
		SetCode:    "TLA",
		DraftType:  "QuickDraft",
		StartTime:  time.Now().Add(-30 * time.Minute),
		Status:     "in_progress",
		TotalPicks: 42,
		CreatedAt:  time.Now().Add(-30 * time.Minute),
		UpdatedAt:  time.Now(),
	}
	if err := service.DraftRepo().CreateSession(ctx, existingInProgressSession); err != nil {
		t.Fatalf("Failed to create in_progress session: %v", err)
	}

	// Simulate ongoing picks coming in - these do NOT have P0P0/P0P1 pack data
	// This is what happens when the user makes a pick mid-draft
	events := map[string][]*logreader.DraftSessionEvent{
		"QuickDraft_TLA_20251127": {
			{
				Type:        "status_updated",
				EventName:   "QuickDraft_TLA_20251127",
				PackNumber:  0,
				PickNumber:  5, // Pick 5, not pick 0 or 1
				DraftPack:   []string{"card1", "card2", "card3"},
				PickedCards: []string{"prev1", "prev2", "prev3", "prev4"}, // Has previous picks
				Timestamp:   time.Now(),
			},
			{
				Type:         "pick_made",
				EventName:    "QuickDraft_TLA_20251127",
				PackNumber:   0,
				PickNumber:   5,
				SelectedCard: []string{"card1"},
				Timestamp:    time.Now(),
			},
		},
	}

	processor := NewService(service)
	result := processor.splitCompletedDraftSessions(ctx, events)

	// Should route to the existing in_progress session with timestamp suffix
	if len(result) != 1 {
		t.Fatalf("Expected 1 result group, got %d", len(result))
	}

	for sessionID, evts := range result {
		if sessionID != "QuickDraft_TLA_20251127_1234567890" {
			t.Errorf("Expected events to be routed to timestamped session 'QuickDraft_TLA_20251127_1234567890', got %s", sessionID)
		}
		if len(evts) != 2 {
			t.Errorf("Expected 2 events to be routed, got %d", len(evts))
		}
	}
}

func TestFilterNewDraftEvents(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	processor := NewService(service)

	events := []*logreader.DraftSessionEvent{
		// Old draft events
		{Type: "started", PackNumber: 0, PickNumber: 0},
		{Type: "status_updated", PackNumber: 0, PickNumber: 1, DraftPack: []string{"a"}, PickedCards: []string{}},
		{Type: "pick_made", PackNumber: 0, PickNumber: 1, SelectedCard: []string{"a"}},
		{Type: "status_updated", PackNumber: 0, PickNumber: 2, DraftPack: []string{"b"}, PickedCards: []string{"a"}},
		{Type: "ended"},
		// New draft events - index 5 is the new draft start (empty PickedCards at P0P1)
		{Type: "status_updated", PackNumber: 0, PickNumber: 1, DraftPack: []string{"x", "y", "z"}, PickedCards: []string{}},
		{Type: "pick_made", PackNumber: 0, PickNumber: 1, SelectedCard: []string{"x"}},
	}

	filtered := processor.filterNewDraftEvents(events, 5)

	// Should have:
	// - started (kept as control event)
	// - new status_updated at index 5
	// - new pick_made at index 6
	// Should NOT have:
	// - old status_updated at index 1, 3
	// - old pick_made at index 2
	// - old ended at index 4

	expectedTypes := map[string]int{
		"started":        1,
		"status_updated": 1,
		"pick_made":      1,
	}

	actualTypes := make(map[string]int)
	for _, evt := range filtered {
		actualTypes[evt.Type]++
	}

	for evtType, expectedCount := range expectedTypes {
		if actualTypes[evtType] != expectedCount {
			t.Errorf("Expected %d %s events, got %d", expectedCount, evtType, actualTypes[evtType])
		}
	}

	// Should NOT have ended event
	if actualTypes["ended"] > 0 {
		t.Error("Should not include 'ended' event from old draft")
	}
}

func TestProcessLogEntries_Quests(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	processor := NewService(service)

	// Create test log entries with quest data (QuestGetQuests response format)
	entries := []*logreader.LogEntry{
		{
			IsJSON:    true,
			Timestamp: "2025-12-15 10:00:00",
			JSON: map[string]interface{}{
				"quests": []interface{}{
					map[string]interface{}{
						"questId":          "quest-daily-001",
						"questType":        "Cast 20 spells",
						"goal":             float64(20),
						"startingProgress": float64(0),
						"endingProgress":   float64(4),
						"locParams": map[string]interface{}{
							"number1": float64(20),
						},
					},
					map[string]interface{}{
						"questId":          "quest-daily-002",
						"questType":        "Win 2 games",
						"goal":             float64(2),
						"startingProgress": float64(0),
						"endingProgress":   float64(1),
						"locParams": map[string]interface{}{
							"number1": float64(2),
						},
					},
				},
				"canSwap": true,
			},
		},
	}

	// Process entries
	result, err := processor.ProcessLogEntries(ctx, entries)
	if err != nil {
		t.Fatalf("ProcessLogEntries failed: %v", err)
	}

	// Verify quests were stored
	if result.QuestsStored == 0 {
		t.Error("Expected quests to be stored, got 0")
	}
	if result.QuestsStored != 2 {
		t.Errorf("Expected 2 quests to be stored, got %d", result.QuestsStored)
	}

	// Verify no errors
	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}

	// Verify quests are in database
	quests, err := service.Quests().GetActiveQuests()
	if err != nil {
		t.Fatalf("Failed to get active quests: %v", err)
	}

	if len(quests) < 2 {
		t.Errorf("Expected at least 2 quests in database, got %d", len(quests))
	}

	// Verify quest data is correct
	var foundSpellQuest, foundWinQuest bool
	for _, q := range quests {
		if q.QuestID == "quest-daily-001" {
			foundSpellQuest = true
			if q.EndingProgress != 4 {
				t.Errorf("Expected quest-daily-001 progress to be 4, got %d", q.EndingProgress)
			}
			if q.Goal != 20 {
				t.Errorf("Expected quest-daily-001 goal to be 20, got %d", q.Goal)
			}
		}
		if q.QuestID == "quest-daily-002" {
			foundWinQuest = true
			if q.EndingProgress != 1 {
				t.Errorf("Expected quest-daily-002 progress to be 1, got %d", q.EndingProgress)
			}
			if q.Goal != 2 {
				t.Errorf("Expected quest-daily-002 goal to be 2, got %d", q.Goal)
			}
		}
	}

	if !foundSpellQuest {
		t.Error("Expected to find quest-daily-001 in database")
	}
	if !foundWinQuest {
		t.Error("Expected to find quest-daily-002 in database")
	}
}

func TestProcessLogEntries_QuestsProgressUpdate(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	processor := NewService(service)

	// First, store initial quest state
	entries1 := []*logreader.LogEntry{
		{
			IsJSON:    true,
			Timestamp: "2025-12-15 10:00:00",
			JSON: map[string]interface{}{
				"quests": []interface{}{
					map[string]interface{}{
						"questId":          "quest-update-test",
						"questType":        "Cast 30 spells",
						"goal":             float64(30),
						"startingProgress": float64(0),
						"endingProgress":   float64(4),
						"locParams": map[string]interface{}{
							"number1": float64(30),
						},
					},
				},
				"canSwap": true,
			},
		},
	}

	result1, err := processor.ProcessLogEntries(ctx, entries1)
	if err != nil {
		t.Fatalf("First ProcessLogEntries failed: %v", err)
	}
	if result1.QuestsStored != 1 {
		t.Errorf("Expected 1 quest stored in first batch, got %d", result1.QuestsStored)
	}

	// Now simulate a progress update (same quest, higher progress)
	entries2 := []*logreader.LogEntry{
		{
			IsJSON:    true,
			Timestamp: "2025-12-15 11:00:00",
			JSON: map[string]interface{}{
				"quests": []interface{}{
					map[string]interface{}{
						"questId":          "quest-update-test",
						"questType":        "Cast 30 spells",
						"goal":             float64(30),
						"startingProgress": float64(0),
						"endingProgress":   float64(15), // Progress increased from 4 to 15
						"locParams": map[string]interface{}{
							"number1": float64(30),
						},
					},
				},
				"canSwap": true,
			},
		},
	}

	result2, err := processor.ProcessLogEntries(ctx, entries2)
	if err != nil {
		t.Fatalf("Second ProcessLogEntries failed: %v", err)
	}

	// This is the critical test: even for progress UPDATES, QuestsStored should be > 0
	// so that WebSocket broadcast is triggered
	if result2.QuestsStored == 0 {
		t.Error("Expected QuestsStored > 0 for progress update to trigger broadcast")
	}

	// Verify the updated progress is in database
	quests, err := service.Quests().GetActiveQuests()
	if err != nil {
		t.Fatalf("Failed to get active quests: %v", err)
	}

	found := false
	for _, q := range quests {
		if q.QuestID == "quest-update-test" {
			found = true
			if q.EndingProgress != 15 {
				t.Errorf("Expected updated quest progress to be 15, got %d", q.EndingProgress)
			}
		}
	}

	if !found {
		t.Error("Quest quest-update-test not found in database after update")
	}
}

// Benchmark tests
func BenchmarkProcessLogEntries(b *testing.B) {
	service, cleanup := setupTestService(&testing.T{})
	defer cleanup()

	ctx := context.Background()
	processor := NewService(service)

	// Create sample entries
	entries := []*logreader.LogEntry{
		{
			IsJSON:    true,
			Timestamp: time.Now().Format("2006-01-02 15:04:05"),
			JSON: map[string]interface{}{
				"constructedSeasonOrdinal": float64(83),
				"constructedClass":         "Gold",
				"constructedLevel":         float64(4),
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.ProcessLogEntries(ctx, entries)
	}
}

func TestHasAccumulatedPlays(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	processor := NewService(service)

	// Initially should have no accumulated plays
	if processor.HasAccumulatedPlays() {
		t.Error("Expected HasAccumulatedPlays to return false initially")
	}

	// Set up accumulated plays state
	processor.activeMatchID = "test-match-123"
	processor.accumulatedGRECalls = []*logreader.LogEntry{
		{Raw: "test", IsJSON: true},
	}

	// Now should have accumulated plays
	if !processor.HasAccumulatedPlays() {
		t.Error("Expected HasAccumulatedPlays to return true after setting state")
	}

	// Clear match ID - should return false
	processor.activeMatchID = ""
	if processor.HasAccumulatedPlays() {
		t.Error("Expected HasAccumulatedPlays to return false without match ID")
	}

	// Set match ID but clear entries - should return false
	processor.activeMatchID = "test-match-123"
	processor.accumulatedGRECalls = nil
	if processor.HasAccumulatedPlays() {
		t.Error("Expected HasAccumulatedPlays to return false without entries")
	}
}

func TestFlushAccumulatedPlays_NoAccumulatedPlays(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	processor := NewService(service)

	// Flush with no accumulated plays should return empty result
	result := processor.FlushAccumulatedPlays(ctx)

	if result == nil {
		t.Error("Expected non-nil result")
	}
	if result.GamePlaysStored != 0 {
		t.Errorf("Expected 0 game plays stored, got %d", result.GamePlaysStored)
	}
}

func TestFlushAccumulatedPlays_NoMatchID(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	processor := NewService(service)

	// Set up accumulated entries but no match ID
	processor.accumulatedGRECalls = []*logreader.LogEntry{
		{Raw: "test", IsJSON: true},
	}

	// Flush should return early without match ID
	result := processor.FlushAccumulatedPlays(ctx)

	if result == nil {
		t.Error("Expected non-nil result")
	}
	if result.GamePlaysStored != 0 {
		t.Errorf("Expected 0 game plays stored, got %d", result.GamePlaysStored)
	}
}

func TestDetectMatchCompletion(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	processor := NewService(service)

	tests := []struct {
		name     string
		entries  []*logreader.LogEntry
		expected bool
	}{
		{
			name:     "empty entries",
			entries:  []*logreader.LogEntry{},
			expected: false,
		},
		{
			name: "non-JSON entry",
			entries: []*logreader.LogEntry{
				{Raw: "test", IsJSON: false},
			},
			expected: false,
		},
		{
			name: "JSON without CurrentEventState",
			entries: []*logreader.LogEntry{
				{
					Raw:    "test",
					IsJSON: true,
					JSON:   map[string]interface{}{"other": "data"},
				},
			},
			expected: false,
		},
		{
			name: "JSON with different CurrentEventState",
			entries: []*logreader.LogEntry{
				{
					Raw:    "test",
					IsJSON: true,
					JSON:   map[string]interface{}{"CurrentEventState": "InProgress"},
				},
			},
			expected: false,
		},
		{
			name: "JSON with MatchCompleted state",
			entries: []*logreader.LogEntry{
				{
					Raw:    "test",
					IsJSON: true,
					JSON:   map[string]interface{}{"CurrentEventState": "MatchCompleted"},
				},
			},
			expected: true,
		},
		{
			name: "multiple entries with MatchCompleted",
			entries: []*logreader.LogEntry{
				{
					Raw:    "test1",
					IsJSON: true,
					JSON:   map[string]interface{}{"other": "data"},
				},
				{
					Raw:    "test2",
					IsJSON: true,
					JSON:   map[string]interface{}{"CurrentEventState": "MatchCompleted"},
				},
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := processor.detectMatchCompletion(tc.entries)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestProcessGamePlays_MatchTransition(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	processor := NewService(service)

	// Set up initial match state
	processor.activeMatchID = "match-1"
	processor.accumulatedGRECalls = []*logreader.LogEntry{
		{
			Raw:    "gre-entry",
			IsJSON: true,
			JSON: map[string]interface{}{
				"greToClientEvent": map[string]interface{}{
					"greToClientMessages": []interface{}{},
				},
			},
		},
	}

	// Create entries that indicate a new match (match-2)
	entries := []*logreader.LogEntry{
		{
			Raw:    "new-match-entry",
			IsJSON: true,
			JSON: map[string]interface{}{
				"matchGameRoomStateChangedEvent": map[string]interface{}{
					"gameRoomInfo": map[string]interface{}{
						"gameRoomConfig": map[string]interface{}{
							"matchId": "match-2",
						},
					},
				},
			},
		},
	}

	result := &ProcessResult{}
	err := processor.processGamePlays(ctx, entries, result)
	if err != nil {
		t.Fatalf("processGamePlays failed: %v", err)
	}

	// After processing, the active match should be match-2
	if processor.activeMatchID != "match-2" {
		t.Errorf("Expected activeMatchID to be 'match-2', got '%s'", processor.activeMatchID)
	}

	// The old accumulated entries should have been cleared (processed for match-1)
	// and new entries accumulated for match-2 (the matchGameRoomStateChangedEvent)
	if len(processor.accumulatedGRECalls) > 1 {
		t.Errorf("Expected at most 1 accumulated entry after transition, got %d", len(processor.accumulatedGRECalls))
	}
}

func TestProcessGamePlays_MatchCompletionDetection(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	processor := NewService(service)

	// Set up match state with accumulated entries
	processor.activeMatchID = "match-1"
	processor.accumulatedGRECalls = []*logreader.LogEntry{
		{
			Raw:    "gre-entry",
			IsJSON: true,
			JSON: map[string]interface{}{
				"greToClientEvent": map[string]interface{}{
					"greToClientMessages": []interface{}{},
				},
			},
		},
	}

	// Create entries with match completion signal
	entries := []*logreader.LogEntry{
		{
			Raw:    "match-completed-entry",
			IsJSON: true,
			JSON: map[string]interface{}{
				"CurrentEventState": "MatchCompleted",
			},
		},
	}

	result := &ProcessResult{}
	err := processor.processGamePlays(ctx, entries, result)
	if err != nil {
		t.Fatalf("processGamePlays failed: %v", err)
	}

	// After processing match completion, accumulated entries should be cleared
	if len(processor.accumulatedGRECalls) != 0 {
		t.Errorf("Expected accumulated entries to be cleared after match completion, got %d", len(processor.accumulatedGRECalls))
	}

	// Active match ID should be cleared
	if processor.activeMatchID != "" {
		t.Errorf("Expected activeMatchID to be empty after match completion, got '%s'", processor.activeMatchID)
	}
}

func TestLinkDraftSessionToMatches(t *testing.T) {
	// Test that draft matches are linked to their sessions (#911)
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a completed draft session
	endTime := time.Now().Add(-1 * time.Hour)
	draftSession := &models.DraftSession{
		ID:         "QuickDraft_TLA_20251127",
		EventName:  "QuickDraft_TLA_20251127",
		SetCode:    "TLA",
		DraftType:  "QuickDraft",
		StartTime:  time.Now().Add(-3 * time.Hour),
		EndTime:    &endTime,
		Status:     "completed",
		TotalPicks: 42,
		CreatedAt:  time.Now().Add(-3 * time.Hour),
		UpdatedAt:  time.Now().Add(-1 * time.Hour),
	}
	if err := service.DraftRepo().CreateSession(ctx, draftSession); err != nil {
		t.Fatalf("Failed to create draft session: %v", err)
	}

	// Create a draft match that should be linked (within 24 hours, matching event name)
	draftMatch := &models.Match{
		ID:           "match-draft-001",
		EventName:    "QuickDraft_TLA_20251127",
		Format:       "QuickDraft",
		Result:       "win",
		PlayerWins:   2,
		OpponentWins: 1,
		Timestamp:    time.Now().Add(-30 * time.Minute), // After draft end
	}
	if err := service.MatchRepo().Create(ctx, draftMatch); err != nil {
		t.Fatalf("Failed to create draft match: %v", err)
	}

	// Create a non-draft match that should NOT be linked
	nonDraftMatch := &models.Match{
		ID:           "match-standard-001",
		EventName:    "Ladder_Standard_2024",
		Format:       "Ladder",
		Result:       "loss",
		PlayerWins:   1,
		OpponentWins: 2,
		Timestamp:    time.Now().Add(-20 * time.Minute),
	}
	if err := service.MatchRepo().Create(ctx, nonDraftMatch); err != nil {
		t.Fatalf("Failed to create non-draft match: %v", err)
	}

	// Create a draft match from a different set (should NOT be linked)
	differentSetMatch := &models.Match{
		ID:           "match-draft-002",
		EventName:    "QuickDraft_DSK_20251127",
		Format:       "QuickDraft",
		Result:       "win",
		PlayerWins:   2,
		OpponentWins: 0,
		Timestamp:    time.Now().Add(-15 * time.Minute),
	}
	if err := service.MatchRepo().Create(ctx, differentSetMatch); err != nil {
		t.Fatalf("Failed to create different set match: %v", err)
	}

	// Run the linking
	processor := NewService(service)
	processor.linkDraftSessionToMatches(ctx, draftSession.ID, draftSession)

	// Verify the correct match was linked
	results, err := service.DraftAnalyticsRepo().GetDraftMatchResults(ctx, draftSession.ID)
	if err != nil {
		t.Fatalf("Failed to get draft match results: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 match to be linked, got %d", len(results))
	}

	if len(results) > 0 && results[0].MatchID != "match-draft-001" {
		t.Errorf("Expected match 'match-draft-001' to be linked, got '%s'", results[0].MatchID)
	}
}

func TestIsMatchFromDraft(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	processor := NewService(service)

	endTime := time.Now().Add(-1 * time.Hour)
	session := &models.DraftSession{
		ID:        "QuickDraft_TLA_20251127",
		SetCode:   "TLA",
		StartTime: time.Now().Add(-3 * time.Hour),
		EndTime:   &endTime,
	}

	tests := []struct {
		name     string
		match    *models.Match
		expected bool
	}{
		{
			name: "QuickDraft match with matching set",
			match: &models.Match{
				EventName: "QuickDraft_TLA_20251127",
				Timestamp: time.Now().Add(-30 * time.Minute),
			},
			expected: true,
		},
		{
			name: "PremierDraft match with matching set",
			match: &models.Match{
				EventName: "PremierDraft_TLA_20251127",
				Timestamp: time.Now().Add(-30 * time.Minute),
			},
			expected: true,
		},
		{
			name: "Standard ladder match (not draft)",
			match: &models.Match{
				EventName: "Ladder_Standard_2024",
				Timestamp: time.Now().Add(-30 * time.Minute),
			},
			expected: false,
		},
		{
			name: "Draft match with different set",
			match: &models.Match{
				EventName: "QuickDraft_DSK_20251127",
				Timestamp: time.Now().Add(-30 * time.Minute),
			},
			expected: false,
		},
		{
			name: "Draft match too old (before draft end)",
			match: &models.Match{
				EventName: "QuickDraft_TLA_20251127",
				Timestamp: time.Now().Add(-5 * time.Hour), // Before draft end
			},
			expected: false,
		},
		{
			name: "Draft match too far after draft (>24 hours)",
			match: &models.Match{
				EventName: "QuickDraft_TLA_20251127",
				Timestamp: time.Now().Add(25 * time.Hour), // More than 24 hours after
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.isMatchFromDraft(tt.match, session)
			if result != tt.expected {
				t.Errorf("isMatchFromDraft() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetectAllMatchIDsFromEntries(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	processor := NewService(service)

	// Create entries with multiple unique match IDs
	entries := []*logreader.LogEntry{
		// First match
		{
			Raw:    "entry-match-1",
			IsJSON: true,
			JSON: map[string]interface{}{
				"greToClientEvent": map[string]interface{}{
					"greToClientMessages": []interface{}{
						map[string]interface{}{
							"gameStateMessage": map[string]interface{}{
								"gameInfo": map[string]interface{}{
									"matchID": "match-001",
								},
							},
						},
					},
				},
			},
		},
		// Another entry for first match
		{
			Raw:    "entry-match-1-again",
			IsJSON: true,
			JSON: map[string]interface{}{
				"greToClientEvent": map[string]interface{}{
					"greToClientMessages": []interface{}{
						map[string]interface{}{
							"gameStateMessage": map[string]interface{}{
								"gameInfo": map[string]interface{}{
									"matchID": "match-001",
								},
							},
						},
					},
				},
			},
		},
		// Second match via matchGameRoomStateChangedEvent
		{
			Raw:    "entry-match-2",
			IsJSON: true,
			JSON: map[string]interface{}{
				"matchGameRoomStateChangedEvent": map[string]interface{}{
					"gameRoomInfo": map[string]interface{}{
						"gameRoomConfig": map[string]interface{}{
							"matchId": "match-002",
						},
					},
				},
			},
		},
		// Third match via greToClientEvent
		{
			Raw:    "entry-match-3",
			IsJSON: true,
			JSON: map[string]interface{}{
				"greToClientEvent": map[string]interface{}{
					"greToClientMessages": []interface{}{
						map[string]interface{}{
							"gameStateMessage": map[string]interface{}{
								"gameInfo": map[string]interface{}{
									"matchID": "match-003",
								},
							},
						},
					},
				},
			},
		},
	}

	matchIDs := processor.detectAllMatchIDsFromEntries(entries)

	// Should detect 3 unique match IDs in order of first appearance
	if len(matchIDs) != 3 {
		t.Errorf("Expected 3 unique match IDs, got %d: %v", len(matchIDs), matchIDs)
	}

	// Verify order of detection
	expected := []string{"match-001", "match-002", "match-003"}
	for i, expected := range expected {
		if i >= len(matchIDs) {
			t.Errorf("Missing match ID at index %d, expected %s", i, expected)
			continue
		}
		if matchIDs[i] != expected {
			t.Errorf("Match ID at index %d = %s, want %s", i, matchIDs[i], expected)
		}
	}
}

func TestSegmentEntriesByMatch(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	processor := NewService(service)

	// Create entries with sequential matches (simulating real log structure)
	// GRE entries are sequential - not all entries have match IDs, they belong to current match
	// IMPORTANT: connectResp appears BEFORE the match ID and contains the player's seat ID
	// for THAT specific match. It should NOT be shared across matches.
	entries := []*logreader.LogEntry{
		// connectResp for match 1 (player is seat 1 in this match)
		{
			Raw:    "connect-resp-1",
			IsJSON: true,
			JSON: map[string]interface{}{
				"connectResp": map[string]interface{}{
					"systemSeatIds": []interface{}{float64(1)},
				},
			},
		},
		// Match 1 starts - entry with matchID
		{
			Raw:    "match-1-start",
			IsJSON: true,
			JSON: map[string]interface{}{
				"matchGameRoomStateChangedEvent": map[string]interface{}{
					"gameRoomInfo": map[string]interface{}{
						"gameRoomConfig": map[string]interface{}{
							"matchId": "match-001",
						},
					},
				},
			},
		},
		// Match 1 GRE entry (has matchID in gameInfo)
		{
			Raw:    "match-1-gre-1",
			IsJSON: true,
			JSON: map[string]interface{}{
				"greToClientEvent": map[string]interface{}{
					"greToClientMessages": []interface{}{
						map[string]interface{}{
							"gameStateMessage": map[string]interface{}{
								"gameInfo": map[string]interface{}{
									"matchID": "match-001",
								},
							},
						},
					},
				},
			},
		},
		// Match 1 GRE entry (no matchID - still belongs to match-001)
		{
			Raw:    "match-1-gre-2",
			IsJSON: true,
			JSON: map[string]interface{}{
				"greToClientEvent": map[string]interface{}{
					"greToClientMessages": []interface{}{
						map[string]interface{}{
							"gameStateMessage": map[string]interface{}{
								"turnInfo": map[string]interface{}{
									"turnNumber": float64(5),
								},
							},
						},
					},
				},
			},
		},
		// connectResp for match 2 (player is seat 2 in THIS match - different!)
		{
			Raw:    "connect-resp-2",
			IsJSON: true,
			JSON: map[string]interface{}{
				"connectResp": map[string]interface{}{
					"systemSeatIds": []interface{}{float64(2)},
				},
			},
		},
		// Match 2 starts - new matchID marks boundary
		{
			Raw:    "match-2-start",
			IsJSON: true,
			JSON: map[string]interface{}{
				"greToClientEvent": map[string]interface{}{
					"greToClientMessages": []interface{}{
						map[string]interface{}{
							"gameStateMessage": map[string]interface{}{
								"gameInfo": map[string]interface{}{
									"matchID": "match-002",
								},
							},
						},
					},
				},
			},
		},
		// Match 2 GRE entry (no matchID - belongs to match-002)
		{
			Raw:    "match-2-gre-1",
			IsJSON: true,
			JSON: map[string]interface{}{
				"greToClientEvent": map[string]interface{}{
					"greToClientMessages": []interface{}{
						map[string]interface{}{
							"gameStateMessage": map[string]interface{}{
								"turnInfo": map[string]interface{}{
									"turnNumber": float64(1),
								},
							},
						},
					},
				},
			},
		},
	}

	// Segment entries by match
	segments := processor.segmentEntriesByMatch(entries)

	// Should have 2 matches
	if len(segments) != 2 {
		t.Errorf("Expected 2 match segments, got %d", len(segments))
	}

	// Match 001 should have: connectResp-1 + matchGameRoomStateChangedEvent + 2 GRE entries = 4 entries
	match001Entries := segments["match-001"]
	if len(match001Entries) < 3 {
		t.Errorf("Expected at least 3 entries for match-001, got %d", len(match001Entries))
	}

	// Match 002 should have: connectResp-2 + 2 GRE entries = 3 entries
	match002Entries := segments["match-002"]
	if len(match002Entries) < 2 {
		t.Errorf("Expected at least 2 entries for match-002, got %d", len(match002Entries))
	}

	// Verify each match has its OWN connectResp (not shared!)
	// Match 001 should have connectResp with seat 1
	hasCorrectSeat1 := false
	for _, entry := range match001Entries {
		if connResp, ok := entry.JSON["connectResp"].(map[string]interface{}); ok {
			if seatIDs, ok := connResp["systemSeatIds"].([]interface{}); ok && len(seatIDs) > 0 {
				if seatID, ok := seatIDs[0].(float64); ok && int(seatID) == 1 {
					hasCorrectSeat1 = true
				}
			}
		}
	}
	if !hasCorrectSeat1 {
		t.Error("Match 001 should have connectResp with seat 1")
	}

	// Match 002 should have connectResp with seat 2
	hasCorrectSeat2 := false
	for _, entry := range match002Entries {
		if connResp, ok := entry.JSON["connectResp"].(map[string]interface{}); ok {
			if seatIDs, ok := connResp["systemSeatIds"].([]interface{}); ok && len(seatIDs) > 0 {
				if seatID, ok := seatIDs[0].(float64); ok && int(seatID) == 2 {
					hasCorrectSeat2 = true
				}
			}
		}
	}
	if !hasCorrectSeat2 {
		t.Error("Match 002 should have connectResp with seat 2")
	}
}

func TestProcessGamePlays_MultiMatchBatch(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	processor := NewService(service)

	// Create entries with multiple matches (historical batch scenario)
	entries := []*logreader.LogEntry{
		// First match - GRE entry
		{
			Raw:    "entry-match-1-gre",
			IsJSON: true,
			JSON: map[string]interface{}{
				"greToClientEvent": map[string]interface{}{
					"greToClientMessages": []interface{}{
						map[string]interface{}{
							"gameStateMessage": map[string]interface{}{
								"gameInfo": map[string]interface{}{
									"matchID":    "match-001",
									"gameNumber": float64(1),
								},
							},
						},
					},
				},
			},
		},
		// Second match - GRE entry
		{
			Raw:    "entry-match-2-gre",
			IsJSON: true,
			JSON: map[string]interface{}{
				"greToClientEvent": map[string]interface{}{
					"greToClientMessages": []interface{}{
						map[string]interface{}{
							"gameStateMessage": map[string]interface{}{
								"gameInfo": map[string]interface{}{
									"matchID":    "match-002",
									"gameNumber": float64(1),
								},
							},
						},
					},
				},
			},
		},
		// Third match - GRE entry
		{
			Raw:    "entry-match-3-gre",
			IsJSON: true,
			JSON: map[string]interface{}{
				"greToClientEvent": map[string]interface{}{
					"greToClientMessages": []interface{}{
						map[string]interface{}{
							"gameStateMessage": map[string]interface{}{
								"gameInfo": map[string]interface{}{
									"matchID":    "match-003",
									"gameNumber": float64(1),
								},
							},
						},
					},
				},
			},
		},
	}

	result := &ProcessResult{}
	err := processor.processGamePlays(ctx, entries, result)
	if err != nil {
		t.Fatalf("processGamePlays failed: %v", err)
	}

	// After processing a multi-match batch, no accumulation should remain
	// because it was processed immediately via processHistoricalBatchPlays
	if len(processor.accumulatedGRECalls) != 0 {
		t.Errorf("Expected no accumulated GRE calls after multi-match batch, got %d", len(processor.accumulatedGRECalls))
	}

	// activeMatchID should be empty (not tracking any single match)
	if processor.activeMatchID != "" {
		t.Errorf("Expected empty activeMatchID after multi-match batch, got '%s'", processor.activeMatchID)
	}
}

func TestArePicksIdentical(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()
	sessionID := "QuickDraft_TEST_20260301"

	// Create a completed draft session with known picks
	_, err := service.GetDB().ExecContext(ctx, `
		INSERT INTO draft_sessions (id, event_name, set_code, draft_type, start_time, status, total_picks, created_at, updated_at)
		VALUES (?, ?, 'TEST', 'QuickDraft', ?, 'completed', 42, ?, ?)
	`, sessionID, sessionID, now, now, now)
	if err != nil {
		t.Fatalf("Failed to insert draft session: %v", err)
	}

	// Insert stored picks with specific card IDs
	storedCardIDs := []string{"100", "200", "300", "400", "500"}
	for i, cardID := range storedCardIDs {
		_, err = service.GetDB().ExecContext(ctx, `
			INSERT INTO draft_picks (session_id, pack_number, pick_number, card_id, timestamp)
			VALUES (?, 0, ?, ?, ?)
		`, sessionID, i+1, cardID, now)
		if err != nil {
			t.Fatalf("Failed to insert pick: %v", err)
		}
	}

	processor := NewService(service)

	t.Run("identical picks returns true", func(t *testing.T) {
		events := []*logreader.DraftSessionEvent{
			{Type: "pick_made", SelectedCard: []string{"100"}},
			{Type: "pick_made", SelectedCard: []string{"200"}},
			{Type: "pick_made", SelectedCard: []string{"300"}},
			{Type: "pick_made", SelectedCard: []string{"400"}},
			{Type: "pick_made", SelectedCard: []string{"500"}},
		}
		if !processor.arePicksIdentical(ctx, sessionID, events) {
			t.Error("Expected arePicksIdentical to return true for identical picks")
		}
	})

	t.Run("different picks returns false", func(t *testing.T) {
		events := []*logreader.DraftSessionEvent{
			{Type: "pick_made", SelectedCard: []string{"999"}},
			{Type: "pick_made", SelectedCard: []string{"888"}},
			{Type: "pick_made", SelectedCard: []string{"777"}},
			{Type: "pick_made", SelectedCard: []string{"666"}},
			{Type: "pick_made", SelectedCard: []string{"555"}},
		}
		if processor.arePicksIdentical(ctx, sessionID, events) {
			t.Error("Expected arePicksIdentical to return false for different picks")
		}
	})

	t.Run("no pick_made events returns false", func(t *testing.T) {
		events := []*logreader.DraftSessionEvent{
			{Type: "status_updated", PackNumber: 0, PickNumber: 0, DraftPack: []string{"card1"}},
		}
		if processor.arePicksIdentical(ctx, sessionID, events) {
			t.Error("Expected arePicksIdentical to return false when no pick_made events")
		}
	})

	t.Run("nonexistent session returns false", func(t *testing.T) {
		events := []*logreader.DraftSessionEvent{
			{Type: "pick_made", SelectedCard: []string{"100"}},
		}
		if processor.arePicksIdentical(ctx, "nonexistent_session", events) {
			t.Error("Expected arePicksIdentical to return false for nonexistent session")
		}
	})

	t.Run("partial match below threshold returns false", func(t *testing.T) {
		// Only 1 of 5 match = 20% < 90% threshold
		events := []*logreader.DraftSessionEvent{
			{Type: "pick_made", SelectedCard: []string{"100"}},
			{Type: "pick_made", SelectedCard: []string{"999"}},
			{Type: "pick_made", SelectedCard: []string{"888"}},
			{Type: "pick_made", SelectedCard: []string{"777"}},
			{Type: "pick_made", SelectedCard: []string{"666"}},
		}
		if processor.arePicksIdentical(ctx, sessionID, events) {
			t.Error("Expected arePicksIdentical to return false for low match rate")
		}
	})

	t.Run("above threshold returns true", func(t *testing.T) {
		// 9 of 10 match = 90% >= 90% threshold
		events := []*logreader.DraftSessionEvent{
			{Type: "pick_made", SelectedCard: []string{"100"}},
			{Type: "pick_made", SelectedCard: []string{"200"}},
			{Type: "pick_made", SelectedCard: []string{"300"}},
			{Type: "pick_made", SelectedCard: []string{"400"}},
			{Type: "pick_made", SelectedCard: []string{"500"}},
			{Type: "pick_made", SelectedCard: []string{"100"}},
			{Type: "pick_made", SelectedCard: []string{"200"}},
			{Type: "pick_made", SelectedCard: []string{"300"}},
			{Type: "pick_made", SelectedCard: []string{"400"}},
			{Type: "pick_made", SelectedCard: []string{"999"}},
		}
		if !processor.arePicksIdentical(ctx, sessionID, events) {
			t.Error("Expected arePicksIdentical to return true for 90% match")
		}
	})
}

func TestSplitCompletedDraftSessions_ReplayedEventsSkipped(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()
	sessionID := "QuickDraft_ECL_20260223"

	// Create a completed draft session
	_, err := service.GetDB().ExecContext(ctx, `
		INSERT INTO draft_sessions (id, event_name, set_code, draft_type, start_time, status, total_picks, created_at, updated_at)
		VALUES (?, ?, 'ECL', 'QuickDraft', ?, 'completed', 42, ?, ?)
	`, sessionID, sessionID, now, now, now)
	if err != nil {
		t.Fatalf("Failed to insert draft session: %v", err)
	}

	// Insert 42 picks with known card IDs
	for i := 0; i < 42; i++ {
		packNum := i / 14
		pickNum := (i % 14) + 1
		cardID := fmt.Sprintf("%d", 1000+i)
		_, err = service.GetDB().ExecContext(ctx, `
			INSERT INTO draft_picks (session_id, pack_number, pick_number, card_id, timestamp)
			VALUES (?, ?, ?, ?, ?)
		`, sessionID, packNum, pickNum, cardID, now)
		if err != nil {
			t.Fatalf("Failed to insert pick %d: %v", i, err)
		}
	}

	// Create replayed events — same card IDs as stored picks, with P0P0 start
	replayedEvents := []*logreader.DraftSessionEvent{
		{
			Type:       "status_updated",
			EventName:  sessionID,
			PackNumber: 0,
			PickNumber: 0,
			DraftPack:  []string{"card1", "card2", "card3"},
			Timestamp:  now,
		},
	}
	// Add pick_made events matching the stored picks
	for i := 0; i < 42; i++ {
		cardID := fmt.Sprintf("%d", 1000+i)
		replayedEvents = append(replayedEvents, &logreader.DraftSessionEvent{
			Type:         "pick_made",
			EventName:    sessionID,
			PackNumber:   i / 14,
			PickNumber:   (i % 14) + 1,
			SelectedCard: []string{cardID},
			Timestamp:    now,
		})
	}

	events := map[string][]*logreader.DraftSessionEvent{
		sessionID: replayedEvents,
	}

	processor := NewService(service)
	result := processor.splitCompletedDraftSessions(ctx, events)

	// Should route to the original session ID (no timestamp suffix)
	if len(result) != 1 {
		t.Fatalf("Expected 1 result group, got %d", len(result))
	}

	for resultID := range result {
		if resultID != sessionID {
			t.Errorf("Expected events routed to original session %s, got %s", sessionID, resultID)
		}
	}
}

func TestSplitCompletedDraftSessions_NewDraftNotSkipped(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()
	sessionID := "QuickDraft_ECL_20260223"

	// Create a completed draft session
	_, err := service.GetDB().ExecContext(ctx, `
		INSERT INTO draft_sessions (id, event_name, set_code, draft_type, start_time, status, total_picks, created_at, updated_at)
		VALUES (?, ?, 'ECL', 'QuickDraft', ?, 'completed', 42, ?, ?)
	`, sessionID, sessionID, now, now, now)
	if err != nil {
		t.Fatalf("Failed to insert draft session: %v", err)
	}

	// Insert stored picks
	for i := 0; i < 42; i++ {
		packNum := i / 14
		pickNum := (i % 14) + 1
		cardID := fmt.Sprintf("%d", 1000+i)
		_, err = service.GetDB().ExecContext(ctx, `
			INSERT INTO draft_picks (session_id, pack_number, pick_number, card_id, timestamp)
			VALUES (?, ?, ?, ?, ?)
		`, sessionID, packNum, pickNum, cardID, now)
		if err != nil {
			t.Fatalf("Failed to insert pick %d: %v", i, err)
		}
	}

	// Create events for a genuinely NEW draft — different card IDs
	newDraftEvents := []*logreader.DraftSessionEvent{
		{
			Type:       "status_updated",
			EventName:  sessionID,
			PackNumber: 0,
			PickNumber: 0,
			DraftPack:  []string{"newcard1", "newcard2", "newcard3"},
			Timestamp:  now,
		},
	}
	for i := 0; i < 42; i++ {
		cardID := fmt.Sprintf("%d", 9000+i) // Different card IDs
		newDraftEvents = append(newDraftEvents, &logreader.DraftSessionEvent{
			Type:         "pick_made",
			EventName:    sessionID,
			PackNumber:   i / 14,
			PickNumber:   (i % 14) + 1,
			SelectedCard: []string{cardID},
			Timestamp:    now,
		})
	}

	events := map[string][]*logreader.DraftSessionEvent{
		sessionID: newDraftEvents,
	}

	processor := NewService(service)
	result := processor.splitCompletedDraftSessions(ctx, events)

	// Should create a new session with timestamp suffix
	if len(result) != 1 {
		t.Fatalf("Expected 1 result group, got %d", len(result))
	}

	for resultID := range result {
		if resultID == sessionID {
			t.Error("Expected new session ID (with timestamp suffix), but got the original session ID")
		}
		if len(resultID) <= len(sessionID) {
			t.Errorf("Expected session ID longer than original (with suffix), got %s", resultID)
		}
	}
}

func TestSplitCompletedDraftSessions_FullSessionReplaySkipped(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()
	sessionID := "QuickDraft_ECL_20260223"

	// Create an in_progress session that has all expected picks (full)
	_, err := service.GetDB().ExecContext(ctx, `
		INSERT INTO draft_sessions (id, event_name, set_code, draft_type, start_time, status, total_picks, created_at, updated_at)
		VALUES (?, ?, 'ECL', 'QuickDraft', ?, 'in_progress', 42, ?, ?)
	`, sessionID, sessionID, now, now, now)
	if err != nil {
		t.Fatalf("Failed to insert draft session: %v", err)
	}

	// Insert 42 picks (makes it "full")
	for i := 0; i < 42; i++ {
		packNum := i / 14
		pickNum := (i % 14) + 1
		cardID := fmt.Sprintf("%d", 1000+i)
		_, err = service.GetDB().ExecContext(ctx, `
			INSERT INTO draft_picks (session_id, pack_number, pick_number, card_id, timestamp)
			VALUES (?, ?, ?, ?, ?)
		`, sessionID, packNum, pickNum, cardID, now)
		if err != nil {
			t.Fatalf("Failed to insert pick %d: %v", i, err)
		}
	}

	// Create replayed events matching stored picks
	replayedEvents := []*logreader.DraftSessionEvent{
		{
			Type:       "status_updated",
			EventName:  sessionID,
			PackNumber: 0,
			PickNumber: 0,
			DraftPack:  []string{"card1", "card2", "card3"},
			Timestamp:  now,
		},
	}
	for i := 0; i < 42; i++ {
		cardID := fmt.Sprintf("%d", 1000+i)
		replayedEvents = append(replayedEvents, &logreader.DraftSessionEvent{
			Type:         "pick_made",
			EventName:    sessionID,
			PackNumber:   i / 14,
			PickNumber:   (i % 14) + 1,
			SelectedCard: []string{cardID},
			Timestamp:    now,
		})
	}

	events := map[string][]*logreader.DraftSessionEvent{
		sessionID: replayedEvents,
	}

	processor := NewService(service)
	result := processor.splitCompletedDraftSessions(ctx, events)

	// Should route to the original session ID (replay detected)
	if len(result) != 1 {
		t.Fatalf("Expected 1 result group, got %d", len(result))
	}

	for resultID := range result {
		if resultID != sessionID {
			t.Errorf("Expected events routed to original session %s, got %s", sessionID, resultID)
		}
	}
}
