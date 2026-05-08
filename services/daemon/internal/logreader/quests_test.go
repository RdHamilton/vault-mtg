package logreader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// questProgressEntry returns a LogEntry that looks like a QuestGetQuests
// response with two active (incomplete) quests.
func questProgressEntry() *LogEntry {
	return &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"quests": []interface{}{
				map[string]interface{}{
					"questId":          "quest-aaa",
					"locKey":           "Quest_Win5Games",
					"goal":             float64(5),
					"startingProgress": float64(0),
					"endingProgress":   float64(3),
					"canSwap":          true,
				},
				map[string]interface{}{
					"questId":          "quest-bbb",
					"locKey":           "Quest_Play10Spells",
					"goal":             float64(10),
					"startingProgress": float64(0),
					"endingProgress":   float64(6),
					"canSwap":          false,
				},
			},
			"canSwap": true,
		},
	}
}

// questCompletedEntry returns a LogEntry that looks like a QuestGetQuests
// response where one quest has endingProgress >= goal.
func questCompletedEntry() *LogEntry {
	return &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"quests": []interface{}{
				map[string]interface{}{
					"questId":          "quest-ccc",
					"locKey":           "Quest_Win3Games",
					"goal":             float64(3),
					"startingProgress": float64(0),
					"endingProgress":   float64(3),
					"canSwap":          true,
					"chestDescription": map[string]interface{}{
						"quantity": float64(750),
					},
				},
			},
			"canSwap": true,
		},
	}
}

// emptyQuestsEntry returns a QuestGetQuests response with an empty quests array.
func emptyQuestsEntry() *LogEntry {
	return &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"quests": []interface{}{},
		},
	}
}

// nonQuestEntry returns a LogEntry that is not quest-related.
func nonQuestEntry() *LogEntry {
	return &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"draftPack": map[string]interface{}{},
		},
	}
}

// --- IsQuestProgressEntry ---

func TestIsQuestProgressEntry_ActiveQuests(t *testing.T) {
	assert.True(t, IsQuestProgressEntry(questProgressEntry()))
}

func TestIsQuestProgressEntry_CompletedQuest(t *testing.T) {
	// A completed-quest entry is also a progress entry (same log message).
	assert.True(t, IsQuestProgressEntry(questCompletedEntry()))
}

func TestIsQuestProgressEntry_EmptyArray(t *testing.T) {
	assert.True(t, IsQuestProgressEntry(emptyQuestsEntry()))
}

func TestIsQuestProgressEntry_NonQuest(t *testing.T) {
	assert.False(t, IsQuestProgressEntry(nonQuestEntry()))
}

func TestIsQuestProgressEntry_Nil(t *testing.T) {
	assert.False(t, IsQuestProgressEntry(nil))
}

func TestIsQuestProgressEntry_NotJSON(t *testing.T) {
	assert.False(t, IsQuestProgressEntry(&LogEntry{IsJSON: false, Raw: "plain"}))
}

// --- IsQuestCompletedEntry ---

func TestIsQuestCompletedEntry_CompletedQuest(t *testing.T) {
	assert.True(t, IsQuestCompletedEntry(questCompletedEntry()))
}

func TestIsQuestCompletedEntry_ActiveQuests(t *testing.T) {
	// No quest has met its goal — not a completed event.
	assert.False(t, IsQuestCompletedEntry(questProgressEntry()))
}

func TestIsQuestCompletedEntry_EmptyArray(t *testing.T) {
	assert.False(t, IsQuestCompletedEntry(emptyQuestsEntry()))
}

func TestIsQuestCompletedEntry_NonQuest(t *testing.T) {
	assert.False(t, IsQuestCompletedEntry(nonQuestEntry()))
}

func TestIsQuestCompletedEntry_Nil(t *testing.T) {
	assert.False(t, IsQuestCompletedEntry(nil))
}

// --- ParseQuestProgressEntry ---

func TestParseQuestProgressEntry_PopulatesQuests(t *testing.T) {
	p, err := ParseQuestProgressEntry(questProgressEntry())
	require.NoError(t, err)
	require.NotNil(t, p)
	require.Len(t, p.Quests, 2)

	q0 := p.Quests[0]
	assert.Equal(t, "quest-aaa", q0.QuestID)
	assert.Equal(t, "Quest_Win5Games", q0.QuestName)
	assert.Equal(t, 3, q0.Progress)
	assert.Equal(t, 5, q0.Goal)
	assert.True(t, q0.CanSwap)

	q1 := p.Quests[1]
	assert.Equal(t, "quest-bbb", q1.QuestID)
	assert.Equal(t, "Quest_Play10Spells", q1.QuestName)
	assert.Equal(t, 6, q1.Progress)
	assert.Equal(t, 10, q1.Goal)
	assert.False(t, q1.CanSwap)
}

func TestParseQuestProgressEntry_EmptyQuests(t *testing.T) {
	p, err := ParseQuestProgressEntry(emptyQuestsEntry())
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Empty(t, p.Quests)
}

func TestParseQuestProgressEntry_NonQuestEntry(t *testing.T) {
	_, err := ParseQuestProgressEntry(nonQuestEntry())
	assert.Error(t, err)
}

func TestParseQuestProgressEntry_NilEntry(t *testing.T) {
	_, err := ParseQuestProgressEntry(nil)
	assert.Error(t, err)
}

func TestParseQuestProgressEntry_NotJSONEntry(t *testing.T) {
	_, err := ParseQuestProgressEntry(&LogEntry{IsJSON: false})
	assert.Error(t, err)
}

func TestParseQuestProgressEntry_QuestNameFallbackToQuestTrack(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"quests": []interface{}{
				map[string]interface{}{
					"questId":        "quest-ddd",
					"questTrack":     "Win5GamesTrack",
					"goal":           float64(5),
					"endingProgress": float64(1),
					"canSwap":        true,
				},
			},
			"canSwap": true,
		},
	}
	p, err := ParseQuestProgressEntry(entry)
	require.NoError(t, err)
	require.Len(t, p.Quests, 1)
	assert.Equal(t, "Win5GamesTrack", p.Quests[0].QuestName)
}

func TestParseQuestProgressEntry_SkipsQuestWithNoID(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"quests": []interface{}{
				map[string]interface{}{
					// No questId — should be skipped.
					"locKey": "Quest_Win5Games",
					"goal":   float64(5),
				},
				map[string]interface{}{
					"questId":        "quest-eee",
					"locKey":         "Quest_Valid",
					"goal":           float64(3),
					"endingProgress": float64(1),
					"canSwap":        true,
				},
			},
			"canSwap": true,
		},
	}
	p, err := ParseQuestProgressEntry(entry)
	require.NoError(t, err)
	require.Len(t, p.Quests, 1)
	assert.Equal(t, "quest-eee", p.Quests[0].QuestID)
}

// --- ParseQuestCompletedEntry ---

func TestParseQuestCompletedEntry_PopulatesFields(t *testing.T) {
	p, err := ParseQuestCompletedEntry(questCompletedEntry())
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "quest-ccc", p.QuestID)
	assert.Equal(t, "Quest_Win3Games", p.QuestName)
	assert.Equal(t, 3, p.Progress)
	assert.Equal(t, 3, p.Goal)
	assert.Equal(t, 750, p.XPReward)
	assert.Equal(t, "progress", p.CompletionSource)
}

func TestParseQuestCompletedEntry_NoCompletedQuest(t *testing.T) {
	_, err := ParseQuestCompletedEntry(questProgressEntry())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no completed quest")
}

func TestParseQuestCompletedEntry_NonQuestEntry(t *testing.T) {
	_, err := ParseQuestCompletedEntry(nonQuestEntry())
	assert.Error(t, err)
}

func TestParseQuestCompletedEntry_NilEntry(t *testing.T) {
	_, err := ParseQuestCompletedEntry(nil)
	assert.Error(t, err)
}

func TestParseQuestCompletedEntry_NoXPReward(t *testing.T) {
	entry := &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"quests": []interface{}{
				map[string]interface{}{
					"questId":        "quest-fff",
					"locKey":         "Quest_SomeQuest",
					"goal":           float64(2),
					"endingProgress": float64(2),
					"canSwap":        true,
					// No chestDescription — xp_reward should default to 0.
				},
			},
			"canSwap": true,
		},
	}
	p, err := ParseQuestCompletedEntry(entry)
	require.NoError(t, err)
	assert.Equal(t, 0, p.XPReward)
	assert.Equal(t, "quest-fff", p.QuestID)
}

func TestParseQuestCompletedEntry_PicksFirstCompleted(t *testing.T) {
	// Two completed quests — first one should be returned.
	entry := &LogEntry{
		IsJSON: true,
		JSON: map[string]interface{}{
			"quests": []interface{}{
				map[string]interface{}{
					"questId":        "quest-g1",
					"locKey":         "Quest_First",
					"goal":           float64(1),
					"endingProgress": float64(1),
					"canSwap":        true,
				},
				map[string]interface{}{
					"questId":        "quest-g2",
					"locKey":         "Quest_Second",
					"goal":           float64(1),
					"endingProgress": float64(1),
					"canSwap":        true,
				},
			},
			"canSwap": true,
		},
	}
	p, err := ParseQuestCompletedEntry(entry)
	require.NoError(t, err)
	assert.Equal(t, "quest-g1", p.QuestID)
}
