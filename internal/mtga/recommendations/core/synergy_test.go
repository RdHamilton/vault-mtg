package core

import (
	"testing"

	"github.com/ramonehamilton/MTGA-Companion/internal/mtga/cards"
	"github.com/stretchr/testify/assert"
)

func TestNewSynergyScorer(t *testing.T) {
	scorer := NewSynergyScorer()

	assert.NotNil(t, scorer)
	assert.Equal(t, 3, scorer.MinTribalCount)
	assert.Greater(t, scorer.KeywordWeight, 0.0)
}

func TestNewLimitedSynergyScorer(t *testing.T) {
	scorer := NewLimitedSynergyScorer()

	// Limited has higher tribal weight and minimum
	assert.Greater(t, scorer.TribalWeight, scorer.KeywordWeight)
	assert.Equal(t, 4, scorer.MinTribalCount)
}

func TestNewConstructedSynergyScorer(t *testing.T) {
	scorer := NewConstructedSynergyScorer()

	// Constructed emphasizes themes more
	assert.Greater(t, scorer.ThemeWeight, scorer.TribalWeight)
	assert.Equal(t, 6, scorer.MinTribalCount)
}

func TestScoreSynergy_NilInputs(t *testing.T) {
	scorer := NewSynergyScorer()

	score, reason := scorer.ScoreSynergy(nil, &DeckAnalysis{})
	assert.Equal(t, 0.5, score)
	assert.Contains(t, reason, "insufficient")

	card := &cards.Card{Name: "Test"}
	score, reason = scorer.ScoreSynergy(card, nil)
	assert.Equal(t, 0.5, score)
	assert.Contains(t, reason, "insufficient")
}

func TestScoreSynergy_KeywordMatch(t *testing.T) {
	scorer := NewSynergyScorer()
	oracleText := "Flying, lifelink"

	card := &cards.Card{
		Name:       "Flying Creature",
		TypeLine:   "Creature — Angel",
		OracleText: &oracleText,
	}

	analysis := &DeckAnalysis{
		Keywords: map[string]int{
			"flying":   4,
			"lifelink": 2,
		},
		CreatureTypes: map[string]int{},
		Themes:        []string{},
	}

	score, _ := scorer.ScoreSynergy(card, analysis)

	// Should have good synergy due to keyword matches
	assert.Greater(t, score, 0.5)
}

func TestScoreSynergy_TribalMatch(t *testing.T) {
	scorer := NewSynergyScorer()

	card := &cards.Card{
		Name:     "Elf Warrior",
		TypeLine: "Creature — Elf Warrior",
	}

	analysis := &DeckAnalysis{
		Keywords:      map[string]int{},
		CreatureTypes: map[string]int{"Elf": 6, "Warrior": 2},
		Themes:        []string{},
	}

	score, _ := scorer.ScoreSynergy(card, analysis)

	// Elf count >= 3, should have tribal synergy
	assert.Greater(t, score, 0.5)
}

func TestScoreSynergy_ThemeMatch(t *testing.T) {
	scorer := NewSynergyScorer()
	oracleText := "Whenever you gain life, draw a card."

	card := &cards.Card{
		Name:       "Lifegain Payoff",
		TypeLine:   "Enchantment",
		OracleText: &oracleText,
	}

	analysis := &DeckAnalysis{
		Keywords:      map[string]int{},
		CreatureTypes: map[string]int{},
		Themes:        []string{"lifegain"},
	}

	score, _ := scorer.ScoreSynergy(card, analysis)

	// Should match lifegain theme
	assert.Greater(t, score, 0.5)
}

func TestScoreSynergyDetailed(t *testing.T) {
	scorer := NewSynergyScorer()
	oracleText := "Flying"

	card := &cards.Card{
		Name:       "Bird",
		TypeLine:   "Creature — Bird",
		OracleText: &oracleText,
	}

	analysis := &DeckAnalysis{
		Keywords:      map[string]int{"flying": 3},
		CreatureTypes: map[string]int{"Bird": 4},
		Themes:        []string{},
	}

	result := scorer.ScoreSynergyDetailed(card, analysis)

	assert.NotNil(t, result)
	assert.Greater(t, result.KeywordSynergy, 0.0)
	assert.Greater(t, result.TribalSynergy, 0.0)
	assert.Contains(t, result.MatchedKeywords, "flying")
	assert.Contains(t, result.MatchedTypes, "Bird")
}

func TestScoreCardPairSynergy_SharedType(t *testing.T) {
	scorer := NewSynergyScorer()

	card1 := &cards.Card{
		Name:     "Elf Druid",
		TypeLine: "Creature — Elf Druid",
	}

	card2 := &cards.Card{
		Name:     "Elf Warrior",
		TypeLine: "Creature — Elf Warrior",
	}

	score, reason := scorer.ScoreCardPairSynergy(card1, card2)

	// Score is weighted by tribal weight (0.35), so 0.6 * 0.35 = 0.21
	assert.Greater(t, score, 0.2)
	assert.Contains(t, reason, "Elf")
}

func TestScoreCardPairSynergy_SharedKeywords(t *testing.T) {
	scorer := NewSynergyScorer()
	oracleText := "Flying, trample"

	card1 := &cards.Card{
		Name:       "Dragon",
		TypeLine:   "Creature — Dragon",
		OracleText: &oracleText,
	}

	card2 := &cards.Card{
		Name:       "Angel",
		TypeLine:   "Creature — Angel",
		OracleText: &oracleText,
	}

	score, reason := scorer.ScoreCardPairSynergy(card1, card2)

	assert.Greater(t, score, 0.0)
	assert.Contains(t, reason, "keyword")
}

func TestScoreCardPairSynergy_NoSynergy(t *testing.T) {
	scorer := NewSynergyScorer()
	oracle1 := "Defender"
	oracle2 := "Haste"

	card1 := &cards.Card{
		Name:       "Wall",
		TypeLine:   "Creature — Wall",
		OracleText: &oracle1,
	}

	card2 := &cards.Card{
		Name:       "Goblin",
		TypeLine:   "Creature — Goblin",
		OracleText: &oracle2,
	}

	score, reason := scorer.ScoreCardPairSynergy(card1, card2)

	assert.Equal(t, 0.5, score)
	assert.Contains(t, reason, "no direct synergy")
}

func TestScoreCardPairSynergy_NilCards(t *testing.T) {
	scorer := NewSynergyScorer()

	score, reason := scorer.ScoreCardPairSynergy(nil, &cards.Card{Name: "Test"})
	assert.Equal(t, 0.5, score)
	assert.Contains(t, reason, "insufficient")

	score, reason = scorer.ScoreCardPairSynergy(&cards.Card{Name: "Test"}, nil)
	assert.Equal(t, 0.5, score)
	assert.Contains(t, reason, "insufficient")
}

func TestCardMatchesTheme(t *testing.T) {
	scorer := NewSynergyScorer()

	tests := []struct {
		name       string
		oracleText string
		typeLine   string
		theme      string
		expected   bool
	}{
		{
			name:       "lifegain card matches lifegain theme",
			oracleText: "whenever you gain life, draw a card.",
			typeLine:   "Enchantment",
			theme:      "lifegain",
			expected:   true,
		},
		{
			name:       "token creator matches tokens theme",
			oracleText: "create a 1/1 white soldier creature token.",
			typeLine:   "Instant",
			theme:      "tokens",
			expected:   true,
		},
		{
			name:       "counter card matches counters theme",
			oracleText: "put a +1/+1 counter on target creature.",
			typeLine:   "Instant",
			theme:      "counters",
			expected:   true,
		},
		{
			name:       "instant matches spellslinger theme",
			oracleText: "deal 3 damage to any target.",
			typeLine:   "Instant",
			theme:      "spellslinger",
			expected:   true,
		},
		{
			name:       "elf matches tribal-Elf theme",
			oracleText: "tap: add {g}.",
			typeLine:   "Creature — Elf Druid",
			theme:      "tribal-Elf",
			expected:   true,
		},
		{
			name:       "no match",
			oracleText: "deal 3 damage to any target.",
			typeLine:   "Sorcery",
			theme:      "lifegain",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scorer.cardMatchesTheme(tt.oracleText, tt.typeLine, tt.theme)
			assert.Equal(t, tt.expected, result)
		})
	}
}
