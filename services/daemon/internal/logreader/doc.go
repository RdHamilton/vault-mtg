// Package logreader provides fsnotify-based polling of the MTGA Player.log file,
// line-by-line JSON parsing, and typed payload extraction for tracked event types.
//
// # Log Preservation Warning
//
// MTGA overwrites Player.log every time it starts. If the daemon was not running
// when MTGA launched, all events written since the previous daemon run are
// permanently lost. The preservation mechanism (preservation.go) is not yet
// functioning correctly; see GitHub issue #1014.
//
// # Parsed Event Types
//
// The daemon currently classifies and forwards the following event types:
//
//   - draft.pack    — Premier Draft.Notify (draftId+PackCards, #338) OR BotDraft
//     status pack (CurrentModule=BotDraft + stringified Payload, #337)
//   - draft.pick    — Premier EventPlayerDraftMakePick (request w/ DraftId, #338)
//     OR BotDraftDraftPick (request w/ PickInfo, #337)
//   - draft.started — scene transition to "Draft"; key: "toSceneName"=="Draft"
//   - draft.ended   — scene transition away from "Draft"; key: "fromSceneName"=="Draft"
//   - match.completed — CurrentEventState == "MatchCompleted"
//   - match.started   — CurrentEventState == "MatchInProgress"
//   - player.authenticated — presence of "authenticateResponse" key
//   - player.rank_updated  — presence of "rankClass" key
//
// # Known Parsing Gaps
//
// The following MTGA log event types are emitted to Player.log but are NOT
// currently parsed or forwarded by the daemon. They are tracked for future
// implementation:
//
//   - inventory.updated  — PlayerInventoryUpdate message; keys: "gems", "gold",
//     "wcCommon", "wcUncommon", "wcRare", "wcMythic", "boosters".
//     The PlayerInventory model in models.go defines the shape but
//     classifyEntry never matches it.
//
//   - quest.progress / quest.completed — DailyQuestUpdate messages; keys:
//     "questId", "currentProgress", "totalProgress". No parser or classifier
//     exists today.
//
//   - collection.updated — GetPlayerCardsV3 response; a large JSON object
//     mapping grpId → count. Emitted after every game session. No parser
//     or classifier exists today.
//
//   - deck.updated — Deck.UpdateDeckV3 response; contains the full deck list.
//     No parser or classifier exists today.
//
//   - match.game_started / match.game_ended — individual game boundaries within
//     a match (GREMessageType_GameStateMessage). No classifier exists today.
//
// Until these gaps are filled, features that depend on inventory, quest, collection,
// or deck change data must rely on the BFF sync path rather than daemon events.
package logreader
