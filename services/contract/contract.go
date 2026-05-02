// Package contract defines the shared wire types used for cross-service
// communication between the daemon and the BFF. Both sides depend on this
// module so that events can be serialized and deserialized without type
// assertions or reflection.
package contract

import (
	"encoding/json"
	"time"
)

// DaemonEvent is the envelope type transmitted from the daemon to the BFF.
// The Payload field carries an event-specific JSON object; use one of the
// typed Payload structs (e.g. SyncRatingsPayload) to unmarshal it.
type DaemonEvent struct {
	// Type identifies the event (e.g. "sync:ratings", "sync:card_metadata").
	Type string `json:"type"`

	// AccountID is the MTGA account that generated the event.
	AccountID string `json:"account_id"`

	// SessionID is a UUID that groups events belonging to the same game session.
	SessionID string `json:"session_id"`

	// OccurredAt is the wall-clock time at which the event occurred in the daemon.
	OccurredAt time.Time `json:"occurred_at"`

	// Payload carries the event-specific data as raw JSON so that each side
	// can decode only what it understands without a dependency on all payload types.
	Payload json.RawMessage `json:"payload"`
}

// SyncRatingsPayload is the Payload body for "sync:ratings" events.
// Sent when the daemon finishes a batch of draft rating updates.
type SyncRatingsPayload struct {
	// SetCode is the three-letter MTG set code (e.g. "BLB").
	SetCode string `json:"set_code"`

	// CardsUpdated is the number of cards whose ratings were refreshed.
	CardsUpdated int `json:"cards_updated"`

	// Source identifies the data source (e.g. "17lands").
	Source string `json:"source"`
}

// SyncCardMetadataPayload is the Payload body for "sync:card_metadata" events.
// Sent when the daemon finishes synchronising card-level metadata.
type SyncCardMetadataPayload struct {
	// SetCode is the three-letter MTG set code (e.g. "BLB").
	SetCode string `json:"set_code"`

	// CardsAdded is the number of new cards inserted into the local store.
	CardsAdded int `json:"cards_added"`

	// CardsUpdated is the number of existing cards that were refreshed.
	CardsUpdated int `json:"cards_updated"`
}

// MatchEventPayload is the Payload body for "match:*" events.
// Sent when a match starts, ends, or changes game state.
type MatchEventPayload struct {
	// MatchID is the opaque MTGA match identifier.
	MatchID string `json:"match_id"`

	// Format is the match format (e.g. "Draft", "Constructed").
	Format string `json:"format"`

	// OpponentName is the display name of the opponent (may be empty mid-match).
	OpponentName string `json:"opponent_name,omitempty"`
}

// DraftEventPayload is the Payload body for "draft:*" events.
// Sent when a draft pick is made or a draft session changes state.
type DraftEventPayload struct {
	// DraftID is the MTGA draft session identifier.
	DraftID string `json:"draft_id"`

	// SetCode is the set being drafted.
	SetCode string `json:"set_code"`

	// PackNumber is the current pack (1–3).
	PackNumber int `json:"pack_number"`

	// PickNumber is the current pick within the pack (1–15).
	PickNumber int `json:"pick_number"`
}
