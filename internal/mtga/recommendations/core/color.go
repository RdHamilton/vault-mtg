package core

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ramonehamilton/MTGA-Companion/internal/mtga/cards"
)

// ColorScorer provides color compatibility scoring for cards.
type ColorScorer struct {
	// AllowSplash determines if off-color cards can get partial scores
	AllowSplash bool
	// SplashPenalty is the multiplier for splash-colored cards (0.0-1.0)
	SplashPenalty float64
}

// NewColorScorer creates a new ColorScorer with default settings.
func NewColorScorer() *ColorScorer {
	return &ColorScorer{
		AllowSplash:   true,
		SplashPenalty: 0.5,
	}
}

// ScoreColorFit scores how well a card fits within a deck's color identity.
// Returns a score from 0.0 to 1.0 and a reason string.
func (cs *ColorScorer) ScoreColorFit(card *cards.Card, deckColors []string) (float64, string) {
	if card == nil {
		return 0, "no card provided"
	}

	cardColors := card.Colors
	if len(cardColors) == 0 {
		// Colorless cards always fit
		return 1.0, "colorless - fits any deck"
	}

	if len(deckColors) == 0 {
		// No deck colors defined - colorless deck or undefined
		return 0.5, "deck has no color identity"
	}

	// Count matching and non-matching colors
	matchingColors := 0
	for _, cardColor := range cardColors {
		if containsStringIgnoreCase(deckColors, cardColor) {
			matchingColors++
		}
	}

	// Perfect match - all card colors are in deck
	if matchingColors == len(cardColors) {
		return 1.0, fmt.Sprintf("perfect color match (%s)", strings.Join(cardColors, ""))
	}

	// Partial match - some colors fit
	if matchingColors > 0 {
		ratio := float64(matchingColors) / float64(len(cardColors))
		if cs.AllowSplash {
			// Apply splash penalty as a multiplier to reduce score for partial matches
			score := ratio * cs.SplashPenalty
			return score, fmt.Sprintf("partial color match (%d/%d colors)", matchingColors, len(cardColors))
		}
		return ratio, fmt.Sprintf("partial color match (%d/%d colors)", matchingColors, len(cardColors))
	}

	// No match - completely off-color
	if cs.AllowSplash {
		return cs.SplashPenalty * 0.5, "off-color splash candidate"
	}
	return 0, "off-color - does not fit deck"
}

// ScoreColorCompatibility scores how well a card's colors match a target color set.
// This is useful for seed-based recommendations where we want cards that share colors.
func (cs *ColorScorer) ScoreColorCompatibility(card *cards.Card, targetColors []string) (float64, string) {
	if card == nil {
		return 0, "no card provided"
	}

	cardColors := card.Colors
	if len(cardColors) == 0 {
		// Colorless cards get a moderate score - they fit but don't synergize on color
		return 0.6, "colorless - compatible but no color synergy"
	}

	if len(targetColors) == 0 {
		return 0.5, "no target colors specified"
	}

	// Check for exact match
	if colorsMatch(cardColors, targetColors) {
		return 1.0, fmt.Sprintf("exact color match (%s)", strings.Join(cardColors, ""))
	}

	// Check if card colors are a subset of target colors
	allMatch := true
	for _, cardColor := range cardColors {
		if !containsStringIgnoreCase(targetColors, cardColor) {
			allMatch = false
			break
		}
	}
	if allMatch {
		return 0.9, "card colors within target identity"
	}

	// Check overlap
	overlap := 0
	for _, cardColor := range cardColors {
		if containsStringIgnoreCase(targetColors, cardColor) {
			overlap++
		}
	}

	if overlap > 0 {
		// Some overlap - calculate based on how much
		overlapRatio := float64(overlap) / float64(len(cardColors))
		score := 0.5 + (overlapRatio * 0.4) // Score between 0.5 and 0.9
		return score, fmt.Sprintf("partial color overlap (%d/%d)", overlap, len(cardColors))
	}

	// No overlap at all
	return 0.2, "no color overlap"
}

// GetDeckColorIdentity determines the color identity of a deck.
// It considers both the colors of cards and their color identity (for things like hybrid mana).
func GetDeckColorIdentity(deckCards []*cards.Card) []string {
	colorCounts := make(map[string]int)

	for _, card := range deckCards {
		if card == nil {
			continue
		}

		// Count colors from the card
		for _, color := range card.Colors {
			colorCounts[color]++
		}

		// Also count from color identity if available
		for _, color := range card.ColorIdentity {
			colorCounts[color]++
		}
	}

	// Sort colors by count (most common first)
	type colorCount struct {
		color string
		count int
	}
	var counts []colorCount
	for color, count := range colorCounts {
		counts = append(counts, colorCount{color, count})
	}
	sort.Slice(counts, func(i, j int) bool {
		return counts[i].count > counts[j].count
	})

	// Return top colors (typically 1-3)
	result := make([]string, 0, len(counts))
	for _, cc := range counts {
		if cc.count > 0 {
			result = append(result, cc.color)
		}
	}

	return result
}

// containsStringIgnoreCase checks if a slice contains a string (case-insensitive).
func containsStringIgnoreCase(slice []string, s string) bool {
	sLower := strings.ToLower(s)
	for _, v := range slice {
		if strings.ToLower(v) == sLower {
			return true
		}
	}
	return false
}

// colorsMatch checks if two color sets are equivalent.
func colorsMatch(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, color := range a {
		if !containsStringIgnoreCase(b, color) {
			return false
		}
	}
	return true
}
