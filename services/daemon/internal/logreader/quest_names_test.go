package logreader

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// resetUnknownQuestKeysSeen replaces the package-level dedup map with a fresh
// zero-value sync.Map so individual tests are isolated.
func resetUnknownQuestKeysSeen() {
	unknownQuestKeysSeen = sync.Map{}
}

// TestQuestNameFromMap_KnownKeys verifies that each confirmed locKey in
// questDisplayNames resolves to its human-readable goal text.
func TestQuestNameFromMap_KnownKeys(t *testing.T) {
	cases := []struct {
		locKey string
		want   string
	}{
		// Original seed entries — verified from Player.log captures
		// (docs/engineering/reference/mtga-log-events.md, 2026-05-29).
		{
			locKey: "Quests/Quest_Nissas_Journey",
			want:   "Cast 25 spells",
		},
		{
			locKey: "Quests/Quest_WinGames",
			want:   "Win 2 games",
		},
		{
			locKey: "Quests/Quest_PlayCards",
			want:   "Play 20 cards",
		},
		// Entries from real Player.log fixture (testdata/real/quest_progress_2026.59.20.log).
		// Display text cross-referenced with MTGA Help Center
		// (https://magic.wizards.com/en/mtgarena/help/quests) and
		// Saffron Olive / MTGGoldfish Arena quest reference (mtggoldfish.com/arena/quests, archived 2025).
		{
			locKey: "Quests/Quest_Simic_Evolution",
			want:   "Cast 30 blue or green spells",
		},
		{
			locKey: "Quests/Quest_Orzhov_Advokist",
			want:   "Cast 20 white or black spells",
		},
		// Two-color guild quests — 500 XP tier (goal: 20 spells).
		// Display text from MTGA Help Center and MTGGoldfish Arena quest reference
		// (mtggoldfish.com/arena/quests, archived 2025).
		{
			locKey: "Quests/Quest_Dimir_Infiltrator",
			want:   "Cast 20 blue or black spells",
		},
		{
			locKey: "Quests/Quest_Golgari_Swarm",
			want:   "Cast 20 black or green spells",
		},
		{
			locKey: "Quests/Quest_Gruul_Clans",
			want:   "Cast 20 red or green spells",
		},
		{
			locKey: "Quests/Quest_Izzet_League",
			want:   "Cast 20 blue or red spells",
		},
		{
			locKey: "Quests/Quest_Azorius_Senate",
			want:   "Cast 20 white or blue spells",
		},
		{
			locKey: "Quests/Quest_Rakdos_Cult",
			want:   "Cast 20 black or red spells",
		},
		{
			locKey: "Quests/Quest_Selesnya_Conclave",
			want:   "Cast 20 white or green spells",
		},
		{
			locKey: "Quests/Quest_Boros_Legion",
			want:   "Cast 20 white or red spells",
		},
	}

	for _, tc := range cases {
		t.Run(tc.locKey, func(t *testing.T) {
			qm := map[string]interface{}{
				"locKey": tc.locKey,
			}
			got := questNameFromMap(qm)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestQuestNameFromMap_UnknownKey verifies that an unrecognised locKey is
// returned verbatim as a fallback rather than an empty string or a crash
// (AC2).
func TestQuestNameFromMap_UnknownKey(t *testing.T) {
	resetUnknownQuestKeysSeen()

	qm := map[string]interface{}{
		"locKey": "Quests/Quest_UnknownFutureSynthetic",
	}
	got := questNameFromMap(qm)
	assert.Equal(t, "Quests/Quest_UnknownFutureSynthetic", got,
		"unknown locKey must fall back to raw value")
}

// TestQuestNameFromMap_UnknownKey_DeduplicatesWarnLog verifies that calling
// questNameFromMap multiple times with the same unknown key records it exactly
// once in the dedup map (dedup requirement from Ray's binding condition).
func TestQuestNameFromMap_UnknownKey_DeduplicatesWarnLog(t *testing.T) {
	resetUnknownQuestKeysSeen()

	const key = "Quests/Quest_DeduplicateSynthetic"
	qm := map[string]interface{}{"locKey": key}

	// Call five times simulating repeated poll cycles.
	for i := 0; i < 5; i++ {
		got := questNameFromMap(qm)
		assert.Equal(t, key, got)
	}

	// After the five calls the key must be present in the dedup map exactly
	// once (LoadOrStore semantics guarantee the store fires only on first call).
	_, seen := unknownQuestKeysSeen.Load(key)
	assert.True(t, seen, "unknown key must be recorded in dedup map after first encounter")
}

// TestQuestNameFromMap_QuestTrackFallback_Unresolved verifies that when
// locKey is absent and questTrack holds an unrecognised value, the raw
// questTrack string is returned.
func TestQuestNameFromMap_QuestTrackFallback_Unresolved(t *testing.T) {
	resetUnknownQuestKeysSeen()

	qm := map[string]interface{}{
		"questTrack": "Default",
	}
	got := questNameFromMap(qm)
	assert.Equal(t, "Default", got)
}

// TestQuestNameFromMap_EmptyMap verifies that an empty quest map returns an
// empty string.
func TestQuestNameFromMap_EmptyMap(t *testing.T) {
	got := questNameFromMap(map[string]interface{}{})
	assert.Equal(t, "", got)
}
