package core

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ramonehamilton/MTGA-Companion/internal/mtga/cards"
	"github.com/ramonehamilton/MTGA-Companion/internal/mtga/cards/seventeenlands"
)

// UnifiedScorer combines all scoring components into a single interface.
type UnifiedScorer struct {
	ColorScorer   *ColorScorer
	CurveScorer   *CurveScorer
	QualityScorer *QualityScorer
	SynergyScorer *SynergyScorer
	Weights       ScoringWeights
}

// NewUnifiedScorer creates a UnifiedScorer with default constructed settings.
func NewUnifiedScorer() *UnifiedScorer {
	return &UnifiedScorer{
		ColorScorer:   NewColorScorer(),
		CurveScorer:   NewCurveScorer(),
		QualityScorer: NewQualityScorer(),
		SynergyScorer: NewSynergyScorer(),
		Weights:       DefaultConstructedWeights(),
	}
}

// NewLimitedUnifiedScorer creates a UnifiedScorer optimized for Limited.
func NewLimitedUnifiedScorer() *UnifiedScorer {
	return &UnifiedScorer{
		ColorScorer:   NewColorScorer(),
		CurveScorer:   NewLimitedCurveScorer(),
		QualityScorer: NewLimitedQualityScorer(),
		SynergyScorer: NewLimitedSynergyScorer(),
		Weights:       DefaultLimitedWeights(),
	}
}

// NewAggroUnifiedScorer creates a UnifiedScorer optimized for aggressive decks.
func NewAggroUnifiedScorer() *UnifiedScorer {
	return &UnifiedScorer{
		ColorScorer:   NewColorScorer(),
		CurveScorer:   NewAggroCurveScorer(),
		QualityScorer: NewQualityScorer(),
		SynergyScorer: NewSynergyScorer(),
		Weights: ScoringWeights{
			ColorFit:    0.20,
			ManaCurve:   0.30, // Curve more important for aggro
			CardQuality: 0.25,
			Synergy:     0.20,
			Playability: 0.05,
		},
	}
}

// NewControlUnifiedScorer creates a UnifiedScorer optimized for control decks.
func NewControlUnifiedScorer() *UnifiedScorer {
	return &UnifiedScorer{
		ColorScorer:   NewColorScorer(),
		CurveScorer:   NewControlCurveScorer(),
		QualityScorer: NewConstructedQualityScorer(),
		SynergyScorer: NewConstructedSynergyScorer(),
		Weights: ScoringWeights{
			ColorFit:    0.25,
			ManaCurve:   0.15, // Less important for control
			CardQuality: 0.30, // Quality more important
			Synergy:     0.25,
			Playability: 0.05,
		},
	}
}

// CardScore represents a scored card with breakdown.
type CardScore struct {
	Card    *cards.Card
	Score   float64
	Factors ScoreFactors
	Reason  string
}

// ScoreCard calculates the unified score for a card in context.
func (us *UnifiedScorer) ScoreCard(card *cards.Card, analysis *DeckAnalysis, ratings *RatingData) *CardScore {
	if card == nil || analysis == nil {
		return &CardScore{Card: card, Score: 0, Reason: "insufficient data"}
	}

	factors := ScoreFactors{}

	// Score color fit
	colorScore, colorReason := us.ColorScorer.ScoreColorFit(card, analysis.ColorIdentity)
	factors.ColorFit = colorScore
	factors.ColorFitReason = colorReason

	// Score mana curve fit
	totalNonLands := analysis.TotalCards - analysis.LandCount
	curveScore, curveReason := us.CurveScorer.ScoreManaCurveFit(card, analysis.ManaCurve, totalNonLands)
	factors.ManaCurve = curveScore
	factors.ManaCurveReason = curveReason

	// Score card quality
	qualityScore, qualityReason := us.QualityScorer.ScoreQuality(card, ratings)
	factors.CardQuality = qualityScore
	factors.CardQualityReason = qualityReason

	// Score synergy
	synergyScore, synergyReason := us.SynergyScorer.ScoreSynergy(card, analysis)
	factors.Synergy = synergyScore
	factors.SynergyReason = synergyReason

	// Default playability score
	factors.Playability = 0.8

	// Calculate weighted overall score
	score := (factors.ColorFit * us.Weights.ColorFit) +
		(factors.ManaCurve * us.Weights.ManaCurve) +
		(factors.CardQuality * us.Weights.CardQuality) +
		(factors.Synergy * us.Weights.Synergy) +
		(factors.Playability * us.Weights.Playability)

	// Normalize to ensure score is 0-1
	totalWeight := us.Weights.ColorFit + us.Weights.ManaCurve +
		us.Weights.CardQuality + us.Weights.Synergy + us.Weights.Playability
	if totalWeight > 0 {
		score = score / totalWeight
	}

	// Build overall reason
	reason := us.buildOverallReason(factors)

	return &CardScore{
		Card:    card,
		Score:   score,
		Factors: factors,
		Reason:  reason,
	}
}

// ScoreCards scores multiple cards and returns them sorted by score.
func (us *UnifiedScorer) ScoreCards(cardList []*cards.Card, analysis *DeckAnalysis, ratingsMap map[int]*RatingData) []*CardScore {
	scores := make([]*CardScore, 0, len(cardList))

	for _, card := range cardList {
		if card == nil {
			continue
		}
		var ratings *RatingData
		if ratingsMap != nil {
			ratings = ratingsMap[card.ArenaID]
		}
		score := us.ScoreCard(card, analysis, ratings)
		scores = append(scores, score)
	}

	// Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	return scores
}

// GetTopRecommendations returns the top N scored cards above a threshold.
func (us *UnifiedScorer) GetTopRecommendations(cardList []*cards.Card, analysis *DeckAnalysis, ratingsMap map[int]*RatingData, maxResults int, minScore float64) []*CardScore {
	allScores := us.ScoreCards(cardList, analysis, ratingsMap)

	results := make([]*CardScore, 0, maxResults)
	for _, score := range allScores {
		if score.Score < minScore {
			break // Already sorted, so no need to continue
		}
		results = append(results, score)
		if len(results) >= maxResults {
			break
		}
	}

	return results
}

// buildOverallReason generates a human-readable explanation.
func (us *UnifiedScorer) buildOverallReason(factors ScoreFactors) string {
	reasons := make([]string, 0)

	if factors.ColorFit >= 0.85 {
		reasons = append(reasons, "excellent color fit")
	} else if factors.ColorFit < 0.4 {
		reasons = append(reasons, "color requirements may be difficult")
	}

	if factors.ManaCurve >= 0.7 {
		reasons = append(reasons, "fills curve gap")
	} else if factors.ManaCurve <= 0.3 {
		reasons = append(reasons, "curve slot is full")
	}

	if factors.CardQuality >= 0.8 {
		reasons = append(reasons, "high quality card")
	}

	if factors.Synergy >= 0.7 {
		reasons = append(reasons, "strong synergy")
	}

	if len(reasons) == 0 {
		return "decent option for deck"
	}

	return strings.Join(reasons, "; ")
}

// DeterminePrimaryFactor identifies which factor most influenced the score.
func (us *UnifiedScorer) DeterminePrimaryFactor(factors ScoreFactors) string {
	type factorScore struct {
		name  string
		score float64
	}

	weightedScores := []factorScore{
		{"color-fit", factors.ColorFit * us.Weights.ColorFit},
		{"mana-curve", factors.ManaCurve * us.Weights.ManaCurve},
		{"quality", factors.CardQuality * us.Weights.CardQuality},
		{"synergy", factors.Synergy * us.Weights.Synergy},
	}

	// Find max
	maxFactor := weightedScores[0]
	for _, fs := range weightedScores[1:] {
		if fs.score > maxFactor.score {
			maxFactor = fs
		}
	}

	return maxFactor.name
}

// CalculateConfidence calculates how confident the scoring is.
func (us *UnifiedScorer) CalculateConfidence(factors ScoreFactors) float64 {
	positiveCount := 0
	highCount := 0
	totalFactors := 4

	if factors.ColorFit > 0.6 {
		positiveCount++
	}
	if factors.ColorFit > 0.8 {
		highCount++
	}

	if factors.ManaCurve > 0.6 {
		positiveCount++
	}
	if factors.ManaCurve > 0.8 {
		highCount++
	}

	if factors.CardQuality > 0.6 {
		positiveCount++
	}
	if factors.CardQuality > 0.8 {
		highCount++
	}

	if factors.Synergy > 0.6 {
		positiveCount++
	}
	if factors.Synergy > 0.8 {
		highCount++
	}

	confidence := float64(positiveCount) / float64(totalFactors)

	// Boost confidence for multiple high scores
	if highCount >= 2 {
		confidence += 0.1
	}
	if highCount >= 3 {
		confidence += 0.1
	}

	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// RatingDataFromSeventeenLands creates RatingData from 17Lands data.
func RatingDataFromSeventeenLands(rating *seventeenlands.CardRating) *RatingData {
	return &RatingData{
		SeventeenLandsRating: rating,
	}
}

// RatingDataWithCFB creates RatingData with both 17Lands and CFB scores.
func RatingDataWithCFB(rating *seventeenlands.CardRating, cfbScore float64) *RatingData {
	return &RatingData{
		SeventeenLandsRating: rating,
		CFBLimitedScore:      cfbScore,
		HasCFBRating:         true,
	}
}

// AnalyzeAndScore is a convenience method that analyzes cards and scores a candidate.
func AnalyzeAndScore(deckCards []*cards.Card, candidate *cards.Card, ratings *RatingData) *CardScore {
	analysis := AnalyzeDeck(deckCards)
	scorer := NewUnifiedScorer()
	return scorer.ScoreCard(candidate, analysis, ratings)
}

// AnalyzeAndScoreForLimited is optimized for Limited play.
func AnalyzeAndScoreForLimited(deckCards []*cards.Card, candidate *cards.Card, ratings *RatingData) *CardScore {
	analysis := AnalyzeDeck(deckCards)
	scorer := NewLimitedUnifiedScorer()
	return scorer.ScoreCard(candidate, analysis, ratings)
}

// FormatScoreBreakdown returns a formatted string of score factors.
func FormatScoreBreakdown(score *CardScore) string {
	if score == nil {
		return "no score data"
	}

	return fmt.Sprintf(
		"Score: %.2f (Color: %.2f, Curve: %.2f, Quality: %.2f, Synergy: %.2f)",
		score.Score,
		score.Factors.ColorFit,
		score.Factors.ManaCurve,
		score.Factors.CardQuality,
		score.Factors.Synergy,
	)
}
