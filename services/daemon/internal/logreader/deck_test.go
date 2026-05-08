package logreader

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// IsDeckEntry
// ---------------------------------------------------------------------------

func TestIsDeckEntry_Nil(t *testing.T) {
	assert.False(t, IsDeckEntry(nil))
}

func TestIsDeckEntry_NotJSON(t *testing.T) {
	entry := &LogEntry{IsJSON: false, Raw: "plain text"}
	assert.False(t, IsDeckEntry(entry))
}

func TestIsDeckEntry_NoRequestKey(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"other": "value"},
	}
	assert.False(t, IsDeckEntry(entry))
}

func TestIsDeckEntry_EmptyRequestString(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": ""},
	}
	assert.False(t, IsDeckEntry(entry))
}

func TestIsDeckEntry_RequestNotString(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": 42},
	}
	assert.False(t, IsDeckEntry(entry))
}

func TestIsDeckEntry_RequestInvalidJSON(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": "not-json"},
	}
	assert.False(t, IsDeckEntry(entry))
}

func TestIsDeckEntry_RequestMissingSummary(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": `{"Deck":{}}`},
	}
	assert.False(t, IsDeckEntry(entry))
}

func TestIsDeckEntry_RequestSummaryMissingDeckId(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": `{"Summary":{"Name":"My Deck"}}`},
	}
	assert.False(t, IsDeckEntry(entry))
}

func TestIsDeckEntry_RequestSummaryEmptyDeckId(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": `{"Summary":{"DeckId":""}}`},
	}
	assert.False(t, IsDeckEntry(entry))
}

func TestIsDeckEntry_ValidDeckUpsert(t *testing.T) {
	req := `{"Summary":{"DeckId":"deck-abc","Name":"My Deck","Attributes":[{"name":"Format","value":"Standard"}]},"Deck":{"MainDeck":[{"cardId":12345,"quantity":4}],"Sideboard":[]}}`
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": req},
	}
	assert.True(t, IsDeckEntry(entry))
}

// ---------------------------------------------------------------------------
// ParseDeckEntry — error paths
// ---------------------------------------------------------------------------

func TestParseDeckEntry_Nil(t *testing.T) {
	_, err := ParseDeckEntry(nil)
	require.Error(t, err)
}

func TestParseDeckEntry_NotJSON(t *testing.T) {
	_, err := ParseDeckEntry(&LogEntry{IsJSON: false, Raw: "text"})
	require.Error(t, err)
}

func TestParseDeckEntry_MissingRequest(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"other": "val"},
	}
	_, err := ParseDeckEntry(entry)
	require.Error(t, err)
}

func TestParseDeckEntry_RequestInvalidJSON(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": "!!invalid"},
	}
	_, err := ParseDeckEntry(entry)
	require.Error(t, err)
}

func TestParseDeckEntry_MissingSummary(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": `{"Deck":{}}`},
	}
	_, err := ParseDeckEntry(entry)
	require.Error(t, err)
}

func TestParseDeckEntry_MissingDeckId(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": `{"Summary":{"Name":"My Deck"}}`},
	}
	_, err := ParseDeckEntry(entry)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// ParseDeckEntry — success paths
// ---------------------------------------------------------------------------

func TestParseDeckEntry_BasicFields(t *testing.T) {
	req := `{"Summary":{"DeckId":"deck-123","Name":"Test Deck","Attributes":[{"name":"Format","value":"Standard"}]},"Deck":{"MainDeck":[],"Sideboard":[]}}`
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": req},
	}

	p, err := ParseDeckEntry(entry)
	require.NoError(t, err)
	assert.Equal(t, "deck-123", p.DeckID)
	assert.Equal(t, "Test Deck", p.Name)
	assert.Equal(t, "Standard", p.Format)
	assert.Empty(t, p.Cards)
}

func TestParseDeckEntry_WithMainDeck(t *testing.T) {
	req := `{"Summary":{"DeckId":"deck-456","Name":"Burn","Attributes":[{"name":"Format","value":"Historic"}]},"Deck":{"MainDeck":[{"cardId":11111,"quantity":4},{"cardId":22222,"quantity":2}],"Sideboard":[]}}`
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": req},
	}

	p, err := ParseDeckEntry(entry)
	require.NoError(t, err)
	assert.Equal(t, "deck-456", p.DeckID)
	assert.Equal(t, "Historic", p.Format)
	require.Len(t, p.Cards, 2)
	assert.Equal(t, 11111, p.Cards[0].ArenaID)
	assert.Equal(t, 4, p.Cards[0].Quantity)
	assert.Equal(t, 22222, p.Cards[1].ArenaID)
	assert.Equal(t, 2, p.Cards[1].Quantity)
}

func TestParseDeckEntry_PascalCaseCardId(t *testing.T) {
	// Some Arena versions emit "CardId" (PascalCase) instead of "cardId".
	req := `{"Summary":{"DeckId":"deck-789","Name":"Control","Attributes":[{"name":"Format","value":"Explorer"}]},"Deck":{"MainDeck":[{"CardId":55555,"Quantity":3}],"Sideboard":[]}}`
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": req},
	}

	p, err := ParseDeckEntry(entry)
	require.NoError(t, err)
	require.Len(t, p.Cards, 1)
	assert.Equal(t, 55555, p.Cards[0].ArenaID)
	assert.Equal(t, 3, p.Cards[0].Quantity)
}

func TestParseDeckEntry_NoFormatAttributeDefaultsToUnknown(t *testing.T) {
	req := `{"Summary":{"DeckId":"deck-nofmt","Name":"No Format"},"Deck":{"MainDeck":[]}}`
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": req},
	}

	p, err := ParseDeckEntry(entry)
	require.NoError(t, err)
	assert.Equal(t, "Unknown", p.Format)
}

func TestParseDeckEntry_NoDeckObject(t *testing.T) {
	// Request has Summary but no Deck key — still valid; cards slice will be empty.
	req := `{"Summary":{"DeckId":"deck-nodeck","Name":"Ghost","Attributes":[{"name":"Format","value":"Standard"}]}}`
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": req},
	}

	p, err := ParseDeckEntry(entry)
	require.NoError(t, err)
	assert.Equal(t, "deck-nodeck", p.DeckID)
	assert.Empty(t, p.Cards)
}

func TestParseDeckEntry_SkipsZeroQuantityCards(t *testing.T) {
	req := `{"Summary":{"DeckId":"deck-zq","Name":"Zero Q"},"Deck":{"MainDeck":[{"cardId":99999,"quantity":0},{"cardId":11111,"quantity":4}]}}`
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": req},
	}

	p, err := ParseDeckEntry(entry)
	require.NoError(t, err)
	require.Len(t, p.Cards, 1)
	assert.Equal(t, 11111, p.Cards[0].ArenaID)
}

func TestParseDeckEntry_SkipsZeroArenaIDCards(t *testing.T) {
	req := `{"Summary":{"DeckId":"deck-zid","Name":"Zero ID"},"Deck":{"MainDeck":[{"cardId":0,"quantity":4},{"cardId":77777,"quantity":2}]}}`
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": req},
	}

	p, err := ParseDeckEntry(entry)
	require.NoError(t, err)
	require.Len(t, p.Cards, 1)
	assert.Equal(t, 77777, p.Cards[0].ArenaID)
}

func TestParseDeckEntry_RoundTrip(t *testing.T) {
	// Verify JSON marshalling produces the expected field names.
	req := `{"Summary":{"DeckId":"deck-rt","Name":"Round Trip","Attributes":[{"name":"Format","value":"Standard"}]},"Deck":{"MainDeck":[{"cardId":12345,"quantity":4}],"Sideboard":[]}}`
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"request": req},
	}

	p, err := ParseDeckEntry(entry)
	require.NoError(t, err)

	b, err := json.Marshal(p)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &m))
	assert.Equal(t, "deck-rt", m["deck_id"])
	assert.Equal(t, "Round Trip", m["name"])
	assert.Equal(t, "Standard", m["format"])
	cards, ok := m["cards"].([]interface{})
	require.True(t, ok)
	require.Len(t, cards, 1)
	card := cards[0].(map[string]interface{})
	assert.Equal(t, float64(12345), card["arena_id"])
	assert.Equal(t, float64(4), card["quantity"])
}
