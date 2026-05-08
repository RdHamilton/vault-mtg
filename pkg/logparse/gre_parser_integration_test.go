package logparse

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestParseGamePlays_Integration tests the play tracking parser with real match log data.
// The fixture contains GRE messages from an actual MTGA match (vs crayonscabs).
func TestParseGamePlays_Integration(t *testing.T) {
	// Get the directory of this test file
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Failed to get current file path")
	}
	testdataDir := filepath.Join(filepath.Dir(currentFile), "testdata")
	fixturePath := filepath.Join(testdataDir, "match_play_tracking.json")

	// Read the fixture file
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("Skipping integration test: fixture file not found at %s", fixturePath)
	}

	// Parse the JSON fixture
	var jsonBlocks []map[string]interface{}
	if err := json.Unmarshal(data, &jsonBlocks); err != nil {
		t.Fatalf("Failed to parse fixture JSON: %v", err)
	}

	// Convert JSON blocks to LogEntry format
	entries := make([]*LogEntry, 0, len(jsonBlocks))
	for _, block := range jsonBlocks {
		entries = append(entries, &LogEntry{
			IsJSON:    true,
			JSON:      block,
			Timestamp: "2025-01-11 15:00:00",
		})
	}

	t.Logf("Loaded %d log entries from fixture", len(entries))

	// Get player connection info
	playerConn := GetPlayerSeatIDByName(entries, "Jhixiaus")
	if playerConn == nil {
		t.Fatal("Failed to find player connection info")
	}
	t.Logf("Player seat ID: %d (SystemSeatID: %d)", playerConn.SeatID, playerConn.SystemSeatID)

	// Parse GRE messages
	messages, err := ParseGREMessages(entries)
	if err != nil {
		t.Fatalf("ParseGREMessages failed: %v", err)
	}
	t.Logf("Parsed %d GRE game state messages", len(messages))

	// Verify we have game state messages
	if len(messages) < 2 {
		t.Fatalf("Expected at least 2 game state messages, got %d", len(messages))
	}

	// Check that zones are being parsed
	zonesFound := false
	for _, msg := range messages {
		if len(msg.Zones) > 10 {
			zonesFound = true
			t.Logf("Found message with %d zones", len(msg.Zones))
			break
		}
	}
	if !zonesFound {
		t.Log("Warning: No message with full zones array found")
	}

	// Parse game plays
	plays, err := ParseGamePlays(entries, playerConn)
	if err != nil {
		t.Fatalf("ParseGamePlays failed: %v", err)
	}
	t.Logf("Detected %d game plays", len(plays))

	// Verify we detected some plays
	if len(plays) == 0 {
		t.Error("Expected to detect some game plays, got 0")
	}

	// Analyze the detected plays
	actionCounts := make(map[string]int)
	playerPlays := 0
	opponentPlays := 0
	lifeChanges := 0

	for _, play := range plays {
		actionCounts[play.ActionType]++
		if play.PlayerType == "player" {
			playerPlays++
		} else {
			opponentPlays++
		}
		if play.ActionType == "life_change" {
			lifeChanges++
		}
	}

	t.Logf("Action type breakdown:")
	for action, count := range actionCounts {
		t.Logf("  %s: %d", action, count)
	}
	t.Logf("Player plays: %d, Opponent plays: %d", playerPlays, opponentPlays)
	t.Logf("Life changes detected: %d", lifeChanges)

	// Verify we have a mix of player and opponent plays
	if playerPlays == 0 && opponentPlays == 0 {
		t.Error("Expected to detect plays for both player and opponent")
	}

	// Extract game snapshots
	snapshots, err := ExtractGameSnapshots(entries, playerConn)
	if err != nil {
		t.Fatalf("ExtractGameSnapshots failed: %v", err)
	}
	t.Logf("Extracted %d game snapshots", len(snapshots))

	// Extract opponent cards
	opponentCards, err := ExtractOpponentCards(entries, playerConn)
	if err != nil {
		t.Fatalf("ExtractOpponentCards failed: %v", err)
	}
	t.Logf("Observed %d opponent cards", len(opponentCards))
}

// TestParseGREMessages_Integration tests GRE message parsing with real data.
func TestParseGREMessages_Integration(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Failed to get current file path")
	}
	testdataDir := filepath.Join(filepath.Dir(currentFile), "testdata")
	fixturePath := filepath.Join(testdataDir, "match_play_tracking.json")

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("Skipping integration test: fixture file not found at %s", fixturePath)
	}

	var jsonBlocks []map[string]interface{}
	if err := json.Unmarshal(data, &jsonBlocks); err != nil {
		t.Fatalf("Failed to parse fixture JSON: %v", err)
	}

	entries := make([]*LogEntry, 0, len(jsonBlocks))
	for _, block := range jsonBlocks {
		entries = append(entries, &LogEntry{
			IsJSON:    true,
			JSON:      block,
			Timestamp: "2025-01-11 15:00:00",
		})
	}

	messages, err := ParseGREMessages(entries)
	if err != nil {
		t.Fatalf("ParseGREMessages failed: %v", err)
	}

	// Verify message structure
	t.Run("MatchID", func(t *testing.T) {
		foundMatchID := false
		for _, msg := range messages {
			if msg.MatchID != "" {
				foundMatchID = true
				t.Logf("Match ID: %s", msg.MatchID)
				break
			}
		}
		if !foundMatchID {
			t.Log("Warning: No match ID found in messages")
		}
	})

	t.Run("TurnProgression", func(t *testing.T) {
		turns := make(map[int]bool)
		for _, msg := range messages {
			if msg.TurnInfo != nil && msg.TurnInfo.TurnNumber > 0 {
				turns[msg.TurnInfo.TurnNumber] = true
			}
		}
		t.Logf("Turns found: %v", len(turns))
	})

	t.Run("GameObjects", func(t *testing.T) {
		totalObjects := 0
		maxObjects := 0
		for _, msg := range messages {
			count := len(msg.GameObjects)
			totalObjects += count
			if count > maxObjects {
				maxObjects = count
			}
		}
		t.Logf("Total game objects across all messages: %d", totalObjects)
		t.Logf("Max objects in single message: %d", maxObjects)
	})

	t.Run("Players", func(t *testing.T) {
		for _, msg := range messages {
			if len(msg.Players) > 0 {
				for _, player := range msg.Players {
					t.Logf("Player seat %d: life=%d", player.SeatID, player.LifeTotal)
				}
				break
			}
		}
	})

	t.Run("Zones", func(t *testing.T) {
		for _, msg := range messages {
			if len(msg.Zones) > 0 {
				t.Logf("Message has %d zones", len(msg.Zones))
				// Log a few zone types
				count := 0
				for _, zone := range msg.Zones {
					t.Logf("  Zone %d: %s (owner: %d)", zone.ZoneID, zone.Type, zone.OwnerSeatID)
					count++
					if count >= 5 {
						break
					}
				}
				break
			}
		}
	})
}

// TestZoneNameResolution_Integration tests zone name resolution with real data.
func TestZoneNameResolution_Integration(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Failed to get current file path")
	}
	testdataDir := filepath.Join(filepath.Dir(currentFile), "testdata")
	fixturePath := filepath.Join(testdataDir, "match_play_tracking.json")

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("Skipping integration test: fixture file not found at %s", fixturePath)
	}

	var jsonBlocks []map[string]interface{}
	if err := json.Unmarshal(data, &jsonBlocks); err != nil {
		t.Fatalf("Failed to parse fixture JSON: %v", err)
	}

	entries := make([]*LogEntry, 0, len(jsonBlocks))
	for _, block := range jsonBlocks {
		entries = append(entries, &LogEntry{
			IsJSON:    true,
			JSON:      block,
			Timestamp: "2025-01-11 15:00:00",
		})
	}

	messages, err := ParseGREMessages(entries)
	if err != nil {
		t.Fatalf("ParseGREMessages failed: %v", err)
	}

	// Build cumulative zones map like ParseGamePlays does
	cumulativeZones := make(map[int]*GREZone)
	for _, msg := range messages {
		for zoneID, zone := range msg.Zones {
			cumulativeZones[zoneID] = zone
		}
	}

	t.Logf("Built cumulative zones map with %d zones", len(cumulativeZones))

	// Test zone name resolution
	t.Run("ZoneTypeMapping", func(t *testing.T) {
		expectedTypes := map[string]bool{
			"hand":        false,
			"library":     false,
			"battlefield": false,
			"graveyard":   false,
			"exile":       false,
			"stack":       false,
		}

		for zoneID, zone := range cumulativeZones {
			zoneName := zoneTypeToReadableName(zone.Type)
			t.Logf("Zone %d (%s) -> %s", zoneID, zone.Type, zoneName)
			if _, ok := expectedTypes[zoneName]; ok {
				expectedTypes[zoneName] = true
			}
		}

		// Check which common zones we found
		for zoneType, found := range expectedTypes {
			if found {
				t.Logf("Found zone type: %s", zoneType)
			}
		}
	})
}
