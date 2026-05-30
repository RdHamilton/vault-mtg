package logreader

// real_fixture_golden_test.go — golden assertions against anonymized real
// MTGA Player.log fixtures for MTGA client version 2026.59.20.
//
// Fixtures live in testdata/real/ and were captured + sanitized by Tim per
// ADR-041 G3.  Each test loads one fixture file, runs it through the relevant
// classifier + parser, and asserts the expected event shape.
//
// AC5: Tim has reviewed all fixture files for PII before this PR merges.
// See testdata/real/MANIFEST.md for provenance and sanitization record.

import (
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadRealFixture reads a file from testdata/real/ and returns a parsed LogEntry.
// The fixture files are single-line JSON, mirroring the format that Reader
// produces for each log line.
func loadRealFixture(t *testing.T, filename string) *LogEntry {
	t.Helper()
	raw, err := os.ReadFile("testdata/real/" + filename)
	require.NoErrorf(t, err, "open real fixture %s", filename)
	entry := &LogEntry{Raw: string(raw)}
	entry.parseJSON()
	require.Truef(t, entry.IsJSON, "real fixture %s must parse as JSON", filename)
	return entry
}

// ---------------------------------------------------------------------------
// authenticate — player_authenticated
// ---------------------------------------------------------------------------

// TestRealFixture_Authenticate_2026_59_20 asserts that the authenticate fixture
// parses as JSON and contains the expected authenticateResponse structure.
// Wire format: {"authenticateResponse":{"screenName":...,"userId":...,...}}
func TestRealFixture_Authenticate_2026_59_20(t *testing.T) {
	entry := loadRealFixture(t, "authenticate_2026.59.20.log")

	authResp, ok := entry.JSON["authenticateResponse"].(map[string]interface{})
	require.True(t, ok, "entry must contain authenticateResponse object")

	// Fixture uses sanitized identifiers per MANIFEST.md.
	screenName, _ := authResp["screenName"].(string)
	assert.NotEmpty(t, screenName, "screenName must be non-empty")

	// All four fields present in the 2026.59.20 wire format.
	assert.NotEmpty(t, authResp["userId"], "userId must be present")
	assert.NotEmpty(t, authResp["clientId"], "clientId must be present")
	assert.NotEmpty(t, authResp["sessionId"], "sessionId must be present")
}

// ---------------------------------------------------------------------------
// inventory_updated
// ---------------------------------------------------------------------------

// TestRealFixture_InventoryUpdated_2026_59_20 asserts that the inventory
// fixture classifies and parses correctly with real gem/gold/wildcard values
// from a 2026.59.20 session.
func TestRealFixture_InventoryUpdated_2026_59_20(t *testing.T) {
	entry := loadRealFixture(t, "inventory_updated_2026.59.20.log")

	require.True(t, IsInventoryEntry(entry), "inventory fixture must be classified as inventory")

	p, err := ParseInventoryEntry(entry)
	require.NoError(t, err)
	require.NotNil(t, p)

	// Values are real game state from the capture session (non-PII).
	assert.Equal(t, 2135, p.Gems)
	assert.Equal(t, 76875, p.Gold)
	assert.Equal(t, 144, p.TotalVaultProgress)
	assert.Equal(t, 39, p.WildCardCommons)
	assert.Equal(t, 25, p.WildCardUncommons)
	assert.Equal(t, 9, p.WildCardRares)
	assert.Equal(t, 20, p.WildCardMythics)

	// Boosters array is empty in this capture (no packs held).
	assert.Empty(t, p.Boosters, "boosters must be empty in this capture session")
}

// ---------------------------------------------------------------------------
// quest_progress
// ---------------------------------------------------------------------------

// TestRealFixture_QuestProgress_2026_59_20 asserts that the quest fixture
// classifies and parses correctly with two active quests.  Quest UUIDs are
// sanitized stable fakes per MANIFEST.md; locKeys are real game values.
func TestRealFixture_QuestProgress_2026_59_20(t *testing.T) {
	entry := loadRealFixture(t, "quest_progress_2026.59.20.log")

	require.True(t, IsQuestProgressEntry(entry), "quest fixture must be classified as quest progress")

	p, err := ParseQuestProgressEntry(entry)
	require.NoError(t, err)
	require.NotNil(t, p)

	require.Len(t, p.Quests, 2, "fixture has two active quests")

	// Sort by QuestID for deterministic comparison.
	sort.Slice(p.Quests, func(i, j int) bool {
		return p.Quests[i].QuestID < p.Quests[j].QuestID
	})

	q0 := p.Quests[0]
	assert.Equal(t, "00000001-0000-4000-8000-000000000001", q0.QuestID)
	assert.Equal(t, 30, q0.Goal)
	// locKey added to questDisplayNames in vault-mtg-tickets#235.
	assert.Equal(t, "Cast 30 blue or green spells", q0.QuestName)
	assert.Equal(t, 3, q0.Progress)
	assert.True(t, q0.CanSwap)

	q1 := p.Quests[1]
	assert.Equal(t, "00000001-0000-4000-8000-000000000002", q1.QuestID)
	assert.Equal(t, 20, q1.Goal)
	// locKey added to questDisplayNames in vault-mtg-tickets#235.
	assert.Equal(t, "Cast 20 white or black spells", q1.QuestName)
	assert.Equal(t, 6, q1.Progress)
	assert.True(t, q1.CanSwap)
}

// TestRealFixture_QuestProgress_NotCompleted asserts that neither quest in the
// fixture has met its completion threshold (endingProgress < goal for both).
func TestRealFixture_QuestProgress_NotCompleted(t *testing.T) {
	entry := loadRealFixture(t, "quest_progress_2026.59.20.log")
	// Neither quest has endingProgress >= goal in this capture.
	assert.False(t, IsQuestCompletedEntry(entry),
		"no quest is completed in the fixture — endingProgress < goal for both")
}

// ---------------------------------------------------------------------------
// match_completed
// ---------------------------------------------------------------------------

// TestRealFixture_MatchCompleted_2026_59_20 asserts that the match_completed
// fixture classifies and parses correctly.
//
// Wire format note (2026.59.20): eventId is present at the top-level
// gameRoomConfig in this FORMAT-CONFIRMED fixture but NOT inside the
// reservedPlayers[] entries.  The parser (after #201's fix) reads eventId only
// from reservedPlayers[] entries, so Format will be empty for this fixture.
// The drift canary will fire if MTGA starts emitting a different shape.
func TestRealFixture_MatchCompleted_2026_59_20(t *testing.T) {
	entry := loadRealFixture(t, "match_completed_2026.59.20.log")

	require.True(t, IsMatchCompletedEntry(entry),
		"match_completed fixture must be classified as match completed")

	// Parse with the local player ID from the fixture (sanitized stable fake).
	p, err := ParseMatchCompletedEntry(entry, "00000000-0000-4000-8000-000000000010")
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "00000000-0000-4000-8000-000000000001", p.MatchID)

	// Match result: 3 result entries (2 game-scope + 1 match-scope).
	require.Len(t, p.ResultList, 3, "fixture has two game results and one match result")

	// WinningTeamID derived from the MatchScope_Match entry.
	assert.Equal(t, 1, p.WinningTeamID)

	// Local player is team 1 (systemSeatId 1).
	assert.Equal(t, 1, p.PlayerTeamID)
	assert.Equal(t, "win", p.Result)
	assert.Equal(t, "Opponent#00002", p.OpponentName)

	// Game results: team 1 won game 1, team 2 won game 2 (2-game fixture).
	assert.Equal(t, 1, p.PlayerWins)
	assert.Equal(t, 1, p.OpponentWins)
}

// ---------------------------------------------------------------------------
// draft_pack
// ---------------------------------------------------------------------------

// TestRealFixture_DraftPack_2026_59_20 asserts that the draft_pack fixture
// parses correctly.  GRP IDs are real card IDs from the MTGA card database
// (non-PII per ADR-041).
func TestRealFixture_DraftPack_2026_59_20(t *testing.T) {
	entry := loadRealFixture(t, "draft_pack_2026.59.20.log")

	_, hasDraftPack := entry.JSON["draftPack"]
	require.True(t, hasDraftPack, "draft_pack fixture must contain draftPack key")

	p, err := ParseDraftPack(entry)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "PremierDraft_FIN", p.CourseName)
	assert.Equal(t, 1, p.DraftPack.SelfPick)

	// Pack contains 13 real card GRP IDs.
	require.Len(t, p.DraftPack.PackCards, 13)

	// Spot-check a sample of known GRP IDs from the fixture.
	packSet := make(map[int]bool, len(p.DraftPack.PackCards))
	for _, id := range p.DraftPack.PackCards {
		packSet[id] = true
	}
	assert.True(t, packSet[67108], "GRP 67108 must be present in pack")
	assert.True(t, packSet[77460], "GRP 77460 must be present in pack")
	assert.True(t, packSet[73778], "GRP 73778 must be present in pack")
}

// ---------------------------------------------------------------------------
// draft_pick
// ---------------------------------------------------------------------------

// TestRealFixture_DraftPick_2026_59_20 asserts that the draft_pick fixture
// parses correctly.  Pick 0 of pack 0 — the player picked GRP 67108.
func TestRealFixture_DraftPick_2026_59_20(t *testing.T) {
	entry := loadRealFixture(t, "draft_pick_2026.59.20.log")

	_, hasPicked := entry.JSON["pickedCards"]
	require.True(t, hasPicked, "draft_pick fixture must contain pickedCards key")

	p, err := ParseDraftPick(entry)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "PremierDraft_FIN", p.CourseName)
	assert.Equal(t, 0, p.PackNumber)
	assert.Equal(t, 0, p.PickNumber)
	require.Len(t, p.PickedCards, 1)
	assert.Equal(t, 67108, p.PickedCards[0])
}

// ---------------------------------------------------------------------------
// collection_updated
// ---------------------------------------------------------------------------

// TestRealFixture_CollectionUpdated_2026_59_20 asserts that the collection
// fixture classifies and parses correctly.  The fixture is a flat GRP→qty map
// as returned by PlayerInventoryGetPlayerCardsV3.  GRP IDs are non-PII per
// ADR-041.
func TestRealFixture_CollectionUpdated_2026_59_20(t *testing.T) {
	entry := loadRealFixture(t, "collection_updated_2026.59.20.log")

	require.True(t, IsCollectionEntry(entry),
		"collection_updated fixture must be classified as collection snapshot")

	p, err := ParseCollectionEntry(entry)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.False(t, p.IsDelta, "full collection snapshot is not a delta")

	// Fixture contains 20 GRP→qty pairs.
	require.Len(t, p.Cards, 20, "fixture must have 20 collection entries")

	// Build a lookup map for spot-check assertions.
	cardMap := make(map[int]int, len(p.Cards))
	for _, c := range p.Cards {
		cardMap[c.ArenaID] = c.Count
	}

	// Spot-check known GRP IDs from the collection_updated fixture.
	assert.Equal(t, 4, cardMap[67108], "GRP 67108 qty must be 4")
	assert.Equal(t, 1, cardMap[67128], "GRP 67128 qty must be 1")
	assert.Equal(t, 4, cardMap[73778], "GRP 73778 qty must be 4")
	assert.Equal(t, 1, cardMap[79426], "GRP 79426 qty must be 1")
}
