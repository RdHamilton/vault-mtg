package logreader

import (
	"log"
	"sync"
)

// questDisplayNames maps MTGA internal locKey values to human-readable goal
// descriptions as displayed in the MTGA UI.
//
// Entries are seeded from verified Player.log captures (see
// docs/engineering/reference/mtga-log-events.md and mtga-log-research.md).
// The locKey format is "Quests/<InternalName>" as emitted by the MTGA client.
//
// TODO(maintenance): Expand this map as new locKey values are confirmed from
// Player.log captures. File new entries in vault-mtg-tickets (see follow-on
// ticket for map coverage expansion). Do not add entries without a verified
// log capture or a citable community reference.
var questDisplayNames = map[string]string{
	// Verified from Player.log captures (docs/engineering/reference/mtga-log-events.md,
	// docs/engineering/reference/mtga-log-research.md — QuestGetQuests response, 2026-05-29).
	"Quests/Quest_Nissas_Journey": "Cast 25 spells",

	// Verified format from log captures; display text from MTGA Help Center
	// (https://magic.wizards.com/en/mtgarena/help/quests) and Arena community
	// reference (reddit.com/r/MagicArena quest FAQ, archived 2025).
	"Quests/Quest_WinGames":  "Win 2 games",
	"Quests/Quest_PlayCards": "Play 20 cards",

	// Two-color guild quests — locKey codenames observed in Player.log
	// (testdata/real/quest_progress_2026.59.20.log) and in community-documented
	// Player.log captures (mtggoldfish.com/arena/quests, archived 2025;
	// MTGA Help Center https://magic.wizards.com/en/mtgarena/help/quests).
	//
	// 750 XP tier: goal = 30 spells of the two guild colors.
	"Quests/Quest_Simic_Evolution": "Cast 30 blue or green spells",

	// 500 XP tier: goal = 20 spells of the two guild colors.
	"Quests/Quest_Orzhov_Advokist":   "Cast 20 white or black spells",
	"Quests/Quest_Dimir_Infiltrator": "Cast 20 blue or black spells",
	"Quests/Quest_Golgari_Swarm":     "Cast 20 black or green spells",
	"Quests/Quest_Gruul_Clans":       "Cast 20 red or green spells",
	"Quests/Quest_Izzet_League":      "Cast 20 blue or red spells",
	"Quests/Quest_Azorius_Senate":    "Cast 20 white or blue spells",
	"Quests/Quest_Rakdos_Cult":       "Cast 20 black or red spells",
	"Quests/Quest_Selesnya_Conclave": "Cast 20 white or green spells",
	"Quests/Quest_Boros_Legion":      "Cast 20 white or red spells",
}

// unknownQuestKeysSeen tracks which unknown locKey values have already been
// logged, so each unknown key is warned exactly once rather than on every poll
// cycle. Keyed by the raw locKey string; value is always struct{}{}.
var unknownQuestKeysSeen sync.Map

// resolveQuestDisplayName looks up key in questDisplayNames and returns the
// human-readable goal text. If key is not in the map, resolveQuestDisplayName
// returns the raw key as a fallback and logs a warning — but only the first
// time that specific key is seen (dedup via unknownQuestKeysSeen).
func resolveQuestDisplayName(key string) string {
	if display, ok := questDisplayNames[key]; ok {
		return display
	}
	// Dedup: log once per unknown key to avoid flooding the daemon log over
	// hours of polling. Load-or-store is atomic; the warning fires only when
	// the key is stored for the first time.
	if _, alreadyLogged := unknownQuestKeysSeen.LoadOrStore(key, struct{}{}); !alreadyLogged {
		log.Printf("[logreader] unknown quest locKey %q — returning raw value as fallback; "+
			"add to questDisplayNames in quest_names.go if confirmed", key)
	}
	return key
}
