package core

import (
	"testing"

	"github.com/ramonehamilton/MTGA-Companion/internal/mtga/cards"
	"github.com/ramonehamilton/MTGA-Companion/internal/mtga/cards/seventeenlands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUnifiedScorer(t *testing.T) {
	scorer := NewUnifiedScorer()

	assert.NotNil(t, scorer)
	assert.NotNil(t, scorer.ColorScorer)
	assert.NotNil(t, scorer.CurveScorer)
	assert.NotNil(t, scorer.QualityScorer)
	assert.NotNil(t, scorer.SynergyScorer)
	assert.Greater(t, scorer.Weights.ColorFit, 0.0)
}

func TestNewLimitedUnifiedScorer(t *testing.T) {
	scorer := NewLimitedUnifiedScorer()

	assert.NotNil(t, scorer)
	// Limited weights prioritize card quality
	assert.Greater(t, scorer.Weights.CardQuality, scorer.Weights.ColorFit)
}

func TestScoreCard_NilInputs(t *testing.T) {
	scorer := NewUnifiedScorer()

	// Nil card
	score := scorer.ScoreCard(nil, &DeckAnalysis{}, nil)
	assert.Equal(t, 0.0, score.Score)
	assert.Equal(t, "insufficient data", score.Reason)

	// Nil analysis
	card := &cards.Card{Name: "Test Card"}
	score = scorer.ScoreCard(card, nil, nil)
	assert.Equal(t, 0.0, score.Score)
	assert.Equal(t, "insufficient data", score.Reason)
}

func TestScoreCard_BasicCard(t *testing.T) {
	scorer := NewUnifiedScorer()
	oracleText := "Flying"

	card := &cards.Card{
		ArenaID:    12345,
		Name:       "Air Elemental",
		TypeLine:   "Creature — Elemental",
		CMC:        5,
		Colors:     []string{"U"},
		Rarity:     "uncommon",
		OracleText: &oracleText,
	}

	analysis := &DeckAnalysis{
		ColorIdentity: []string{"U"},
		ColorCounts:   map[string]int{"U": 10},
		TotalCards:    23,
		LandCount:     7,
		CreatureCount: 12,
		ManaCurve:     []int{0, 2, 4, 3, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		AverageCMC:    3.0,
		CreatureTypes: map[string]int{"Elemental": 3},
		Keywords:      map[string]int{"flying": 2},
		Themes:        []string{},
	}

	score := scorer.ScoreCard(card, analysis, nil)

	require.NotNil(t, score)
	assert.Greater(t, score.Score, 0.0)
	assert.LessOrEqual(t, score.Score, 1.0)
	assert.Equal(t, card, score.Card)

	// Should have high color fit for mono-blue card in mono-blue deck
	assert.GreaterOrEqual(t, score.Factors.ColorFit, 0.8)

	// Should have keyword synergy (flying)
	assert.Greater(t, score.Factors.Synergy, 0.4)
}

func TestScoreCard_WithRatings(t *testing.T) {
	scorer := NewUnifiedScorer()

	card := &cards.Card{
		ArenaID:  12345,
		Name:     "Excellent Card",
		TypeLine: "Creature — Human",
		CMC:      3,
		Colors:   []string{"W"},
		Rarity:   "rare",
	}

	analysis := &DeckAnalysis{
		ColorIdentity: []string{"W"},
		ColorCounts:   map[string]int{"W": 15},
		TotalCards:    23,
		LandCount:     7,
		ManaCurve:     []int{0, 2, 4, 2, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		AverageCMC:    2.8,
	}

	ratings := &RatingData{
		SeventeenLandsRating: &seventeenlands.CardRating{
			GIHWR: 0.60, // 60% win rate
			OHWR:  0.55,
			ATA:   3.0, // Picked early
			ALSA:  4.0,
		},
	}

	score := scorer.ScoreCard(card, analysis, ratings)

	require.NotNil(t, score)
	// With high ratings, quality should be high
	assert.Greater(t, score.Factors.CardQuality, 0.6)
}

func TestScoreCards_Sorting(t *testing.T) {
	scorer := NewUnifiedScorer()

	oracleGood := "Flying, lifelink"
	oracleBad := "Defender"

	cardList := []*cards.Card{
		{
			ArenaID:    1,
			Name:       "Bad Card",
			TypeLine:   "Creature — Wall",
			CMC:        5,
			Colors:     []string{"R"}, // Off-color
			Rarity:     "common",
			OracleText: &oracleBad,
		},
		{
			ArenaID:    2,
			Name:       "Good Card",
			TypeLine:   "Creature — Angel",
			CMC:        3,
			Colors:     []string{"W"}, // On-color
			Rarity:     "rare",
			OracleText: &oracleGood,
		},
	}

	analysis := &DeckAnalysis{
		ColorIdentity: []string{"W"},
		ColorCounts:   map[string]int{"W": 15},
		TotalCards:    23,
		LandCount:     7,
		ManaCurve:     []int{0, 2, 4, 2, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		Keywords:      map[string]int{"flying": 3, "lifelink": 2},
		Themes:        []string{"lifegain"},
	}

	scores := scorer.ScoreCards(cardList, analysis, nil)

	require.Len(t, scores, 2)
	// Good Card should be first (higher score)
	assert.Equal(t, "Good Card", scores[0].Card.Name)
	assert.Equal(t, "Bad Card", scores[1].Card.Name)
	assert.Greater(t, scores[0].Score, scores[1].Score)
}

func TestGetTopRecommendations(t *testing.T) {
	scorer := NewUnifiedScorer()

	cardList := make([]*cards.Card, 10)
	for i := 0; i < 10; i++ {
		cardList[i] = &cards.Card{
			ArenaID:  i,
			Name:     "Test Card",
			TypeLine: "Creature",
			CMC:      float64(i % 5),
			Colors:   []string{"W"},
			Rarity:   "common",
		}
	}

	analysis := &DeckAnalysis{
		ColorIdentity: []string{"W"},
		TotalCards:    23,
		LandCount:     7,
		ManaCurve:     []int{0, 2, 4, 2, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}

	top5 := scorer.GetTopRecommendations(cardList, analysis, nil, 5, 0.0)

	require.LessOrEqual(t, len(top5), 5)
}

func TestDeterminePrimaryFactor(t *testing.T) {
	scorer := NewUnifiedScorer()

	tests := []struct {
		name     string
		factors  ScoreFactors
		expected string
	}{
		{
			name: "color fit highest",
			factors: ScoreFactors{
				ColorFit:    1.0,
				ManaCurve:   0.5,
				CardQuality: 0.5,
				Synergy:     0.5,
			},
			expected: "color-fit",
		},
		{
			name: "quality highest",
			factors: ScoreFactors{
				ColorFit:    0.5,
				ManaCurve:   0.5,
				CardQuality: 1.0,
				Synergy:     0.5,
			},
			expected: "quality",
		},
		{
			name: "synergy highest",
			factors: ScoreFactors{
				ColorFit:    0.5,
				ManaCurve:   0.5,
				CardQuality: 0.5,
				Synergy:     1.0,
			},
			expected: "synergy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scorer.DeterminePrimaryFactor(tt.factors)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateConfidence(t *testing.T) {
	scorer := NewUnifiedScorer()

	// Low scores = low confidence
	lowFactors := ScoreFactors{
		ColorFit:    0.3,
		ManaCurve:   0.3,
		CardQuality: 0.3,
		Synergy:     0.3,
	}
	assert.Less(t, scorer.CalculateConfidence(lowFactors), 0.5)

	// High scores = high confidence
	highFactors := ScoreFactors{
		ColorFit:    0.9,
		ManaCurve:   0.85,
		CardQuality: 0.9,
		Synergy:     0.8,
	}
	assert.Greater(t, scorer.CalculateConfidence(highFactors), 0.7)
}

func TestRatingDataFromSeventeenLands(t *testing.T) {
	rating := &seventeenlands.CardRating{
		Name:  "Test Card",
		GIHWR: 0.55,
		OHWR:  0.52,
		ATA:   5.0,
		ALSA:  6.0,
	}

	data := RatingDataFromSeventeenLands(rating)

	require.NotNil(t, data)
	assert.Equal(t, rating, data.SeventeenLandsRating)
	assert.False(t, data.HasCFBRating)
}

func TestRatingDataWithCFB(t *testing.T) {
	rating := &seventeenlands.CardRating{
		Name:  "Test Card",
		GIHWR: 0.55,
	}

	data := RatingDataWithCFB(rating, 0.75)

	require.NotNil(t, data)
	assert.Equal(t, rating, data.SeventeenLandsRating)
	assert.Equal(t, 0.75, data.CFBLimitedScore)
	assert.True(t, data.HasCFBRating)
}

func TestAnalyzeAndScore(t *testing.T) {
	oracleText := "Flying"

	deckCards := []*cards.Card{
		{ArenaID: 1, Name: "Plains", TypeLine: "Basic Land — Plains", Colors: []string{}},
		{ArenaID: 2, Name: "Flying Creature", TypeLine: "Creature — Bird", Colors: []string{"W"}, CMC: 2, OracleText: &oracleText},
	}

	candidate := &cards.Card{
		ArenaID:    3,
		Name:       "Another Flyer",
		TypeLine:   "Creature — Spirit",
		Colors:     []string{"W"},
		CMC:        3,
		OracleText: &oracleText,
	}

	score := AnalyzeAndScore(deckCards, candidate, nil)

	require.NotNil(t, score)
	assert.Greater(t, score.Score, 0.0)
}

func TestFormatScoreBreakdown(t *testing.T) {
	score := &CardScore{
		Card:  &cards.Card{Name: "Test"},
		Score: 0.75,
		Factors: ScoreFactors{
			ColorFit:    0.8,
			ManaCurve:   0.7,
			CardQuality: 0.75,
			Synergy:     0.65,
		},
	}

	formatted := FormatScoreBreakdown(score)

	assert.Contains(t, formatted, "Score: 0.75")
	assert.Contains(t, formatted, "Color: 0.80")
	assert.Contains(t, formatted, "Curve: 0.70")
	assert.Contains(t, formatted, "Quality: 0.75")
	assert.Contains(t, formatted, "Synergy: 0.65")
}

func TestFormatScoreBreakdown_Nil(t *testing.T) {
	result := FormatScoreBreakdown(nil)
	assert.Equal(t, "no score data", result)
}
