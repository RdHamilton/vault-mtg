package logreader

// PlayerProfile contains player identification information.
type PlayerProfile struct {
	ScreenName string `json:"screenName"`
	ClientID   string `json:"clientId"`
}

// PlayerInventory contains player resources and collection information.
type PlayerInventory struct {
	Gems               int            `json:"gems"`
	Gold               int            `json:"gold"`
	TotalVaultProgress int            `json:"totalVaultProgress"`
	WildCardCommons    int            `json:"wcCommon"`
	WildCardUncommons  int            `json:"wcUncommon"`
	WildCardRares      int            `json:"wcRare"`
	WildCardMythics    int            `json:"wcMythic"`
	Boosters           []Booster      `json:"boosters"`
	CustomTokens       map[string]int `json:"customTokens"`
}

// Booster represents a booster pack in the player's inventory.
type Booster struct {
	SetCode     string `json:"setCode"`
	Count       int    `json:"count"`
	CollationID int    `json:"collationId"`
}

// PlayerRank contains player ranking information for both constructed and limited formats.
type PlayerRank struct {
	// Constructed format
	ConstructedSeasonOrdinal int     `json:"constructedSeasonOrdinal"`
	ConstructedClass         string  `json:"constructedClass"`
	ConstructedLevel         int     `json:"constructedLevel"`
	ConstructedPercentile    float64 `json:"constructedPercentile"`
	ConstructedStep          int     `json:"constructedStep"`

	// Limited format
	LimitedSeasonOrdinal int     `json:"limitedSeasonOrdinal"`
	LimitedClass         string  `json:"limitedClass"`
	LimitedLevel         int     `json:"limitedLevel"`
	LimitedPercentile    float64 `json:"limitedPercentile"`
	LimitedStep          int     `json:"limitedStep"`

	// Match statistics
	LimitedMatchesWon  int `json:"limitedMatchesWon"`
	LimitedMatchesLost int `json:"limitedMatchesLost"`
}

// DraftHistory contains a history of draft/limited events.
// NOTE: This struct is used for player-level draft history, not for
// in-session draft pack/pick events.  For pack and pick parsing see
// DraftPackPayload and DraftPickPayload in draft_pick.go.
type DraftHistory struct {
	Drafts []DraftEvent `json:"Drafts"`
}

// DraftEvent represents a single draft or limited event in the player's history.
type DraftEvent struct {
	// EventID is the CourseId emitted by MTGA, e.g. "e3f9a1b2-...".
	EventID string `json:"CourseId"`
	// EventName is the internal event name, e.g. "PremierDraft_BLB".
	EventName string `json:"InternalEventName"`
	// Status is the current module/phase, e.g. "Draft", "DeckBuild", "CreateMatch".
	Status string `json:"ModuleInstanceData"`
	// Wins is the number of wins accumulated in this event.
	Wins int `json:"CurrentWins"`
	// Losses is the number of losses accumulated in this event.
	Losses int `json:"CurrentLosses"`
	// Deck is the deck built during this event.
	Deck DraftDeck `json:"CourseDeck"`
}

// DraftDeck represents the deck built during a draft.
type DraftDeck struct {
	Name     string     `json:"name"`
	MainDeck []DeckCard `json:"mainDeck"`
}

// DeckCard represents a card in a deck with its quantity.
type DeckCard struct {
	// CardID is the Arena grpId for the card.
	CardID   int `json:"id"`
	Quantity int `json:"quantity"`
}

// ArenaStats contains gameplay statistics from the log session.
type ArenaStats struct {
	TotalMatches   int                     `json:"totalMatches"`
	MatchWins      int                     `json:"matchWins"`
	MatchLosses    int                     `json:"matchLosses"`
	TotalGames     int                     `json:"totalGames"`
	GameWins       int                     `json:"gameWins"`
	GameLosses     int                     `json:"gameLosses"`
	FormatStats    map[string]*FormatStats `json:"formatStats"`
	UniqueMatchIDs int                     `json:"uniqueMatchIds"`
}

// FormatStats contains statistics for a specific format/event type.
type FormatStats struct {
	EventName     string `json:"eventName"`
	MatchesPlayed int    `json:"matchesPlayed"`
	MatchWins     int    `json:"matchWins"`
	MatchLosses   int    `json:"matchLosses"`
	GamesPlayed   int    `json:"gamesPlayed"`
	GameWins      int    `json:"gameWins"`
	GameLosses    int    `json:"gameLosses"`
}

// PeriodicRewards contains daily and weekly reward progress.
type PeriodicRewards struct {
	DailyWins  int `json:"dailyWins"`
	WeeklyWins int `json:"weeklyWins"`
}

// MasteryPass contains mastery pass progression information.
type MasteryPass struct {
	CurrentLevel int    `json:"currentLevel"`
	PassType     string `json:"passType"`
	MaxLevel     int    `json:"maxLevel"`
}
