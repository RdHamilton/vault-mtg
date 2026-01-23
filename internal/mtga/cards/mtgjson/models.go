package mtgjson

// SetFile represents the root structure of an MTGJSON set file.
// Example URL: https://mtgjson.com/api/v5/ECL.json
type SetFile struct {
	Data SetData `json:"data"`
	Meta Meta    `json:"meta"`
}

// Meta contains metadata about the MTGJSON data.
type Meta struct {
	Date    string `json:"date"`
	Version string `json:"version"`
}

// SetData contains all the data for a single set.
type SetData struct {
	BaseSetSize       int            `json:"baseSetSize"`
	Block             string         `json:"block,omitempty"`
	BoosterTypes      []string       `json:"boosterTypes,omitempty"`
	Cards             []Card         `json:"cards"`
	Code              string         `json:"code"`
	IsFoilOnly        bool           `json:"isFoilOnly"`
	IsOnlineOnly      bool           `json:"isOnlineOnly"`
	KeyruneCode       string         `json:"keyruneCode"`
	MCMId             int            `json:"mcmId,omitempty"`
	MCMIdExtras       int            `json:"mcmIdExtras,omitempty"`
	MCMName           string         `json:"mcmName,omitempty"`
	Name              string         `json:"name"`
	ParentCode        string         `json:"parentCode,omitempty"`
	ReleaseDate       string         `json:"releaseDate"`
	TCGPlayerGroupId  int            `json:"tcgplayerGroupId,omitempty"`
	Tokens            []Card         `json:"tokens,omitempty"`
	TotalSetSize      int            `json:"totalSetSize"`
	TranslationsJSON  string         `json:"translationsJson,omitempty"`
	Type              string         `json:"type"`
	SealedProduct     []any          `json:"sealedProduct,omitempty"`
	CardCountByRarity map[string]int `json:"cardCountByRarity,omitempty"`
}

// Card represents a single card from MTGJSON.
type Card struct {
	// Core identifiers
	UUID        string          `json:"uuid"`
	Identifiers CardIdentifiers `json:"identifiers"`

	// Card name and type info
	Name           string   `json:"name"`
	FaceName       string   `json:"faceName,omitempty"`
	Side           string   `json:"side,omitempty"`
	ManaCost       string   `json:"manaCost,omitempty"`
	ManaValue      float64  `json:"manaValue"`
	Type           string   `json:"type"`
	Types          []string `json:"types"`
	Subtypes       []string `json:"subtypes,omitempty"`
	Supertypes     []string `json:"supertypes,omitempty"`
	Colors         []string `json:"colors,omitempty"`
	ColorIdentity  []string `json:"colorIdentity"`
	ColorIndicator []string `json:"colorIndicator,omitempty"`

	// Card text
	Text       string   `json:"text,omitempty"`
	FlavorText string   `json:"flavorText,omitempty"`
	Keywords   []string `json:"keywords,omitempty"`

	// Combat stats
	Power     string `json:"power,omitempty"`
	Toughness string `json:"toughness,omitempty"`
	Loyalty   string `json:"loyalty,omitempty"`
	Defense   string `json:"defense,omitempty"` // For battles
	Hand      string `json:"hand,omitempty"`    // Vanguard
	Life      string `json:"life,omitempty"`    // Vanguard

	// Print details
	Artist        string   `json:"artist,omitempty"`
	ArtistIds     []string `json:"artistIds,omitempty"`
	BorderColor   string   `json:"borderColor"`
	Number        string   `json:"number"`
	Rarity        string   `json:"rarity"`
	SetCode       string   `json:"setCode"`
	Watermark     string   `json:"watermark,omitempty"`
	OriginalText  string   `json:"originalText,omitempty"`
	OriginalType  string   `json:"originalType,omitempty"`
	FrameVersion  string   `json:"frameVersion"`
	FrameEffects  []string `json:"frameEffects,omitempty"`
	Language      string   `json:"language"`
	Layout        string   `json:"layout"`
	SecurityStamp string   `json:"securityStamp,omitempty"`

	// Availability
	Availability     []string `json:"availability"`
	BoosterTypes     []string `json:"boosterTypes,omitempty"`
	IsAlternative    bool     `json:"isAlternative,omitempty"`
	IsFullArt        bool     `json:"isFullArt,omitempty"`
	IsOnlineOnly     bool     `json:"isOnlineOnly,omitempty"`
	IsPromo          bool     `json:"isPromo,omitempty"`
	IsRebalanced     bool     `json:"isRebalanced,omitempty"`
	IsReprint        bool     `json:"isReprint,omitempty"`
	IsReserved       bool     `json:"isReserved,omitempty"`
	IsStarter        bool     `json:"isStarter,omitempty"`
	IsStorySpotlight bool     `json:"isStorySpotlight,omitempty"`
	IsTimeshifted    bool     `json:"isTimeshifted,omitempty"`
	HasFoil          bool     `json:"hasFoil"`
	HasNonFoil       bool     `json:"hasNonFoil"`

	// Legalities
	Legalities Legalities `json:"legalities,omitempty"`

	// Related cards
	OtherFaceIds []string     `json:"otherFaceIds,omitempty"`
	Variations   []string     `json:"variations,omitempty"`
	RelatedCards RelatedCards `json:"relatedCards,omitempty"`

	// Card-specific fields
	Finishes       []string            `json:"finishes,omitempty"`
	PromoTypes     []string            `json:"promoTypes,omitempty"`
	SourceProducts map[string][]string `json:"sourceProducts,omitempty"`
}

// CardIdentifiers contains all external identifiers for a card.
type CardIdentifiers struct {
	CardKingdomEtchedId      string `json:"cardKingdomEtchedId,omitempty"`
	CardKingdomFoilId        string `json:"cardKingdomFoilId,omitempty"`
	CardKingdomId            string `json:"cardKingdomId,omitempty"`
	CardsphereId             string `json:"cardsphereId,omitempty"`
	MCMId                    string `json:"mcmId,omitempty"`
	MCMMetaId                string `json:"mcmMetaId,omitempty"`
	MtgArenaId               string `json:"mtgArenaId,omitempty"`
	MtgoFoilId               string `json:"mtgoFoilId,omitempty"`
	MtgoId                   string `json:"mtgoId,omitempty"`
	MultiverseId             string `json:"multiverseId,omitempty"`
	ScryfallId               string `json:"scryfallId,omitempty"`
	ScryfallOracleId         string `json:"scryfallOracleId,omitempty"`
	ScryfallIllustrationId   string `json:"scryfallIllustrationId,omitempty"`
	TCGPlayerProductId       string `json:"tcgplayerProductId,omitempty"`
	TCGPlayerEtchedProductId string `json:"tcgplayerEtchedProductId,omitempty"`
}

// Legalities represents card legality across various formats.
type Legalities struct {
	Alchemy         string `json:"alchemy,omitempty"`
	Brawl           string `json:"brawl,omitempty"`
	Commander       string `json:"commander,omitempty"`
	Duel            string `json:"duel,omitempty"`
	Explorer        string `json:"explorer,omitempty"`
	Future          string `json:"future,omitempty"`
	Gladiator       string `json:"gladiator,omitempty"`
	Historic        string `json:"historic,omitempty"`
	HistoricBrawl   string `json:"historicbrawl,omitempty"`
	Legacy          string `json:"legacy,omitempty"`
	Modern          string `json:"modern,omitempty"`
	Oathbreaker     string `json:"oathbreaker,omitempty"`
	OldSchool       string `json:"oldschool,omitempty"`
	Pauper          string `json:"pauper,omitempty"`
	PauperCommander string `json:"paupercommander,omitempty"`
	Penny           string `json:"penny,omitempty"`
	Pioneer         string `json:"pioneer,omitempty"`
	Predh           string `json:"predh,omitempty"`
	Premodern       string `json:"premodern,omitempty"`
	Standard        string `json:"standard,omitempty"`
	Timeless        string `json:"timeless,omitempty"`
	Vintage         string `json:"vintage,omitempty"`
}

// RelatedCards contains related card references.
type RelatedCards struct {
	ReverseRelated []string `json:"reverseRelated,omitempty"`
	Spellbook      []string `json:"spellbook,omitempty"`
}

// HasArenaID returns true if the card has an MTG Arena ID.
func (c *Card) HasArenaID() bool {
	return c.Identifiers.MtgArenaId != ""
}

// IsAvailableInArena returns true if the card is available in MTG Arena.
func (c *Card) IsAvailableInArena() bool {
	for _, a := range c.Availability {
		if a == "arena" {
			return true
		}
	}
	return false
}
