package core

import (
	"testing"

	"github.com/ramonehamilton/MTGA-Companion/internal/mtga/cards"
	"github.com/stretchr/testify/assert"
)

func TestNewColorScorer(t *testing.T) {
	scorer := NewColorScorer()

	assert.NotNil(t, scorer)
	assert.True(t, scorer.AllowSplash)
	assert.Equal(t, 0.5, scorer.SplashPenalty)
}

func TestScoreColorFit_Colorless(t *testing.T) {
	scorer := NewColorScorer()
	card := &cards.Card{
		Name:   "Sol Ring",
		Colors: []string{},
	}

	score, reason := scorer.ScoreColorFit(card, []string{"W", "U"})

	assert.Equal(t, 1.0, score)
	assert.Contains(t, reason, "colorless")
}

func TestScoreColorFit_PerfectMatch(t *testing.T) {
	scorer := NewColorScorer()
	card := &cards.Card{
		Name:   "Lightning Bolt",
		Colors: []string{"R"},
	}

	score, reason := scorer.ScoreColorFit(card, []string{"R", "W"})

	assert.Equal(t, 1.0, score)
	assert.Contains(t, reason, "perfect color match")
}

func TestScoreColorFit_PartialMatch(t *testing.T) {
	scorer := NewColorScorer()
	card := &cards.Card{
		Name:   "Multicolor Card",
		Colors: []string{"R", "U"},
	}

	score, _ := scorer.ScoreColorFit(card, []string{"R"})

	// Partial match - one of two colors
	assert.Greater(t, score, 0.0)
	assert.Less(t, score, 1.0)
}

func TestScoreColorFit_NoMatch(t *testing.T) {
	scorer := NewColorScorer()
	scorer.AllowSplash = false

	card := &cards.Card{
		Name:   "Off-Color Card",
		Colors: []string{"G"},
	}

	score, reason := scorer.ScoreColorFit(card, []string{"R", "W"})

	assert.Equal(t, 0.0, score)
	assert.Contains(t, reason, "off-color")
}

func TestScoreColorFit_NoMatchWithSplash(t *testing.T) {
	scorer := NewColorScorer()
	scorer.AllowSplash = true
	scorer.SplashPenalty = 0.5

	card := &cards.Card{
		Name:   "Off-Color Card",
		Colors: []string{"G"},
	}

	score, reason := scorer.ScoreColorFit(card, []string{"R", "W"})

	assert.Greater(t, score, 0.0)
	assert.Less(t, score, 0.5)
	assert.Contains(t, reason, "splash")
}

func TestScoreColorFit_NilCard(t *testing.T) {
	scorer := NewColorScorer()

	score, reason := scorer.ScoreColorFit(nil, []string{"W"})

	assert.Equal(t, 0.0, score)
	assert.Contains(t, reason, "no card provided")
}

func TestScoreColorFit_NoDeckColors(t *testing.T) {
	scorer := NewColorScorer()
	card := &cards.Card{
		Name:   "Test Card",
		Colors: []string{"W"},
	}

	score, reason := scorer.ScoreColorFit(card, []string{})

	assert.Equal(t, 0.5, score)
	assert.Contains(t, reason, "no color identity")
}

func TestScoreColorCompatibility_ExactMatch(t *testing.T) {
	scorer := NewColorScorer()
	card := &cards.Card{
		Name:   "Azorius Card",
		Colors: []string{"W", "U"},
	}

	score, reason := scorer.ScoreColorCompatibility(card, []string{"W", "U"})

	assert.Equal(t, 1.0, score)
	assert.Contains(t, reason, "exact color match")
}

func TestScoreColorCompatibility_Subset(t *testing.T) {
	scorer := NewColorScorer()
	card := &cards.Card{
		Name:   "Mono-White Card",
		Colors: []string{"W"},
	}

	score, reason := scorer.ScoreColorCompatibility(card, []string{"W", "U", "B"})

	assert.Equal(t, 0.9, score)
	assert.Contains(t, reason, "within target identity")
}

func TestScoreColorCompatibility_Colorless(t *testing.T) {
	scorer := NewColorScorer()
	card := &cards.Card{
		Name:   "Artifact",
		Colors: []string{},
	}

	score, reason := scorer.ScoreColorCompatibility(card, []string{"W", "U"})

	assert.Equal(t, 0.6, score)
	assert.Contains(t, reason, "colorless")
}

func TestScoreColorCompatibility_NoOverlap(t *testing.T) {
	scorer := NewColorScorer()
	card := &cards.Card{
		Name:   "Green Card",
		Colors: []string{"G"},
	}

	score, reason := scorer.ScoreColorCompatibility(card, []string{"W", "U"})

	assert.Equal(t, 0.2, score)
	assert.Contains(t, reason, "no color overlap")
}

func TestGetDeckColorIdentity(t *testing.T) {
	deckCards := []*cards.Card{
		{Name: "Card1", Colors: []string{"W"}, ColorIdentity: []string{"W"}},
		{Name: "Card2", Colors: []string{"W"}, ColorIdentity: []string{"W"}},
		{Name: "Card3", Colors: []string{"U"}, ColorIdentity: []string{"U"}},
		{Name: "Card4", Colors: []string{}, ColorIdentity: []string{}}, // Colorless
	}

	identity := GetDeckColorIdentity(deckCards)

	assert.Contains(t, identity, "W")
	assert.Contains(t, identity, "U")
	// White should be first (more common)
	assert.Equal(t, "W", identity[0])
}

func TestGetDeckColorIdentity_Empty(t *testing.T) {
	identity := GetDeckColorIdentity([]*cards.Card{})
	assert.Empty(t, identity)
}

func TestGetDeckColorIdentity_NilCards(t *testing.T) {
	deckCards := []*cards.Card{nil, nil}
	identity := GetDeckColorIdentity(deckCards)
	assert.Empty(t, identity)
}
