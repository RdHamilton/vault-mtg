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
