package storage

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
)

// setupTestDB creates an in-memory SQLite database with the quests table
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE quests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			quest_id TEXT NOT NULL,
			quest_type TEXT,
			goal INTEGER DEFAULT 0,
			starting_progress INTEGER DEFAULT 0,
			ending_progress INTEGER DEFAULT 0,
			completed INTEGER DEFAULT 0,
			can_swap INTEGER DEFAULT 1,
			rewards TEXT,
			assigned_at TEXT NOT NULL,
			completed_at TEXT,
			last_seen_at TEXT,
			rerolled INTEGER DEFAULT 0,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			session_id TEXT,
			completion_source TEXT
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create quests table: %v", err)
	}

	return db
}

func TestParseGoldFromRewards(t *testing.T) {
	tests := []struct {
		name     string
		rewards  string
		expected int
	}{
		{
			name:     "standard 500 gold",
			rewards:  "500",
			expected: 500,
		},
		{
			name:     "750 gold quest",
			rewards:  "750",
			expected: 750,
		},
		{
			name:     "empty string defaults to 500",
			rewards:  "",
			expected: 500,
		},
		{
			name:     "whitespace only defaults to 500",
			rewards:  "   ",
			expected: 500,
		},
		{
			name:     "invalid string defaults to 500",
			rewards:  "invalid",
			expected: 500,
		},
		{
			name:     "negative number defaults to 500",
			rewards:  "-100",
			expected: 500,
		},
		{
			name:     "zero defaults to 500",
			rewards:  "0",
			expected: 500,
		},
		{
			name:     "number with whitespace",
			rewards:  " 750 ",
			expected: 750,
		},
		{
			name:     "1000 gold",
			rewards:  "1000",
			expected: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGoldFromRewards(tt.rewards)
			if result != tt.expected {
				t.Errorf("parseGoldFromRewards(%q) = %d, want %d", tt.rewards, result, tt.expected)
			}
		})
	}
}

func TestCalculateTotalGoldEarned(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewQuestRepository(db)

	// Insert test quests
	now := time.Now().UTC()
	assignedAt := now.Add(-24 * time.Hour).Format("2006-01-02 15:04:05")
	completedAt := now.Format("2006-01-02 15:04:05")
	createdAt := now.Format("2006-01-02 15:04:05")

	// Quest 1: 500 gold, completed
	_, err := db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rewards, assigned_at, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "quest-1", "Daily", 5, 5, 1, "500", assignedAt, completedAt, createdAt)
	if err != nil {
		t.Fatalf("Failed to insert quest 1: %v", err)
	}

	// Quest 2: 750 gold, completed
	_, err = db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rewards, assigned_at, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "quest-2", "Daily", 10, 10, 1, "750", assignedAt, completedAt, createdAt)
	if err != nil {
		t.Fatalf("Failed to insert quest 2: %v", err)
	}

	// Quest 3: Not completed (should not be counted)
	_, err = db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rewards, assigned_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "quest-3", "Daily", 5, 2, 0, "500", assignedAt, createdAt)
	if err != nil {
		t.Fatalf("Failed to insert quest 3: %v", err)
	}

	// Quest 4: Empty rewards, completed (should default to 500)
	_, err = db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rewards, assigned_at, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "quest-4", "Daily", 3, 3, 1, "", assignedAt, completedAt, createdAt)
	if err != nil {
		t.Fatalf("Failed to insert quest 4: %v", err)
	}

	// Calculate total: 500 + 750 + 500 (default) = 1750
	total := repo.calculateTotalGoldEarned(nil, nil)
	expected := 1750

	if total != expected {
		t.Errorf("calculateTotalGoldEarned() = %d, want %d", total, expected)
	}
}

func TestCalculateTotalGoldEarnedWithDateFilter(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewQuestRepository(db)

	// Insert quests at different dates
	oldDate := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	recentDate := time.Date(2024, 11, 15, 12, 0, 0, 0, time.UTC)

	// Old quest: 500 gold
	_, err := db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rewards, assigned_at, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "old-quest", "Daily", 5, 5, 1, "500",
		oldDate.Add(-24*time.Hour).Format("2006-01-02 15:04:05"),
		oldDate.Format("2006-01-02 15:04:05"),
		oldDate.Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("Failed to insert old quest: %v", err)
	}

	// Recent quest: 750 gold
	_, err = db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rewards, assigned_at, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "recent-quest", "Daily", 5, 5, 1, "750",
		recentDate.Add(-24*time.Hour).Format("2006-01-02 15:04:05"),
		recentDate.Format("2006-01-02 15:04:05"),
		recentDate.Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("Failed to insert recent quest: %v", err)
	}

	// Test with date filter that only includes recent quest
	startDate := time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 11, 30, 0, 0, 0, 0, time.UTC)

	total := repo.calculateTotalGoldEarned(&startDate, &endDate)
	expected := 750 // Only the recent quest

	if total != expected {
		t.Errorf("calculateTotalGoldEarned() with date filter = %d, want %d", total, expected)
	}

	// Test without date filter (should include both)
	totalAll := repo.calculateTotalGoldEarned(nil, nil)
	expectedAll := 1250 // 500 + 750

	if totalAll != expectedAll {
		t.Errorf("calculateTotalGoldEarned() without filter = %d, want %d", totalAll, expectedAll)
	}
}

func TestGetQuestStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewQuestRepository(db)

	// Insert test quests with small time difference to avoid float precision issues
	now := time.Now().UTC()
	// Use a very small time difference (1 second) to get an integer result from AVG
	assignedAt := now.Add(-1 * time.Second).Format("2006-01-02 15:04:05")
	completedAt := now.Format("2006-01-02 15:04:05")
	createdAt := now.Format("2006-01-02 15:04:05")

	// Completed quest with 750 gold
	_, err := db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rewards, assigned_at, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "quest-1", "Daily", 5, 5, 1, "750", assignedAt, completedAt, createdAt)
	if err != nil {
		t.Fatalf("Failed to insert quest: %v", err)
	}

	// Active quest (no completed_at)
	_, err = db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rewards, assigned_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "quest-2", "Daily", 10, 5, 0, "500", assignedAt, createdAt)
	if err != nil {
		t.Fatalf("Failed to insert quest: %v", err)
	}

	stats, err := repo.GetQuestStats(nil, nil)
	if err != nil {
		t.Fatalf("GetQuestStats failed: %v", err)
	}

	if stats.TotalQuests != 2 {
		t.Errorf("TotalQuests = %d, want 2", stats.TotalQuests)
	}

	if stats.CompletedQuests != 1 {
		t.Errorf("CompletedQuests = %d, want 1", stats.CompletedQuests)
	}

	if stats.ActiveQuests != 1 {
		t.Errorf("ActiveQuests = %d, want 1", stats.ActiveQuests)
	}

	if stats.TotalGoldEarned != 750 {
		t.Errorf("TotalGoldEarned = %d, want 750", stats.TotalGoldEarned)
	}
}

func TestQuestRepositorySave(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewQuestRepository(db)

	quest := &models.Quest{
		QuestID:          "test-quest-123",
		QuestType:        "Daily_Win",
		Goal:             5,
		StartingProgress: 0,
		EndingProgress:   2,
		Completed:        false,
		CanSwap:          true,
		Rewards:          "750",
		AssignedAt:       time.Now().UTC(),
	}

	err := repo.Save(quest)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if quest.ID == 0 {
		t.Error("Quest ID should be set after save")
	}

	// Retrieve and verify
	retrieved, err := repo.GetQuestByID(quest.ID)
	if err != nil {
		t.Fatalf("GetQuestByID failed: %v", err)
	}

	if retrieved.QuestID != quest.QuestID {
		t.Errorf("QuestID = %s, want %s", retrieved.QuestID, quest.QuestID)
	}

	if retrieved.Rewards != "750" {
		t.Errorf("Rewards = %s, want 750", retrieved.Rewards)
	}
}

func TestDeduplicateQuestsByQuestID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewQuestRepository(db)

	// Insert multiple entries for the same quest_id with different created_at
	now := time.Now().UTC()

	// Older entry
	_, err := db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rewards, assigned_at, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "dup-quest", "Daily", 5, 3, 0, "500",
		now.Add(-48*time.Hour).Format("2006-01-02 15:04:05"),
		nil,
		now.Add(-24*time.Hour).Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("Failed to insert older quest: %v", err)
	}

	// Newer entry (completed with 750 gold)
	_, err = db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rewards, assigned_at, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "dup-quest", "Daily", 5, 5, 1, "750",
		now.Add(-48*time.Hour).Format("2006-01-02 15:04:05"),
		now.Format("2006-01-02 15:04:05"),
		now.Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("Failed to insert newer quest: %v", err)
	}

	// Calculate gold - should only count the newer entry (750)
	total := repo.calculateTotalGoldEarned(nil, nil)
	if total != 750 {
		t.Errorf("calculateTotalGoldEarned with duplicates = %d, want 750", total)
	}
}

func TestQuestReassignment(t *testing.T) {
	// Test that when MTGA reuses a quest_id for a new quest after the old one was completed,
	// we create a new record instead of updating the old completed one.
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewQuestRepository(db)
	now := time.Now().UTC()

	// First, save a quest and complete it
	completedAt := now.Add(-24 * time.Hour)
	quest1 := &models.Quest{
		QuestID:        "reused-quest-id",
		QuestType:      "First Quest",
		Goal:           5,
		EndingProgress: 5,
		Completed:      true,
		AssignedAt:     now.Add(-48 * time.Hour),
		CompletedAt:    &completedAt,
		LastSeenAt:     &now,
	}

	err := repo.Save(quest1)
	if err != nil {
		t.Fatalf("Failed to save first quest: %v", err)
	}
	firstQuestID := quest1.ID

	// Now MTGA reuses the same quest_id for a NEW quest
	newLastSeen := now
	quest2 := &models.Quest{
		QuestID:        "reused-quest-id", // Same ID!
		QuestType:      "Second Quest (Reused ID)",
		Goal:           10,
		EndingProgress: 0,
		Completed:      false, // Not completed - this is a NEW quest
		AssignedAt:     now,
		LastSeenAt:     &newLastSeen,
	}

	err = repo.Save(quest2)
	if err != nil {
		t.Fatalf("Failed to save second quest: %v", err)
	}

	// The second quest should get a NEW ID (not update the first)
	if quest2.ID == firstQuestID {
		t.Errorf("Quest reassignment should create new record, but got same ID: %d", quest2.ID)
	}

	// Verify we now have 2 records with the same quest_id
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM quests WHERE quest_id = ?", "reused-quest-id").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count quests: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 quests with same quest_id, got %d", count)
	}

	// Verify the first quest is still completed
	var firstCompleted bool
	err = db.QueryRow("SELECT completed FROM quests WHERE id = ?", firstQuestID).Scan(&firstCompleted)
	if err != nil {
		t.Fatalf("Failed to query first quest: %v", err)
	}
	if !firstCompleted {
		t.Error("First quest should still be completed")
	}

	// Verify the second quest is not completed
	var secondCompleted bool
	err = db.QueryRow("SELECT completed FROM quests WHERE id = ?", quest2.ID).Scan(&secondCompleted)
	if err != nil {
		t.Fatalf("Failed to query second quest: %v", err)
	}
	if secondCompleted {
		t.Error("Second quest should not be completed")
	}
}

func TestQuestReassignmentAfterReroll(t *testing.T) {
	// Test that when MTGA reuses a quest_id for a new quest after the old one was rerolled,
	// we create a new record instead of updating the old rerolled one.
	// This is the bug fix for issue where rerolled quests were not properly tracked.
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewQuestRepository(db)
	now := time.Now().UTC()

	// First, save a quest and mark it as rerolled
	lastSeen := now.Add(-24 * time.Hour)
	quest1 := &models.Quest{
		QuestID:        "reused-quest-id",
		QuestType:      "Quests/Quest_Nissas_Journey",
		Goal:           25,
		EndingProgress: 0,
		Completed:      false,
		Rerolled:       true, // This quest was rerolled
		AssignedAt:     now.Add(-48 * time.Hour),
		LastSeenAt:     &lastSeen,
	}

	err := repo.Save(quest1)
	if err != nil {
		t.Fatalf("Failed to save first (rerolled) quest: %v", err)
	}
	firstQuestID := quest1.ID

	// Now MTGA reuses the same quest_id for a NEW quest (after reroll)
	newLastSeen := now
	quest2 := &models.Quest{
		QuestID:        "reused-quest-id", // Same ID!
		QuestType:      "Quests/Quest_Nissas_Journey",
		Goal:           25,
		EndingProgress: 0,
		Completed:      false, // Not completed - this is a NEW quest
		Rerolled:       false, // This one is NOT rerolled
		AssignedAt:     now,
		LastSeenAt:     &newLastSeen,
	}

	err = repo.Save(quest2)
	if err != nil {
		t.Fatalf("Failed to save second quest: %v", err)
	}

	// The second quest should get a NEW ID (not update the first)
	if quest2.ID == firstQuestID {
		t.Errorf("Quest reassignment after reroll should create new record, but got same ID: %d", quest2.ID)
	}

	// Verify we now have 2 records with the same quest_id
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM quests WHERE quest_id = ?", "reused-quest-id").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count quests: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 quests with same quest_id, got %d", count)
	}

	// Verify the first quest is still marked as rerolled
	var firstRerolled bool
	err = db.QueryRow("SELECT rerolled FROM quests WHERE id = ?", firstQuestID).Scan(&firstRerolled)
	if err != nil {
		t.Fatalf("Failed to query first quest: %v", err)
	}
	if !firstRerolled {
		t.Error("First quest should still be marked as rerolled")
	}

	// Verify the second quest is NOT rerolled
	var secondRerolled bool
	err = db.QueryRow("SELECT rerolled FROM quests WHERE id = ?", quest2.ID).Scan(&secondRerolled)
	if err != nil {
		t.Fatalf("Failed to query second quest: %v", err)
	}
	if secondRerolled {
		t.Error("Second quest should not be rerolled")
	}
}

func TestSaveQuestAfterRerollCreatesNewRecord(t *testing.T) {
	// Test that when a quest was marked as rerolled and MTGA sends a new quest
	// with the same quest_id, we create a NEW record (not update the old one).
	// This is the expected behavior when MTGA reuses quest_ids.
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewQuestRepository(db)
	now := time.Now().UTC()

	// First, save a quest
	lastSeen := now.Add(-24 * time.Hour)
	quest := &models.Quest{
		QuestID:        "active-quest-id",
		QuestType:      "Quests/Quest_Fatal_Push",
		Goal:           25,
		EndingProgress: 10,
		Completed:      false,
		Rerolled:       false,
		AssignedAt:     now.Add(-48 * time.Hour),
		LastSeenAt:     &lastSeen,
	}

	err := repo.Save(quest)
	if err != nil {
		t.Fatalf("Failed to save quest: %v", err)
	}
	firstQuestID := quest.ID

	// Mark it as rerolled (simulating the quest disappearing from MTGA response)
	_, err = db.Exec("UPDATE quests SET rerolled = 1 WHERE id = ?", firstQuestID)
	if err != nil {
		t.Fatalf("Failed to set rerolled flag: %v", err)
	}

	// Verify it's now rerolled
	var rerolled bool
	err = db.QueryRow("SELECT rerolled FROM quests WHERE id = ?", firstQuestID).Scan(&rerolled)
	if err != nil {
		t.Fatalf("Failed to query quest: %v", err)
	}
	if !rerolled {
		t.Error("Quest should be rerolled before second save")
	}

	// Now save a NEW quest with the same quest_id (simulating MTGA reusing the ID)
	newLastSeen := now
	quest.EndingProgress = 0 // New quest has 0 progress
	quest.LastSeenAt = &newLastSeen
	quest.AssignedAt = now // New assigned_at

	err = repo.Save(quest)
	if err != nil {
		t.Fatalf("Failed to save second quest: %v", err)
	}

	// The second save should create a NEW record (different ID)
	if quest.ID == firstQuestID {
		t.Errorf("Expected new record to be created, but got same ID: %d", quest.ID)
	}

	// Verify we now have 2 records
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM quests WHERE quest_id = ?", "active-quest-id").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count quests: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 quests, got %d", count)
	}

	// The first record should still be rerolled
	err = db.QueryRow("SELECT rerolled FROM quests WHERE id = ?", firstQuestID).Scan(&rerolled)
	if err != nil {
		t.Fatalf("Failed to query first quest: %v", err)
	}
	if !rerolled {
		t.Error("First quest should still be rerolled")
	}

	// The new record should NOT be rerolled
	var newRerolled bool
	err = db.QueryRow("SELECT rerolled FROM quests WHERE id = ?", quest.ID).Scan(&newRerolled)
	if err != nil {
		t.Fatalf("Failed to query new quest: %v", err)
	}
	if newRerolled {
		t.Error("New quest should not be rerolled")
	}
}

func TestSaveQuest_WithSessionID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewQuestRepository(db)
	now := time.Now().UTC()

	quest := &models.Quest{
		QuestID:    "session-quest-1",
		QuestType:  "Daily_Win",
		Goal:       5,
		AssignedAt: now,
		SessionID:  "session-abc-123",
	}

	err := repo.Save(quest)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	retrieved, err := repo.GetQuestByID(quest.ID)
	if err != nil {
		t.Fatalf("GetQuestByID failed: %v", err)
	}

	if retrieved.SessionID != "session-abc-123" {
		t.Errorf("SessionID = %q, want %q", retrieved.SessionID, "session-abc-123")
	}
}

func TestSaveQuest_WithCompletionSource(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewQuestRepository(db)
	now := time.Now().UTC()
	completedAt := now

	quest := &models.Quest{
		QuestID:          "completion-source-quest-1",
		QuestType:        "Daily_Win",
		Goal:             5,
		EndingProgress:   5,
		Completed:        true,
		AssignedAt:       now.Add(-1 * time.Hour),
		CompletedAt:      &completedAt,
		CompletionSource: "progress",
	}

	err := repo.Save(quest)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	retrieved, err := repo.GetQuestByID(quest.ID)
	if err != nil {
		t.Fatalf("GetQuestByID failed: %v", err)
	}

	if retrieved.CompletionSource != "progress" {
		t.Errorf("CompletionSource = %q, want %q", retrieved.CompletionSource, "progress")
	}
}

func TestGetActiveQuests_NoTimeFilter(t *testing.T) {
	// Insert a quest with last_seen_at from 48 hours ago.
	// With no 24-hour filter, GetActiveQuests should still return it.
	db := setupTestDB(t)
	defer db.Close()

	repo := NewQuestRepository(db)
	now := time.Now().UTC()

	oldLastSeen := now.Add(-48 * time.Hour)
	_, err := db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rerolled, rewards, assigned_at, last_seen_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "old-active-quest", "Daily_Win", 5, 2, 0, 0, "500",
		now.Add(-48*time.Hour).Format("2006-01-02 15:04:05"),
		oldLastSeen.Format("2006-01-02 15:04:05"),
		now.Add(-48*time.Hour).Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("Failed to insert quest: %v", err)
	}

	quests, err := repo.GetActiveQuests()
	if err != nil {
		t.Fatalf("GetActiveQuests failed: %v", err)
	}

	if len(quests) != 1 {
		t.Errorf("GetActiveQuests returned %d quests, want 1 (no time filter should apply)", len(quests))
	}

	if len(quests) > 0 && quests[0].QuestID != "old-active-quest" {
		t.Errorf("QuestID = %q, want %q", quests[0].QuestID, "old-active-quest")
	}
}

func TestGetActiveQuests_ExcludesCompleted(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewQuestRepository(db)
	now := time.Now().UTC()

	// Insert an incomplete quest
	_, err := db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rerolled, rewards, assigned_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "incomplete-quest", "Daily_Win", 5, 2, 0, 0, "500",
		now.Add(-1*time.Hour).Format("2006-01-02 15:04:05"),
		now.Add(-1*time.Hour).Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("Failed to insert incomplete quest: %v", err)
	}

	// Insert a completed quest
	completedAt := now.Format("2006-01-02 15:04:05")
	_, err = db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rerolled, rewards, assigned_at, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "completed-quest", "Daily_Win", 5, 5, 1, 0, "500",
		now.Add(-2*time.Hour).Format("2006-01-02 15:04:05"),
		completedAt,
		now.Add(-2*time.Hour).Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("Failed to insert completed quest: %v", err)
	}

	quests, err := repo.GetActiveQuests()
	if err != nil {
		t.Fatalf("GetActiveQuests failed: %v", err)
	}

	if len(quests) != 1 {
		t.Errorf("GetActiveQuests returned %d quests, want 1 (should exclude completed)", len(quests))
	}

	if len(quests) > 0 && quests[0].QuestID != "incomplete-quest" {
		t.Errorf("QuestID = %q, want %q", quests[0].QuestID, "incomplete-quest")
	}
}

func TestGetQuestHistory_AllInstances(t *testing.T) {
	// Insert TWO quests with the SAME quest_id but different created_at and assigned_at.
	// GetQuestHistory (with GROUP BY removed) should return BOTH.
	db := setupTestDB(t)
	defer db.Close()

	repo := NewQuestRepository(db)
	now := time.Now().UTC()

	// First instance
	_, err := db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rewards, assigned_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "reused-quest-id", "Daily_Win", 5, 5, 1, "500",
		now.Add(-72*time.Hour).Format("2006-01-02 15:04:05"),
		now.Add(-72*time.Hour).Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("Failed to insert first quest instance: %v", err)
	}

	// Second instance (same quest_id, different assigned_at)
	_, err = db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rewards, assigned_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "reused-quest-id", "Daily_Win", 5, 2, 0, "500",
		now.Add(-24*time.Hour).Format("2006-01-02 15:04:05"),
		now.Add(-24*time.Hour).Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("Failed to insert second quest instance: %v", err)
	}

	history, err := repo.GetQuestHistory(nil, nil, 0)
	if err != nil {
		t.Fatalf("GetQuestHistory failed: %v", err)
	}

	if len(history) != 2 {
		t.Errorf("GetQuestHistory returned %d quests, want 2 (no GROUP BY dedup)", len(history))
	}
}

func TestGetQuestHistory_FiltersByAssignedAt(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewQuestRepository(db)

	// Quest assigned in January 2024
	_, err := db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rewards, assigned_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "jan-quest", "Daily_Win", 5, 5, 1, "500",
		"2024-01-15 12:00:00",
		"2024-01-15 12:00:00")
	if err != nil {
		t.Fatalf("Failed to insert January quest: %v", err)
	}

	// Quest assigned in November 2024
	_, err = db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rewards, assigned_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "nov-quest", "Daily_Win", 5, 5, 1, "750",
		"2024-11-15 12:00:00",
		"2024-11-15 12:00:00")
	if err != nil {
		t.Fatalf("Failed to insert November quest: %v", err)
	}

	// Filter to only November
	startDate := time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 11, 30, 0, 0, 0, 0, time.UTC)

	history, err := repo.GetQuestHistory(&startDate, &endDate, 0)
	if err != nil {
		t.Fatalf("GetQuestHistory with date filter failed: %v", err)
	}

	if len(history) != 1 {
		t.Errorf("GetQuestHistory returned %d quests, want 1 (only November)", len(history))
	}

	if len(history) > 0 && history[0].QuestID != "nov-quest" {
		t.Errorf("QuestID = %q, want %q", history[0].QuestID, "nov-quest")
	}

	// No filter should return both
	all, err := repo.GetQuestHistory(nil, nil, 0)
	if err != nil {
		t.Fatalf("GetQuestHistory without filter failed: %v", err)
	}

	if len(all) != 2 {
		t.Errorf("GetQuestHistory (no filter) returned %d quests, want 2", len(all))
	}
}

func TestHasAnyQuestData(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewQuestRepository(db)

	// Empty DB should return false
	if repo.HasAnyQuestData() {
		t.Error("HasAnyQuestData should return false on empty DB")
	}

	now := time.Now().UTC()

	// Insert quest WITHOUT last_seen_at - should still return false
	_, err := db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rewards, assigned_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "no-last-seen-quest", "Daily_Win", 5, 2, 0, "500",
		now.Format("2006-01-02 15:04:05"),
		now.Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("Failed to insert quest without last_seen_at: %v", err)
	}

	if repo.HasAnyQuestData() {
		t.Error("HasAnyQuestData should return false when no quests have last_seen_at")
	}

	// Insert quest WITH last_seen_at - should now return true
	lastSeen := now.Format("2006-01-02 15:04:05")
	_, err = db.Exec(`
		INSERT INTO quests (quest_id, quest_type, goal, ending_progress, completed, rewards, assigned_at, last_seen_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "seen-quest", "Daily_Win", 5, 2, 0, "500",
		now.Format("2006-01-02 15:04:05"),
		lastSeen,
		now.Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("Failed to insert quest with last_seen_at: %v", err)
	}

	if !repo.HasAnyQuestData() {
		t.Error("HasAnyQuestData should return true after inserting quest with last_seen_at")
	}
}
