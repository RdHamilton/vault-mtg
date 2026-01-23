package mtgjson

import (
	"fmt"
	"strings"
)

// ScryfallImageSize represents the available image sizes from Scryfall.
type ScryfallImageSize string

const (
	// ScryfallImageSmall is 146x204 JPG (for inline display, tooltips)
	ScryfallImageSmall ScryfallImageSize = "small"
	// ScryfallImageNormal is 488x680 JPG (standard quality)
	ScryfallImageNormal ScryfallImageSize = "normal"
	// ScryfallImageLarge is 672x936 JPG (high quality)
	ScryfallImageLarge ScryfallImageSize = "large"
	// ScryfallImagePNG is 745x1040 lossless PNG
	ScryfallImagePNG ScryfallImageSize = "png"
	// ScryfallImageArtCrop is 626x457 JPG (cropped to art only)
	ScryfallImageArtCrop ScryfallImageSize = "art_crop"
	// ScryfallImageBorderCrop is 480x680 JPG (full card, no border)
	ScryfallImageBorderCrop ScryfallImageSize = "border_crop"
)

// scryfallImageBaseURL is the base URL for Scryfall card images.
const scryfallImageBaseURL = "https://cards.scryfall.io"

// ConstructImageURL builds a Scryfall image URL from a Scryfall UUID.
// Scryfall image URLs follow the pattern:
// https://cards.scryfall.io/{size}/front/{a}/{b}/{uuid}.jpg
// where {a} and {b} are the first two characters of the UUID.
//
// Example:
//
//	UUID: fa940e68-010e-4b68-be8a-555d7068f7b4
//	URL:  https://cards.scryfall.io/normal/front/f/a/fa940e68-010e-4b68-be8a-555d7068f7b4.jpg
func ConstructImageURL(scryfallID string, size ScryfallImageSize) string {
	if scryfallID == "" {
		return ""
	}

	// Remove hyphens to get clean UUID chars
	cleanID := strings.ReplaceAll(scryfallID, "-", "")
	if len(cleanID) < 2 {
		return ""
	}

	a := string(scryfallID[0])
	b := string(scryfallID[1])

	// Determine file extension based on size
	ext := "jpg"
	if size == ScryfallImagePNG {
		ext = "png"
	}

	return fmt.Sprintf("%s/%s/front/%s/%s/%s.%s",
		scryfallImageBaseURL,
		size,
		a,
		b,
		scryfallID,
		ext,
	)
}

// ConstructImageURLs builds all standard image URLs from a Scryfall UUID.
// Returns normal, small, and art_crop URLs.
type ImageURLs struct {
	Normal  string
	Small   string
	Large   string
	ArtCrop string
}

// ConstructAllImageURLs builds all image URLs from a Scryfall UUID.
func ConstructAllImageURLs(scryfallID string) ImageURLs {
	return ImageURLs{
		Normal:  ConstructImageURL(scryfallID, ScryfallImageNormal),
		Small:   ConstructImageURL(scryfallID, ScryfallImageSmall),
		Large:   ConstructImageURL(scryfallID, ScryfallImageLarge),
		ArtCrop: ConstructImageURL(scryfallID, ScryfallImageArtCrop),
	}
}

// ConstructBackFaceImageURL builds a Scryfall image URL for the back face of a DFC.
// Back face URLs use "back" instead of "front" in the path.
func ConstructBackFaceImageURL(scryfallID string, size ScryfallImageSize) string {
	if scryfallID == "" {
		return ""
	}

	cleanID := strings.ReplaceAll(scryfallID, "-", "")
	if len(cleanID) < 2 {
		return ""
	}

	a := string(scryfallID[0])
	b := string(scryfallID[1])

	ext := "jpg"
	if size == ScryfallImagePNG {
		ext = "png"
	}

	return fmt.Sprintf("%s/%s/back/%s/%s/%s.%s",
		scryfallImageBaseURL,
		size,
		a,
		b,
		scryfallID,
		ext,
	)
}
