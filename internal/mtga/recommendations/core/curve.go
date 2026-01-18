package core

import (
	"fmt"
	"math"

	"github.com/ramonehamilton/MTGA-Companion/internal/mtga/cards"
)

// CurveScorer provides mana curve analysis and scoring.
type CurveScorer struct {
	// TargetCurve defines the ideal distribution of cards by CMC
	// Key is CMC, value is the ideal count/proportion
	TargetCurve map[int]float64

	// TargetAverageCMC is the ideal average mana cost
	TargetAverageCMC float64

	// MaxCMC is the highest CMC to consider (cards above this get grouped)
	MaxCMC int
}

// NewCurveScorer creates a new CurveScorer with default settings for 60-card constructed.
func NewCurveScorer() *CurveScorer {
	return &CurveScorer{
		TargetCurve: map[int]float64{
			1: 0.10, // 10% 1-drops
			2: 0.25, // 25% 2-drops
			3: 0.25, // 25% 3-drops
			4: 0.20, // 20% 4-drops
			5: 0.12, // 12% 5-drops
			6: 0.08, // 8% 6+ drops
		},
		TargetAverageCMC: 2.8,
		MaxCMC:           6,
	}
}

// NewAggroCurveScorer creates a CurveScorer optimized for aggressive decks.
func NewAggroCurveScorer() *CurveScorer {
	return &CurveScorer{
		TargetCurve: map[int]float64{
			1: 0.25, // 25% 1-drops
			2: 0.35, // 35% 2-drops
			3: 0.25, // 25% 3-drops
			4: 0.10, // 10% 4-drops
			5: 0.05, // 5% 5+ drops
		},
		TargetAverageCMC: 2.0,
		MaxCMC:           5,
	}
}

// NewControlCurveScorer creates a CurveScorer optimized for control decks.
func NewControlCurveScorer() *CurveScorer {
	return &CurveScorer{
		TargetCurve: map[int]float64{
			1: 0.05, // 5% 1-drops
			2: 0.20, // 20% 2-drops
			3: 0.25, // 25% 3-drops
			4: 0.20, // 20% 4-drops
			5: 0.15, // 15% 5-drops
			6: 0.15, // 15% 6+ drops
		},
		TargetAverageCMC: 3.5,
		MaxCMC:           6,
	}
}

// NewLimitedCurveScorer creates a CurveScorer for 40-card limited decks.
func NewLimitedCurveScorer() *CurveScorer {
	return &CurveScorer{
		TargetCurve: map[int]float64{
			1: 0.04, // 4% 1-drops (1-2 cards)
			2: 0.22, // 22% 2-drops (5-6 cards)
			3: 0.30, // 30% 3-drops (7-8 cards)
			4: 0.22, // 22% 4-drops (5-6 cards)
			5: 0.13, // 13% 5-drops (3 cards)
			6: 0.09, // 9% 6+ drops (2 cards)
		},
		TargetAverageCMC: 3.2,
		MaxCMC:           6,
	}
}

// ScoreManaCurveFit scores how well a card would fit the deck's mana curve.
// Returns a score from 0.0 to 1.0 and a reason string.
func (cs *CurveScorer) ScoreManaCurveFit(card *cards.Card, currentCurve []int, totalNonLands int) (float64, string) {
	if card == nil {
		return 0, "no card provided"
	}

	cmc := int(card.CMC)
	if cmc < 0 {
		cmc = 0
	}

	// Group high CMC cards
	cmcBucket := cmc
	if cmcBucket > cs.MaxCMC {
		cmcBucket = cs.MaxCMC
	}

	// Get current count at this CMC
	currentCount := 0
	if cmcBucket < len(currentCurve) {
		currentCount = currentCurve[cmcBucket]
	}

	// Get target proportion for this CMC
	targetProp, hasTarget := cs.TargetCurve[cmcBucket]
	if !hasTarget {
		// CMC not in target curve - use diminishing returns
		return 0.3, fmt.Sprintf("CMC %d not in target curve", cmc)
	}

	// Calculate how full this slot is
	if totalNonLands == 0 {
		totalNonLands = 1 // Avoid division by zero
	}

	targetCount := int(targetProp * float64(totalNonLands))
	if targetCount < 1 {
		targetCount = 1
	}

	// Calculate fill ratio
	fillRatio := float64(currentCount) / float64(targetCount)

	// Score based on how much room is left
	if fillRatio >= 1.5 {
		// Slot is overfilled
		return 0.2, fmt.Sprintf("CMC %d slot overfilled (%.0f%% of target)", cmc, fillRatio*100)
	} else if fillRatio >= 1.0 {
		// Slot is at or slightly over target
		excess := fillRatio - 1.0
		score := 0.5 - (excess * 0.6) // Linearly decrease from 0.5 to 0.2
		return score, fmt.Sprintf("CMC %d slot at target", cmc)
	} else if fillRatio >= 0.5 {
		// Slot is partially filled - good to add more
		score := 0.7 + ((1.0 - fillRatio) * 0.3)
		return score, fmt.Sprintf("CMC %d slot needs more cards (%.0f%% full)", cmc, fillRatio*100)
	} else {
		// Slot is empty or nearly empty - high priority
		return 1.0, fmt.Sprintf("CMC %d slot underserved (%.0f%% full)", cmc, fillRatio*100)
	}
}

// ScoreManaCurveGaps scores how well a card helps fill gaps in the mana curve.
// This focuses on identifying where the deck needs cards most.
func (cs *CurveScorer) ScoreManaCurveGaps(card *cards.Card, analysis *DeckAnalysis) (float64, string) {
	if card == nil || analysis == nil {
		return 0.5, "insufficient data"
	}

	cmc := int(card.CMC)
	if cmc < 0 {
		cmc = 0
	}

	// Calculate ideal counts based on total non-land cards
	totalNonLands := analysis.TotalCards - analysis.LandCount
	if totalNonLands <= 0 {
		return 0.5, "no non-land cards in deck"
	}

	// Find the most underserved CMC slots
	gaps := cs.findCurveGaps(analysis.ManaCurve, totalNonLands)

	// Group high CMC
	cmcBucket := cmc
	if cmcBucket > cs.MaxCMC {
		cmcBucket = cs.MaxCMC
	}

	// Check if this card fills a gap
	for _, gap := range gaps {
		if gap.CMC == cmcBucket {
			// This card fills a gap!
			return gap.Score, gap.Reason
		}
	}

	// Card doesn't fill a priority gap
	return 0.5, fmt.Sprintf("CMC %d is not a priority gap", cmc)
}

// CurveGap represents an underserved CMC slot.
type CurveGap struct {
	CMC    int
	Score  float64
	Reason string
}

// findCurveGaps identifies the most underserved CMC slots.
func (cs *CurveScorer) findCurveGaps(currentCurve []int, totalNonLands int) []CurveGap {
	gaps := []CurveGap{}

	for cmc, targetProp := range cs.TargetCurve {
		currentCount := 0
		if cmc < len(currentCurve) {
			currentCount = currentCurve[cmc]
		}

		targetCount := int(targetProp * float64(totalNonLands))
		if targetCount < 1 {
			targetCount = 1
		}

		deficit := targetCount - currentCount
		if deficit > 0 {
			// Calculate gap severity
			deficitRatio := float64(deficit) / float64(targetCount)
			score := 0.5 + (deficitRatio * 0.5) // Score 0.5 to 1.0 based on deficit

			gaps = append(gaps, CurveGap{
				CMC:    cmc,
				Score:  score,
				Reason: fmt.Sprintf("CMC %d needs %d more cards", cmc, deficit),
			})
		}
	}

	return gaps
}

// ScoreAverageCMCImpact scores how a card affects the deck's average CMC.
func (cs *CurveScorer) ScoreAverageCMCImpact(card *cards.Card, currentAvgCMC float64, totalNonLands int) (float64, string) {
	if card == nil {
		return 0.5, "no card provided"
	}

	cardCMC := card.CMC
	if totalNonLands == 0 {
		totalNonLands = 1
	}

	// Calculate new average if we add this card
	newAvgCMC := (currentAvgCMC*float64(totalNonLands) + cardCMC) / float64(totalNonLands+1)

	// Calculate how close to target
	currentDiff := math.Abs(currentAvgCMC - cs.TargetAverageCMC)
	newDiff := math.Abs(newAvgCMC - cs.TargetAverageCMC)

	if newDiff < currentDiff {
		// Card brings us closer to target
		improvement := currentDiff - newDiff
		score := 0.7 + math.Min(improvement*0.3, 0.3)
		return score, fmt.Sprintf("improves avg CMC toward %.1f", cs.TargetAverageCMC)
	} else if newDiff > currentDiff {
		// Card takes us further from target
		penalty := newDiff - currentDiff
		score := 0.5 - math.Min(penalty*0.3, 0.3)
		return score, fmt.Sprintf("moves avg CMC away from %.1f", cs.TargetAverageCMC)
	}

	return 0.6, "no significant impact on avg CMC"
}
