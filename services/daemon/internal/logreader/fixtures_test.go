package logreader

import (
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseMatchCompletedFromFile reads testdata/match_completed.log and verifies
// the parsed MatchCompletedPayload with the local player ID resolved.
//
// This fixture uses the real MTGA wire format (eventId inside reservedPlayers[]
// entries only; no top-level eventId in gameRoomConfig).  A passing result here
// is the fail-before/pass-after proof that the eventId extraction fix is
// correct.
func TestParseMatchCompletedFromFile(t *testing.T) {
	raw, err := os.ReadFile("testdata/match_completed.log")
	require.NoError(t, err)

	entry := &LogEntry{Raw: string(raw)}
	entry.parseJSON()
	require.True(t, entry.IsJSON, "match_completed.log must parse as JSON")

	require.True(t, IsMatchCompletedEntry(entry), "entry must be classified as match completed")

	// Parse with the local player ID so derived fields are populated.
	p, err := ParseMatchCompletedEntry(entry, "USER_LOCAL")
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "5e1f2961-3036-4dd4-98ed-4b7810b62e4c", p.MatchID)
	assert.Equal(t, "Ladder", p.Format)
	assert.Equal(t, 1, p.WinningTeamID)
	assert.Equal(t, 1, p.PlayerTeamID)
	assert.Equal(t, "win", p.Result)
	assert.Equal(t, "OpponentName#67890", p.OpponentName)
	// 2 game-scope results with winningTeamId=1 → 2 player wins; 1 win for opponent.
	assert.Equal(t, 2, p.PlayerWins)
	assert.Equal(t, 1, p.OpponentWins)
	require.Len(t, p.ResultList, 4)
}

// TestParseMatchCompletedNoEventIDFromFile reads testdata/match_completed_no_eventid.log
// and verifies that when no reservedPlayers entry carries an eventId field the
// parsed Format is empty (daemon stays honest; BFF owns the display default).
func TestParseMatchCompletedNoEventIDFromFile(t *testing.T) {
	raw, err := os.ReadFile("testdata/match_completed_no_eventid.log")
	require.NoError(t, err)

	entry := &LogEntry{Raw: string(raw)}
	entry.parseJSON()
	require.True(t, entry.IsJSON, "match_completed_no_eventid.log must parse as JSON")

	require.True(t, IsMatchCompletedEntry(entry), "entry must be classified as match completed")

	p, err := ParseMatchCompletedEntry(entry, "USER_LOCAL")
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Empty(t, p.Format, "Format must be empty when no reservedPlayers entry carries eventId")
}

// TestParseInventoryFromFile reads testdata/inventory_updated.log and verifies
// the parsed InventoryUpdatedPayload has the expected field values.
func TestParseInventoryFromFile(t *testing.T) {
	raw, err := os.ReadFile("testdata/inventory_updated.log")
	require.NoError(t, err)

	entry := &LogEntry{Raw: string(raw)}
	entry.parseJSON()
	require.True(t, entry.IsJSON, "inventory_updated.log must parse as JSON")

	require.True(t, IsInventoryEntry(entry), "entry must be classified as inventory")

	p, err := ParseInventoryEntry(entry)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, 1200, p.Gems)
	assert.Equal(t, 5000, p.Gold)
	assert.Equal(t, 75, p.TotalVaultProgress)
	assert.Equal(t, 10, p.WildCardCommons)
	assert.Equal(t, 5, p.WildCardUncommons)
	assert.Equal(t, 3, p.WildCardRares)
	assert.Equal(t, 1, p.WildCardMythics)
	require.Len(t, p.Boosters, 2)

	// Sort boosters by SetCode for deterministic comparison.
	sort.Slice(p.Boosters, func(i, j int) bool {
		return p.Boosters[i].SetCode < p.Boosters[j].SetCode
	})
	assert.Equal(t, "BLB", p.Boosters[0].SetCode)
	assert.Equal(t, 2, p.Boosters[0].Count)
	assert.Equal(t, 100078, p.Boosters[0].CollationID)
	assert.Equal(t, "MKM", p.Boosters[1].SetCode)
	assert.Equal(t, 1, p.Boosters[1].Count)
	assert.Equal(t, 100079, p.Boosters[1].CollationID)
}

// TestParseCollectionFromFile reads testdata/collection_updated.log and verifies
// the parsed CollectionUpdatedPayload contains the expected cards.
func TestParseCollectionFromFile(t *testing.T) {
	raw, err := os.ReadFile("testdata/collection_updated.log")
	require.NoError(t, err)

	entry := &LogEntry{Raw: string(raw)}
	entry.parseJSON()
	require.True(t, entry.IsJSON, "collection_updated.log must parse as JSON")

	require.True(t, IsCollectionEntry(entry), "entry must be classified as collection")

	p, err := ParseCollectionEntry(entry)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.False(t, p.IsDelta)
	require.Len(t, p.Cards, 5)

	// Sort by ArenaID for deterministic comparison.
	sort.Slice(p.Cards, func(i, j int) bool { return p.Cards[i].ArenaID < p.Cards[j].ArenaID })
	assert.Equal(t, 12345, p.Cards[0].ArenaID)
	assert.Equal(t, 4, p.Cards[0].Count)
	assert.Equal(t, 67890, p.Cards[1].ArenaID)
	assert.Equal(t, 2, p.Cards[1].Count)
	assert.Equal(t, 99999, p.Cards[2].ArenaID)
	assert.Equal(t, 1, p.Cards[2].Count)
	assert.Equal(t, 100001, p.Cards[3].ArenaID)
	assert.Equal(t, 3, p.Cards[3].Count)
	assert.Equal(t, 100002, p.Cards[4].ArenaID)
	assert.Equal(t, 1, p.Cards[4].Count)
}

// TestParseDeckFromFile reads testdata/deck_updated.log and verifies the parsed
// DeckUpdatedPayload has the expected deck ID, name, format, and card list.
func TestParseDeckFromFile(t *testing.T) {
	raw, err := os.ReadFile("testdata/deck_updated.log")
	require.NoError(t, err)

	entry := &LogEntry{Raw: string(raw)}
	entry.parseJSON()
	require.True(t, entry.IsJSON, "deck_updated.log must parse as JSON")

	require.True(t, IsDeckEntry(entry), "entry must be classified as deck upsert")

	p, err := ParseDeckEntry(entry)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "deck-fixture-abc123", p.DeckID)
	assert.Equal(t, "Fixture Burn Deck", p.Name)
	assert.Equal(t, "Standard", p.Format)
	require.Len(t, p.Cards, 3)
	assert.Equal(t, 12345, p.Cards[0].ArenaID)
	assert.Equal(t, 4, p.Cards[0].Quantity)
	assert.Equal(t, 67890, p.Cards[1].ArenaID)
	assert.Equal(t, 4, p.Cards[1].Quantity)
	assert.Equal(t, 11111, p.Cards[2].ArenaID)
	assert.Equal(t, 2, p.Cards[2].Quantity)
}

// TestParsePlayerAuthenticatedFromFile reads testdata/player_authenticated.log and
// verifies that the entry parses as JSON and contains the authenticateResponse key.
func TestParsePlayerAuthenticatedFromFile(t *testing.T) {
	raw, err := os.ReadFile("testdata/player_authenticated.log")
	require.NoError(t, err)

	entry := &LogEntry{Raw: string(raw)}
	entry.parseJSON()
	require.True(t, entry.IsJSON, "player_authenticated.log must parse as JSON")

	authResp, ok := entry.JSON["authenticateResponse"].(map[string]interface{})
	require.True(t, ok, "entry must contain authenticateResponse object")

	assert.Equal(t, "SynthPlayer#12345", authResp["screenName"])
	assert.Equal(t, "USER_LOCAL", authResp["userId"])
	assert.NotEmpty(t, authResp["clientId"])
	assert.NotEmpty(t, authResp["sessionId"])
}
