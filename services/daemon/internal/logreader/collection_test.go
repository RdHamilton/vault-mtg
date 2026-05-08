package logreader

import (
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collectionEntry builds a LogEntry representing a minimal
// PlayerInventoryGetPlayerCardsV3 response with three cards.
func collectionEntry() *LogEntry {
	return &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"12345": float64(4),
			"67890": float64(2),
			"99999": float64(1),
		},
	}
}

// TestIsCollectionEntry_True verifies that a flat card-ID map is recognised.
func TestIsCollectionEntry_True(t *testing.T) {
	assert.True(t, IsCollectionEntry(collectionEntry()))
}

// TestIsCollectionEntry_EmptyObject accepts an explicit empty-object response.
func TestIsCollectionEntry_EmptyObject(t *testing.T) {
	entry := &LogEntry{IsJSON: true, JSON: map[string]interface{}{}}
	assert.True(t, IsCollectionEntry(entry))
}

// TestIsCollectionEntry_False_NilEntry rejects a nil entry.
func TestIsCollectionEntry_False_NilEntry(t *testing.T) {
	assert.False(t, IsCollectionEntry(nil))
}

// TestIsCollectionEntry_False_NotJSON rejects a non-JSON entry.
func TestIsCollectionEntry_False_NotJSON(t *testing.T) {
	entry := &LogEntry{IsJSON: false, Raw: "plain text"}
	assert.False(t, IsCollectionEntry(entry))
}

// TestIsCollectionEntry_False_HasInventoryInfoKey rejects entries with
// the InventoryInfo wrapper key (inventory.updated, not collection.updated).
func TestIsCollectionEntry_False_HasInventoryInfoKey(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"InventoryInfo": map[string]interface{}{"Gems": float64(100)},
		},
	}
	assert.False(t, IsCollectionEntry(entry))
}

// TestIsCollectionEntry_False_HasQuestsKey rejects quest responses.
func TestIsCollectionEntry_False_HasQuestsKey(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"quests":  []interface{}{},
			"canSwap": false,
		},
	}
	assert.False(t, IsCollectionEntry(entry))
}

// TestIsCollectionEntry_False_HasDraftPackKey rejects draft pack responses.
func TestIsCollectionEntry_False_HasDraftPackKey(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"draftPack": map[string]interface{}{},
		},
	}
	assert.False(t, IsCollectionEntry(entry))
}

// TestIsCollectionEntry_False_NonIntegerKey rejects objects that have named
// (non-integer) keys, e.g. an unrecognised API response.
func TestIsCollectionEntry_False_NonIntegerKey(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"cardName": "Lightning Bolt",
		},
	}
	assert.False(t, IsCollectionEntry(entry))
}

// TestIsCollectionEntry_False_ZeroKey rejects maps whose key parses to 0
// (not a valid arena ID).
func TestIsCollectionEntry_False_ZeroKey(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"0": float64(5),
		},
	}
	assert.False(t, IsCollectionEntry(entry))
}

// TestIsCollectionEntry_False_NegativeKey rejects maps with negative keys.
func TestIsCollectionEntry_False_NegativeKey(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"-1": float64(3),
		},
	}
	assert.False(t, IsCollectionEntry(entry))
}

// TestParseCollectionEntry_PopulatesAllFields verifies that all card entries
// are parsed into the payload.
func TestParseCollectionEntry_PopulatesAllFields(t *testing.T) {
	p, err := ParseCollectionEntry(collectionEntry())
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.False(t, p.IsDelta)
	require.Len(t, p.Cards, 3)

	// Sort for deterministic comparison.
	sort.Slice(p.Cards, func(i, j int) bool { return p.Cards[i].ArenaID < p.Cards[j].ArenaID })
	assert.Equal(t, 12345, p.Cards[0].ArenaID)
	assert.Equal(t, 4, p.Cards[0].Count)
	assert.Equal(t, 67890, p.Cards[1].ArenaID)
	assert.Equal(t, 2, p.Cards[1].Count)
	assert.Equal(t, 99999, p.Cards[2].ArenaID)
	assert.Equal(t, 1, p.Cards[2].Count)
}

// TestParseCollectionEntry_EmptyObject returns an empty card slice, no error.
func TestParseCollectionEntry_EmptyObject(t *testing.T) {
	entry := &LogEntry{IsJSON: true, JSON: map[string]interface{}{}}
	p, err := ParseCollectionEntry(entry)
	require.NoError(t, err)
	assert.Empty(t, p.Cards)
	assert.False(t, p.IsDelta)
}

// TestParseCollectionEntry_NilEntry returns an error for a nil entry.
func TestParseCollectionEntry_NilEntry(t *testing.T) {
	_, err := ParseCollectionEntry(nil)
	require.Error(t, err)
}

// TestParseCollectionEntry_NotJSONEntry returns an error for a non-JSON entry.
func TestParseCollectionEntry_NotJSONEntry(t *testing.T) {
	entry := &LogEntry{IsJSON: false, Raw: "plain text"}
	_, err := ParseCollectionEntry(entry)
	require.Error(t, err)
}

// TestParseCollectionEntry_NotCollectionEntry returns an error when the entry
// is not a collection snapshot (has a named key).
func TestParseCollectionEntry_NotCollectionEntry(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"InventoryInfo": map[string]interface{}{}},
	}
	_, err := ParseCollectionEntry(entry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collection snapshot")
}

// TestParseCollectionEntry_LargePayload verifies parsing handles a larger map
// without error (simulates the ~100 KB payloads produced by Arena).
func TestParseCollectionEntry_LargePayload(t *testing.T) {
	const cardCount = 500
	jsonMap := make(map[string]interface{}, cardCount)
	for i := 1; i <= cardCount; i++ {
		key := strconv.Itoa(100000 + i)
		jsonMap[key] = float64(4)
	}
	entry := &LogEntry{IsJSON: true, JSON: jsonMap}

	p, err := ParseCollectionEntry(entry)
	require.NoError(t, err)
	assert.Len(t, p.Cards, cardCount)
}
