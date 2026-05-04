package logreader

import (
	"encoding/json"
	"fmt"
)

// DraftPackPayload is the parsed payload for a "draft.pack" log entry.
//
// MTGA emits a BotDraft_DraftPack or HumanDraft_Notify message whose top-level
// JSON contains a "draftPack" key.  The nested object holds the cards offered
// to the player and which pick number this is.
//
// Example log line (bot draft):
//
//	{"draftPack":{"PackCards":[12345,67890,11111],"SelfPick":1},"CourseName":"PremierDraft_BLB"}
//
// Example log line (human draft):
//
//	{"draftPack":{"PackCards":[12345,67890],"SelfPick":2},"CourseName":"TradDraft_BLB"}
type DraftPackPayload struct {
	// CourseName is the event identifier, e.g. "PremierDraft_BLB".
	CourseName string `json:"CourseName"`
	// DraftPack holds the offered card list and pick position.
	DraftPack DraftPackDetail `json:"draftPack"`
}

// DraftPackDetail contains the cards in the pack and the current pick index.
type DraftPackDetail struct {
	// PackCards is the list of Arena grpIds available in this pack.
	PackCards []int `json:"PackCards"`
	// SelfPick is the 1-based pick number within the pack.
	SelfPick int `json:"SelfPick"`
}

// DraftPickPayload is the parsed payload for a "draft.pick" log entry.
//
// MTGA emits a BotDraft_DraftPickResp message whose top-level JSON contains
// a "pickedCards" key listing the grpIds the player selected.
//
// Example log line:
//
//	{"pickedCards":[12345],"PackNumber":0,"PickNumber":0,"CourseName":"PremierDraft_BLB"}
type DraftPickPayload struct {
	// CourseName is the event identifier, e.g. "PremierDraft_BLB".
	CourseName string `json:"CourseName"`
	// PickedCards is the list of grpIds the player picked (usually one card).
	PickedCards []int `json:"pickedCards"`
	// PackNumber is the 0-based pack number (0 = pack 1, 1 = pack 2, 2 = pack 3).
	PackNumber int `json:"PackNumber"`
	// PickNumber is the 0-based pick index within the pack.
	PickNumber int `json:"PickNumber"`
}

// ParseDraftPack parses a draft.pack log entry into a DraftPackPayload.
// Returns an error if the entry is not a valid draft pack event.
func ParseDraftPack(entry *LogEntry) (*DraftPackPayload, error) {
	if entry == nil || !entry.IsJSON {
		return nil, fmt.Errorf("entry is not JSON")
	}
	if _, ok := entry.JSON["draftPack"]; !ok {
		return nil, fmt.Errorf("entry does not contain draftPack key")
	}

	raw, err := json.Marshal(entry.JSON)
	if err != nil {
		return nil, fmt.Errorf("re-marshal entry JSON: %w", err)
	}

	var p DraftPackPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("unmarshal DraftPackPayload: %w", err)
	}
	return &p, nil
}

// ParseDraftPick parses a draft.pick log entry into a DraftPickPayload.
// Returns an error if the entry is not a valid draft pick event.
func ParseDraftPick(entry *LogEntry) (*DraftPickPayload, error) {
	if entry == nil || !entry.IsJSON {
		return nil, fmt.Errorf("entry is not JSON")
	}
	if _, ok := entry.JSON["pickedCards"]; !ok {
		return nil, fmt.Errorf("entry does not contain pickedCards key")
	}

	raw, err := json.Marshal(entry.JSON)
	if err != nil {
		return nil, fmt.Errorf("re-marshal entry JSON: %w", err)
	}

	var p DraftPickPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("unmarshal DraftPickPayload: %w", err)
	}
	return &p, nil
}
