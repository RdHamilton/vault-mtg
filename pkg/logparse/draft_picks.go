package logparse

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DraftPick represents a single pick made during a draft.
type DraftPick struct {
	CourseID       string    // CourseId from the draft event
	PackNumber     int       // Pack number (1, 2, 3)
	PickNumber     int       // Pick number within the pack (1-14 or 1-15)
	AvailableCards []string  // Card IDs available in the pack (Arena 2026.58+: strings)
	SelectedCard   string    // Card ID selected by the player (Arena 2026.58+: string)
	Timestamp      time.Time // Timestamp of the pick
}

// DraftPicks represents all picks for a draft event.
type DraftPicks struct {
	CourseID string
	Picks    []DraftPick
}

// botDraftWrapper is the outer envelope Arena 2026.58+ emits for BotDraft events.
// {"CurrentModule":"BotDraft","Payload":"<JSON-encoded-string>"}
type botDraftWrapper struct {
	CurrentModule string `json:"CurrentModule"`
	Payload       string `json:"Payload"`
}

// botDraftPayload is the inner object decoded from the Payload string.
//
// Fix #1 changes:
//   - Card IDs are now strings (not ints).
//   - CourseName → EventName inside the payload.
//   - SelfPick removed; PickNumber now serves that role.
type botDraftPayload struct {
	EventName   string   `json:"EventName"`
	PackNumber  int      `json:"PackNumber"`
	PickNumber  int      `json:"PickNumber"`
	DraftPack   []string `json:"DraftPack"`
	PickedCards []string `json:"PickedCards"`
}

// ParseDraftPicks extracts individual draft picks from log entries.
//
// It handles two wire formats:
//  1. Legacy humanDraftEvent (Premier/Traditional Draft, pre-2026.58)
//  2. Arena 2026.58+ BotDraft wrapper: {"CurrentModule":"BotDraft","Payload":"<JSON>"}
func ParseDraftPicks(entries []*LogEntry) ([]*DraftPicks, error) {
	var allDraftPicks []*DraftPicks
	picksByCourse := make(map[string]*DraftPicks)

	for _, entry := range entries {
		if !entry.IsJSON {
			continue
		}

		// --- Format A: Arena 2026.58+ BotDraft wrapper ---
		if currentModule, ok := entry.JSON["CurrentModule"].(string); ok && currentModule == "BotDraft" {
			if payloadStr, ok := entry.JSON["Payload"].(string); ok && payloadStr != "" {
				var payload botDraftPayload
				if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
					continue
				}

				courseID := payload.EventName
				if courseID == "" {
					continue
				}

				if picksByCourse[courseID] == nil {
					picksByCourse[courseID] = &DraftPicks{
						CourseID: courseID,
						Picks:    []DraftPick{},
					}
				}

				timestamp := time.Now()
				if entry.Timestamp != "" {
					if t, err := parseLogTimestamp(entry.Timestamp); err == nil {
						timestamp = t
					}
				}

				if payload.PackNumber > 0 && payload.PickNumber > 0 && len(payload.DraftPack) > 0 {
					pick := DraftPick{
						CourseID:       courseID,
						PackNumber:     payload.PackNumber,
						PickNumber:     payload.PickNumber,
						AvailableCards: payload.DraftPack,
						// SelectedCard is populated from the BotDraftDraftPick request entry
						Timestamp: timestamp,
					}
					picksByCourse[courseID].Picks = append(picksByCourse[courseID].Picks, pick)
				}
			}
			continue
		}

		// --- Format B: Legacy humanDraftEvent ---
		if eventData, ok := entry.JSON["humanDraftEvent"]; ok {
			eventMap, ok := eventData.(map[string]interface{})
			if !ok {
				continue
			}

			courseID, _ := eventMap["CourseId"].(string)
			if courseID == "" {
				continue
			}

			if picksByCourse[courseID] == nil {
				picksByCourse[courseID] = &DraftPicks{
					CourseID: courseID,
					Picks:    []DraftPick{},
				}
			}

			timestamp := time.Now()
			if entry.Timestamp != "" {
				if t, err := parseLogTimestamp(entry.Timestamp); err == nil {
					timestamp = t
				}
			}

			packNumber := 0
			if pack, ok := eventMap["SelfPack"].(float64); ok {
				packNumber = int(pack)
			} else if pack, ok := eventMap["selfPack"].(float64); ok {
				packNumber = int(pack)
			}

			// In 2026.58+ SelfPick is gone; PickNumber is the canonical field.
			// Fall back through multiple key spellings for older formats.
			pickNumber := 0
			if pick, ok := eventMap["PickNumber"].(float64); ok {
				pickNumber = int(pick)
			} else if pick, ok := eventMap["SelfPick"].(float64); ok {
				pickNumber = int(pick)
			} else if pick, ok := eventMap["selfPick"].(float64); ok {
				pickNumber = int(pick)
			}

			// Extract pack cards — may be strings (2026.58+) or numbers (legacy).
			var availableCards []string
			if packCards, ok := eventMap["PackCards"].([]interface{}); ok {
				for _, cardData := range packCards {
					availableCards = append(availableCards, toCardIDString(cardData))
				}
			} else if packCards, ok := eventMap["packCards"].([]interface{}); ok {
				for _, cardData := range packCards {
					availableCards = append(availableCards, toCardIDString(cardData))
				}
			}

			// Selected card — may be string or number.
			selectedCard := ""
			for _, key := range []string{"SelectedCard", "selectedCard", "SelectedCardId", "selectedCardId"} {
				if v, ok := eventMap[key]; ok {
					selectedCard = toCardIDString(v)
					if selectedCard != "" {
						break
					}
				}
			}

			if packNumber > 0 && pickNumber > 0 && selectedCard != "" {
				pick := DraftPick{
					CourseID:       courseID,
					PackNumber:     packNumber,
					PickNumber:     pickNumber,
					AvailableCards: availableCards,
					SelectedCard:   selectedCard,
					Timestamp:      timestamp,
				}
				picksByCourse[courseID].Picks = append(picksByCourse[courseID].Picks, pick)
			}
		}
	}

	for _, picks := range picksByCourse {
		if len(picks.Picks) > 0 {
			allDraftPicks = append(allDraftPicks, picks)
		}
	}

	if len(allDraftPicks) == 0 {
		return nil, nil
	}

	return allDraftPicks, nil
}

// toCardIDString converts a JSON-decoded card ID value to a string.
// Arena 2026.58+ uses string IDs; older formats used float64.
func toCardIDString(v interface{}) string {
	switch id := v.(type) {
	case string:
		return id
	case float64:
		return strconv.Itoa(int(id))
	case int:
		return strconv.Itoa(id)
	default:
		return ""
	}
}

// parseLogTimestamp attempts to parse a timestamp from the log entry format.
// MTGA log timestamps are in local time (the user's machine timezone).
// We convert to UTC for consistent storage and comparison with query boundaries.
func parseLogTimestamp(timestampStr string) (time.Time, error) {
	// Format: [UnityCrossThreadLogger]2024-01-15 10:30:45
	// Try to extract the date/time portion
	parts := strings.Fields(timestampStr)
	if len(parts) < 2 {
		return time.Time{}, fmt.Errorf("invalid timestamp format")
	}

	// Try common formats
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.000",
	}

	dateTimeStr := parts[len(parts)-2] + " " + parts[len(parts)-1]
	for _, format := range formats {
		if t, err := time.ParseInLocation(format, dateTimeStr, time.Local); err == nil {
			// Convert local time to UTC for consistent storage and comparison
			// This ensures GetDailyWins/GetWeeklyWins queries (which use UTC boundaries)
			// correctly compare against stored timestamps
			return t.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", timestampStr)
}
