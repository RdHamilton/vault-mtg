// Win-rate prediction for a draft deck. Same heuristic the Wails app
// shipped with: baseline 50% adjusted by deck-average GIHWR, color
// count, mana curve, bomb count, and pairwise synergy. Clamped to
// [30%, 70%].

package prediction

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// PredictionFactors is the per-component breakdown of a prediction.
// PascalCase JSON keys match the SPA's prediction.DeckPrediction +
// prediction.PredictionFactors types (frontend/src/types/models.ts).
type PredictionFactors struct {
	DeckAverageGIHWR  float64            `json:"deck_average_gihwr"`
	ColorAdjustment   float64            `json:"color_adjustment"`
	CurveScore        float64            `json:"curve_score"`
	BombBonus         float64            `json:"bomb_bonus"`
	SynergyScore      float64            `json:"synergy_score"`
	SynergyDetails    *SynergyResult     `json:"synergy_details"`
	BaselineWinRate   float64            `json:"baseline_win_rate"`
	Explanation       string             `json:"explanation"`
	CardBreakdown     map[string]float64 `json:"card_breakdown"`
	ColorDistribution map[string]int     `json:"color_distribution"`
	CurveDistribution map[int]int        `json:"curve_distribution"`
	TotalCards        int                `json:"total_cards"`
	HighPerformers    []string           `json:"high_performers"`
	LowPerformers     []string           `json:"low_performers"`
	ConfidenceLevel   string             `json:"confidence_level"`
}

// DeckPrediction is the wire-shape the SPA's prediction.DeckPrediction
// consumes. Field names are PascalCase to match the Wails-generated SPA
// model (kept stable to avoid SPA changes).
type DeckPrediction struct {
	PredictedWinRate    float64
	PredictedWinRateMin float64
	PredictedWinRateMax float64
	Factors             PredictionFactors
	PredictedAt         time.Time
}

// Card is the minimal per-card input the predictor needs. The daemon
// (PR #17b) will build a []Card from the live draft pool + cached
// 17Lands ratings + cards metadata.
type Card struct {
	Name   string
	CMC    int
	Color  string
	GIHWR  float64
	Rarity string
}

const (
	// Baseline format average win rate (50%).
	baselineWinRate = 0.50

	// GIHWR threshold for a "bomb".
	bombThreshold = 0.60

	// Color combination adjustments.
	twoColorBonus     = 0.02
	threeColorPenalty = -0.01

	// Default confidence interval (±3%).
	confidenceRange = 0.03
)

// PredictWinRate produces a DeckPrediction from a deck. Returns an
// error only when the deck is empty.
func PredictWinRate(cards []Card) (*DeckPrediction, error) {
	if len(cards) == 0 {
		return nil, fmt.Errorf("no cards provided for prediction")
	}

	factors := PredictionFactors{
		BaselineWinRate:   baselineWinRate,
		CardBreakdown:     make(map[string]float64),
		ColorDistribution: make(map[string]int),
		CurveDistribution: make(map[int]int),
		TotalCards:        len(cards),
		HighPerformers:    []string{},
		LowPerformers:     []string{},
	}

	totalGIHWR := 0.0
	bombCount := 0
	for _, c := range cards {
		totalGIHWR += c.GIHWR
		factors.CardBreakdown[c.Name] = c.GIHWR
		factors.ColorDistribution[c.Color]++
		factors.CurveDistribution[c.CMC]++
		if c.GIHWR >= bombThreshold {
			bombCount++
			factors.HighPerformers = append(factors.HighPerformers, c.Name)
		}
	}
	factors.DeckAverageGIHWR = totalGIHWR / float64(len(cards))

	if len(factors.HighPerformers) > 5 {
		factors.HighPerformers = factors.HighPerformers[:5]
	}
	for _, c := range cards {
		if c.GIHWR < 0.48 {
			factors.LowPerformers = append(factors.LowPerformers, c.Name)
			if len(factors.LowPerformers) >= 5 {
				break
			}
		}
	}

	// 2. Color adjustment. Count only "real" colors — colorless cards
	// (Color == "" or "C") sit alongside any number of colored cards
	// without affecting the deck's mana-base complexity, so they don't
	// count toward the colorCount switch below.
	colorCount := 0
	for c := range factors.ColorDistribution {
		if c == "" || c == "C" {
			continue
		}
		colorCount++
	}
	switch {
	case colorCount == 2:
		factors.ColorAdjustment = twoColorBonus
	case colorCount >= 3:
		factors.ColorAdjustment = threeColorPenalty
	}

	// 3. Curve.
	factors.CurveScore = evaluateCurve(factors.CurveDistribution, len(cards))

	// 4. Bomb bonus (+1% per bomb).
	factors.BombBonus = float64(bombCount) * 0.01

	// 5. Synergy.
	syn := CalculateSynergy(ConvertCardsToCardData(cards))
	factors.SynergyScore = syn.OverallScore
	factors.SynergyDetails = syn

	deckQualityDelta := factors.DeckAverageGIHWR - baselineWinRate
	curveBonus := (factors.CurveScore - 0.5) * 0.05
	synergyBonus := (factors.SynergyScore - 0.5) * 0.04

	predicted := baselineWinRate + deckQualityDelta + factors.ColorAdjustment + curveBonus + factors.BombBonus + synergyBonus
	predicted = math.Max(0.30, math.Min(0.70, predicted))

	confidence := confidenceRange
	switch {
	case len(cards) < 30:
		confidence = 0.05
		factors.ConfidenceLevel = "low"
	case len(cards) < 40:
		factors.ConfidenceLevel = "medium"
	default:
		factors.ConfidenceLevel = "high"
	}

	factors.Explanation = generateExplanation(factors, predicted)

	return &DeckPrediction{
		PredictedWinRate:    predicted,
		PredictedWinRateMin: math.Max(0.30, predicted-confidence),
		PredictedWinRateMax: math.Min(0.70, predicted+confidence),
		Factors:             factors,
		PredictedAt:         time.Now(),
	}, nil
}

// evaluateCurve scores the mana curve (0.0–1.0). Ideal curve has most
// cards at 2–4 CMC; penalize extremes.
func evaluateCurve(curve map[int]int, totalCards int) float64 {
	score := 0.5

	total := float64(totalCards)
	oneDrop := float64(curve[1]) / total
	twoDrop := float64(curve[2]) / total
	threeDrop := float64(curve[3]) / total
	fourDrop := float64(curve[4]) / total
	fivePlus := float64(curve[5]+curve[6]+curve[7]+curve[8]+curve[9]) / total

	if twoDrop+threeDrop+fourDrop > 0.55 {
		score += 0.2
	}
	if fivePlus > 0.30 {
		score -= 0.2
	}
	if oneDrop+twoDrop < 0.15 {
		score -= 0.1
	}
	return clamp01(score)
}

// generateExplanation produces a one-sentence summary of the
// prediction.
func generateExplanation(f PredictionFactors, winRate float64) string {
	s := fmt.Sprintf("Predicted %.1f%% win rate based on: ", winRate*100)

	switch {
	case f.DeckAverageGIHWR > 0.53:
		s += "strong card quality, "
	case f.DeckAverageGIHWR < 0.48:
		s += "weak card quality, "
	default:
		s += "average card quality, "
	}

	// Same colorless filter as the adjustment pass above.
	colorCount := 0
	for c := range f.ColorDistribution {
		if c == "" || c == "C" {
			continue
		}
		colorCount++
	}
	switch {
	case colorCount == 2:
		s += "focused 2-color deck, "
	case colorCount >= 3:
		s += "3+ color deck (consistency risk), "
	}

	switch {
	case f.CurveScore > 0.6:
		s += "good mana curve"
	case f.CurveScore < 0.4:
		s += "poor mana curve"
	default:
		s += "acceptable mana curve"
	}

	if len(f.HighPerformers) > 0 {
		s += fmt.Sprintf(", %d premium cards", len(f.HighPerformers))
	}

	if f.SynergyDetails != nil {
		totalSyn := f.SynergyDetails.TribalSynergies + f.SynergyDetails.MechSynergies
		switch {
		case totalSyn > 5:
			s += ", strong synergies"
		case totalSyn > 2:
			s += ", some synergies"
		}
	}

	return s + "."
}

// ToJSON encodes the factors to a JSON string. Same surface the Wails
// app used for persistence.
func (pf *PredictionFactors) ToJSON() (string, error) {
	data, err := json.Marshal(pf)
	if err != nil {
		return "", fmt.Errorf("marshal prediction factors: %w", err)
	}
	return string(data), nil
}

// FromJSON decodes the JSON produced by ToJSON.
func FromJSON(jsonStr string) (*PredictionFactors, error) {
	var f PredictionFactors
	if err := json.Unmarshal([]byte(jsonStr), &f); err != nil {
		return nil, fmt.Errorf("unmarshal prediction factors: %w", err)
	}
	return &f, nil
}

// ConvertCardsToCardData lifts the predictor's minimal Card into the
// richer CardData synergy.go consumes. Types / keywords / oracle text
// are left empty by default — callers can populate them from their
// card lookup when richer detection is desired.
func ConvertCardsToCardData(cards []Card) []CardData {
	out := make([]CardData, len(cards))
	for i, c := range cards {
		out[i] = CardData{
			Name:       c.Name,
			CMC:        c.CMC,
			Color:      c.Color,
			GIHWR:      c.GIHWR,
			Rarity:     c.Rarity,
			Types:      []string{},
			Keywords:   []string{},
			IsCreature: true, // default assumption — Wails app behavior
		}
	}
	return out
}
