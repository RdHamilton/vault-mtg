package logreader

import (
	"encoding/json"
	"fmt"

	"github.com/RdHamilton/MTGA-Companion/services/contract"
)

// IsDeckEntry reports whether the log entry is a DeckUpsertDeckV2 response.
// Arena emits deck updates with a top-level "request" key whose value is a
// JSON-encoded string containing a "Summary" object with a "DeckId" field.
//
// Detection heuristic: entry is JSON, has a "request" key whose string value
// parses as JSON containing a non-empty "Summary.DeckId".
func IsDeckEntry(entry *LogEntry) bool {
	if entry == nil || !entry.IsJSON {
		return false
	}
	requestStr, ok := entry.JSON["request"].(string)
	if !ok || requestStr == "" {
		return false
	}
	var req map[string]interface{}
	if err := json.Unmarshal([]byte(requestStr), &req); err != nil {
		return false
	}
	summary, ok := req["Summary"].(map[string]interface{})
	if !ok {
		return false
	}
	deckID, _ := summary["DeckId"].(string)
	return deckID != ""
}

// ParseDeckEntry parses a DeckUpsertDeckV2 log entry into a
// contract.DeckUpdatedPayload. Returns an error if the entry is not a valid
// deck upsert event or cannot be decoded.
func ParseDeckEntry(entry *LogEntry) (*contract.DeckUpdatedPayload, error) {
	if entry == nil || !entry.IsJSON {
		return nil, fmt.Errorf("entry is not JSON")
	}
	requestStr, ok := entry.JSON["request"].(string)
	if !ok || requestStr == "" {
		return nil, fmt.Errorf("entry does not contain a request string")
	}

	var req map[string]interface{}
	if err := json.Unmarshal([]byte(requestStr), &req); err != nil {
		return nil, fmt.Errorf("request field is not valid JSON: %w", err)
	}

	summary, ok := req["Summary"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("request has no Summary object")
	}

	deckID, _ := summary["DeckId"].(string)
	if deckID == "" {
		return nil, fmt.Errorf("Summary.DeckId is missing or empty")
	}

	p := &contract.DeckUpdatedPayload{
		DeckID: deckID,
		Format: "Unknown",
		Cards:  []contract.DeckCard{},
	}

	if name, ok := summary["Name"].(string); ok {
		p.Name = name
	}

	// Format is stored as an Attribute with name "Format".
	if attrs, ok := summary["Attributes"].([]interface{}); ok {
		for _, attr := range attrs {
			attrMap, ok := attr.(map[string]interface{})
			if !ok {
				continue
			}
			attrName, _ := attrMap["name"].(string)
			attrVal, _ := attrMap["value"].(string)
			if attrName == "Format" && attrVal != "" {
				p.Format = attrVal
			}
		}
	}

	// Collect cards from MainDeck (sideboard is intentionally excluded as
	// DeckUpdatedPayload represents the playable 60-card list).
	deckObj, _ := req["Deck"].(map[string]interface{})
	if mainDeck, ok := deckObj["MainDeck"].([]interface{}); ok {
		for _, slot := range mainDeck {
			slotMap, ok := slot.(map[string]interface{})
			if !ok {
				continue
			}
			card := contract.DeckCard{}
			// Arena uses "cardId" (lowercase) in the deck object.
			if v, ok := slotMap["cardId"].(float64); ok {
				card.ArenaID = int(v)
			} else if v, ok := slotMap["CardId"].(float64); ok {
				card.ArenaID = int(v)
			}
			if v, ok := slotMap["quantity"].(float64); ok {
				card.Quantity = int(v)
			} else if v, ok := slotMap["Quantity"].(float64); ok {
				card.Quantity = int(v)
			}
			if card.ArenaID > 0 && card.Quantity > 0 {
				p.Cards = append(p.Cards, card)
			}
		}
	}

	return p, nil
}
