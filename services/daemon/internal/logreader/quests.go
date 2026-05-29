package logreader

import (
	"fmt"

	"github.com/RdHamilton/vault-mtg/services/contract"
)

// isQuestResponse reports whether an entry is a QuestGetQuests response.
// The response always contains a top-level "quests" key. When quests exist the
// response also carries a "canSwap" key; an empty response still has "quests"
// (as an empty array) and at least one sibling key we can rely on.
func isQuestResponse(entry *LogEntry) bool {
	if entry == nil || !entry.IsJSON {
		return false
	}
	_, hasQuests := entry.JSON["quests"]
	_, hasCanSwap := entry.JSON["canSwap"]
	if !hasQuests {
		return false
	}
	// Either canSwap is present (non-empty response) or the quests array is
	// explicitly empty (still a valid QuestGetQuests response).
	if hasCanSwap {
		return true
	}
	if qa, ok := entry.JSON["quests"].([]interface{}); ok && len(qa) == 0 {
		return true
	}
	return false
}

// IsQuestProgressEntry reports whether the log entry is a QuestGetQuests
// response that should be emitted as a "quest.progress" event.
func IsQuestProgressEntry(entry *LogEntry) bool {
	return isQuestResponse(entry)
}

// IsQuestCompletedEntry reports whether the log entry is a QuestGetQuests
// response in which at least one quest has met its completion target
// (endingProgress >= goal).
func IsQuestCompletedEntry(entry *LogEntry) bool {
	if !isQuestResponse(entry) {
		return false
	}
	qa, ok := entry.JSON["quests"].([]interface{})
	if !ok {
		return false
	}
	for _, q := range qa {
		qm, ok := q.(map[string]interface{})
		if !ok {
			continue
		}
		goal, hasGoal := qm["goal"].(float64)
		ending, hasEnding := qm["endingProgress"].(float64)
		if hasGoal && hasEnding && goal > 0 && ending >= goal {
			return true
		}
	}
	return false
}

// ParseQuestProgressEntry parses a QuestGetQuests log entry into a
// QuestProgressPayload. Returns an error if the entry is not a valid quest
// response.
func ParseQuestProgressEntry(entry *LogEntry) (*contract.QuestProgressPayload, error) {
	if entry == nil || !entry.IsJSON {
		return nil, fmt.Errorf("entry is not JSON")
	}
	if !isQuestResponse(entry) {
		return nil, fmt.Errorf("entry is not a QuestGetQuests response")
	}

	p := &contract.QuestProgressPayload{}

	qa, ok := entry.JSON["quests"].([]interface{})
	if !ok {
		return p, nil
	}

	for _, q := range qa {
		qm, ok := q.(map[string]interface{})
		if !ok {
			continue
		}
		qe := questEntryFromMap(qm)
		if qe == nil {
			continue
		}
		p.Quests = append(p.Quests, *qe)
	}

	return p, nil
}

// ParseQuestCompletedEntry parses a QuestGetQuests log entry and returns a
// QuestCompletedPayload for the first completed quest found. Returns an error
// if the entry contains no completed quest.
func ParseQuestCompletedEntry(entry *LogEntry) (*contract.QuestCompletedPayload, error) {
	if entry == nil || !entry.IsJSON {
		return nil, fmt.Errorf("entry is not JSON")
	}
	if !isQuestResponse(entry) {
		return nil, fmt.Errorf("entry is not a QuestGetQuests response")
	}

	qa, ok := entry.JSON["quests"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("entry has no quests array")
	}

	for _, q := range qa {
		qm, ok := q.(map[string]interface{})
		if !ok {
			continue
		}
		goal, hasGoal := qm["goal"].(float64)
		ending, hasEnding := qm["endingProgress"].(float64)
		if !hasGoal || !hasEnding || goal <= 0 || ending < goal {
			continue
		}

		questID, _ := qm["questId"].(string)
		questName := questNameFromMap(qm)

		xp := 0
		if cd, ok := qm["chestDescription"].(map[string]interface{}); ok {
			if v, ok := cd["quantity"].(float64); ok {
				xp = int(v)
			}
		}

		return &contract.QuestCompletedPayload{
			QuestID:          questID,
			QuestName:        questName,
			Progress:         int(ending),
			Goal:             int(goal),
			XPReward:         xp,
			CompletionSource: "progress",
		}, nil
	}

	return nil, fmt.Errorf("no completed quest found in entry")
}

// questEntryFromMap converts a raw quest JSON map to a QuestEntry.
// Returns nil if the map has no questId.
func questEntryFromMap(qm map[string]interface{}) *contract.QuestEntry {
	questID, ok := qm["questId"].(string)
	if !ok || questID == "" {
		return nil
	}

	progress := 0
	if v, ok := qm["endingProgress"].(float64); ok {
		progress = int(v)
	}

	goal := 0
	if v, ok := qm["goal"].(float64); ok {
		goal = int(v)
	}

	canSwap := true
	if v, ok := qm["canSwap"].(bool); ok {
		canSwap = v
	}

	return &contract.QuestEntry{
		QuestID:   questID,
		QuestName: questNameFromMap(qm),
		Progress:  progress,
		Goal:      goal,
		CanSwap:   canSwap,
	}
}

// questNameFromMap extracts the human-readable quest name from the JSON map.
// MTGA provides either a "locKey" or a "questTrack" string; both are resolved
// through the static display-name map in quest_names.go. Unknown keys fall
// back to the raw string value with a one-time warning log.
func questNameFromMap(qm map[string]interface{}) string {
	if v, ok := qm["locKey"].(string); ok && v != "" {
		return resolveQuestDisplayName(v)
	}
	if v, ok := qm["questTrack"].(string); ok && v != "" {
		return resolveQuestDisplayName(v)
	}
	return ""
}
