package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
)

func setupNotesTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db := repoTestDB(t)

	// Insert prerequisite rows required by FK constraints.
	_, err := db.Exec(`INSERT INTO accounts (name, is_default, created_at, updated_at) VALUES ('Test Account', true, $1, $1) ON CONFLICT DO NOTHING`, time.Now().UTC())
	if err != nil {
		t.Fatalf("Failed to insert test account: %v", err)
	}
	var accountID int64
	if err := db.QueryRow(`SELECT id FROM accounts WHERE name = 'Test Account' LIMIT 1`).Scan(&accountID); err != nil {
		t.Fatalf("Failed to get test account ID: %v", err)
	}
	_, err = db.Exec(`INSERT INTO decks (id, account_id, name, format, created_at, modified_at) VALUES ('deck-1', $1, 'Test Deck', 'Standard', $2, $2) ON CONFLICT DO NOTHING`, accountID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Failed to insert test deck: %v", err)
	}
	_, err = db.Exec(`INSERT INTO matches (id, account_id, event_id, event_name, timestamp, player_wins, opponent_wins, player_team_id) VALUES ('match-1', $1, 'event-1', 'Test Event', $2, 0, 0, 1) ON CONFLICT DO NOTHING`, accountID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Failed to insert test match: %v", err)
	}

	return db
}

func TestNotesRepository_CreateDeckNote(t *testing.T) {
	db := setupNotesTestDB(t)
	defer db.Close()

	repo := NewNotesRepository(db)
	ctx := context.Background()

	note := &models.DeckNote{
		DeckID:   "deck-1",
		Content:  "Test note content",
		Category: models.NoteCategoryGeneral,
	}

	err := repo.CreateDeckNote(ctx, note)
	if err != nil {
		t.Fatalf("Failed to create deck note: %v", err)
	}

	if note.ID == 0 {
		t.Error("Expected note ID to be set after creation")
	}
}

func TestNotesRepository_GetDeckNotes(t *testing.T) {
	db := setupNotesTestDB(t)
	defer db.Close()

	repo := NewNotesRepository(db)
	ctx := context.Background()

	// Create multiple notes
	note1 := &models.DeckNote{DeckID: "deck-1", Content: "Note 1", Category: models.NoteCategoryGeneral}
	note2 := &models.DeckNote{DeckID: "deck-1", Content: "Note 2", Category: models.NoteCategoryMatchup}
	note3 := &models.DeckNote{DeckID: "deck-1", Content: "Note 3", Category: models.NoteCategoryGeneral}

	_ = repo.CreateDeckNote(ctx, note1)
	_ = repo.CreateDeckNote(ctx, note2)
	_ = repo.CreateDeckNote(ctx, note3)

	// Get all notes for deck
	notes, err := repo.GetDeckNotes(ctx, "deck-1")
	if err != nil {
		t.Fatalf("Failed to get deck notes: %v", err)
	}

	if len(notes) != 3 {
		t.Errorf("Expected 3 notes, got %d", len(notes))
	}
}

func TestNotesRepository_GetDeckNotesByCategory(t *testing.T) {
	db := setupNotesTestDB(t)
	defer db.Close()

	repo := NewNotesRepository(db)
	ctx := context.Background()

	// Create notes with different categories
	note1 := &models.DeckNote{DeckID: "deck-1", Content: "Note 1", Category: models.NoteCategoryGeneral}
	note2 := &models.DeckNote{DeckID: "deck-1", Content: "Note 2", Category: models.NoteCategoryMatchup}
	note3 := &models.DeckNote{DeckID: "deck-1", Content: "Note 3", Category: models.NoteCategoryGeneral}

	_ = repo.CreateDeckNote(ctx, note1)
	_ = repo.CreateDeckNote(ctx, note2)
	_ = repo.CreateDeckNote(ctx, note3)

	// Get only general notes
	notes, err := repo.GetDeckNotesByCategory(ctx, "deck-1", models.NoteCategoryGeneral)
	if err != nil {
		t.Fatalf("Failed to get notes by category: %v", err)
	}

	if len(notes) != 2 {
		t.Errorf("Expected 2 general notes, got %d", len(notes))
	}

	for _, note := range notes {
		if note.Category != models.NoteCategoryGeneral {
			t.Errorf("Expected category 'general', got '%s'", note.Category)
		}
	}
}

func TestNotesRepository_GetDeckNoteByID(t *testing.T) {
	db := setupNotesTestDB(t)
	defer db.Close()

	repo := NewNotesRepository(db)
	ctx := context.Background()

	// Create a note
	note := &models.DeckNote{DeckID: "deck-1", Content: "Test content", Category: models.NoteCategoryGeneral}
	_ = repo.CreateDeckNote(ctx, note)

	// Get by ID
	retrieved, err := repo.GetDeckNoteByID(ctx, note.ID)
	if err != nil {
		t.Fatalf("Failed to get note by ID: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected note, got nil")
	}

	if retrieved.Content != "Test content" {
		t.Errorf("Expected content 'Test content', got '%s'", retrieved.Content)
	}
}

func TestNotesRepository_UpdateDeckNote(t *testing.T) {
	db := setupNotesTestDB(t)
	defer db.Close()

	repo := NewNotesRepository(db)
	ctx := context.Background()

	// Create a note
	note := &models.DeckNote{DeckID: "deck-1", Content: "Original content", Category: models.NoteCategoryGeneral}
	_ = repo.CreateDeckNote(ctx, note)

	// Update the note
	note.Content = "Updated content"
	note.Category = models.NoteCategoryMatchup
	err := repo.UpdateDeckNote(ctx, note)
	if err != nil {
		t.Fatalf("Failed to update note: %v", err)
	}

	// Verify update
	retrieved, _ := repo.GetDeckNoteByID(ctx, note.ID)
	if retrieved.Content != "Updated content" {
		t.Errorf("Expected content 'Updated content', got '%s'", retrieved.Content)
	}
	if retrieved.Category != models.NoteCategoryMatchup {
		t.Errorf("Expected category 'matchup', got '%s'", retrieved.Category)
	}
}

func TestNotesRepository_DeleteDeckNote(t *testing.T) {
	db := setupNotesTestDB(t)
	defer db.Close()

	repo := NewNotesRepository(db)
	ctx := context.Background()

	// Create a note
	note := &models.DeckNote{DeckID: "deck-1", Content: "To be deleted", Category: models.NoteCategoryGeneral}
	_ = repo.CreateDeckNote(ctx, note)

	// Delete the note
	err := repo.DeleteDeckNote(ctx, note.ID)
	if err != nil {
		t.Fatalf("Failed to delete note: %v", err)
	}

	// Verify deletion
	retrieved, err := repo.GetDeckNoteByID(ctx, note.ID)
	if err == nil && retrieved != nil {
		t.Error("Expected note to be deleted")
	}
}

func TestNotesRepository_GetMatchNotes(t *testing.T) {
	db := setupNotesTestDB(t)
	defer db.Close()

	repo := NewNotesRepository(db)
	ctx := context.Background()

	// Get notes for match (should return empty notes initially)
	notes, err := repo.GetMatchNotes(ctx, "match-1")
	if err != nil {
		t.Fatalf("Failed to get match notes: %v", err)
	}

	if notes == nil {
		t.Fatal("Expected notes object, got nil")
	}

	if notes.Notes != "" {
		t.Errorf("Expected empty notes, got '%s'", notes.Notes)
	}
}

func TestNotesRepository_UpdateMatchNotes(t *testing.T) {
	db := setupNotesTestDB(t)
	defer db.Close()

	repo := NewNotesRepository(db)
	ctx := context.Background()

	// Update match notes
	err := repo.UpdateMatchNotes(ctx, "match-1", "Great game!", 5)
	if err != nil {
		t.Fatalf("Failed to update match notes: %v", err)
	}

	// Verify update
	notes, err := repo.GetMatchNotes(ctx, "match-1")
	if err != nil {
		t.Fatalf("Failed to get match notes: %v", err)
	}

	if notes.Notes != "Great game!" {
		t.Errorf("Expected notes 'Great game!', got '%s'", notes.Notes)
	}
	if notes.Rating != 5 {
		t.Errorf("Expected rating 5, got %d", notes.Rating)
	}
}

func TestNotesRepository_UpdateMatchNotesMultipleTimes(t *testing.T) {
	db := setupNotesTestDB(t)
	defer db.Close()

	repo := NewNotesRepository(db)
	ctx := context.Background()

	// Update once
	err := repo.UpdateMatchNotes(ctx, "match-1", "First notes", 3)
	if err != nil {
		t.Fatalf("Failed to update match notes first time: %v", err)
	}

	// Update again
	err = repo.UpdateMatchNotes(ctx, "match-1", "Updated notes", 4)
	if err != nil {
		t.Fatalf("Failed to update match notes second time: %v", err)
	}

	// Verify latest update
	notes, err := repo.GetMatchNotes(ctx, "match-1")
	if err != nil {
		t.Fatalf("Failed to get match notes: %v", err)
	}

	if notes.Notes != "Updated notes" {
		t.Errorf("Expected notes 'Updated notes', got '%s'", notes.Notes)
	}
	if notes.Rating != 4 {
		t.Errorf("Expected rating 4, got %d", notes.Rating)
	}
}

func TestNotesRepository_DeleteDeckNotesByDeck(t *testing.T) {
	db := setupNotesTestDB(t)
	defer db.Close()

	repo := NewNotesRepository(db)
	ctx := context.Background()

	// Create multiple notes
	note1 := &models.DeckNote{DeckID: "deck-1", Content: "Note 1", Category: models.NoteCategoryGeneral}
	note2 := &models.DeckNote{DeckID: "deck-1", Content: "Note 2", Category: models.NoteCategoryMatchup}

	_ = repo.CreateDeckNote(ctx, note1)
	_ = repo.CreateDeckNote(ctx, note2)

	// Delete all notes for deck
	err := repo.DeleteDeckNotesByDeck(ctx, "deck-1")
	if err != nil {
		t.Fatalf("Failed to delete deck notes: %v", err)
	}

	// Verify deletion
	notes, err := repo.GetDeckNotes(ctx, "deck-1")
	if err != nil {
		t.Fatalf("Failed to get deck notes: %v", err)
	}

	if len(notes) != 0 {
		t.Errorf("Expected 0 notes after deletion, got %d", len(notes))
	}
}
