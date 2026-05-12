package prediction_test

import (
	"strings"
	"testing"

	"github.com/RdHamilton/MTGA-Companion/pkg/draftalgo/prediction"
)

// ─── PredictWinRate ─────────────────────────────────────────────────────────

func TestPredictWinRate_EmptyDeckErrors(t *testing.T) {
	_, err := prediction.PredictWinRate(nil)
	if err == nil {
		t.Fatal("expected error for empty deck")
	}
}

func TestPredictWinRate_BaselineWhenAllNeutral(t *testing.T) {
	// 40 cards, 2-color, GIHWR exactly at 0.50, decent curve.
	deck := make([]prediction.Card, 40)
	colors := []string{"W", "U"}
	for i := range deck {
		deck[i] = prediction.Card{
			Name:  "Card " + string(rune('A'+(i%26))) + string(rune('a'+(i/26))),
			CMC:   2 + i%3, // 2/3/4 drops
			Color: colors[i%2],
			GIHWR: 0.50,
		}
	}
	p, err := prediction.PredictWinRate(deck)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if p.PredictedWinRate < 0.30 || p.PredictedWinRate > 0.70 {
		t.Errorf("WinRate %v out of [0.30, 0.70]", p.PredictedWinRate)
	}
	if p.Factors.ConfidenceLevel != "high" {
		t.Errorf("ConfidenceLevel = %q, want high (40-card deck)", p.Factors.ConfidenceLevel)
	}
}

func TestPredictWinRate_StrongDeckBeatsWeakDeck(t *testing.T) {
	strong := make([]prediction.Card, 40)
	weak := make([]prediction.Card, 40)
	for i := range strong {
		strong[i] = prediction.Card{
			Name:  "Strong",
			CMC:   2 + i%3,
			Color: "W",
			GIHWR: 0.60,
		}
		weak[i] = prediction.Card{
			Name:  "Weak",
			CMC:   2 + i%3,
			Color: "W",
			GIHWR: 0.40,
		}
	}
	sp, err := prediction.PredictWinRate(strong)
	if err != nil {
		t.Fatal(err)
	}
	wp, err := prediction.PredictWinRate(weak)
	if err != nil {
		t.Fatal(err)
	}
	if sp.PredictedWinRate <= wp.PredictedWinRate {
		t.Errorf("strong deck WR (%v) should exceed weak deck WR (%v)",
			sp.PredictedWinRate, wp.PredictedWinRate)
	}
}

func TestPredictWinRate_ClampsToBoundaries(t *testing.T) {
	// Force the prediction up past 0.70 with all bombs.
	deck := make([]prediction.Card, 40)
	for i := range deck {
		deck[i] = prediction.Card{
			Name: "Bomb", CMC: 3, Color: "W", GIHWR: 0.99,
		}
	}
	p, err := prediction.PredictWinRate(deck)
	if err != nil {
		t.Fatal(err)
	}
	if p.PredictedWinRate > 0.70+1e-9 {
		t.Errorf("WinRate %v exceeds upper clamp 0.70", p.PredictedWinRate)
	}
	if p.PredictedWinRateMax > 0.70+1e-9 {
		t.Errorf("WinRateMax %v exceeds upper clamp 0.70", p.PredictedWinRateMax)
	}
}

func TestPredictWinRate_ColorAdjustmentApplied(t *testing.T) {
	mk := func(colors []string) []prediction.Card {
		deck := make([]prediction.Card, 40)
		for i := range deck {
			deck[i] = prediction.Card{
				Name: "X", CMC: 2 + i%3, Color: colors[i%len(colors)], GIHWR: 0.50,
			}
		}
		return deck
	}
	two, _ := prediction.PredictWinRate(mk([]string{"W", "U"}))
	three, _ := prediction.PredictWinRate(mk([]string{"W", "U", "B"}))

	if two.Factors.ColorAdjustment <= 0 {
		t.Errorf("2-color ColorAdjustment = %v, want >0", two.Factors.ColorAdjustment)
	}
	if three.Factors.ColorAdjustment >= 0 {
		t.Errorf("3-color ColorAdjustment = %v, want <0", three.Factors.ColorAdjustment)
	}
}

func TestPredictWinRate_ConfidenceLevelByDeckSize(t *testing.T) {
	mk := func(n int) []prediction.Card {
		deck := make([]prediction.Card, n)
		for i := range deck {
			deck[i] = prediction.Card{Name: "X", CMC: 3, Color: "W", GIHWR: 0.5}
		}
		return deck
	}
	cases := []struct {
		size int
		want string
	}{
		{20, "low"},
		{35, "medium"},
		{42, "high"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			p, _ := prediction.PredictWinRate(mk(c.size))
			if p.Factors.ConfidenceLevel != c.want {
				t.Errorf("size %d: ConfidenceLevel = %q, want %q", c.size, p.Factors.ConfidenceLevel, c.want)
			}
		})
	}
}

func TestFactorsJSONRoundTrip(t *testing.T) {
	deck := []prediction.Card{{Name: "A", CMC: 2, Color: "W", GIHWR: 0.6}}
	p, err := prediction.PredictWinRate(deck)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := p.Factors.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	back, err := prediction.FromJSON(encoded)
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	if back.DeckAverageGIHWR != p.Factors.DeckAverageGIHWR {
		t.Errorf("DeckAverageGIHWR lost in round-trip: %v vs %v",
			back.DeckAverageGIHWR, p.Factors.DeckAverageGIHWR)
	}
}

// ─── CalculateSynergy ───────────────────────────────────────────────────────

func TestCalculateSynergy_TooFewCardsReturnsNeutral(t *testing.T) {
	r := prediction.CalculateSynergy([]prediction.CardData{
		{Name: "Solo"},
	})
	if r.OverallScore != 0.5 {
		t.Errorf("OverallScore = %v, want 0.5 (neutral)", r.OverallScore)
	}
}

func TestCalculateSynergy_ColorSynergyFound(t *testing.T) {
	r := prediction.CalculateSynergy([]prediction.CardData{
		{Name: "A", Color: "W"},
		{Name: "B", Color: "W"},
	})
	if r.ColorSynergies != 1 {
		t.Errorf("ColorSynergies = %d, want 1", r.ColorSynergies)
	}
}

func TestCalculateSynergy_TribalLordBuffsType(t *testing.T) {
	r := prediction.CalculateSynergy([]prediction.CardData{
		{Name: "Elvish Archdruid", Types: []string{"Elf"}},
		{Name: "Forest Walker", Types: []string{"Elf"}},
	})
	if r.TribalSynergies < 1 {
		t.Errorf("TribalSynergies = %d, want ≥1", r.TribalSynergies)
	}
}

func TestCalculateSynergy_MechanicalSacrificePairing(t *testing.T) {
	r := prediction.CalculateSynergy([]prediction.CardData{
		{Name: "Sac Outlet", Keywords: []string{"sacrifice"}, OracleText: "Sacrifice a creature"},
		{Name: "Blood Artist", Keywords: []string{"dies"}, OracleText: "Whenever a creature dies"},
	})
	if r.MechSynergies < 1 {
		t.Errorf("MechSynergies = %d, want ≥1", r.MechSynergies)
	}
	found := false
	for _, reason := range r.TopSynergies {
		if strings.Contains(strings.ToLower(reason), "synergize") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected at least one synergy reason in TopSynergies, got %v", r.TopSynergies)
	}
}

func TestCalculateSynergy_OverallScoreInRange(t *testing.T) {
	cards := make([]prediction.CardData, 40)
	for i := range cards {
		cards[i] = prediction.CardData{Name: "Card", Color: "W"}
	}
	r := prediction.CalculateSynergy(cards)
	if r.OverallScore < 0 || r.OverallScore > 1 {
		t.Errorf("OverallScore = %v, want in [0, 1]", r.OverallScore)
	}
}
