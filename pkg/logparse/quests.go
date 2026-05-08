package logparse

import (
	"fmt"
	"log"
	"time"
)

// QuestData represents a quest parsed from MTGA logs.
type QuestData struct {
	QuestID          string
	QuestType        string
	Goal             int
	StartingProgress int
	EndingProgress   int
	CanSwap          bool
	Rewards          string // ChestDescription as JSON string
	AssignedAt       time.Time
	CompletedAt      *time.Time
	LastSeenAt       *time.Time // Tracks when quest was last seen in QuestGetQuests response
	Completed        bool
	Rerolled         bool
	CompletionSource string // How completion was detected: "disappeared", "progress", or "" (not completed)
}

// ParseQuestsResult contains the results of parsing quests from log entries.
type ParseQuestsResult struct {
	Quests             []*QuestData    // All parsed quests
	HasQuestResponse   bool            // Whether we found any QuestGetQuests responses
	CurrentQuestIDs    map[string]bool // Quest IDs present in the most recent QuestGetQuests response
	LatestResponseTime time.Time       // Timestamp of the most recent QuestGetQuests response
}

// ParseQuests extracts quest data from log entries.
func ParseQuests(entries []*LogEntry) ([]*QuestData, error) {
	var quests []*QuestData
	questMap := make(map[string]*QuestData)

	questsFound := 0
	responsesFound := 0

	for _, entry := range entries {
		if !entry.IsJSON {
			continue
		}

		logTimestamp := time.Now()
		if entry.Timestamp != "" {
			if parsedTime, err := parseLogTimestamp(entry.Timestamp); err == nil {
				logTimestamp = parsedTime
			}
		}

		if questsData, ok := entry.JSON["quests"]; ok {
			isEmptyQuests := false
			if questArray, ok := questsData.([]interface{}); ok && len(questArray) == 0 {
				isEmptyQuests = true
			}
			_, hasCanSwap := entry.JSON["canSwap"]
			if hasCanSwap || isEmptyQuests {
				responsesFound++

				currentQuestIDs := make(map[string]bool)
				now := time.Now()

				if questArray, ok := questsData.([]interface{}); ok {
					for _, q := range questArray {
						if questJSON, ok := q.(map[string]interface{}); ok {
							quest := parseQuestFromMap(questJSON, logTimestamp)
							if quest != nil {
								currentQuestIDs[quest.QuestID] = true

								if existing, exists := questMap[quest.QuestID]; exists {
									if isQuestRerolled(existing, quest) {
										existing.Rerolled = true
										log.Printf("Quest parser: Quest %s rerolled - type changed from '%s' to '%s'",
											quest.QuestID, existing.QuestType, quest.QuestType)

										quest.LastSeenAt = &now
										newQuestKey := quest.QuestID + "_rerolled_" + logTimestamp.Format("20060102150405")
										questMap[newQuestKey] = quest
										questsFound++
									} else {
										existing.EndingProgress = quest.EndingProgress
										existing.CanSwap = quest.CanSwap
										existing.LastSeenAt = &now
									}
								} else {
									quest.LastSeenAt = &now
									questMap[quest.QuestID] = quest
									questsFound++
								}
							}
						}
					}
				}

				for _, quest := range questMap {
					if !quest.Completed && !quest.Rerolled && !currentQuestIDs[quest.QuestID] {
						quest.Completed = true
						completedAt := logTimestamp
						quest.CompletedAt = &completedAt
						quest.EndingProgress = quest.Goal
						quest.CompletionSource = "disappeared"
						log.Printf("Quest parser: Quest %s completed (disappeared from response)", quest.QuestID)
					}
				}
			}
		}

		if newQuestsData, ok := entry.JSON["newQuests"]; ok {
			if questArray, ok := newQuestsData.([]interface{}); ok {
				now := time.Now()
				for _, q := range questArray {
					if questJSON, ok := q.(map[string]interface{}); ok {
						quest := parseQuestFromMap(questJSON, logTimestamp)
						if quest != nil {
							quest.LastSeenAt = &now

							if _, exists := questMap[quest.QuestID]; !exists {
								questMap[quest.QuestID] = quest
								questsFound++
							}
						}
					}
				}
			}
		}
	}

	for _, quest := range questMap {
		quests = append(quests, quest)
	}

	if responsesFound > 0 || questsFound > 0 {
		log.Printf("Quest parser: Found %d QuestGetQuests responses, parsed %d unique quests", responsesFound, questsFound)
	}

	return quests, nil
}

// ParseQuestsDetailed extracts quest data from log entries with detailed information.
func ParseQuestsDetailed(entries []*LogEntry) (*ParseQuestsResult, error) {
	result := &ParseQuestsResult{
		Quests:          []*QuestData{},
		CurrentQuestIDs: make(map[string]bool),
	}

	questMap := make(map[string]*QuestData)
	var latestResponseTime time.Time
	var latestResponseQuestIDs map[string]bool

	questsFound := 0
	responsesFound := 0

	for _, entry := range entries {
		if !entry.IsJSON {
			continue
		}

		logTimestamp := time.Now()
		if entry.Timestamp != "" {
			if parsedTime, err := parseLogTimestamp(entry.Timestamp); err == nil {
				logTimestamp = parsedTime
			}
		}

		if questsData, ok := entry.JSON["quests"]; ok {
			isEmptyQuests := false
			if questArray, ok := questsData.([]interface{}); ok && len(questArray) == 0 {
				isEmptyQuests = true
			}
			_, hasCanSwap := entry.JSON["canSwap"]
			if hasCanSwap || isEmptyQuests {
				responsesFound++
				result.HasQuestResponse = true

				currentQuestIDs := make(map[string]bool)
				now := time.Now()

				if questArray, ok := questsData.([]interface{}); ok {
					for _, q := range questArray {
						if questJSON, ok := q.(map[string]interface{}); ok {
							quest := parseQuestFromMap(questJSON, logTimestamp)
							if quest != nil {
								currentQuestIDs[quest.QuestID] = true

								if existing, exists := questMap[quest.QuestID]; exists {
									if isQuestRerolled(existing, quest) {
										existing.Rerolled = true
										log.Printf("Quest parser: Quest %s rerolled - type changed from '%s' to '%s'",
											quest.QuestID, existing.QuestType, quest.QuestType)

										quest.LastSeenAt = &now
										newQuestKey := quest.QuestID + "_rerolled_" + logTimestamp.Format("20060102150405")
										questMap[newQuestKey] = quest
										questsFound++
									} else {
										existing.EndingProgress = quest.EndingProgress
										existing.CanSwap = quest.CanSwap
										existing.LastSeenAt = &now
									}
								} else {
									quest.LastSeenAt = &now
									questMap[quest.QuestID] = quest
									questsFound++
								}
							}
						}
					}
				}

				if logTimestamp.After(latestResponseTime) {
					latestResponseTime = logTimestamp
					latestResponseQuestIDs = currentQuestIDs
				}

				for _, quest := range questMap {
					if !quest.Completed && !quest.Rerolled && !currentQuestIDs[quest.QuestID] {
						quest.Completed = true
						completedAt := logTimestamp
						quest.CompletedAt = &completedAt
						quest.EndingProgress = quest.Goal
						quest.CompletionSource = "disappeared"
						log.Printf("Quest parser: Quest %s completed (disappeared from response)", quest.QuestID)
					}
				}
			}
		}

		if newQuestsData, ok := entry.JSON["newQuests"]; ok {
			if questArray, ok := newQuestsData.([]interface{}); ok {
				now := time.Now()
				for _, q := range questArray {
					if questJSON, ok := q.(map[string]interface{}); ok {
						quest := parseQuestFromMap(questJSON, logTimestamp)
						if quest != nil {
							quest.LastSeenAt = &now

							if _, exists := questMap[quest.QuestID]; !exists {
								questMap[quest.QuestID] = quest
								questsFound++
							}
						}
					}
				}
			}
		}
	}

	for _, quest := range questMap {
		result.Quests = append(result.Quests, quest)
	}

	if latestResponseQuestIDs != nil {
		result.CurrentQuestIDs = latestResponseQuestIDs
		result.LatestResponseTime = latestResponseTime
	}

	if responsesFound > 0 || questsFound > 0 {
		log.Printf("Quest parser (detailed): Found %d QuestGetQuests responses, parsed %d unique quests, %d current quest IDs",
			responsesFound, questsFound, len(result.CurrentQuestIDs))
		if len(result.CurrentQuestIDs) > 0 {
			log.Printf("Quest parser (detailed): Latest response timestamp: %s", latestResponseTime.Format("2006-01-02 15:04:05 MST"))
			for questID := range result.CurrentQuestIDs {
				for _, q := range result.Quests {
					if q.QuestID == questID && !q.Completed && !q.Rerolled {
						displayID := q.QuestID
						if len(displayID) > 8 {
							displayID = displayID[:8]
						}
						log.Printf("Quest parser (detailed): Current quest: %s (%s) %d/%d",
							displayID, q.QuestType, q.EndingProgress, q.Goal)
						break
					}
				}
			}
		}
	}

	return result, nil
}

// isQuestRerolled detects if a quest was rerolled.
func isQuestRerolled(existing, new *QuestData) bool {
	if existing.QuestType != new.QuestType && existing.QuestType != "" && new.QuestType != "" {
		return true
	}

	if existing.Goal != new.Goal && existing.Goal > 0 && new.Goal > 0 {
		return true
	}

	if existing.EndingProgress > 0 && new.StartingProgress == 0 && new.EndingProgress == 0 {
		return true
	}

	return false
}

// parseQuestFromMap extracts quest data from a JSON map.
func parseQuestFromMap(json map[string]interface{}, timestamp time.Time) *QuestData {
	quest := &QuestData{
		AssignedAt: timestamp,
	}

	if questID, ok := json["questId"].(string); ok {
		quest.QuestID = questID
	} else {
		return nil
	}

	if locKey, ok := json["locKey"].(string); ok {
		quest.QuestType = locKey
	} else if questTrack, ok := json["questTrack"].(string); ok {
		quest.QuestType = questTrack
	}

	if goal, ok := json["goal"].(float64); ok {
		quest.Goal = int(goal)
	}

	if startingProgress, ok := json["startingProgress"].(float64); ok {
		quest.StartingProgress = int(startingProgress)
	}

	if endingProgress, ok := json["endingProgress"].(float64); ok {
		quest.EndingProgress = int(endingProgress)
	}

	if canSwap, ok := json["canSwap"].(bool); ok {
		quest.CanSwap = canSwap
	} else {
		quest.CanSwap = true
	}

	if chestDesc, ok := json["chestDescription"].(map[string]interface{}); ok {
		if quantity, ok := chestDesc["quantity"].(string); ok {
			quest.Rewards = quantity
		} else if quantityNum, ok := chestDesc["quantity"].(float64); ok {
			quest.Rewards = fmt.Sprintf("%.0f", quantityNum)
		}
	} else if chestDescStr, ok := json["chestDescription"].(string); ok {
		quest.Rewards = chestDescStr
	}

	if quest.Goal > 0 && quest.EndingProgress >= quest.Goal {
		quest.Completed = true
		quest.CompletedAt = &timestamp
		quest.CompletionSource = "progress"
		log.Printf("Quest parser: Quest %s marked completed by progress (%d/%d)",
			quest.QuestID, quest.EndingProgress, quest.Goal)
	}

	return quest
}
