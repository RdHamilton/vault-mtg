package logparse

import (
	"time"
)

// QuestCompletion represents a completed quest with payout status.
type QuestCompletion struct {
	QuestNumber     int       // Quest number (1-7)
	QuestCompleted  bool      // Quest itself is completed
	PayoutCompleted bool      // Payout has been claimed
	Timestamp       time.Time // When this state was observed
}

// NodeState represents the status of any node in the graph.
type NodeState struct {
	NodeName string
	Status   string // "Available", "Completed", "Locked", etc.
}

// GraphStateData contains parsed data from GraphGetGraphState events.
type GraphStateData struct {
	CompletedQuests []QuestCompletion
	AllNodes        []NodeState // All node states for future use
	Timestamp       time.Time
}

// ParseGraphState parses GraphGetGraphState events to extract quest completion status.
func ParseGraphState(entries []*LogEntry) ([]*GraphStateData, error) {
	var results []*GraphStateData

	for _, entry := range entries {
		if !entry.IsJSON {
			continue
		}

		timestamp := time.Now()
		if entry.Timestamp != "" {
			if parsedTime, err := parseLogTimestamp(entry.Timestamp); err == nil {
				timestamp = parsedTime
			}
		}

		nodeStates, ok := entry.JSON["NodeStates"].(map[string]interface{})
		if !ok || nodeStates == nil {
			continue
		}

		data := &GraphStateData{
			CompletedQuests: []QuestCompletion{},
			AllNodes:        []NodeState{},
			Timestamp:       timestamp,
		}

		for nodeName, nodeData := range nodeStates {
			if nodeMap, ok := nodeData.(map[string]interface{}); ok {
				if status, ok := nodeMap["Status"].(string); ok {
					data.AllNodes = append(data.AllNodes, NodeState{
						NodeName: nodeName,
						Status:   status,
					})
				}
			}
		}

		questKeys := []string{"Quest1", "Quest2", "Quest3", "Quest4", "Quest5", "Quest6", "Quest7"}
		for i, questKey := range questKeys {
			questNum := i + 1
			payoutKey := questKey + "Payout"

			questCompleted := false
			payoutCompleted := false

			if questNode, ok := nodeStates[questKey].(map[string]interface{}); ok {
				if status, ok := questNode["Status"].(string); ok && status == "Completed" {
					questCompleted = true
				}
			}

			if payoutNode, ok := nodeStates[payoutKey].(map[string]interface{}); ok {
				if status, ok := payoutNode["Status"].(string); ok && status == "Completed" {
					payoutCompleted = true
				}
			}

			if questCompleted || payoutCompleted {
				data.CompletedQuests = append(data.CompletedQuests, QuestCompletion{
					QuestNumber:     questNum,
					QuestCompleted:  questCompleted,
					PayoutCompleted: payoutCompleted,
					Timestamp:       timestamp,
				})
			}
		}

		if len(data.CompletedQuests) > 0 || len(data.AllNodes) > 0 {
			results = append(results, data)
		}
	}

	return results, nil
}
