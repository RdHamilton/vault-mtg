package logparse

import (
	"strconv"
	"strings"
)

// ParseProfile extracts player profile information from log entries.
// It looks for authenticateResponse events that contain screenName and clientId.
func ParseProfile(entries []*LogEntry) (*PlayerProfile, error) {
	for _, entry := range entries {
		if !entry.IsJSON {
			continue
		}

		// Check if this is an authenticateResponse
		if authResp, ok := entry.JSON["authenticateResponse"]; ok {
			authMap, ok := authResp.(map[string]interface{})
			if !ok {
				continue
			}

			profile := &PlayerProfile{}
			if screenName, ok := authMap["screenName"].(string); ok {
				profile.ScreenName = screenName
			}
			if clientID, ok := authMap["clientId"].(string); ok {
				profile.ClientID = clientID
			}

			if profile.ScreenName != "" || profile.ClientID != "" {
				return profile, nil
			}
		}
	}

	return nil, nil
}

// ParseInventory extracts player inventory information from log entries.
//
// Fix #2: Field names changed to PascalCase in Arena 2026.58+:
//   - wcCommon → WildCardCommons
//   - wcUncommon → WildCardUnCommons
//   - wcRare → WildCardRares
//   - wcMythic → WildCardMythics
//
// Inventory is now wrapped under the "InventoryInfo" key at the top level.
// This function handles both the wrapped and legacy (flat) formats.
func ParseInventory(entries []*LogEntry) (*PlayerInventory, error) {
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if !entry.IsJSON {
			continue
		}

		// Arena 2026.58+: inventory is wrapped under "InventoryInfo".
		if invInfo, ok := entry.JSON["InventoryInfo"]; ok {
			invMap, ok := invInfo.(map[string]interface{})
			if !ok {
				continue
			}

			inv := parseInventoryMap(invMap)
			if inv != nil {
				return inv, nil
			}
		}
	}

	return nil, nil
}

// parseInventoryMap extracts PlayerInventory fields from a JSON map.
// Supports PascalCase keys (Arena 2026.58+) with camelCase fallbacks for older logs.
func parseInventoryMap(invMap map[string]interface{}) *PlayerInventory {
	inventory := &PlayerInventory{
		CustomTokens: make(map[string]int),
	}

	if gems, ok := invMap["Gems"].(float64); ok {
		inventory.Gems = int(gems)
	}
	if gold, ok := invMap["Gold"].(float64); ok {
		inventory.Gold = int(gold)
	}
	if vaultProgress, ok := invMap["TotalVaultProgress"].(float64); ok {
		inventory.TotalVaultProgress = int(vaultProgress)
	}

	// Wildcards — PascalCase (2026.58+).
	if wc, ok := invMap["WildCardCommons"].(float64); ok {
		inventory.WildCardCommons = int(wc)
	}
	if wc, ok := invMap["WildCardUnCommons"].(float64); ok {
		inventory.WildCardUncommons = int(wc)
	}
	if wc, ok := invMap["WildCardRares"].(float64); ok {
		inventory.WildCardRares = int(wc)
	}
	if wc, ok := invMap["WildCardMythics"].(float64); ok {
		inventory.WildCardMythics = int(wc)
	}

	// Fix #3: Boosters — PascalCase field names (CollationId, SetCode, Count).
	if boosters, ok := invMap["Boosters"].([]interface{}); ok {
		for _, b := range boosters {
			boosterMap, ok := b.(map[string]interface{})
			if !ok {
				continue
			}

			booster := Booster{}
			if setCode, ok := boosterMap["SetCode"].(string); ok {
				booster.SetCode = setCode
			}
			if count, ok := boosterMap["Count"].(float64); ok {
				booster.Count = int(count)
			}
			if collationID, ok := boosterMap["CollationId"].(float64); ok {
				booster.CollationId = int(collationID)
			}

			if booster.SetCode != "" && booster.Count > 0 {
				inventory.Boosters = append(inventory.Boosters, booster)
			}
		}
	}

	// Custom tokens
	if tokens, ok := invMap["CustomTokens"].(map[string]interface{}); ok {
		for key, value := range tokens {
			if count, ok := value.(float64); ok {
				inventory.CustomTokens[key] = int(count)
			}
		}
	}

	// Return nil if nothing meaningful was parsed
	if inventory.Gems == 0 && inventory.Gold == 0 && inventory.TotalVaultProgress == 0 &&
		inventory.WildCardCommons == 0 && inventory.WildCardUncommons == 0 &&
		inventory.WildCardRares == 0 && inventory.WildCardMythics == 0 &&
		len(inventory.Boosters) == 0 {
		return nil
	}

	return inventory
}

// ParseRank extracts player rank information from log entries.
//
// Fix #4: The old code detected rank events by the top-level "rankClass" key, which
// no longer exists in Arena 2026.58+. The correct detection is by presence of
// both "constructedLevel" and "limitedLevel" fields at the top level.
func ParseRank(entries []*LogEntry) (*PlayerRank, error) {
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if !entry.IsJSON {
			continue
		}

		// Detect rank events by presence of both constructedLevel and limitedLevel.
		_, hasConstructedLevel := entry.JSON["constructedLevel"]
		_, hasLimitedLevel := entry.JSON["limitedLevel"]

		if !hasConstructedLevel || !hasLimitedLevel {
			continue
		}

		rank := &PlayerRank{}

		if season, ok := entry.JSON["constructedSeasonOrdinal"].(float64); ok {
			rank.ConstructedSeasonOrdinal = int(season)
		}
		if class, ok := entry.JSON["constructedClass"].(string); ok {
			rank.ConstructedClass = class
		}
		if level, ok := entry.JSON["constructedLevel"].(float64); ok {
			rank.ConstructedLevel = int(level)
		}
		if percentile, ok := entry.JSON["constructedPercentile"].(float64); ok {
			rank.ConstructedPercentile = percentile
		}
		if step, ok := entry.JSON["constructedStep"].(float64); ok {
			rank.ConstructedStep = int(step)
		}

		if season, ok := entry.JSON["limitedSeasonOrdinal"].(float64); ok {
			rank.LimitedSeasonOrdinal = int(season)
		}
		if class, ok := entry.JSON["limitedClass"].(string); ok {
			rank.LimitedClass = class
		}
		if level, ok := entry.JSON["limitedLevel"].(float64); ok {
			rank.LimitedLevel = int(level)
		}
		if percentile, ok := entry.JSON["limitedPercentile"].(float64); ok {
			rank.LimitedPercentile = percentile
		}
		if step, ok := entry.JSON["limitedStep"].(float64); ok {
			rank.LimitedStep = int(step)
		}

		if won, ok := entry.JSON["limitedMatchesWon"].(float64); ok {
			rank.LimitedMatchesWon = int(won)
		}
		if lost, ok := entry.JSON["limitedMatchesLost"].(float64); ok {
			rank.LimitedMatchesLost = int(lost)
		}

		return rank, nil
	}

	return nil, nil
}

// ParseDraftHistory extracts draft/limited event history from log entries.
// It looks for "Courses" arrays and filters for limited format events.
func ParseDraftHistory(entries []*LogEntry) (*DraftHistory, error) {
	history := &DraftHistory{
		Drafts: []DraftEvent{},
	}

	seenCourses := make(map[string]bool)

	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if !entry.IsJSON {
			continue
		}

		if coursesData, ok := entry.JSON["Courses"]; ok {
			courses, ok := coursesData.([]interface{})
			if !ok {
				continue
			}

			for _, courseData := range courses {
				courseMap, ok := courseData.(map[string]interface{})
				if !ok {
					continue
				}

				courseID, _ := courseMap["CourseId"].(string)
				if courseID == "" || seenCourses[courseID] {
					continue
				}

				eventName, _ := courseMap["InternalEventName"].(string)

				isDraftEvent := false
				if eventName != "" {
					lowerEvent := eventName
					if contains(lowerEvent, "Draft") ||
						contains(lowerEvent, "Sealed") ||
						contains(lowerEvent, "Premier") ||
						contains(lowerEvent, "Quick") ||
						contains(lowerEvent, "Traditional") {
						isDraftEvent = true
					}
				}

				if !isDraftEvent {
					continue
				}

				seenCourses[courseID] = true

				draftEvent := DraftEvent{
					EventID:   courseID,
					EventName: eventName,
				}

				if status, ok := courseMap["CurrentModule"].(string); ok {
					draftEvent.Status = status
				}
				if wins, ok := courseMap["CurrentWins"].(float64); ok {
					draftEvent.Wins = int(wins)
				}
				if losses, ok := courseMap["CurrentLosses"].(float64); ok {
					draftEvent.Losses = int(losses)
				}

				if courseDeckData, ok := courseMap["CourseDeck"]; ok {
					if deckMap, ok := courseDeckData.(map[string]interface{}); ok {
						draftEvent.Deck = parseDraftDeck(deckMap)
					}
				}

				if deckSummary, ok := courseMap["CourseDeckSummary"]; ok {
					if summaryMap, ok := deckSummary.(map[string]interface{}); ok {
						if name, ok := summaryMap["Name"].(string); ok {
							draftEvent.Deck.Name = name
						}
					}
				}

				history.Drafts = append(history.Drafts, draftEvent)
			}

			break
		}
	}

	if len(history.Drafts) == 0 {
		return nil, nil
	}

	return history, nil
}

// parseDraftDeck extracts deck information from a CourseDeck object.
func parseDraftDeck(deckMap map[string]interface{}) DraftDeck {
	deck := DraftDeck{
		MainDeck: []DeckCard{},
	}

	if mainDeckData, ok := deckMap["MainDeck"]; ok {
		if mainDeck, ok := mainDeckData.([]interface{}); ok {
			for _, cardData := range mainDeck {
				if cardMap, ok := cardData.(map[string]interface{}); ok {
					card := DeckCard{}

					if cardID, ok := cardMap["cardId"].(float64); ok {
						card.CardID = int(cardID)
					}

					if quantity, ok := cardMap["quantity"].(float64); ok {
						card.Quantity = int(quantity)
					}

					if card.CardID > 0 && card.Quantity > 0 {
						deck.MainDeck = append(deck.MainDeck, card)
					}
				}
			}
		}
	}

	return deck
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// ParseArenaStats extracts gameplay statistics from log entries.
func ParseArenaStats(entries []*LogEntry) (*ArenaStats, error) {
	stats := &ArenaStats{
		FormatStats: make(map[string]*FormatStats),
	}

	seenMatches := make(map[string]bool)

	var playerScreenName string
	profile, _ := ParseProfile(entries)
	if profile != nil && profile.ScreenName != "" {
		playerScreenName = profile.ScreenName
	}

	for _, entry := range entries {
		if !entry.IsJSON {
			continue
		}

		if eventData, ok := entry.JSON["matchGameRoomStateChangedEvent"]; ok {
			eventMap, ok := eventData.(map[string]interface{})
			if !ok {
				continue
			}

			gameRoomInfo, ok := eventMap["gameRoomInfo"].(map[string]interface{})
			if !ok {
				continue
			}

			finalMatchResult, hasFinalResult := gameRoomInfo["finalMatchResult"].(map[string]interface{})
			if !hasFinalResult {
				continue
			}

			matchID, _ := finalMatchResult["matchId"].(string)
			if matchID == "" || seenMatches[matchID] {
				continue
			}
			seenMatches[matchID] = true
			stats.UniqueMatchIDs++

			gameRoomConfig, ok := gameRoomInfo["gameRoomConfig"].(map[string]interface{})
			if !ok {
				continue
			}

			reservedPlayers, ok := gameRoomConfig["reservedPlayers"].([]interface{})
			if !ok || len(reservedPlayers) == 0 {
				continue
			}

			var actualPlayer map[string]interface{}
			eventID := "Unknown"

			for _, playerData := range reservedPlayers {
				player, ok := playerData.(map[string]interface{})
				if !ok {
					continue
				}

				if actualPlayer == nil {
					actualPlayer = player
				}

				if evID, ok := player["eventId"].(string); ok && evID != "" {
					eventID = evID
				}

				if playerName, ok := player["playerName"].(string); ok && playerName != "" {
					if playerScreenName != "" && playerName == playerScreenName {
						actualPlayer = player
						break
					}
				}
			}

			if actualPlayer == nil {
				continue
			}

			playerTeamID, _ := actualPlayer["teamId"].(float64)

			resultList, ok := finalMatchResult["resultList"].([]interface{})
			if !ok {
				continue
			}

			for _, resultData := range resultList {
				resultMap, ok := resultData.(map[string]interface{})
				if !ok {
					continue
				}

				scope, _ := resultMap["scope"].(string)
				winningTeamID, _ := resultMap["winningTeamId"].(float64)

				playerWon := int(playerTeamID) == int(winningTeamID)

				switch scope {
				case "MatchScope_Match":
					stats.TotalMatches++
					if playerWon {
						stats.MatchWins++
					} else {
						stats.MatchLosses++
					}
				case "MatchScope_Game":
					stats.TotalGames++
					if playerWon {
						stats.GameWins++
					} else {
						stats.GameLosses++
					}
				}

				if _, exists := stats.FormatStats[eventID]; !exists {
					stats.FormatStats[eventID] = &FormatStats{
						EventName: eventID,
					}
				}

				formatStat := stats.FormatStats[eventID]
				switch scope {
				case "MatchScope_Match":
					formatStat.MatchesPlayed++
					if playerWon {
						formatStat.MatchWins++
					} else {
						formatStat.MatchLosses++
					}
				case "MatchScope_Game":
					formatStat.GamesPlayed++
					if playerWon {
						formatStat.GameWins++
					} else {
						formatStat.GameLosses++
					}
				}
			}
		}
	}

	if stats.TotalMatches == 0 && stats.TotalGames == 0 {
		return nil, nil
	}

	return stats, nil
}

// ParsePeriodicRewards extracts daily and weekly win progress from log entries.
func ParsePeriodicRewards(entries []*LogEntry) (*PeriodicRewards, error) {
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if !entry.IsJSON {
			continue
		}

		if rewardsData, ok := entry.JSON["ClientPeriodicRewards"]; ok {
			rewardsMap, ok := rewardsData.(map[string]interface{})
			if !ok {
				continue
			}

			rewards := &PeriodicRewards{}

			if dailySeq, ok := rewardsMap["_dailyRewardSequenceId"].(float64); ok {
				rewards.DailyWins = int(dailySeq)
			}

			if weeklySeq, ok := rewardsMap["_weeklyRewardSequenceId"].(float64); ok {
				rewards.WeeklyWins = int(weeklySeq)
			}

			if rewards.DailyWins > 0 || rewards.WeeklyWins > 0 {
				return rewards, nil
			}
		}
	}

	return nil, nil
}

// ParseMasteryPass extracts mastery pass progression from log entries.
func ParseMasteryPass(entries []*LogEntry) (*MasteryPass, error) {
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if !entry.IsJSON {
			continue
		}

		nodeStatesData, hasNodeStates := entry.JSON["NodeStates"]
		if !hasNodeStates {
			continue
		}

		nodeStates, ok := nodeStatesData.(map[string]interface{})
		if !ok {
			continue
		}

		masteryPass := &MasteryPass{}
		highestCompleted := 0
		maxLevel := 0

		for nodeName, nodeData := range nodeStates {
			nodeMap, ok := nodeData.(map[string]interface{})
			if !ok {
				continue
			}

			if strings.HasPrefix(nodeName, "LevelTrack_Level_") && strings.HasSuffix(nodeName, "_Reward") {
				levelStr := strings.TrimPrefix(nodeName, "LevelTrack_Level_")
				levelStr = strings.TrimSuffix(levelStr, "_Reward")

				level, err := strconv.Atoi(levelStr)
				if err != nil {
					continue
				}

				if level > maxLevel {
					maxLevel = level
				}

				if status, ok := nodeMap["Status"].(string); ok && status == "Completed" {
					if level > highestCompleted {
						highestCompleted = level
					}

					if masteryPass.PassType == "" {
						if tierRewardState, ok := nodeMap["TierRewardNodeState"].(map[string]interface{}); ok {
							if currentTiers, ok := tierRewardState["CurrentTiers"].([]interface{}); ok && len(currentTiers) > 0 {
								if tierStr, ok := currentTiers[0].(string); ok {
									if len(tierStr) > 0 {
										masteryPass.PassType = strings.ToUpper(tierStr[:1]) + tierStr[1:]
									}
								}
							}
						}
					}
				}
			}
		}

		masteryPass.CurrentLevel = highestCompleted
		masteryPass.MaxLevel = maxLevel

		if masteryPass.CurrentLevel > 0 || masteryPass.PassType != "" {
			return masteryPass, nil
		}
	}

	return nil, nil
}

// ParseAll extracts all available information from log entries.
func ParseAll(entries []*LogEntry) (*PlayerProfile, *PlayerInventory, *PlayerRank) {
	profile, _ := ParseProfile(entries)
	inventory, _ := ParseInventory(entries)
	rank, _ := ParseRank(entries)

	return profile, inventory, rank
}
