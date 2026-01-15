package cards

// ArenaExclusiveCard represents a card that exists in MTG Arena but is not
// available on Scryfall (either no Arena ID mapping or not in their database).
// This is used as a fallback when all other lookup methods fail.
type ArenaExclusiveCard struct {
	ArenaID  int
	Name     string
	TypeLine string
	SetCode  string
	SetName  string
	// For basic lands, this identifies the color of mana produced
	ProducedMana []string
}

// ArenaExclusiveCards maps Arena IDs to card information for cards that
// cannot be looked up via Scryfall. This includes:
// - Arena-exclusive card variants with unique IDs
// - Cards from sets where Scryfall doesn't have Arena ID mappings (e.g., FIN)
// - Promotional or special event cards
//
// This map is manually maintained as these cards are discovered.
// To add a new card, find the Arena ID from the game logs or deck exports,
// then add an entry with the card's basic information.
var ArenaExclusiveCards = map[int]*ArenaExclusiveCard{
	// Basic Lands - Arena-exclusive variants
	// These are alternate art basic lands that have Arena IDs not tracked by Scryfall
	81181: {
		ArenaID:      81181,
		Name:         "Swamp",
		TypeLine:     "Basic Land — Swamp",
		SetCode:      "JMP",
		SetName:      "Jumpstart",
		ProducedMana: []string{"B"},
	},

	// Final Fantasy (FIN) set cards
	// Scryfall has these cards but without Arena ID mappings
	// The CSV parser doesn't have arena IDs, only the web API does
	96172: {
		ArenaID:  96172,
		Name:     "Starting Town",
		TypeLine: "Land",
		SetCode:  "FIN",
		SetName:  "Final Fantasy",
	},
}

// GetArenaExclusiveCard returns card info for an Arena-exclusive card by Arena ID.
// Returns nil if the card is not in the mapping.
func GetArenaExclusiveCard(arenaID int) *ArenaExclusiveCard {
	return ArenaExclusiveCards[arenaID]
}

// ToCard converts an ArenaExclusiveCard to the standard Card type.
func (aec *ArenaExclusiveCard) ToCard() *Card {
	typeLine := aec.TypeLine
	return &Card{
		ArenaID:       aec.ArenaID,
		Name:          aec.Name,
		TypeLine:      typeLine,
		SetCode:       aec.SetCode,
		SetName:       aec.SetName,
		Colors:        aec.ProducedMana,
		ColorIdentity: aec.ProducedMana,
		Rarity:        "common",
		Layout:        "normal",
	}
}
