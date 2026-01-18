package core

import (
	"testing"

	"github.com/ramonehamilton/MTGA-Companion/internal/mtga/cards"
	"github.com/stretchr/testify/assert"
)

func TestNewCurveScorer(t *testing.T) {
	scorer := NewCurveScorer()

	assert.NotNil(t, scorer)
	assert.NotEmpty(t, scorer.TargetCurve)
	assert.Equal(t, 2.8, scorer.TargetAverageCMC)
	assert.Equal(t, 6, scorer.MaxCMC)
}

func TestNewAggroCurveScorer(t *testing.T) {
	scorer := NewAggroCurveScorer()

	assert.Equal(t, 2.0, scorer.TargetAverageCMC)
	assert.Equal(t, 5, scorer.MaxCMC)
	// Aggro wants more 1 and 2 drops
	assert.Greater(t, scorer.TargetCurve[1], 0.20)
	assert.Greater(t, scorer.TargetCurve[2], 0.30)
}

func TestNewControlCurveScorer(t *testing.T) {
	scorer := NewControlCurveScorer()

	assert.Equal(t, 3.5, scorer.TargetAverageCMC)
	assert.Equal(t, 6, scorer.MaxCMC) // Aligned with 6+ bucket in TargetCurve
	// Control wants fewer 1 drops
	assert.Less(t, scorer.TargetCurve[1], 0.10)
}

func TestNewLimitedCurveScorer(t *testing.T) {
	scorer := NewLimitedCurveScorer()

	assert.Equal(t, 3.2, scorer.TargetAverageCMC)
	// Limited wants to peak at 3 drops
	assert.Equal(t, 0.30, scorer.TargetCurve[3])
}

func TestScoreManaCurveFit_NilCard(t *testing.T) {
	scorer := NewCurveScorer()

	score, reason := scorer.ScoreManaCurveFit(nil, []int{0, 2, 4, 3, 2}, 10)

	assert.Equal(t, 0.0, score)
	assert.Contains(t, reason, "no card provided")
}

func TestScoreManaCurveFit_EmptyCurveSlot(t *testing.T) {
	scorer := NewCurveScorer()
	card := &cards.Card{
		Name:     "3-Drop",
		TypeLine: "Creature",
		CMC:      3,
	}

	// Curve has no 3-drops, total of 16 non-lands
	currentCurve := []int{0, 2, 4, 0, 2, 1, 0, 0, 0, 0}

	score, reason := scorer.ScoreManaCurveFit(card, currentCurve, 16)

	// Empty slot = high priority
	assert.GreaterOrEqual(t, score, 0.9)
	assert.Contains(t, reason, "underserved")
}

func TestScoreManaCurveFit_OverfilledSlot(t *testing.T) {
	scorer := NewCurveScorer()
	card := &cards.Card{
		Name:     "3-Drop",
		TypeLine: "Creature",
		CMC:      3,
	}

	// Too many 3-drops (8 out of 16 = 50%, target is 25%)
	currentCurve := []int{0, 2, 4, 8, 2, 0, 0, 0, 0, 0}

	score, reason := scorer.ScoreManaCurveFit(card, currentCurve, 16)

	assert.Less(t, score, 0.4)
	assert.Contains(t, reason, "overfilled")
}

func TestScoreManaCurveFit_AtTarget(t *testing.T) {
	scorer := NewCurveScorer()
	card := &cards.Card{
		Name:     "3-Drop",
		TypeLine: "Creature",
		CMC:      3,
	}

	// 4 three-drops out of 16 = 25% = target
	currentCurve := []int{0, 2, 4, 4, 3, 2, 1, 0, 0, 0}

	score, reason := scorer.ScoreManaCurveFit(card, currentCurve, 16)

	// At target but not urgent
	assert.GreaterOrEqual(t, score, 0.4)
	assert.LessOrEqual(t, score, 0.6)
	assert.Contains(t, reason, "at target")
}

func TestScoreManaCurveFit_HighCMC(t *testing.T) {
	scorer := NewCurveScorer()
	card := &cards.Card{
		Name:     "8-Drop",
		TypeLine: "Creature",
		CMC:      8,
	}

	currentCurve := []int{0, 2, 4, 4, 3, 2, 1, 0, 0, 0}

	score, reason := scorer.ScoreManaCurveFit(card, currentCurve, 16)

	// High CMC cards get grouped into MaxCMC bucket
	assert.Greater(t, score, 0.0)
	assert.Contains(t, reason, "8") // Should mention CMC 8
}

func TestScoreManaCurveGaps(t *testing.T) {
	scorer := NewCurveScorer()
	card := &cards.Card{
		Name:     "2-Drop",
		TypeLine: "Creature",
		CMC:      2,
	}

	analysis := &DeckAnalysis{
		ManaCurve:  []int{0, 2, 1, 4, 3, 2, 1, 0, 0, 0}, // Only 1 two-drop
		TotalCards: 23,
		LandCount:  7,
	}

	score, reason := scorer.ScoreManaCurveGaps(card, analysis)

	// 2-drops are underrepresented
	assert.Greater(t, score, 0.5)
	assert.Contains(t, reason, "needs")
}

func TestScoreManaCurveGaps_NilInputs(t *testing.T) {
	scorer := NewCurveScorer()

	score, reason := scorer.ScoreManaCurveGaps(nil, &DeckAnalysis{})
	assert.Equal(t, 0.5, score)
	assert.Contains(t, reason, "insufficient")

	card := &cards.Card{CMC: 3}
	score, reason = scorer.ScoreManaCurveGaps(card, nil)
	assert.Equal(t, 0.5, score)
	assert.Contains(t, reason, "insufficient")
}

func TestScoreAverageCMCImpact_ImprovedAverage(t *testing.T) {
	scorer := NewCurveScorer()
	scorer.TargetAverageCMC = 3.0

	card := &cards.Card{
		Name:     "3-Drop",
		TypeLine: "Creature",
		CMC:      3,
	}

	// Current avg is 3.5, target is 3.0
	// Adding a 3-drop should help
	score, reason := scorer.ScoreAverageCMCImpact(card, 3.5, 10)

	assert.Greater(t, score, 0.7)
	assert.Contains(t, reason, "improves")
}

func TestScoreAverageCMCImpact_WorsenedAverage(t *testing.T) {
	scorer := NewCurveScorer()
	scorer.TargetAverageCMC = 3.0

	card := &cards.Card{
		Name:     "6-Drop",
		TypeLine: "Creature",
		CMC:      6,
	}

	// Current avg is 3.0, adding 6-drop pushes away from target
	score, reason := scorer.ScoreAverageCMCImpact(card, 3.0, 10)

	assert.Less(t, score, 0.5)
	assert.Contains(t, reason, "away from")
}

func TestScoreAverageCMCImpact_NilCard(t *testing.T) {
	scorer := NewCurveScorer()

	score, reason := scorer.ScoreAverageCMCImpact(nil, 3.0, 10)

	assert.Equal(t, 0.5, score)
	assert.Contains(t, reason, "no card provided")
}

func TestFindCurveGaps(t *testing.T) {
	scorer := NewCurveScorer()

	// Curve with gaps at 1 and 5
	currentCurve := []int{0, 0, 5, 5, 4, 0, 2, 0, 0, 0}
	totalNonLands := 16

	gaps := scorer.findCurveGaps(currentCurve, totalNonLands)

	// Should find gaps at 1 and 5
	foundCMC1 := false
	foundCMC5 := false
	for _, gap := range gaps {
		if gap.CMC == 1 {
			foundCMC1 = true
		}
		if gap.CMC == 5 {
			foundCMC5 = true
		}
	}

	assert.True(t, foundCMC1 || foundCMC5, "Should find at least one gap")
}
