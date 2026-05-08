package logreader

import (
	"encoding/json"
	"fmt"

	"github.com/RdHamilton/MTGA-Companion/services/contract"
)

// IsInventoryEntry reports whether the log entry is an inventory update event.
// Arena 2026.58+ wraps inventory data under the "InventoryInfo" key.
func IsInventoryEntry(entry *LogEntry) bool {
	if entry == nil || !entry.IsJSON {
		return false
	}
	_, ok := entry.JSON["InventoryInfo"]
	return ok
}

// ParseInventoryEntry parses a single inventory log entry into a
// contract.InventoryUpdatedPayload. Returns an error if the entry is not a
// valid inventory event or the InventoryInfo object cannot be decoded.
func ParseInventoryEntry(entry *LogEntry) (*contract.InventoryUpdatedPayload, error) {
	if entry == nil || !entry.IsJSON {
		return nil, fmt.Errorf("entry is not JSON")
	}

	raw, ok := entry.JSON["InventoryInfo"]
	if !ok {
		return nil, fmt.Errorf("entry does not contain InventoryInfo key")
	}

	invMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("InventoryInfo is not a JSON object")
	}

	p := &contract.InventoryUpdatedPayload{}

	if v, ok := invMap["Gems"].(float64); ok {
		p.Gems = int(v)
	}
	if v, ok := invMap["Gold"].(float64); ok {
		p.Gold = int(v)
	}
	if v, ok := invMap["TotalVaultProgress"].(float64); ok {
		p.TotalVaultProgress = int(v)
	}

	// Wildcards — PascalCase (Arena 2026.58+).
	if v, ok := invMap["WildCardCommons"].(float64); ok {
		p.WildCardCommons = int(v)
	}
	if v, ok := invMap["WildCardUnCommons"].(float64); ok {
		p.WildCardUncommons = int(v)
	}
	if v, ok := invMap["WildCardRares"].(float64); ok {
		p.WildCardRares = int(v)
	}
	if v, ok := invMap["WildCardMythics"].(float64); ok {
		p.WildCardMythics = int(v)
	}

	// Boosters — PascalCase (Arena 2026.58+): CollationId, SetCode, Count.
	if boosters, ok := invMap["Boosters"].([]interface{}); ok {
		for _, b := range boosters {
			bMap, ok := b.(map[string]interface{})
			if !ok {
				continue
			}
			booster := contract.InventoryBooster{}
			if v, ok := bMap["SetCode"].(string); ok {
				booster.SetCode = v
			}
			if v, ok := bMap["Count"].(float64); ok {
				booster.Count = int(v)
			}
			if v, ok := bMap["CollationId"].(float64); ok {
				booster.CollationID = int(v)
			}
			if booster.SetCode != "" || booster.Count > 0 {
				p.Boosters = append(p.Boosters, booster)
			}
		}
	}

	return p, nil
}

// MarshalInventoryEntry is a convenience helper used in tests to round-trip
// an inventory entry through JSON and back via contract types.
func MarshalInventoryEntry(entry *LogEntry) ([]byte, error) {
	p, err := ParseInventoryEntry(entry)
	if err != nil {
		return nil, err
	}
	return json.Marshal(p)
}
