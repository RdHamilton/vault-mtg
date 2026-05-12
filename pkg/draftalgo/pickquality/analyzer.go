// Package pickquality grades an individual draft pick relative to the
// other cards in the pack. Restored from
// internal/mtga/draft/pickquality/analyzer.go (deleted in commit
// 783cf66). The original took repository handles; this version takes
// the small draftalgo.{RatingsLookup,CardLookup} interfaces so the
// daemon can satisfy them in-process without dragging in the deleted
// Wails SQLite layer.
package pickquality

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/RdHamilton/MTGA-Companion/pkg/draftalgo"
)

// Alternative represents an alternative card pick with its rating.
// JSON keys stay snake_case to match the SPA's pickquality.Alternative.
type Alternative struct {
	CardID   string  `json:"card_id"`
	CardName string  `json:"card_name"`
	GIHWR    float64 `json:"gihwr"`
	Rank     int     `json:"rank"`
}

// PickQuality represents the quality analysis of a draft pick. Mirrors
// the SPA's pickquality.PickQuality wire shape.
type PickQuality struct {
	Grade           string        `json:"grade"`             // A+, A, B, C, D, F, or N/A
	Rank            int           `json:"rank"`              // 1-indexed position in pack
	PackBestGIHWR   float64       `json:"pack_best_gihwr"`   // GIHWR of the best card in pack
	PickedCardGIHWR float64       `json:"picked_card_gihwr"` // GIHWR of the picked card
	Alternatives    []Alternative `json:"alternatives"`      // Top 5 alternatives
}

// Analyze grades a draft pick. packCardIDs are the Arena card IDs the
// player was offered; pickedCardID is the one they chose. format is the
// draft format (e.g. "PremierDraft") used to look up 17Lands ratings.
//
// Returns an error when the pack is empty or the picked card isn't in
// the pack. If no rating data is available for any card in the pack the
// grade is "N/A" rather than F — distinguishing "we couldn't grade"
// from "you picked the worst card".
func Analyze(
	format string,
	packCardIDs []string,
	pickedCardID string,
	ratings draftalgo.RatingsLookup,
	cards draftalgo.CardLookup,
) (*PickQuality, error) {
	if len(packCardIDs) == 0 {
		return nil, fmt.Errorf("no cards in pack")
	}

	type entry struct {
		cardID string
		gihwr  float64
		name   string
	}

	entries := make([]entry, 0, len(packCardIDs))
	for _, id := range packCardIDs {
		gihwr := 0.0
		if ratings != nil {
			if v, ok := ratings.GIHWR(id, format); ok {
				gihwr = v
			}
		}
		name := ""
		if cards != nil {
			name = cards.CardName(id)
		}
		if name == "" {
			name = "Unknown Card"
		}
		entries = append(entries, entry{cardID: id, gihwr: gihwr, name: name})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].gihwr > entries[j].gihwr
	})

	pickedRank := 0
	pickedGIHWR := 0.0
	for i, e := range entries {
		if e.cardID == pickedCardID {
			pickedRank = i + 1
			pickedGIHWR = e.gihwr
			break
		}
	}
	if pickedRank == 0 {
		return nil, fmt.Errorf("picked card %q not found in pack", pickedCardID)
	}

	// If every card scored 0 we have no rating coverage and can't grade.
	hasRatings := false
	for _, e := range entries {
		if e.gihwr > 0 {
			hasRatings = true
			break
		}
	}

	grade := "N/A"
	if hasRatings {
		grade = calculateGrade(pickedRank, len(entries))
	}

	alternatives := make([]Alternative, 0, 5)
	for i, e := range entries {
		if e.cardID == pickedCardID {
			continue
		}
		if len(alternatives) >= 5 {
			break
		}
		alternatives = append(alternatives, Alternative{
			CardID:   e.cardID,
			CardName: e.name,
			GIHWR:    e.gihwr,
			Rank:     i + 1,
		})
	}

	return &PickQuality{
		Grade:           grade,
		Rank:            pickedRank,
		PackBestGIHWR:   entries[0].gihwr,
		PickedCardGIHWR: pickedGIHWR,
		Alternatives:    alternatives,
	}, nil
}

// calculateGrade — letter grade based on pick rank, same buckets the
// Wails analyzer used.
//
//	A+: rank 1 (best card in pack)
//	A:  rank 2–3
//	B:  rank 4–5
//	C:  rank 6–8
//	D:  rank 9–10
//	F:  rank 11+
func calculateGrade(rank, _ int) string {
	switch {
	case rank == 1:
		return "A+"
	case rank <= 3:
		return "A"
	case rank <= 5:
		return "B"
	case rank <= 8:
		return "C"
	case rank <= 10:
		return "D"
	default:
		return "F"
	}
}

// SerializeAlternatives encodes alternatives as JSON. Same surface the
// original analyzer exposed for callers that persisted the analysis.
func SerializeAlternatives(alternatives []Alternative) (string, error) {
	data, err := json.Marshal(alternatives)
	if err != nil {
		return "", fmt.Errorf("marshal alternatives: %w", err)
	}
	return string(data), nil
}

// DeserializeAlternatives decodes the JSON produced by
// SerializeAlternatives.
func DeserializeAlternatives(jsonStr string) ([]Alternative, error) {
	var alternatives []Alternative
	if err := json.Unmarshal([]byte(jsonStr), &alternatives); err != nil {
		return nil, fmt.Errorf("unmarshal alternatives: %w", err)
	}
	return alternatives, nil
}
