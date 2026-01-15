package models

import (
	"testing"
)

func TestLimitedRatingToScore(t *testing.T) {
	tests := []struct {
		rating   float64
		expected float64
	}{
		{5.0, 1.0},
		{4.0, 0.8},
		{3.0, 0.6},
		{2.5, 0.5},
		{2.0, 0.4},
		{1.0, 0.2},
		{0.0, 0.0},
		{-1.0, 0.0}, // Below minimum should clamp to 0
		{6.0, 1.0},  // Above maximum should clamp to 1
		{10.0, 1.0}, // Far above maximum should clamp to 1
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := LimitedRatingToScore(tt.rating)
			if result != tt.expected {
				t.Errorf("LimitedRatingToScore(%v) = %v, want %v", tt.rating, result, tt.expected)
			}
		})
	}
}

func TestLimitedRatingToGrade(t *testing.T) {
	tests := []struct {
		rating   float64
		expected string
	}{
		{5.0, "A+"},
		{4.75, "A+"},
		{4.74, "A"},
		{4.5, "A"},
		{4.25, "A"},
		{4.24, "A-"},
		{4.0, "A-"},
		{3.75, "A-"},
		{3.74, "B+"},
		{3.5, "B+"},
		{3.25, "B+"},
		{3.24, "B"},
		{3.0, "B"},
		{2.75, "B"},
		{2.74, "B-"},
		{2.5, "B-"},
		{2.25, "B-"},
		{2.24, "C+"},
		{2.0, "C+"},
		{1.75, "C+"},
		{1.74, "C"},
		{1.5, "C"},
		{1.25, "C"},
		{1.24, "C-"},
		{1.0, "C-"},
		{0.75, "C-"},
		{0.74, "D"},
		{0.5, "D"},
		{0.25, "D"},
		{0.24, "F"},
		{0.0, "F"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := LimitedRatingToGrade(tt.rating)
			if result != tt.expected {
				t.Errorf("LimitedRatingToGrade(%v) = %q, want %q", tt.rating, result, tt.expected)
			}
		})
	}
}

func TestConstructedRatingToScore(t *testing.T) {
	tests := []struct {
		rating   string
		expected float64
	}{
		{CFBConstructedStaple, 1.00},
		{CFBConstructedPlayable, 0.70},
		{CFBConstructedFringe, 0.40},
		{CFBConstructedUnplayable, 0.10},
		{"unknown", 0.5}, // Default for unknown ratings
		{"", 0.5},        // Empty string
	}

	for _, tt := range tests {
		t.Run(tt.rating, func(t *testing.T) {
			result := ConstructedRatingToScore(tt.rating)
			if result != tt.expected {
				t.Errorf("ConstructedRatingToScore(%q) = %v, want %v", tt.rating, result, tt.expected)
			}
		})
	}
}

func TestCFBConstructedRatingConstants(t *testing.T) {
	// Verify constructed rating constants
	constructedRatings := []string{
		CFBConstructedStaple,
		CFBConstructedPlayable,
		CFBConstructedFringe,
		CFBConstructedUnplayable,
	}

	expectedConstructed := []string{
		"Staple", "Playable", "Fringe", "Unplayable",
	}

	for i, rating := range constructedRatings {
		if rating != expectedConstructed[i] {
			t.Errorf("Constructed rating constant at index %d = %q, want %q", i, rating, expectedConstructed[i])
		}
	}
}

func TestCFBRating_Struct(t *testing.T) {
	arenaID := 12345
	rating := CFBRating{
		ID:                1,
		CardName:          "Test Card",
		SetCode:           "TST",
		ArenaID:           &arenaID,
		LimitedRating:     4.5, // Numerical rating (0-5 scale)
		LimitedScore:      0.9, // Normalized score (0-1)
		ConstructedRating: CFBConstructedPlayable,
		ConstructedScore:  0.70,
		ArchetypeFit:      "Aggro",
		Commentary:        "Good card for aggro decks",
		SourceURL:         "https://example.com/review",
		Author:            "Test Author",
	}

	if rating.CardName != "Test Card" {
		t.Errorf("CardName = %q, want %q", rating.CardName, "Test Card")
	}
	if rating.LimitedRating != 4.5 {
		t.Errorf("LimitedRating = %v, want %v", rating.LimitedRating, 4.5)
	}
	if rating.LimitedScore != 0.9 {
		t.Errorf("LimitedScore = %v, want %v", rating.LimitedScore, 0.9)
	}
	if *rating.ArenaID != 12345 {
		t.Errorf("ArenaID = %v, want %v", *rating.ArenaID, 12345)
	}

	// Test that the rating converts to the expected grade
	expectedGrade := "A"
	if LimitedRatingToGrade(rating.LimitedRating) != expectedGrade {
		t.Errorf("LimitedRatingToGrade(%v) = %q, want %q",
			rating.LimitedRating, LimitedRatingToGrade(rating.LimitedRating), expectedGrade)
	}
}

func TestCFBRatingImport_Struct(t *testing.T) {
	importData := CFBRatingImport{
		CardName:          "Test Card",
		SetCode:           "TST",
		LimitedRating:     3.5, // B+ rating
		ConstructedRating: CFBConstructedPlayable,
		ArchetypeFit:      "Aggro",
		Commentary:        "Good aggro card",
		SourceURL:         "https://example.com",
		Author:            "Test Author",
	}

	if importData.CardName != "Test Card" {
		t.Errorf("CardName = %q, want %q", importData.CardName, "Test Card")
	}
	if importData.LimitedRating != 3.5 {
		t.Errorf("LimitedRating = %v, want %v", importData.LimitedRating, 3.5)
	}

	// Test that the imported rating converts correctly
	expectedScore := 0.7 // 3.5 / 5.0 = 0.7
	if LimitedRatingToScore(importData.LimitedRating) != expectedScore {
		t.Errorf("LimitedRatingToScore(%v) = %v, want %v",
			importData.LimitedRating, LimitedRatingToScore(importData.LimitedRating), expectedScore)
	}
}
