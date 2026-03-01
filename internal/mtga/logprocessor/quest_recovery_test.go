package logprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/ramonehamilton/MTGA-Companion/internal/mtga/logreader"
	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
)

// makeQuestEntry builds a LogEntry that mimics a QuestGetQuests API response.
// quests is a slice of quest maps (each with questId, locKey, goal, etc.).
// When canSwap is true, the top-level canSwap field is included so the parser
// recognises the entry as a proper QuestGetQuests response.
func makeQuestEntry(timestamp string, canSwap bool, quests []map[string]interface{}) *logreader.LogEntry {
	questsSlice := make([]interface{}, len(quests))
	for i, q := range quests {
		questsSlice[i] = q
	}
	json := map[string]interface{}{
		"quests": questsSlice,
	}
	if canSwap {
		json["canSwap"] = true
	}
	return &logreader.LogEntry{
		IsJSON:    true,
		Timestamp: timestamp,
		JSON:      json,
	}
}

// makeQuestMap returns a minimal quest map suitable for use inside a
// QuestGetQuests entry.
func makeQuestMap(questID, locKey string, goal, endingProgress int) map[string]interface{} {
	return map[string]interface{}{
		"questId":          questID,
		"locKey":           locKey,
		"goal":             float64(goal),
		"endingProgress":   float64(endingProgress),
		"startingProgress": float64(0),
		"canSwap":          true,
		"chestDescription": map[string]interface{}{
			"quantity": "500",
		},
	}
}

// TestProcessQuests_RecoveryMode_SuppressesDisappeared verifies that when recovery
// mode is enabled, a quest that "disappears" between two QuestGetQuests responses is
// NOT saved as completed to the database.
func TestProcessQuests_RecoveryMode_SuppressesDisappeared(t *testing.T) {
	storageService, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	processor := NewService(storageService)
	processor.SetRecoveryMode(true)

	ts1 := "2025-01-10 10:00:00"
	ts2 := "2025-01-10 11:00:00"

	// First response: quest is present
	entry1 := makeQuestEntry(ts1, true, []map[string]interface{}{
		makeQuestMap("quest-disappear-1", "Quests/Quest_Daily_Win", 5, 2),
	})

	// Second response: quest has disappeared (simulates completion-by-disappearance)
	entry2 := makeQuestEntry(ts2, true, []map[string]interface{}{})

	entries := []*logreader.LogEntry{entry1, entry2}

	result, err := processor.ProcessLogEntries(ctx, entries)
	if err != nil {
		t.Fatalf("ProcessLogEntries failed: %v", err)
	}

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}

	// In recovery mode, the disappeared completion should be suppressed, so the
	// quest saved to the database should NOT be marked completed.
	quests, err := storageService.Quests().GetQuestHistory(nil, nil, 0)
	if err != nil {
		t.Fatalf("GetQuestHistory failed: %v", err)
	}

	// The quest should have been stored (parser still returns it)
	if len(quests) == 0 {
		t.Fatal("Expected quest to be stored, but none found")
	}

	for _, q := range quests {
		if q.QuestID == "quest-disappear-1" && q.Completed {
			t.Errorf("Quest should NOT be marked completed in recovery mode (disappeared completion suppressed), but got completed=true")
		}
	}
}

// TestProcessQuests_RecoveryMode_AllowsProgress verifies that in recovery mode,
// progress-based completions (EndingProgress >= Goal) are still trusted and the
// quest is saved as completed.
func TestProcessQuests_RecoveryMode_AllowsProgress(t *testing.T) {
	storageService, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	processor := NewService(storageService)
	processor.SetRecoveryMode(true)

	ts := "2025-01-10 10:00:00"

	// Quest with endingProgress == goal => parser marks it "progress"-completed
	entry := makeQuestEntry(ts, true, []map[string]interface{}{
		makeQuestMap("quest-progress-1", "Quests/Quest_Daily_Win", 5, 5),
	})

	result, err := processor.ProcessLogEntries(ctx, []*logreader.LogEntry{entry})
	if err != nil {
		t.Fatalf("ProcessLogEntries failed: %v", err)
	}

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}

	quests, err := storageService.Quests().GetQuestHistory(nil, nil, 0)
	if err != nil {
		t.Fatalf("GetQuestHistory failed: %v", err)
	}

	found := false
	for _, q := range quests {
		if q.QuestID == "quest-progress-1" {
			found = true
			if !q.Completed {
				t.Errorf("Quest should be marked completed via progress in recovery mode, but completed=false")
			}
			if q.CompletionSource != "progress" {
				t.Errorf("CompletionSource = %q, want %q", q.CompletionSource, "progress")
			}
		}
	}

	if !found {
		t.Error("Quest was not stored in the database")
	}
}

// TestProcessQuests_LiveMode_CompletesNormally verifies that with recovery mode
// OFF, a quest that disappears between two responses IS saved as completed.
func TestProcessQuests_LiveMode_CompletesNormally(t *testing.T) {
	storageService, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	processor := NewService(storageService)
	// Recovery mode is OFF by default

	ts1 := "2025-01-10 10:00:00"
	ts2 := "2025-01-10 11:00:00"

	// First response: quest present
	entry1 := makeQuestEntry(ts1, true, []map[string]interface{}{
		makeQuestMap("quest-live-1", "Quests/Quest_Daily_Win", 5, 2),
	})

	// Second response: quest disappeared
	entry2 := makeQuestEntry(ts2, true, []map[string]interface{}{})

	result, err := processor.ProcessLogEntries(ctx, []*logreader.LogEntry{entry1, entry2})
	if err != nil {
		t.Fatalf("ProcessLogEntries failed: %v", err)
	}

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}

	quests, err := storageService.Quests().GetQuestHistory(nil, nil, 0)
	if err != nil {
		t.Fatalf("GetQuestHistory failed: %v", err)
	}

	found := false
	for _, q := range quests {
		if q.QuestID == "quest-live-1" {
			found = true
			if !q.Completed {
				t.Errorf("In live mode, quest should be marked completed when it disappears, but completed=false")
			}
			if q.CompletionSource != "disappeared" {
				t.Errorf("CompletionSource = %q, want %q", q.CompletionSource, "disappeared")
			}
		}
	}

	if !found {
		t.Error("Quest was not stored in the database")
	}
}

// TestProcessQuests_RecoveryMode_SkipsRerollDetection verifies that when recovery
// mode is ON, markRerolledQuests is NOT called. Concretely: if the database has an
// incomplete quest that is absent from the QuestGetQuests response, it should NOT
// be marked as rerolled.
func TestProcessQuests_RecoveryMode_SkipsRerollDetection(t *testing.T) {
	storageService, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Pre-seed an incomplete quest in the DB.
	now := time.Now().UTC()
	lastSeen := now.Add(-1 * time.Hour)
	seedQuest := &models.Quest{
		QuestID:    "quest-seed-reroll",
		QuestType:  "Quests/Quest_Daily_Win",
		Goal:       5,
		AssignedAt: now.Add(-2 * time.Hour),
		LastSeenAt: &lastSeen,
		Completed:  false,
		Rerolled:   false,
	}
	if err := storageService.Quests().Save(seedQuest); err != nil {
		t.Fatalf("Failed to pre-seed quest: %v", err)
	}

	// Process entries in recovery mode: the response contains a DIFFERENT quest,
	// so "quest-seed-reroll" is absent – reroll detection would normally fire.
	processor := NewService(storageService)
	processor.SetRecoveryMode(true)

	ts := "2025-01-10 10:00:00"
	entry := makeQuestEntry(ts, true, []map[string]interface{}{
		makeQuestMap("quest-other", "Quests/Quest_Other", 3, 1),
	})

	_, err := processor.ProcessLogEntries(ctx, []*logreader.LogEntry{entry})
	if err != nil {
		t.Fatalf("ProcessLogEntries failed: %v", err)
	}

	// In recovery mode, the seeded quest must NOT have been rerolled.
	retrieved, err := storageService.Quests().GetQuestByID(seedQuest.ID)
	if err != nil {
		t.Fatalf("GetQuestByID failed: %v", err)
	}

	if retrieved.Rerolled {
		t.Error("Quest should NOT be marked as rerolled in recovery mode, but rerolled=true")
	}
}

// TestProcessQuests_LiveMode_EmptyQuestList_MarksRerolled verifies that when MTGA
// returns an empty quest list in live mode, any incomplete quest in the DB is
// marked as rerolled. This handles the case where a user has completed all quests
// and the stale active quest should be cleaned up.
func TestProcessQuests_LiveMode_EmptyQuestList_MarksRerolled(t *testing.T) {
	storageService, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Pre-seed an incomplete quest in the DB (simulates quest stored during recovery)
	now := time.Now().UTC()
	lastSeen := now.Add(-1 * time.Hour)
	seedQuest := &models.Quest{
		QuestID:    "quest-stale-active",
		QuestType:  "Quests/Quest_Daily_Win",
		Goal:       5,
		AssignedAt: now.Add(-2 * time.Hour),
		LastSeenAt: &lastSeen,
		Completed:  false,
		Rerolled:   false,
	}
	if err := storageService.Quests().Save(seedQuest); err != nil {
		t.Fatalf("Failed to pre-seed quest: %v", err)
	}

	// Process an empty QuestGetQuests response in live mode
	processor := NewService(storageService)
	// Recovery mode is OFF by default

	ts := "2025-01-10 10:00:00"
	entry := makeQuestEntry(ts, true, []map[string]interface{}{})

	result, err := processor.ProcessLogEntries(ctx, []*logreader.LogEntry{entry})
	if err != nil {
		t.Fatalf("ProcessLogEntries failed: %v", err)
	}

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}

	// The stale quest should now be marked as rerolled
	retrieved, err := storageService.Quests().GetQuestByID(seedQuest.ID)
	if err != nil {
		t.Fatalf("GetQuestByID failed: %v", err)
	}

	if !retrieved.Rerolled {
		t.Error("Quest should be marked as rerolled when MTGA returns empty quest list in live mode, but rerolled=false")
	}

	if result.QuestsRerolled != 1 {
		t.Errorf("Expected 1 quest rerolled, got %d", result.QuestsRerolled)
	}
}

// TestProcessQuests_SessionIDSet verifies that when a session ID is configured on
// the processor, quests saved to the database carry that session ID.
func TestProcessQuests_SessionIDSet(t *testing.T) {
	storageService, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	processor := NewService(storageService)
	processor.SetSessionID("test-session-xyz")

	ts := "2025-01-10 10:00:00"
	entry := makeQuestEntry(ts, true, []map[string]interface{}{
		makeQuestMap("quest-session-1", "Quests/Quest_Daily_Win", 5, 2),
	})

	result, err := processor.ProcessLogEntries(ctx, []*logreader.LogEntry{entry})
	if err != nil {
		t.Fatalf("ProcessLogEntries failed: %v", err)
	}

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}

	quests, err := storageService.Quests().GetQuestHistory(nil, nil, 0)
	if err != nil {
		t.Fatalf("GetQuestHistory failed: %v", err)
	}

	found := false
	for _, q := range quests {
		if q.QuestID == "quest-session-1" {
			found = true
			if q.SessionID != "test-session-xyz" {
				t.Errorf("SessionID = %q, want %q", q.SessionID, "test-session-xyz")
			}
		}
	}

	if !found {
		t.Error("Quest was not stored in the database")
	}
}
