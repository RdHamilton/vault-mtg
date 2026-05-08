package logreader

import (
	"fmt"
	"strconv"

	"github.com/RdHamilton/MTGA-Companion/services/contract"
)

// IsCollectionEntry reports whether the log entry is a
// PlayerInventoryGetPlayerCardsV3 response. Arena emits these as a flat JSON
// object whose keys are string-encoded arena IDs and whose values are integer
// copy counts (e.g. {"12345": 4, "67890": 2}).
//
// Detection heuristic: the entry is JSON, has no recognisable wrapper keys
// (InventoryInfo, quests, draftPack, etc.), and every key can be parsed as a
// positive integer — indicating a card-ID map rather than a named-field object.
// An empty object ({}) is accepted as a valid (empty) collection snapshot.
func IsCollectionEntry(entry *LogEntry) bool {
	if entry == nil || !entry.IsJSON {
		return false
	}

	m := entry.JSON
	if len(m) == 0 {
		// An explicit empty-object response counts as a collection snapshot.
		return true
	}

	// Reject entries that contain well-known named wrapper keys used by other
	// classifiers. This prevents misclassifying unrecognised responses that
	// happen to have some integer-like keys mixed with named ones.
	knownKeys := []string{
		"InventoryInfo", "quests", "canSwap",
		"draftPack", "pickedCards",
		"toSceneName", "fromSceneName",
		"CurrentEventState", "authenticateResponse", "rankClass",
	}
	for _, k := range knownKeys {
		if _, has := m[k]; has {
			return false
		}
	}

	// Every key must be a parseable positive integer (arena card ID).
	for k := range m {
		n, err := strconv.Atoi(k)
		if err != nil || n <= 0 {
			return false
		}
	}
	return true
}

// ParseCollectionEntry parses a PlayerInventoryGetPlayerCardsV3 log entry into
// a contract.CollectionUpdatedPayload. Returns an error if the entry is not a
// valid collection snapshot.
func ParseCollectionEntry(entry *LogEntry) (*contract.CollectionUpdatedPayload, error) {
	if entry == nil || !entry.IsJSON {
		return nil, fmt.Errorf("entry is not JSON")
	}
	if !IsCollectionEntry(entry) {
		return nil, fmt.Errorf("entry is not a collection snapshot")
	}

	p := &contract.CollectionUpdatedPayload{
		Cards:   make([]contract.CollectionCard, 0, len(entry.JSON)),
		IsDelta: false,
	}

	for k, v := range entry.JSON {
		arenaID, err := strconv.Atoi(k)
		if err != nil || arenaID <= 0 {
			continue
		}
		count, ok := v.(float64)
		if !ok {
			continue
		}
		p.Cards = append(p.Cards, contract.CollectionCard{
			ArenaID: arenaID,
			Count:   int(count),
		})
	}

	return p, nil
}
