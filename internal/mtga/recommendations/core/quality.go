package core

import (
	"math"
	"strings"

	"github.com/ramonehamilton/MTGA-Companion/internal/mtga/cards"
	"github.com/ramonehamilton/MTGA-Companion/internal/mtga/cards/seventeenlands"
)

// QualityScorer provides card quality scoring based on ratings and rarity.
type QualityScorer struct {
	// Use17LandsWeight is the weight for 17Lands data (0.0-1.0)
	Use17LandsWeight float64
	// UseCFBWeight is the weight for ChannelFireball ratings (0.0-1.0)
	UseCFBWeight float64
	// UseRarityFallback enables rarity-based scoring when no ratings available
	UseRarityFallback bool
}

// NewQualityScorer creates a QualityScorer with default settings.
func NewQualityScorer() *QualityScorer {
	return &QualityScorer{
		Use17LandsWeight:  0.70, // 17Lands gets more weight (data-driven)
		UseCFBWeight:      0.30, // CFB is expert opinion
		UseRarityFallback: true,
	}
}

// NewLimitedQualityScorer creates a QualityScorer optimized for Limited.
func NewLimitedQualityScorer() *QualityScorer {
	return &QualityScorer{
		Use17LandsWeight:  0.80, // 17Lands more important for Limited
		UseCFBWeight:      0.20,
		UseRarityFallback: true,
	}
}

// NewConstructedQualityScorer creates a QualityScorer optimized for Constructed.
func NewConstructedQualityScorer() *QualityScorer {
	return &QualityScorer{
		Use17LandsWeight:  0.50, // Less reliance on 17Lands for constructed
		UseCFBWeight:      0.50, // Expert opinion more valuable
		UseRarityFallback: true,
	}
}

// RatingData contains ratings from various sources for scoring.
type RatingData struct {
	SeventeenLandsRating *seventeenlands.CardRating
	CFBLimitedScore      float64 // Normalized 0-1 score
	HasCFBRating         bool
}

// ScoreQuality calculates card quality from available ratings.
// Returns a score from 0.0 to 1.0 and a reason string.
func (qs *QualityScorer) ScoreQuality(card *cards.Card, ratings *RatingData) (float64, string) {
	if card == nil {
		return 0.5, "no card provided"
	}

	var scores []float64
	var weights []float64

	// Calculate 17Lands score
	if ratings != nil && ratings.SeventeenLandsRating != nil {
		score17L := qs.calculate17LandsScore(ratings.SeventeenLandsRating)
		scores = append(scores, score17L)
		weights = append(weights, qs.Use17LandsWeight)
	}

	// Add CFB score if available
	if ratings != nil && ratings.HasCFBRating {
		scores = append(scores, ratings.CFBLimitedScore)
		weights = append(weights, qs.UseCFBWeight)
	}

	// If we have ratings, blend them
	if len(scores) > 0 {
		blended := blendScores(scores, weights)
		reason := qs.buildReason(blended, len(scores) > 1)
		return blended, reason
	}

	// Fallback to rarity
	if qs.UseRarityFallback {
		score := qs.scoreByRarity(card.Rarity)
		return score, "quality based on rarity"
	}

	return 0.5, "no rating data available"
}

// ScoreQualityByRarity provides a quality score based only on rarity.
func (qs *QualityScorer) ScoreQualityByRarity(card *cards.Card) (float64, string) {
	if card == nil {
		return 0.5, "no card provided"
	}
	score := qs.scoreByRarity(card.Rarity)
	return score, "quality based on rarity"
}

// calculate17LandsScore calculates quality from 17Lands metrics.
func (qs *QualityScorer) calculate17LandsScore(rating *seventeenlands.CardRating) float64 {
	// Weight: 50% GIHWR, 30% OHWR, 10% ATA, 10% ALSA

	// Normalize GIHWR and OHWR (they're percentages, typically 45-60% range)
	gihScore := normalize(rating.GIHWR, 0.45, 0.65)
	ohScore := normalize(rating.OHWR, 0.45, 0.65)

	// ATA (Average Taken At): Lower is better. Typical range 1-14
	ataScore := 1.0 - normalize(rating.ATA, 1.0, 14.0)

	// ALSA (Average Last Seen At): Higher means it wheels more (less valuable)
	alsaScore := 1.0 - normalize(rating.ALSA, 1.0, 14.0)

	// Weighted combination
	return (gihScore * 0.50) + (ohScore * 0.30) + (ataScore * 0.10) + (alsaScore * 0.10)
}

// scoreByRarity provides a quality score based on card rarity.
func (qs *QualityScorer) scoreByRarity(rarity string) float64 {
	rarityScores := map[string]float64{
		"mythic":   0.85,
		"rare":     0.75,
		"uncommon": 0.60,
		"common":   0.50,
	}

	if score, ok := rarityScores[strings.ToLower(rarity)]; ok {
		return score
	}
	return 0.5
}

// buildReason generates a human-readable quality reason.
func (qs *QualityScorer) buildReason(score float64, hasMultipleSources bool) string {
	quality := "moderate"
	if score >= 0.8 {
		quality = "excellent"
	} else if score >= 0.7 {
		quality = "high"
	} else if score >= 0.6 {
		quality = "above average"
	} else if score < 0.4 {
		quality = "below average"
	}

	if hasMultipleSources {
		return quality + " quality (multiple rating sources)"
	}
	return quality + " quality card"
}

// normalize maps a value from [min, max] to [0, 1].
func normalize(value, min, max float64) float64 {
	if value <= min {
		return 0.0
	}
	if value >= max {
		return 1.0
	}
	return (value - min) / (max - min)
}

// blendScores blends multiple scores with their weights.
func blendScores(scores, weights []float64) float64 {
	if len(scores) == 0 || len(weights) == 0 {
		return 0.5
	}

	totalWeight := 0.0
	for _, w := range weights {
		totalWeight += w
	}

	if totalWeight == 0 {
		return 0.5
	}

	blended := 0.0
	for i, score := range scores {
		blended += score * (weights[i] / totalWeight)
	}

	return math.Min(1.0, math.Max(0.0, blended))
}
