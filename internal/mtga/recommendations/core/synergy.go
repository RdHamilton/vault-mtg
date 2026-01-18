package core

import (
	"fmt"
	"strings"

	"github.com/ramonehamilton/MTGA-Companion/internal/mtga/cards"
)

// SynergyScorer provides synergy analysis between cards and decks.
type SynergyScorer struct {
	// KeywordWeight is the weight for keyword-based synergy
	KeywordWeight float64
	// TribalWeight is the weight for creature type synergy
	TribalWeight float64
	// ThemeWeight is the weight for theme-based synergy
	ThemeWeight float64
	// MinTribalCount is the minimum creature count to consider tribal synergy
	MinTribalCount int
}

// NewSynergyScorer creates a new SynergyScorer with default settings.
func NewSynergyScorer() *SynergyScorer {
	return &SynergyScorer{
		KeywordWeight:  0.40,
		TribalWeight:   0.35,
		ThemeWeight:    0.25,
		MinTribalCount: 3,
	}
}

// NewLimitedSynergyScorer creates a SynergyScorer optimized for Limited.
func NewLimitedSynergyScorer() *SynergyScorer {
	return &SynergyScorer{
		KeywordWeight:  0.35,
		TribalWeight:   0.40, // Tribal more important in limited
		ThemeWeight:    0.25,
		MinTribalCount: 4,
	}
}

// NewConstructedSynergyScorer creates a SynergyScorer for Constructed.
func NewConstructedSynergyScorer() *SynergyScorer {
	return &SynergyScorer{
		KeywordWeight:  0.35,
		TribalWeight:   0.25, // Less tribal focus in constructed
		ThemeWeight:    0.40, // Theme combos more important
		MinTribalCount: 6,
	}
}

// SynergyResult contains detailed synergy analysis.
type SynergyResult struct {
	Score           float64
	Reason          string
	KeywordSynergy  float64
	TribalSynergy   float64
	ThemeSynergy    float64
	MatchedKeywords []string
	MatchedTypes    []string
	MatchedThemes   []string
}

// ScoreSynergy calculates synergy between a card and deck analysis.
// Returns score (0.0-1.0) and reason.
func (ss *SynergyScorer) ScoreSynergy(card *cards.Card, analysis *DeckAnalysis) (float64, string) {
	result := ss.ScoreSynergyDetailed(card, analysis)
	return result.Score, result.Reason
}

// ScoreSynergyDetailed provides detailed synergy breakdown.
func (ss *SynergyScorer) ScoreSynergyDetailed(card *cards.Card, analysis *DeckAnalysis) *SynergyResult {
	if card == nil || analysis == nil {
		return &SynergyResult{Score: 0.5, Reason: "insufficient data"}
	}

	result := &SynergyResult{
		MatchedKeywords: []string{},
		MatchedTypes:    []string{},
		MatchedThemes:   []string{},
	}

	// Score keyword synergy
	if card.OracleText != nil {
		result.KeywordSynergy = ss.scoreKeywordSynergy(*card.OracleText, analysis, result)
	}

	// Score tribal synergy
	if containsTypeInTypeLine(card.TypeLine, "Creature") {
		result.TribalSynergy = ss.scoreTribalSynergy(card.TypeLine, analysis, result)
	}

	// Score theme synergy
	result.ThemeSynergy = ss.scoreThemeSynergy(card, analysis, result)

	// Calculate weighted score
	totalWeight := 0.0
	totalScore := 0.0

	if result.KeywordSynergy > 0 {
		totalScore += result.KeywordSynergy * ss.KeywordWeight
		totalWeight += ss.KeywordWeight
	}

	if result.TribalSynergy > 0 {
		totalScore += result.TribalSynergy * ss.TribalWeight
		totalWeight += ss.TribalWeight
	}

	if result.ThemeSynergy > 0 {
		totalScore += result.ThemeSynergy * ss.ThemeWeight
		totalWeight += ss.ThemeWeight
	}

	if totalWeight > 0 {
		result.Score = totalScore / totalWeight
	} else {
		result.Score = 0.5 // Neutral if no synergy detected
	}

	// Build reason
	result.Reason = ss.buildReason(result)

	return result
}

// scoreKeywordSynergy calculates keyword-based synergy.
func (ss *SynergyScorer) scoreKeywordSynergy(oracleText string, analysis *DeckAnalysis, result *SynergyResult) float64 {
	cardKeywords := extractKeywordsFromText(oracleText)
	if len(cardKeywords) == 0 {
		return 0
	}

	matchCount := 0
	for keyword := range cardKeywords {
		if count, ok := analysis.Keywords[keyword]; ok && count > 0 {
			matchCount++
			result.MatchedKeywords = append(result.MatchedKeywords, keyword)
		}
	}

	if matchCount == 0 {
		return 0
	}

	// Score increases with matches but with diminishing returns
	// 1 match = 0.5, 2 matches = 0.7, 3+ matches = 0.85+ (capped at 1.0)
	if matchCount >= 3 {
		score := 0.85 + float64(matchCount-3)*0.05
		if score > 1.0 {
			score = 1.0
		}
		return score
	}
	if matchCount == 2 {
		return 0.7
	}
	return 0.5
}

// scoreTribalSynergy calculates creature type synergy.
func (ss *SynergyScorer) scoreTribalSynergy(typeLine string, analysis *DeckAnalysis, result *SynergyResult) float64 {
	cardTypes := extractCreatureTypes(typeLine)
	if len(cardTypes) == 0 {
		return 0
	}

	bestSynergy := 0.0
	for _, creatureType := range cardTypes {
		if count, ok := analysis.CreatureTypes[creatureType]; ok && count >= ss.MinTribalCount {
			result.MatchedTypes = append(result.MatchedTypes, creatureType)

			// Calculate synergy based on tribal concentration
			var synergy float64
			if count >= 8 {
				synergy = 0.9 // Strong tribal theme
			} else if count >= 6 {
				synergy = 0.75
			} else if count >= ss.MinTribalCount {
				synergy = 0.6
			}

			if synergy > bestSynergy {
				bestSynergy = synergy
			}
		}
	}

	return bestSynergy
}

// scoreThemeSynergy calculates theme-based synergy.
func (ss *SynergyScorer) scoreThemeSynergy(card *cards.Card, analysis *DeckAnalysis, result *SynergyResult) float64 {
	if len(analysis.Themes) == 0 || card.OracleText == nil {
		return 0
	}

	oracleText := strings.ToLower(*card.OracleText)
	matchedThemes := 0

	for _, theme := range analysis.Themes {
		if ss.cardMatchesTheme(oracleText, card.TypeLine, theme) {
			matchedThemes++
			result.MatchedThemes = append(result.MatchedThemes, theme)
		}
	}

	if matchedThemes == 0 {
		return 0
	}

	// Score based on theme matches
	if matchedThemes >= 2 {
		return 0.9 // Card supports multiple deck themes
	}
	return 0.7 // Card supports one deck theme
}

// cardMatchesTheme checks if a card matches a deck theme.
func (ss *SynergyScorer) cardMatchesTheme(oracleText, typeLine, theme string) bool {
	theme = strings.ToLower(theme)

	switch {
	case theme == "lifegain":
		return strings.Contains(oracleText, "lifelink") ||
			strings.Contains(oracleText, "gain life") ||
			strings.Contains(oracleText, "whenever you gain life")

	case theme == "tokens":
		return strings.Contains(oracleText, "create a") ||
			strings.Contains(oracleText, "creates a") ||
			strings.Contains(oracleText, "token")

	case theme == "counters":
		return strings.Contains(oracleText, "+1/+1 counter") ||
			strings.Contains(oracleText, "put a counter")

	case theme == "graveyard":
		return strings.Contains(oracleText, "graveyard") ||
			strings.Contains(oracleText, "mill") ||
			strings.Contains(oracleText, "return")

	case theme == "spellslinger":
		return containsTypeInTypeLine(typeLine, "Instant") ||
			containsTypeInTypeLine(typeLine, "Sorcery") ||
			strings.Contains(oracleText, "instant or sorcery") ||
			strings.Contains(oracleText, "noncreature spell")

	case strings.HasPrefix(theme, "tribal-"):
		creatureType := strings.TrimPrefix(theme, "tribal-")
		return containsTypeInTypeLine(typeLine, creatureType)

	default:
		return strings.Contains(oracleText, theme)
	}
}

// buildReason generates human-readable synergy explanation.
func (ss *SynergyScorer) buildReason(result *SynergyResult) string {
	if result.Score < 0.4 {
		return "no significant synergy detected"
	}

	reasons := []string{}

	if len(result.MatchedTypes) > 0 {
		reasons = append(reasons, fmt.Sprintf("tribal synergy with %s", strings.Join(result.MatchedTypes, ", ")))
	}

	if len(result.MatchedThemes) > 0 {
		reasons = append(reasons, fmt.Sprintf("supports %s theme", strings.Join(result.MatchedThemes, ", ")))
	}

	if len(result.MatchedKeywords) > 0 && len(result.MatchedKeywords) <= 3 {
		reasons = append(reasons, fmt.Sprintf("shares %s", strings.Join(result.MatchedKeywords, ", ")))
	} else if len(result.MatchedKeywords) > 3 {
		reasons = append(reasons, fmt.Sprintf("shares %d keywords", len(result.MatchedKeywords)))
	}

	if len(reasons) == 0 {
		if result.Score >= 0.7 {
			return "strong synergy with deck strategy"
		}
		return "moderate synergy with deck"
	}

	return strings.Join(reasons, "; ")
}

// ScoreCardPairSynergy scores synergy between two specific cards.
func (ss *SynergyScorer) ScoreCardPairSynergy(card1, card2 *cards.Card) (float64, string) {
	if card1 == nil || card2 == nil {
		return 0.5, "insufficient card data"
	}

	score := 0.0
	reasons := []string{}

	// Keyword synergy
	if card1.OracleText != nil && card2.OracleText != nil {
		kw1 := ExtractKeywordsWithInfo(*card1.OracleText)
		kw2 := ExtractKeywordsWithInfo(*card2.OracleText)
		kwScore := CalculateKeywordSynergy(kw1, kw2)
		if kwScore > 0.3 {
			score += kwScore * ss.KeywordWeight
			reasons = append(reasons, "shared keywords")
		}
	}

	// Tribal synergy
	if containsTypeInTypeLine(card1.TypeLine, "Creature") &&
		containsTypeInTypeLine(card2.TypeLine, "Creature") {
		types1 := extractCreatureTypes(card1.TypeLine)
		types2 := extractCreatureTypes(card2.TypeLine)
		for _, t1 := range types1 {
			for _, t2 := range types2 {
				if strings.EqualFold(t1, t2) {
					score += 0.6 * ss.TribalWeight
					reasons = append(reasons, fmt.Sprintf("shared type: %s", t1))
					break
				}
			}
		}
	}

	if len(reasons) == 0 {
		return 0.5, "no direct synergy"
	}

	if score > 1.0 {
		score = 1.0
	}

	return score, strings.Join(reasons, "; ")
}
