package contract

import (
	"encoding/json"
	"time"
)

// DaemonEvent is the wire type the daemon sends to the BFF /v1/ingest/events endpoint.
type DaemonEvent struct {
	Type       string          `json:"type"`
	AccountID  string          `json:"account_id"`
	EventID    string          `json:"event_id"`
	SessionID  string          `json:"session_id"`
	Sequence   uint64          `json:"sequence"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
}

// SyncRatingsPayload is embedded in a DaemonEvent with Type "sync:ratings".
type SyncRatingsPayload struct {
	SetCode      string `json:"set_code"`
	CardsUpdated int    `json:"cards_updated"`
	Source       string `json:"source"`
}

// SyncCardMetadataPayload is embedded in a DaemonEvent with Type "sync:card_metadata".
type SyncCardMetadataPayload struct {
	SetCode      string `json:"set_code"`
	CardsAdded   int    `json:"cards_added"`
	CardsUpdated int    `json:"cards_updated"`
}

// DraftEventPayload is embedded in a DaemonEvent with Type "draft:pick" or similar.
type DraftEventPayload struct {
	DraftID    string `json:"draft_id"`
	SetCode    string `json:"set_code"`
	PackNumber int    `json:"pack_number"`
	PickNumber int    `json:"pick_number"`
}

// MatchEventPayload is embedded in a DaemonEvent with Type "match:result" or similar.
type MatchEventPayload struct {
	MatchID      string `json:"match_id"`
	Format       string `json:"format"`
	OpponentName string `json:"opponent_name"`
}

// InventoryBooster represents a single booster pack in the player's inventory.
// Arena 2026.58+: on-wire field names are PascalCase (CollationId, SetCode, Count).
type InventoryBooster struct {
	CollationID int    `json:"collation_id"`
	SetCode     string `json:"set_code"`
	Count       int    `json:"count"`
}

// InventoryUpdatedPayload is embedded in a DaemonEvent with Type "inventory.updated".
// It carries the player's current gem/gold/wildcard counts and booster holdings.
type InventoryUpdatedPayload struct {
	Gems               int                `json:"gems"`
	Gold               int                `json:"gold"`
	TotalVaultProgress int                `json:"total_vault_progress"`
	WildCardCommons    int                `json:"wild_card_commons"`
	WildCardUncommons  int                `json:"wild_card_uncommons"`
	WildCardRares      int                `json:"wild_card_rares"`
	WildCardMythics    int                `json:"wild_card_mythics"`
	Boosters           []InventoryBooster `json:"boosters"`
}

// QuestProgressPayload is embedded in a DaemonEvent with Type "quest.progress".
// It carries the state of all active quests from a QuestGetQuests response.
type QuestProgressPayload struct {
	Quests []QuestEntry `json:"quests"`
}

// QuestCompletedPayload is embedded in a DaemonEvent with Type "quest.completed".
// It is emitted when at least one quest in a QuestGetQuests response has
// endingProgress >= goal (i.e. the player has met the quest's completion target).
type QuestCompletedPayload struct {
	QuestID          string `json:"quest_id"`
	QuestName        string `json:"quest_name"`
	Progress         int    `json:"progress"`
	Goal             int    `json:"goal"`
	XPReward         int    `json:"xp_reward"`
	CompletionSource string `json:"completion_source"`
}

// QuestEntry represents a single quest within a QuestProgressPayload.
type QuestEntry struct {
	QuestID   string `json:"quest_id"`
	QuestName string `json:"quest_name"`
	Progress  int    `json:"progress"`
	Goal      int    `json:"goal"`
	CanSwap   bool   `json:"can_swap"`
}

// DeckCard represents a single card slot in a deck — arena grpId plus quantity.
type DeckCard struct {
	ArenaID  int `json:"arena_id"`
	Quantity int `json:"quantity"`
}

// DeckUpdatedPayload is embedded in a DaemonEvent with Type "deck.updated".
// It carries the identity and card list for a single player deck as reported
// by a DeckUpsertDeckV2 log entry.
type DeckUpdatedPayload struct {
	DeckID string     `json:"deck_id"`
	Name   string     `json:"name"`
	Format string     `json:"format"`
	Cards  []DeckCard `json:"cards"`
}

// CollectionCard represents a single card entry in a collection snapshot.
// ArenaID is the MTGA numeric card identifier; Count is the number of copies
// the player owns.
type CollectionCard struct {
	ArenaID int `json:"arena_id"`
	Count   int `json:"count"`
}

// CollectionUpdatedPayload is embedded in a DaemonEvent with Type
// "collection.updated". It carries a full snapshot of the player's collection
// as returned by PlayerInventoryGetPlayerCardsV3. The daemon may compute a
// delta before dispatch; when a delta is sent, Cards contains only the changed
// entries and IsDelta is true.
type CollectionUpdatedPayload struct {
	Cards   []CollectionCard `json:"cards"`
	IsDelta bool             `json:"is_delta"`
}

// MatchResult represents a single result entry from the MTGA resultList.
// Scope distinguishes whether the result applies to a single game or the
// overall match ("MatchScope_Game" / "MatchScope_Match").
type MatchResult struct {
	Scope         string `json:"scope"`
	Result        string `json:"result"`
	WinningTeamID int    `json:"winning_team_id"`
	Reason        string `json:"reason"`
}

// MatchCompletedPayload is embedded in a DaemonEvent with Type
// "match.completed". It is derived from the matchGameRoomStateChangedEvent
// with stateType "MatchGameRoomStateType_MatchCompleted" that Arena emits
// at the end of every match.
//
// WinningTeamID is the teamId of the winning side as reported in the
// MatchScope_Match result entry (0 if indeterminate).
// ResultList carries every result entry from finalMatchResult.resultList.
// OpponentName is the playerName of the opponent as listed in reservedPlayers;
// it is empty when the daemon cannot determine which seat belongs to the local
// player.
// Format is sourced from the eventId field in gameRoomConfig (e.g. "Ladder",
// "QuickDraft_SOS_20260430"); it is empty when absent.
//
// Result, PlayerTeamID, PlayerWins, and OpponentWins are derived when the
// daemon knows the local player's MTGA userId (from a preceding
// player.authenticated event). They are empty/zero when the player cannot
// be identified — the projection worker falls back to WinningTeamID +
// PlayerTeamID in that case.
type MatchCompletedPayload struct {
	MatchID       string        `json:"match_id"`
	WinningTeamID int           `json:"winning_team_id"`
	ResultList    []MatchResult `json:"result_list"`
	Format        string        `json:"format"`
	OpponentName  string        `json:"opponent_name"`
	// Derived fields — populated when the local player's MTGA userId is known.
	Result       string `json:"result"`         // "win" or "loss"; empty when indeterminate
	PlayerTeamID int    `json:"player_team_id"` // 0 when indeterminate
	PlayerWins   int    `json:"player_wins"`
	OpponentWins int    `json:"opponent_wins"`
}

// LifeChangeEntry records a single life-total mutation observed in a game.
type LifeChangeEntry struct {
	TeamID     int `json:"team_id"`
	LifeTotal  int `json:"life_total"`
	Delta      int `json:"delta"`
	TurnNumber int `json:"turn_number"`
}

// GamePlayPayload is embedded in a DaemonEvent with Type "match.game_ended".
// It carries per-game telemetry collected from the GRE session buffer.
//
// Partial indicates the event was emitted before the game was confirmed
// complete — either because the GRE buffer reached its flush threshold or
// because the stale-buffer sweep evicted it.  When Partial is true the BFF
// must set partial=true on the corresponding game_plays row.
type GamePlayPayload struct {
	MatchID       string            `json:"match_id"`
	GameNumber    int               `json:"game_number"`
	WinningTeamID int               `json:"winning_team_id"`
	TurnCount     int               `json:"turn_count"`
	DurationSecs  int               `json:"duration_secs"`
	LifeChanges   []LifeChangeEntry `json:"life_changes"`
	Partial       bool              `json:"partial"`
}
