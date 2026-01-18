// Package core provides shared scoring components for the unified suggestion engine.
// This package extracts common functionality from RuleBasedEngine, SeedDeckBuilder,
// DeckSuggester, and SuggestionGenerator into reusable, testable components.
package core

import (
	"github.com/ramonehamilton/MTGA-Companion/internal/mtga/cards"
)

// ScoreFactors represents the breakdown of scoring components.
type ScoreFactors struct {
	ColorFit    float64 `json:"colorFit"`
	ManaCurve   float64 `json:"manaCurve"`
	CardQuality float64 `json:"cardQuality"`
	Synergy     float64 `json:"synergy"`
	Playability float64 `json:"playability,omitempty"`

	// Optional detailed breakdowns
	ColorFitReason    string `json:"colorFitReason,omitempty"`
	ManaCurveReason   string `json:"manaCurveReason,omitempty"`
	CardQualityReason string `json:"cardQualityReason,omitempty"`
	SynergyReason     string `json:"synergyReason,omitempty"`
}

// ScoringWeights configures the weight of each factor in the final score.
type ScoringWeights struct {
	ColorFit    float64
	ManaCurve   float64
	CardQuality float64
	Synergy     float64
	Playability float64
}

// DefaultConstructedWeights returns standard weights for 60-card constructed decks.
func DefaultConstructedWeights() ScoringWeights {
	return ScoringWeights{
		ColorFit:    0.25,
		ManaCurve:   0.20,
		CardQuality: 0.25,
		Synergy:     0.25,
		Playability: 0.05,
	}
}

// DefaultLimitedWeights returns weights optimized for draft/limited.
func DefaultLimitedWeights() ScoringWeights {
	return ScoringWeights{
		ColorFit:    0.20,
		ManaCurve:   0.25,
		CardQuality: 0.35,
		Synergy:     0.15,
		Playability: 0.05,
	}
}

// DeckAnalysis contains analyzed information about a deck's composition.
type DeckAnalysis struct {
	// Color distribution
	ColorIdentity []string
	ColorCounts   map[string]int
	TotalCards    int

	// Type distribution
	CreatureCount     int
	InstantCount      int
	SorceryCount      int
	EnchantmentCount  int
	ArtifactCount     int
	PlaneswalkerCount int
	LandCount         int

	// Mana curve (index = CMC, value = count)
	ManaCurve []int

	// Average CMC (excluding lands)
	AverageCMC float64

	// Creature types for tribal detection
	CreatureTypes map[string]int

	// Keywords present in the deck
	Keywords map[string]int

	// Themes detected (e.g., "lifegain", "tokens", "counters")
	Themes []string
}

// NewDeckAnalysis creates an empty deck analysis.
func NewDeckAnalysis() *DeckAnalysis {
	return &DeckAnalysis{
		ColorIdentity: []string{},
		ColorCounts:   make(map[string]int),
		ManaCurve:     make([]int, 16), // Support up to CMC 15
		CreatureTypes: make(map[string]int),
		Keywords:      make(map[string]int),
		Themes:        []string{},
	}
}

// AnalyzeDeck analyzes a slice of cards and returns a DeckAnalysis.
func AnalyzeDeck(deckCards []*cards.Card) *DeckAnalysis {
	analysis := NewDeckAnalysis()
	totalCMC := 0
	nonLandCount := 0

	for _, card := range deckCards {
		if card == nil {
			continue
		}

		analysis.TotalCards++

		// Count colors (include both Colors and ColorIdentity, deduplicated per card)
		seenColors := make(map[string]bool)
		allColors := append(card.Colors, card.ColorIdentity...)
		for _, color := range allColors {
			if seenColors[color] {
				continue
			}
			seenColors[color] = true
			analysis.ColorCounts[color]++
			if !containsString(analysis.ColorIdentity, color) {
				analysis.ColorIdentity = append(analysis.ColorIdentity, color)
			}
		}

		// Count card types
		typeLine := card.TypeLine
		if containsTypeInTypeLine(typeLine, "Creature") {
			analysis.CreatureCount++
			// Extract creature types
			for _, ct := range extractCreatureTypes(typeLine) {
				analysis.CreatureTypes[ct]++
			}
		}
		if containsTypeInTypeLine(typeLine, "Instant") {
			analysis.InstantCount++
		}
		if containsTypeInTypeLine(typeLine, "Sorcery") {
			analysis.SorceryCount++
		}
		if containsTypeInTypeLine(typeLine, "Enchantment") {
			analysis.EnchantmentCount++
		}
		if containsTypeInTypeLine(typeLine, "Artifact") {
			analysis.ArtifactCount++
		}
		if containsTypeInTypeLine(typeLine, "Planeswalker") {
			analysis.PlaneswalkerCount++
		}
		if containsTypeInTypeLine(typeLine, "Land") {
			analysis.LandCount++
		} else {
			// Track CMC for non-land cards
			cmc := int(card.CMC)
			if cmc >= 0 {
				// Clamp high CMC cards to the last bucket
				cmcBucket := cmc
				if cmcBucket >= len(analysis.ManaCurve) {
					cmcBucket = len(analysis.ManaCurve) - 1
				}
				analysis.ManaCurve[cmcBucket]++
			}
			totalCMC += cmc
			nonLandCount++
		}

		// Extract keywords from oracle text
		if card.OracleText != nil {
			keywords := ExtractKeywords(*card.OracleText)
			for _, kw := range keywords {
				analysis.Keywords[kw]++
			}
		}
	}

	// Calculate average CMC
	if nonLandCount > 0 {
		analysis.AverageCMC = float64(totalCMC) / float64(nonLandCount)
	}

	// Detect themes based on keywords and card counts
	analysis.Themes = detectThemes(analysis)

	return analysis
}

// detectThemes identifies deck themes based on keywords and card composition.
func detectThemes(analysis *DeckAnalysis) []string {
	themes := []string{}

	// Lifegain theme
	if analysis.Keywords["lifelink"] >= 3 || analysis.Keywords["gain life"] >= 2 {
		themes = append(themes, "lifegain")
	}

	// Tokens theme
	if analysis.Keywords["create"] >= 4 || analysis.Keywords["token"] >= 3 {
		themes = append(themes, "tokens")
	}

	// Counters theme
	if analysis.Keywords["+1/+1 counter"] >= 3 {
		themes = append(themes, "counters")
	}

	// Graveyard theme
	if analysis.Keywords["graveyard"] >= 2 || analysis.Keywords["return from your graveyard"] >= 2 {
		themes = append(themes, "graveyard")
	}

	// Spellslinger theme
	spellCount := analysis.InstantCount + analysis.SorceryCount
	if spellCount >= 15 {
		themes = append(themes, "spellslinger")
	}

	// Tribal themes (check for creature type concentration)
	for creatureType, count := range analysis.CreatureTypes {
		if count >= 6 {
			themes = append(themes, "tribal-"+creatureType)
		}
	}

	return themes
}

// containsString checks if a slice contains a string.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
