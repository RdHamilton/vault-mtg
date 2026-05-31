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
// Wire format (2026.59.20): {"authenticateResponse":{"clientId":...,"sessionId":...,"screenName":...}}
// There is NO "userId" or "accountId" key — clientId is the player join key.
// clientId equals reservedPlayers[].userId in matchGameRoomStateChangedEvent.
func TestRealFixture_Authenticate_2026_59_20(t *testing.T) {
	entry := loadRealFixture(t, "authenticate_2026.59.20.log")

	authResp, ok := entry.JSON["authenticateResponse"].(map[string]interface{})
	require.True(t, ok, "entry must contain authenticateResponse object")

	// Fixture uses sanitized identifiers per MANIFEST.md.
	screenName, _ := authResp["screenName"].(string)
	assert.NotEmpty(t, screenName, "screenName must be non-empty")

	// Real 2026.59.20 wire format: clientId and sessionId present, no userId/accountId.
	assert.NotEmpty(t, authResp["clientId"], "clientId must be present (join key for match events)")
	assert.NotEmpty(t, authResp["sessionId"], "sessionId must be present")
	assert.Nil(t, authResp["userId"], "userId must NOT be present in real 2026.59.20 wire format")
	assert.Nil(t, authResp["accountId"], "accountId must NOT be present in real 2026.59.20 wire format")
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
// parses correctly.  Promoted to REAL Premier Draft.Notify shape (#338) from
// the 2026.59.20 Premier capture.  GRP IDs are real card IDs from the MTGA
// card database (non-PII per ADR-041); the draftId is a sanitized stable fake.
func TestRealFixture_DraftPack_2026_59_20(t *testing.T) {
	entry := loadRealFixture(t, "draft_pack_2026.59.20.log")

	_, hasDraftID := entry.JSON["draftId"]
	require.True(t, hasDraftID, "Premier draft_pack fixture must contain draftId key")
	_, hasPackCards := entry.JSON["PackCards"]
	require.True(t, hasPackCards, "Premier draft_pack fixture must contain PackCards key")

	p, err := ParsePremierDraftNotify(entry)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "00000000-0000-4000-8000-0000000003a8", p.DraftID)
	// Premier carries no CourseName.
	assert.Equal(t, "", p.CourseName)
	// Pack 1 / Pick 1 → cumulative 1-based SelfPick = 1.
	assert.Equal(t, 1, p.DraftPack.SelfPick)

	// Pack contains 14 real card GRP IDs (real Premier pack-1-pick-1).
	require.Len(t, p.DraftPack.PackCards, 14)

	// Spot-check a sample of known GRP IDs from the real corpus.
	packSet := make(map[int]bool, len(p.DraftPack.PackCards))
	for _, id := range p.DraftPack.PackCards {
		packSet[id] = true
	}
	assert.True(t, packSet[102614], "GRP 102614 must be present in pack")
	assert.True(t, packSet[102647], "GRP 102647 must be present in pack")
	assert.True(t, packSet[102714], "GRP 102714 must be present in pack")
}

// ---------------------------------------------------------------------------
// draft_pick
// ---------------------------------------------------------------------------

// TestRealFixture_DraftPick_2026_59_20 asserts that the draft_pick fixture
// parses correctly.  Promoted to REAL Premier EventPlayerDraftMakePick shape
// (#338) from the 2026.59.20 Premier capture: pack 1 / pick 1 (0-based 0/0) —
// the player picked GRP 102647.  draftId is a sanitized stable fake.
func TestRealFixture_DraftPick_2026_59_20(t *testing.T) {
	entry := loadRealFixture(t, "draft_pick_2026.59.20.log")

	req, hasReq := entry.JSON["request"].(string)
	require.True(t, hasReq, "Premier draft_pick fixture must contain request string")
	require.Contains(t, req, `"DraftId"`, "request must carry inner DraftId")

	p, err := ParsePremierDraftMakePick(entry)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "00000000-0000-4000-8000-0000000003a8", p.DraftID)
	// Premier carries no CourseName.
	assert.Equal(t, "", p.CourseName)
	assert.Equal(t, 0, p.PackNumber)
	assert.Equal(t, 0, p.PickNumber)
	require.Len(t, p.PickedCards, 1)
	assert.Equal(t, 102647, p.PickedCards[0])
}

// ---------------------------------------------------------------------------
// botdraft_pack — BotDraft (QuickDraft) draft.pack (#337)
// ---------------------------------------------------------------------------

// TestRealFixture_BotDraftPack_2026_59_20 asserts that the BotDraft pack fixture
// (CurrentModule=BotDraft + stringified Payload) parses correctly. Real
// QuickDraft_SOS_20260526 pack 0 / pick 0, 14 real grpIds (non-PII per ADR-041).
// There is no draftId on the BotDraft wire — the session is keyed by EventName.
func TestRealFixture_BotDraftPack_2026_59_20(t *testing.T) {
	entry := loadRealFixture(t, "botdraft_pack_2026.59.20.log")

	mod, ok := entry.JSON["CurrentModule"].(string)
	require.True(t, ok, "BotDraft pack fixture must contain CurrentModule key")
	require.Equal(t, "BotDraft", mod)
	_, hasPayload := entry.JSON["Payload"].(string)
	require.True(t, hasPayload, "BotDraft pack fixture must contain a stringified Payload")

	p, err := ParseBotDraftStatusPack(entry)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "QuickDraft_SOS_20260526", p.CourseName)
	assert.Equal(t, "", p.DraftID)
	// Pack 0 / Pick 0 → cumulative 1-based SelfPick = 1.
	assert.Equal(t, 1, p.DraftPack.SelfPick)

	require.Len(t, p.DraftPack.PackCards, 14)
	packSet := make(map[int]bool, len(p.DraftPack.PackCards))
	for _, id := range p.DraftPack.PackCards {
		packSet[id] = true
	}
	assert.True(t, packSet[102470], "GRP 102470 must be present in pack")
	assert.True(t, packSet[102704], "GRP 102704 must be present in pack")
	assert.True(t, packSet[102715], "GRP 102715 must be present in pack")
}

// ---------------------------------------------------------------------------
// botdraft_pick — BotDraft (QuickDraft) draft.pick (#337)
// ---------------------------------------------------------------------------

// TestRealFixture_BotDraftPick_2026_59_20 asserts that the BotDraftDraftPick
// fixture parses correctly. Real QuickDraft_SOS_20260526 pack 0 / pick 0 — the
// player picked GRP 102704. The outer correlation id is a sanitized stable fake;
// grpIds are real (non-PII per ADR-041).
func TestRealFixture_BotDraftPick_2026_59_20(t *testing.T) {
	entry := loadRealFixture(t, "botdraft_pick_2026.59.20.log")

	req, hasReq := entry.JSON["request"].(string)
	require.True(t, hasReq, "BotDraft pick fixture must contain request string")
	require.Contains(t, req, `"PickInfo"`, "request must carry a PickInfo block")

	p, err := ParseBotDraftPick(entry)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "QuickDraft_SOS_20260526", p.CourseName)
	assert.Equal(t, "", p.DraftID)
	assert.Equal(t, 0, p.PackNumber)
	assert.Equal(t, 0, p.PickNumber)
	require.Len(t, p.PickedCards, 1)
	assert.Equal(t, 102704, p.PickedCards[0])
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
