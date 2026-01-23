package mtgjson

import (
	"testing"
)

func TestConstructImageURL(t *testing.T) {
	tests := []struct {
		name       string
		scryfallID string
		size       ScryfallImageSize
		expected   string
	}{
		{
			name:       "normal size",
			scryfallID: "fa940e68-010e-4b68-be8a-555d7068f7b4",
			size:       ScryfallImageNormal,
			expected:   "https://cards.scryfall.io/normal/front/f/a/fa940e68-010e-4b68-be8a-555d7068f7b4.jpg",
		},
		{
			name:       "small size",
			scryfallID: "fa940e68-010e-4b68-be8a-555d7068f7b4",
			size:       ScryfallImageSmall,
			expected:   "https://cards.scryfall.io/small/front/f/a/fa940e68-010e-4b68-be8a-555d7068f7b4.jpg",
		},
		{
			name:       "large size",
			scryfallID: "fa940e68-010e-4b68-be8a-555d7068f7b4",
			size:       ScryfallImageLarge,
			expected:   "https://cards.scryfall.io/large/front/f/a/fa940e68-010e-4b68-be8a-555d7068f7b4.jpg",
		},
		{
			name:       "PNG format",
			scryfallID: "fa940e68-010e-4b68-be8a-555d7068f7b4",
			size:       ScryfallImagePNG,
			expected:   "https://cards.scryfall.io/png/front/f/a/fa940e68-010e-4b68-be8a-555d7068f7b4.png",
		},
		{
			name:       "art crop",
			scryfallID: "fa940e68-010e-4b68-be8a-555d7068f7b4",
			size:       ScryfallImageArtCrop,
			expected:   "https://cards.scryfall.io/art_crop/front/f/a/fa940e68-010e-4b68-be8a-555d7068f7b4.jpg",
		},
		{
			name:       "different UUID starting chars",
			scryfallID: "12345678-abcd-ef01-2345-6789abcdef01",
			size:       ScryfallImageNormal,
			expected:   "https://cards.scryfall.io/normal/front/1/2/12345678-abcd-ef01-2345-6789abcdef01.jpg",
		},
		{
			name:       "empty scryfall ID",
			scryfallID: "",
			size:       ScryfallImageNormal,
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConstructImageURL(tt.scryfallID, tt.size)
			if got != tt.expected {
				t.Errorf("ConstructImageURL(%q, %q) = %q, want %q",
					tt.scryfallID, tt.size, got, tt.expected)
			}
		})
	}
}

func TestConstructAllImageURLs(t *testing.T) {
	scryfallID := "fa940e68-010e-4b68-be8a-555d7068f7b4"
	urls := ConstructAllImageURLs(scryfallID)

	expectedNormal := "https://cards.scryfall.io/normal/front/f/a/fa940e68-010e-4b68-be8a-555d7068f7b4.jpg"
	if urls.Normal != expectedNormal {
		t.Errorf("Normal = %q, want %q", urls.Normal, expectedNormal)
	}

	expectedSmall := "https://cards.scryfall.io/small/front/f/a/fa940e68-010e-4b68-be8a-555d7068f7b4.jpg"
	if urls.Small != expectedSmall {
		t.Errorf("Small = %q, want %q", urls.Small, expectedSmall)
	}

	expectedLarge := "https://cards.scryfall.io/large/front/f/a/fa940e68-010e-4b68-be8a-555d7068f7b4.jpg"
	if urls.Large != expectedLarge {
		t.Errorf("Large = %q, want %q", urls.Large, expectedLarge)
	}

	expectedArtCrop := "https://cards.scryfall.io/art_crop/front/f/a/fa940e68-010e-4b68-be8a-555d7068f7b4.jpg"
	if urls.ArtCrop != expectedArtCrop {
		t.Errorf("ArtCrop = %q, want %q", urls.ArtCrop, expectedArtCrop)
	}
}

func TestConstructBackFaceImageURL(t *testing.T) {
	tests := []struct {
		name       string
		scryfallID string
		size       ScryfallImageSize
		expected   string
	}{
		{
			name:       "normal size back face",
			scryfallID: "fa940e68-010e-4b68-be8a-555d7068f7b4",
			size:       ScryfallImageNormal,
			expected:   "https://cards.scryfall.io/normal/back/f/a/fa940e68-010e-4b68-be8a-555d7068f7b4.jpg",
		},
		{
			name:       "large size back face",
			scryfallID: "fa940e68-010e-4b68-be8a-555d7068f7b4",
			size:       ScryfallImageLarge,
			expected:   "https://cards.scryfall.io/large/back/f/a/fa940e68-010e-4b68-be8a-555d7068f7b4.jpg",
		},
		{
			name:       "PNG back face",
			scryfallID: "fa940e68-010e-4b68-be8a-555d7068f7b4",
			size:       ScryfallImagePNG,
			expected:   "https://cards.scryfall.io/png/back/f/a/fa940e68-010e-4b68-be8a-555d7068f7b4.png",
		},
		{
			name:       "empty scryfall ID",
			scryfallID: "",
			size:       ScryfallImageNormal,
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConstructBackFaceImageURL(tt.scryfallID, tt.size)
			if got != tt.expected {
				t.Errorf("ConstructBackFaceImageURL(%q, %q) = %q, want %q",
					tt.scryfallID, tt.size, got, tt.expected)
			}
		})
	}
}

func TestConstructImageURL_EdgeCases(t *testing.T) {
	// Test with very short ID (should return empty since len < 2 after cleaning)
	shortID := "a"
	result := ConstructImageURL(shortID, ScryfallImageNormal)
	if result != "" {
		t.Errorf("Short ID should return empty string, got %q", result)
	}

	// Test with ID starting with numbers
	numericID := "12345678-1234-1234-1234-123456789012"
	result = ConstructImageURL(numericID, ScryfallImageNormal)
	expected := "https://cards.scryfall.io/normal/front/1/2/12345678-1234-1234-1234-123456789012.jpg"
	if result != expected {
		t.Errorf("Numeric ID: got %q, want %q", result, expected)
	}
}
