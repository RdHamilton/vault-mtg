package logreader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helpers

func makeMatchCompletedEntry(stateType, matchID string, resultList []interface{}, players []interface{}, eventID string) *LogEntry {
	gameRoomConfig := map[string]interface{}{
		"reservedPlayers": players,
	}
	if eventID != "" {
		gameRoomConfig["eventId"] = eventID
	}

	finalMatchResult := map[string]interface{}{
		"matchId":              matchID,
		"matchCompletedReason": "MatchCompletedReasonType_Success",
		"resultList":           resultList,
	}

	return &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"matchGameRoomStateChangedEvent": map[string]interface{}{
				"gameRoomInfo": map[string]interface{}{
					"stateType":        stateType,
					"gameRoomConfig":   gameRoomConfig,
					"finalMatchResult": finalMatchResult,
				},
			},
		},
	}
}

func ladderResultList() []interface{} {
	return []interface{}{
		map[string]interface{}{
			"scope":         "MatchScope_Game",
			"result":        "ResultType_WinLoss",
			"winningTeamId": float64(2),
			"reason":        "ResultReason_Game",
		},
		map[string]interface{}{
			"scope":         "MatchScope_Match",
			"result":        "ResultType_WinLoss",
			"winningTeamId": float64(2),
			"reason":        "ResultReason_Game",
		},
	}
}

func twoPlayers() []interface{} {
	return []interface{}{
		map[string]interface{}{
			"userId":       "USER_A",
			"playerName":   "PlayerOne",
			"systemSeatId": float64(1),
			"teamId":       float64(1),
		},
		map[string]interface{}{
			"userId":       "USER_B",
			"playerName":   "PlayerTwo",
			"systemSeatId": float64(2),
			"teamId":       float64(2),
		},
	}
}

// ---------------------------------------------------------------------------
// IsMatchCompletedEntry
// ---------------------------------------------------------------------------

func TestIsMatchCompletedEntry_NilEntry(t *testing.T) {
	assert.False(t, IsMatchCompletedEntry(nil))
}

func TestIsMatchCompletedEntry_NotJSON(t *testing.T) {
	entry := &LogEntry{IsJSON: false, Raw: "plain text"}
	assert.False(t, IsMatchCompletedEntry(entry))
}

func TestIsMatchCompletedEntry_NoMatchEvent(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"someKey": "someValue"},
	}
	assert.False(t, IsMatchCompletedEntry(entry))
}

func TestIsMatchCompletedEntry_WrongStateType(t *testing.T) {
	entry := makeMatchCompletedEntry(
		"MatchGameRoomStateType_Playing",
		"match-1", ladderResultList(), twoPlayers(), "Ladder",
	)
	assert.False(t, IsMatchCompletedEntry(entry))
}

func TestIsMatchCompletedEntry_MatchCompleted(t *testing.T) {
	entry := makeMatchCompletedEntry(
		"MatchGameRoomStateType_MatchCompleted",
		"match-1", ladderResultList(), twoPlayers(), "Ladder",
	)
	assert.True(t, IsMatchCompletedEntry(entry))
}

func TestIsMatchCompletedEntry_NoGameRoomInfo(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"matchGameRoomStateChangedEvent": map[string]interface{}{
				"otherKey": "value",
			},
		},
	}
	assert.False(t, IsMatchCompletedEntry(entry))
}

// ---------------------------------------------------------------------------
// ParseMatchCompletedEntry — error paths
// ---------------------------------------------------------------------------

func TestParseMatchCompletedEntry_NilEntry(t *testing.T) {
	_, err := ParseMatchCompletedEntry(nil, "")
	assert.Error(t, err)
}

func TestParseMatchCompletedEntry_NotJSON(t *testing.T) {
	_, err := ParseMatchCompletedEntry(&LogEntry{IsJSON: false, Raw: "text"}, "")
	assert.Error(t, err)
}

func TestParseMatchCompletedEntry_NoMatchEvent(t *testing.T) {
	_, err := ParseMatchCompletedEntry(&LogEntry{
		IsJSON: true,
		JSON:   map[string]interface{}{"other": "val"},
	}, "")
	assert.Error(t, err)
}

func TestParseMatchCompletedEntry_NoGameRoomInfo(t *testing.T) {
	_, err := ParseMatchCompletedEntry(&LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"matchGameRoomStateChangedEvent": map[string]interface{}{},
		},
	}, "")
	assert.Error(t, err)
}

func TestParseMatchCompletedEntry_WrongStateType(t *testing.T) {
	entry := makeMatchCompletedEntry(
		"MatchGameRoomStateType_Playing",
		"match-1", ladderResultList(), twoPlayers(), "Ladder",
	)
	_, err := ParseMatchCompletedEntry(entry, "")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// ParseMatchCompletedEntry — success paths
// ---------------------------------------------------------------------------

func TestParseMatchCompletedEntry_BasicFields(t *testing.T) {
	entry := makeMatchCompletedEntry(
		"MatchGameRoomStateType_MatchCompleted",
		"5e1f2961-3036-4dd4-98ed-4b7810b62e4c",
		ladderResultList(),
		twoPlayers(),
		"Ladder",
	)
	p, err := ParseMatchCompletedEntry(entry, "")
	require.NoError(t, err)
	assert.Equal(t, "5e1f2961-3036-4dd4-98ed-4b7810b62e4c", p.MatchID)
	assert.Equal(t, "Ladder", p.Format)
	assert.Equal(t, 2, p.WinningTeamID)
	require.Len(t, p.ResultList, 2)
}

func TestParseMatchCompletedEntry_ResultListPopulated(t *testing.T) {
	entry := makeMatchCompletedEntry(
		"MatchGameRoomStateType_MatchCompleted",
		"match-abc",
		ladderResultList(),
		twoPlayers(),
		"Ladder",
	)
	p, err := ParseMatchCompletedEntry(entry, "")
	require.NoError(t, err)

	gameResult := p.ResultList[0]
	assert.Equal(t, "MatchScope_Game", gameResult.Scope)
	assert.Equal(t, "ResultType_WinLoss", gameResult.Result)
	assert.Equal(t, 2, gameResult.WinningTeamID)
	assert.Equal(t, "ResultReason_Game", gameResult.Reason)

	matchResult := p.ResultList[1]
	assert.Equal(t, "MatchScope_Match", matchResult.Scope)
	assert.Equal(t, 2, matchResult.WinningTeamID)
}

func TestParseMatchCompletedEntry_OpponentNameNoPlayerID(t *testing.T) {
	// When playerUserID is empty, the first player in reservedPlayers is
	// returned as opponent.
	entry := makeMatchCompletedEntry(
		"MatchGameRoomStateType_MatchCompleted",
		"match-1", ladderResultList(), twoPlayers(), "Ladder",
	)
	p, err := ParseMatchCompletedEntry(entry, "")
	require.NoError(t, err)
	assert.Equal(t, "PlayerOne", p.OpponentName)
}

func TestParseMatchCompletedEntry_OpponentNameWithPlayerID(t *testing.T) {
	// When playerUserID matches the first player, the second player is the opponent.
	entry := makeMatchCompletedEntry(
		"MatchGameRoomStateType_MatchCompleted",
		"match-1", ladderResultList(), twoPlayers(), "Ladder",
	)
	p, err := ParseMatchCompletedEntry(entry, "USER_A")
	require.NoError(t, err)
	assert.Equal(t, "PlayerTwo", p.OpponentName)
}

func TestParseMatchCompletedEntry_FormatFromEventID(t *testing.T) {
	entry := makeMatchCompletedEntry(
		"MatchGameRoomStateType_MatchCompleted",
		"match-draft",
		ladderResultList(),
		twoPlayers(),
		"QuickDraft_SOS_20260430",
	)
	p, err := ParseMatchCompletedEntry(entry, "")
	require.NoError(t, err)
	assert.Equal(t, "QuickDraft_SOS_20260430", p.Format)
}

func TestParseMatchCompletedEntry_NoEventIDEmptyFormat(t *testing.T) {
	entry := makeMatchCompletedEntry(
		"MatchGameRoomStateType_MatchCompleted",
		"match-1", ladderResultList(), twoPlayers(),
		"", // no eventId
	)
	p, err := ParseMatchCompletedEntry(entry, "")
	require.NoError(t, err)
	assert.Empty(t, p.Format)
}

func TestParseMatchCompletedEntry_EmptyResultList(t *testing.T) {
	entry := makeMatchCompletedEntry(
		"MatchGameRoomStateType_MatchCompleted",
		"match-empty", []interface{}{}, twoPlayers(), "Ladder",
	)
	p, err := ParseMatchCompletedEntry(entry, "")
	require.NoError(t, err)
	assert.Equal(t, "match-empty", p.MatchID)
	assert.Empty(t, p.ResultList)
	assert.Equal(t, 0, p.WinningTeamID)
}

func TestParseMatchCompletedEntry_ConcedeReason(t *testing.T) {
	results := []interface{}{
		map[string]interface{}{
			"scope":         "MatchScope_Game",
			"result":        "ResultType_WinLoss",
			"winningTeamId": float64(1),
			"reason":        "ResultReason_Concede",
		},
		map[string]interface{}{
			"scope":         "MatchScope_Match",
			"result":        "ResultType_WinLoss",
			"winningTeamId": float64(1),
			"reason":        "ResultReason_Concede",
		},
	}
	entry := makeMatchCompletedEntry(
		"MatchGameRoomStateType_MatchCompleted",
		"match-concede", results, twoPlayers(), "Ladder",
	)
	p, err := ParseMatchCompletedEntry(entry, "")
	require.NoError(t, err)
	assert.Equal(t, 1, p.WinningTeamID)
	assert.Equal(t, "ResultReason_Concede", p.ResultList[0].Reason)
}

func TestParseMatchCompletedEntry_MissingFinalMatchResult(t *testing.T) {
	// finalMatchResult absent — MatchID and ResultList should be zero values.
	entry := &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"matchGameRoomStateChangedEvent": map[string]interface{}{
				"gameRoomInfo": map[string]interface{}{
					"stateType": "MatchGameRoomStateType_MatchCompleted",
					"gameRoomConfig": map[string]interface{}{
						"eventId":         "Ladder",
						"reservedPlayers": twoPlayers(),
					},
				},
			},
		},
	}
	p, err := ParseMatchCompletedEntry(entry, "")
	require.NoError(t, err)
	assert.Empty(t, p.MatchID)
	assert.Empty(t, p.ResultList)
}

// ---------------------------------------------------------------------------
// Derived fields: Result, PlayerTeamID, PlayerWins, OpponentWins
// ---------------------------------------------------------------------------

func TestParseMatchCompletedEntry_DerivedResult_Win(t *testing.T) {
	// USER_B is team 2 and WinningTeamID is 2 → result = "win"
	entry := makeMatchCompletedEntry(
		"MatchGameRoomStateType_MatchCompleted",
		"match-win", ladderResultList(), twoPlayers(), "Ladder",
	)
	p, err := ParseMatchCompletedEntry(entry, "USER_B")
	require.NoError(t, err)
	assert.Equal(t, "win", p.Result)
	assert.Equal(t, 2, p.PlayerTeamID)
	assert.Equal(t, 1, p.PlayerWins)
	assert.Equal(t, 0, p.OpponentWins)
	assert.Equal(t, "PlayerOne", p.OpponentName)
}

func TestParseMatchCompletedEntry_DerivedResult_Loss(t *testing.T) {
	// USER_A is team 1, WinningTeamID is 2 → result = "loss"
	entry := makeMatchCompletedEntry(
		"MatchGameRoomStateType_MatchCompleted",
		"match-loss", ladderResultList(), twoPlayers(), "Ladder",
	)
	p, err := ParseMatchCompletedEntry(entry, "USER_A")
	require.NoError(t, err)
	assert.Equal(t, "loss", p.Result)
	assert.Equal(t, 1, p.PlayerTeamID)
	assert.Equal(t, 0, p.PlayerWins)
	assert.Equal(t, 1, p.OpponentWins)
	assert.Equal(t, "PlayerTwo", p.OpponentName)
}

func TestParseMatchCompletedEntry_DerivedResult_EmptyWhenPlayerUnknown(t *testing.T) {
	// No playerUserID — cannot determine player team, result stays empty.
	entry := makeMatchCompletedEntry(
		"MatchGameRoomStateType_MatchCompleted",
		"match-unknown", ladderResultList(), twoPlayers(), "Ladder",
	)
	p, err := ParseMatchCompletedEntry(entry, "")
	require.NoError(t, err)
	assert.Empty(t, p.Result)
	assert.Equal(t, 0, p.PlayerTeamID)
}

func TestParseMatchCompletedEntry_PlayerWins_MultiGame(t *testing.T) {
	// 2-1 match: USER_B (team 2) won games 1 and 3; USER_A (team 1) won game 2.
	multiGameResults := []interface{}{
		map[string]interface{}{"scope": "MatchScope_Game", "result": "ResultType_WinLoss", "winningTeamId": float64(2), "reason": "ResultReason_Game"},
		map[string]interface{}{"scope": "MatchScope_Game", "result": "ResultType_WinLoss", "winningTeamId": float64(1), "reason": "ResultReason_Game"},
		map[string]interface{}{"scope": "MatchScope_Game", "result": "ResultType_WinLoss", "winningTeamId": float64(2), "reason": "ResultReason_Game"},
		map[string]interface{}{"scope": "MatchScope_Match", "result": "ResultType_WinLoss", "winningTeamId": float64(2), "reason": "ResultReason_Game"},
	}
	entry := makeMatchCompletedEntry(
		"MatchGameRoomStateType_MatchCompleted",
		"match-2v1", multiGameResults, twoPlayers(), "Ladder",
	)
	p, err := ParseMatchCompletedEntry(entry, "USER_B")
	require.NoError(t, err)
	assert.Equal(t, "win", p.Result)
	assert.Equal(t, 2, p.PlayerWins)
	assert.Equal(t, 1, p.OpponentWins)
}
