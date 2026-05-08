package logreader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inventoryEntry builds a LogEntry containing an InventoryInfo object
// as Arena 2026.58+ would emit.
func inventoryEntry() *LogEntry {
	return &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"InventoryInfo": map[string]interface{}{
				"Gems":               float64(1200),
				"Gold":               float64(5000),
				"TotalVaultProgress": float64(75),
				"WildCardCommons":    float64(10),
				"WildCardUnCommons":  float64(5),
				"WildCardRares":      float64(3),
				"WildCardMythics":    float64(1),
				"Boosters": []interface{}{
					map[string]interface{}{
						"CollationId": float64(100078),
						"SetCode":     "BLB",
						"Count":       float64(2),
					},
				},
			},
		},
	}
}

func TestIsInventoryEntry_True(t *testing.T) {
	assert.True(t, IsInventoryEntry(inventoryEntry()))
}

func TestIsInventoryEntry_False_NoKey(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"draftPack": map[string]interface{}{}},
	}
	assert.False(t, IsInventoryEntry(entry))
}

func TestIsInventoryEntry_False_NilEntry(t *testing.T) {
	assert.False(t, IsInventoryEntry(nil))
}

func TestIsInventoryEntry_False_NotJSON(t *testing.T) {
	entry := &LogEntry{IsJSON: false, Raw: "plain text"}
	assert.False(t, IsInventoryEntry(entry))
}

func TestParseInventoryEntry_PopulatesAllFields(t *testing.T) {
	p, err := ParseInventoryEntry(inventoryEntry())
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, 1200, p.Gems)
	assert.Equal(t, 5000, p.Gold)
	assert.Equal(t, 75, p.TotalVaultProgress)
	assert.Equal(t, 10, p.WildCardCommons)
	assert.Equal(t, 5, p.WildCardUncommons)
	assert.Equal(t, 3, p.WildCardRares)
	assert.Equal(t, 1, p.WildCardMythics)
	require.Len(t, p.Boosters, 1)
	assert.Equal(t, "BLB", p.Boosters[0].SetCode)
	assert.Equal(t, 2, p.Boosters[0].Count)
	assert.Equal(t, 100078, p.Boosters[0].CollationID)
}

func TestParseInventoryEntry_NoInventoryInfoKey(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"otherKey": "value"},
	}
	_, err := ParseInventoryEntry(entry)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "InventoryInfo")
}

func TestParseInventoryEntry_NilEntry(t *testing.T) {
	_, err := ParseInventoryEntry(nil)
	assert.Error(t, err)
}

func TestParseInventoryEntry_NotJSONEntry(t *testing.T) {
	entry := &LogEntry{IsJSON: false, Raw: "plain text"}
	_, err := ParseInventoryEntry(entry)
	assert.Error(t, err)
}

func TestParseInventoryEntry_EmptyBoosters(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"InventoryInfo": map[string]interface{}{
				"Gems":     float64(500),
				"Gold":     float64(200),
				"Boosters": []interface{}{},
			},
		},
	}
	p, err := ParseInventoryEntry(entry)
	require.NoError(t, err)
	assert.Equal(t, 500, p.Gems)
	assert.Equal(t, 200, p.Gold)
	assert.Empty(t, p.Boosters)
}

func TestParseInventoryEntry_PartialFields(t *testing.T) {
	// Only Gems populated — other fields default to zero.
	entry := &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"InventoryInfo": map[string]interface{}{
				"Gems": float64(9999),
			},
		},
	}
	p, err := ParseInventoryEntry(entry)
	require.NoError(t, err)
	assert.Equal(t, 9999, p.Gems)
	assert.Equal(t, 0, p.Gold)
	assert.Equal(t, 0, p.WildCardRares)
}
