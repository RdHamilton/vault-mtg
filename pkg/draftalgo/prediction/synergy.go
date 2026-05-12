// Package prediction estimates a draft deck's expected win rate. The
// package has two files:
//
//	synergy.go    — pair-wise card synergy scoring (this file)
//	predictor.go  — composes synergy + card quality + curve into a
//	                final win-rate point estimate
//
// Restored from internal/mtga/draft/prediction/{synergy,predictor}.go
// (deleted in commit 783cf66). The original was self-contained — no
// storage coupling — so the port mostly drops the Wails-era `min` /
// `max` shim functions (Go 1.21+ has them in the language) and lives
// under pkg/draftalgo/prediction/ alongside the rest of pkg/draftalgo.
package prediction

import "strings"

// SynergyType labels the kind of synergy two cards share.
type SynergyType string

const (
	SynergyTribal     SynergyType = "tribal"
	SynergyMechanical SynergyType = "mechanical"
	SynergyColor      SynergyType = "color"
	SynergyArchetype  SynergyType = "archetype"
)

// SynergyScore is one pairwise synergy between two cards.
type SynergyScore struct {
	CardA       string      `json:"card_a"`
	CardB       string      `json:"card_b"`
	SynergyType SynergyType `json:"synergy_type"`
	Score       float64     `json:"score"`  // 0.0 to 1.0
	Reason      string      `json:"reason"` // human-readable explanation
	Weight      float64     `json:"weight"` // contribution to overall synergy
}

// SynergyResult is the full synergy analysis for a deck.
type SynergyResult struct {
	OverallScore    float64        `json:"overall_score"`    // 0.0 to 1.0
	SynergyPairs    []SynergyScore `json:"synergy_pairs"`    // individual pairs
	TribalSynergies int            `json:"tribal_synergies"` // count of tribal pairs
	MechSynergies   int            `json:"mech_synergies"`   // count of mechanical pairs
	ColorSynergies  int            `json:"color_synergies"`  // count of color pairs
	TopSynergies    []string       `json:"top_synergies"`    // top 5 reasons by weight
}

// CardData carries the per-card attributes synergy detection needs.
// Callers in PR #17b will populate this from a combination of the
// daemon's live draft state and the cached BFF card metadata.
type CardData struct {
	Name          string
	CMC           int
	Color         string
	GIHWR         float64
	Rarity        string
	Types         []string // creature types: "Elf", "Goblin", etc.
	Keywords      []string // keywords: "flying", "sacrifice", etc.
	OracleText    string   // full text for pattern matching
	IsCreature    bool
	IsSpell       bool
	IsArtifact    bool
	IsEnchantment bool
}

// tribalLords maps creature types to cards that buff that type. Curated
// by hand in the original Wails app; preserved verbatim here.
var tribalLords = map[string][]string{
	"elf":       {"elvish archdruid", "elvish warmaster", "dwynen", "imperious perfect"},
	"goblin":    {"goblin chieftain", "goblin warchief", "krenko", "goblin trashmaster"},
	"vampire":   {"bloodline keeper", "vampire nocturnus", "captivating vampire"},
	"zombie":    {"lord of the accursed", "death baron", "undead warchief"},
	"human":     {"thalia's lieutenant", "champion of the parish", "mayor of avabruck"},
	"warrior":   {"blood-chin fanatic", "chief of the edge", "blood-chin rager"},
	"merfolk":   {"master of the pearl trident", "lord of atlantis", "merfolk mistbinder"},
	"wizard":    {"naban", "patron wizard", "sage of fables"},
	"knight":    {"knight exemplar", "acclaimed contender", "inspiring veteran"},
	"spirit":    {"drogskol captain", "supreme phantom", "rattlechains"},
	"elemental": {"risen reef", "omnath", "creeping trailblazer"},
	"cleric":    {"righteous valkyrie", "orah", "speaker of the heavens"},
	"rogue":     {"soaring thought-thief", "thieves' guild enforcer", "robber of the rich"},
	"angel":     {"resplendent angel", "lyra dawnbringer", "angel of vitality"},
	"dragon":    {"utvara hellkite", "lathliss", "dragon tempest"},
}

// mechanicalSynergies maps mechanics to keywords/patterns that pair
// well with them. Same curation as the Wails original.
var mechanicalSynergies = map[string][]string{
	"sacrifice":    {"death trigger", "blood artist", "mayhem devil", "cruel celebrant", "creates token", "dies"},
	"tokens":       {"populate", "anthem", "goes wide", "+1/+1 counters", "convoke", "sacrifice"},
	"counters":     {"proliferate", "modular", "evolve", "mentor", "adapt"},
	"graveyard":    {"escape", "flashback", "surveil", "delve", "self-mill", "reanimate"},
	"spells":       {"magecraft", "prowess", "storm", "cantrip", "instant", "sorcery"},
	"lifegain":     {"soul sister", "ajani's pridemate", "heliod", "resplendent angel"},
	"artifacts":    {"affinity", "metalcraft", "improvise", "modular"},
	"enchantments": {"constellation", "aura", "enchantress"},
	"flying":       {"favorable winds", "empyrean eagle", "thunderclap wyvern"},
	"deathtouch":   {"fight", "first strike", "ping", "pinger"},
}

// CalculateSynergy returns the synergy analysis for a deck.
func CalculateSynergy(cards []CardData) *SynergyResult {
	result := &SynergyResult{
		SynergyPairs: []SynergyScore{},
		TopSynergies: []string{},
	}

	if len(cards) < 2 {
		result.OverallScore = 0.5
		return result
	}

	for i := 0; i < len(cards); i++ {
		for j := i + 1; j < len(cards); j++ {
			result.SynergyPairs = append(result.SynergyPairs, findSynergies(cards[i], cards[j])...)
		}
	}

	totalSynergyWeight := 0.0
	for _, s := range result.SynergyPairs {
		switch s.SynergyType {
		case SynergyTribal:
			result.TribalSynergies++
		case SynergyMechanical:
			result.MechSynergies++
		case SynergyColor:
			result.ColorSynergies++
		}
		totalSynergyWeight += s.Weight
	}

	maxPairs := len(cards) * (len(cards) - 1) / 2
	synergyDensity := float64(len(result.SynergyPairs)) / float64(maxPairs)

	// Base 0.5, up to 0.3 from synergy density, up to 0.2 from weight.
	score := 0.5 + synergyDensity*0.3 + min(totalSynergyWeight/float64(len(cards)), 1.0)*0.2
	result.OverallScore = clamp01(score)

	result.TopSynergies = getTopSynergyReasons(result.SynergyPairs, 5)
	return result
}

// findSynergies returns every synergy pair (tribal, mechanical, color)
// between the two cards.
func findSynergies(a, b CardData) []SynergyScore {
	var out []SynergyScore
	if s := checkTribalSynergy(a, b); s != nil {
		out = append(out, *s)
	}
	out = append(out, checkMechanicalSynergies(a, b)...)
	if s := checkColorSynergy(a, b); s != nil {
		out = append(out, *s)
	}
	return out
}

func checkTribalSynergy(a, b CardData) *SynergyScore {
	// Forward: does cardA have a type that cardB (named like a lord) buffs?
	for _, typeA := range a.Types {
		typeLower := strings.ToLower(typeA)
		lords, ok := tribalLords[typeLower]
		if !ok {
			continue
		}
		for _, lord := range lords {
			if containsIgnoreCase(b.Name, lord) {
				return &SynergyScore{
					CardA: a.Name, CardB: b.Name,
					SynergyType: SynergyTribal, Score: 0.8,
					Reason: b.Name + " buffs " + typeA + " creatures like " + a.Name,
					Weight: 0.15,
				}
			}
		}
	}
	// Reverse direction.
	for _, typeB := range b.Types {
		typeLower := strings.ToLower(typeB)
		lords, ok := tribalLords[typeLower]
		if !ok {
			continue
		}
		for _, lord := range lords {
			if containsIgnoreCase(a.Name, lord) {
				return &SynergyScore{
					CardA: a.Name, CardB: b.Name,
					SynergyType: SynergyTribal, Score: 0.8,
					Reason: a.Name + " buffs " + typeB + " creatures like " + b.Name,
					Weight: 0.15,
				}
			}
		}
	}
	// Shared relevant creature type → minor bonus.
	for _, typeA := range a.Types {
		for _, typeB := range b.Types {
			if strings.EqualFold(typeA, typeB) && isRelevantCreatureType(typeA) {
				return &SynergyScore{
					CardA: a.Name, CardB: b.Name,
					SynergyType: SynergyTribal, Score: 0.5,
					Reason: "Both cards are " + typeA + "s",
					Weight: 0.05,
				}
			}
		}
	}
	return nil
}

func checkMechanicalSynergies(a, b CardData) []SynergyScore {
	var out []SynergyScore
	for mechanic, keywords := range mechanicalSynergies {
		if hasMechanic(a, mechanic) {
			for _, kw := range keywords {
				if hasKeyword(b, kw) {
					out = append(out, SynergyScore{
						CardA: a.Name, CardB: b.Name,
						SynergyType: SynergyMechanical, Score: 0.7,
						Reason: a.Name + " (" + mechanic + ") synergizes with " + b.Name,
						Weight: 0.12,
					})
					break
				}
			}
		}
		if hasMechanic(b, mechanic) {
			for _, kw := range keywords {
				if hasKeyword(a, kw) {
					duplicate := false
					for _, existing := range out {
						if existing.CardA == a.Name && existing.CardB == b.Name {
							duplicate = true
							break
						}
					}
					if !duplicate {
						out = append(out, SynergyScore{
							CardA: a.Name, CardB: b.Name,
							SynergyType: SynergyMechanical, Score: 0.7,
							Reason: b.Name + " (" + mechanic + ") synergizes with " + a.Name,
							Weight: 0.12,
						})
					}
					break
				}
			}
		}
	}

	// Magecraft-style spell payoff pairs.
	if a.IsSpell && hasKeyword(b, "magecraft") {
		out = append(out, SynergyScore{
			CardA: a.Name, CardB: b.Name,
			SynergyType: SynergyMechanical, Score: 0.75,
			Reason: a.Name + " triggers " + b.Name + "'s spell payoff",
			Weight: 0.13,
		})
	}
	if b.IsSpell && hasKeyword(a, "magecraft") {
		out = append(out, SynergyScore{
			CardA: a.Name, CardB: b.Name,
			SynergyType: SynergyMechanical, Score: 0.75,
			Reason: b.Name + " triggers " + a.Name + "'s spell payoff",
			Weight: 0.13,
		})
	}
	return out
}

func checkColorSynergy(a, b CardData) *SynergyScore {
	if a.Color != "" && a.Color != "C" && a.Color == b.Color {
		return &SynergyScore{
			CardA: a.Name, CardB: b.Name,
			SynergyType: SynergyColor, Score: 0.4,
			Reason: "Both cards are " + colorName(a.Color) + " (color consistency)",
			Weight: 0.03,
		}
	}
	return nil
}

func hasMechanic(card CardData, mechanic string) bool {
	for _, kw := range card.Keywords {
		if containsIgnoreCase(kw, mechanic) {
			return true
		}
	}
	if containsIgnoreCase(card.OracleText, mechanic) {
		return true
	}
	if mechanic == "spells" && card.IsSpell {
		return true
	}
	return false
}

func hasKeyword(card CardData, keyword string) bool {
	for _, kw := range card.Keywords {
		if containsIgnoreCase(kw, keyword) {
			return true
		}
	}
	if containsIgnoreCase(card.OracleText, keyword) {
		return true
	}
	if keyword == "instant" && card.IsSpell && !card.IsCreature {
		return true
	}
	if keyword == "sorcery" && card.IsSpell && !card.IsCreature {
		return true
	}
	return false
}

var relevantCreatureTypes = map[string]struct{}{
	"elf": {}, "goblin": {}, "vampire": {}, "zombie": {}, "human": {},
	"warrior": {}, "merfolk": {}, "wizard": {}, "knight": {}, "spirit": {},
	"elemental": {}, "cleric": {}, "rogue": {}, "angel": {}, "dragon": {},
	"soldier": {}, "pirate": {}, "dinosaur": {}, "faerie": {}, "beast": {},
	"cat": {}, "bird": {}, "dog": {}, "rat": {}, "sliver": {},
}

func isRelevantCreatureType(t string) bool {
	_, ok := relevantCreatureTypes[strings.ToLower(t)]
	return ok
}

func containsIgnoreCase(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func colorName(abbrev string) string {
	switch abbrev {
	case "W":
		return "White"
	case "U":
		return "Blue"
	case "B":
		return "Black"
	case "R":
		return "Red"
	case "G":
		return "Green"
	case "C":
		return "Colorless"
	}
	return abbrev
}

// getTopSynergyReasons returns up to n top reasons, sorted by Weight
// descending.
func getTopSynergyReasons(synergies []SynergyScore, n int) []string {
	sorted := make([]SynergyScore, len(synergies))
	copy(sorted, synergies)
	// Stable insertion sort by Weight desc; deck sizes are small (≤ 60
	// cards → ≤ 1770 pairs) so this is fine.
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].Weight > sorted[j-1].Weight; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	out := make([]string, 0, n)
	for i := 0; i < len(sorted) && i < n; i++ {
		out = append(out, sorted[i].Reason)
	}
	return out
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
