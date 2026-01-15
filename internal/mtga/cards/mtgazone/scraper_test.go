package mtgazone

import (
	"testing"
)

func TestNormalizeCardName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Lightning Bolt", "Lightning Bolt"},
		{"  Lightning Bolt  ", "Lightning Bolt"},
		{"Lightning  Bolt", "Lightning Bolt"},
		{"Delver of Secrets // Insectile Aberration", "Delver of Secrets"},
		{"", ""},
		{"A", ""}, // Too short
		{"AB", "AB"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeCardName(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeCardName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsCommonWord(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"the", true},
		{"and", true},
		{"rating", true},
		{"draft", true},
		{"Lightning", false},
		{"Bolt", false},
		{"Counterspell", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isCommonWord(tt.input)
			if result != tt.expected {
				t.Errorf("isCommonWord(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractRatingsFromText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		color    string
		minCount int
	}{
		{
			name:     "colon format",
			text:     "Lightning Bolt: 4.5/5 This is a great card. Counterspell: 3.5 Another good one.",
			color:    "red",
			minCount: 2,
		},
		{
			name:     "dash format",
			text:     "Lightning Bolt – 4.5 Great removal. Counterspell – 3.0 Solid counter.",
			color:    "blue",
			minCount: 2,
		},
		{
			name:     "rating label format",
			text:     "Aang Rating: 5.0 Best card in the set.",
			color:    "white",
			minCount: 1,
		},
		{
			name:     "no ratings",
			text:     "This is just some text without any ratings.",
			color:    "green",
			minCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratings := extractRatingsFromText(tt.text, tt.color)
			if len(ratings) < tt.minCount {
				t.Errorf("extractRatingsFromText() returned %d ratings, want at least %d", len(ratings), tt.minCount)
			}
			for _, r := range ratings {
				if r.Rating < 0 || r.Rating > 5 {
					t.Errorf("rating %v is out of range [0, 5]", r.Rating)
				}
				if r.Color != tt.color {
					t.Errorf("color = %q, want %q", r.Color, tt.color)
				}
			}
		})
	}
}

func TestDeduplicateRatings(t *testing.T) {
	ratings := []CardRating{
		{CardName: "Lightning Bolt", Rating: 4.5, Color: "red"},
		{CardName: "lightning bolt", Rating: 4.0, Color: "red"}, // Duplicate (different case)
		{CardName: "Counterspell", Rating: 3.5, Color: "blue"},
	}

	result := deduplicateRatings(ratings)

	if len(result) != 2 {
		t.Errorf("deduplicateRatings() returned %d ratings, want 2", len(result))
	}
}

func TestBuildSetMappings(t *testing.T) {
	mappings := buildSetMappings()

	// Check some known mappings
	expectedMappings := map[string]string{
		"BLB": "bloomburrow-blb",
		"FDN": "foundations-fdn",
		"TLA": "avatar-the-last-airbender-tla",
		"MOM": "march-of-the-machine-mom",
	}

	for code, expectedSlug := range expectedMappings {
		slug, ok := mappings[code]
		if !ok {
			t.Errorf("mapping for %s not found", code)
			continue
		}
		if slug != expectedSlug {
			t.Errorf("mapping for %s = %q, want %q", code, slug, expectedSlug)
		}
	}
}

func TestNewScraper(t *testing.T) {
	// Test with default options
	scraper := NewScraper(DefaultScraperOptions())
	if scraper == nil {
		t.Fatal("NewScraper() returned nil")
	}
	if scraper.httpClient == nil {
		t.Error("scraper.httpClient is nil")
	}
	if scraper.limiter == nil {
		t.Error("scraper.limiter is nil")
	}
	if len(scraper.setMappings) == 0 {
		t.Error("scraper.setMappings is empty")
	}
}

func TestScraper_GetSetMapping(t *testing.T) {
	scraper := NewScraper(DefaultScraperOptions())

	// Test existing mapping
	slug, ok := scraper.GetSetMapping("BLB")
	if !ok {
		t.Error("GetSetMapping(BLB) returned false")
	}
	if slug != "bloomburrow-blb" {
		t.Errorf("GetSetMapping(BLB) = %q, want %q", slug, "bloomburrow-blb")
	}

	// Test non-existing mapping
	_, ok = scraper.GetSetMapping("UNKNOWN")
	if ok {
		t.Error("GetSetMapping(UNKNOWN) should return false")
	}
}

func TestScraper_AddSetMapping(t *testing.T) {
	scraper := NewScraper(DefaultScraperOptions())

	// Add a new mapping
	scraper.AddSetMapping("NEW", "new-set-slug")

	slug, ok := scraper.GetSetMapping("NEW")
	if !ok {
		t.Error("GetSetMapping(NEW) returned false after AddSetMapping")
	}
	if slug != "new-set-slug" {
		t.Errorf("GetSetMapping(NEW) = %q, want %q", slug, "new-set-slug")
	}
}

func TestExtractFirstRating(t *testing.T) {
	tests := []struct {
		text     string
		expected float64
	}{
		{"Rating: 4.5/5", 4.5},
		{"3.5", 3.5},
		{"Score: 2.0 out of 5", 2.0},
		{"no rating here", -1},
		{"6.0", -1}, // Out of range
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := extractFirstRating(tt.text)
			if result != tt.expected {
				t.Errorf("extractFirstRating(%q) = %v, want %v", tt.text, result, tt.expected)
			}
		})
	}
}
