package logreader_test

// contract_emit_test.go — Layer 2 contract/serialisation gate (ADR-042).
//
// This test validates that the daemon's event-assembly code produces JSON
// payloads that are semantically correct for every event class in the corpus.
// It catches the class of failure that bit us in PR #201: a required
// enrichment field emitted with a wrong/empty value that passed the type
// system but broke downstream projection.
//
// Fixtures are read exclusively from the committed corpus. No inline JSON.
// See: vault-mtg-docs/engineering/architecture/adr/2026-05-ADR-042-*.md

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/contract"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/dispatch"
	"github.com/RdHamilton/vault-mtg/services/daemon/internal/logreader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// corpusDir is the path from the package directory to the corpus root.
// Go's test runner sets the working directory to the package directory before
// running any test in that package, making this relative path stable across
// all invocation modes (go test ./..., go test ./internal/logreader/, etc.).
const corpusDir = "../../testdata/corpus"

// Test account and session IDs used when calling dispatch.BuildEvent.
// These are test-only constants — not real account or session identifiers.
const (
	testAccountID = "test-account-001"
	testSessionID = "22222222-0000-0000-0000-000000000001"
	// localPlayerID is the stable fake MTGA userId of the local player in the
	// match-completed corpus fixture (#262 promotion: the fixture is now REAL,
	// and the local player's account token is keyed to the same stable fake as
	// the corrected player-authenticated fixture's clientId == reservedPlayers[].userId).
	// ParseMatchCompletedEntry requires it to derive win/loss and identify the opponent.
	localPlayerID = "TESTACCOUNT000000000000003"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// loadCorpusLogEntry reads a corpus player-log fixture and returns the first
// parsed LogEntry. Each corpus player-log file is a single-line JSON entry.
func loadCorpusLogEntry(t *testing.T, rel string) *logreader.LogEntry {
	t.Helper()
	path := filepath.Join(corpusDir, rel)
	r, err := logreader.NewReader(path)
	require.NoErrorf(t, err, "[contract-gate] open corpus fixture %s", rel)
	t.Cleanup(func() { _ = r.Close() })
	entry, err := r.ReadEntry()
	require.NotEqual(t, io.EOF, err, "[contract-gate] corpus fixture %s must have at least one entry", rel)
	require.NoErrorf(t, err, "[contract-gate] read corpus fixture %s", rel)
	require.NotNil(t, entry)
	return entry
}

// loadCorpusDaemonEvent reads a corpus daemon-emit fixture and unmarshals it
// into a contract.DaemonEvent.
func loadCorpusDaemonEvent(t *testing.T, rel string) contract.DaemonEvent {
	t.Helper()
	path := filepath.Join(corpusDir, rel)
	data, err := os.ReadFile(path)
	require.NoErrorf(t, err, "[contract-gate] open corpus fixture %s", rel)
	var evt contract.DaemonEvent
	require.NoErrorf(t, json.Unmarshal(data, &evt),
		"[contract-gate] unmarshal corpus fixture %s into DaemonEvent", rel)
	return evt
}

// assertEnvelopeFields checks required DaemonEvent envelope fields.
// Sequence and EventID are NOT asserted here:
//   - Sequence is assigned by dispatch.Send/SendOrBuffer (per-session counter),
//     not by BuildEvent.
//   - EventID is not populated by the current daemon code path; it is present
//     in the corpus fixtures as a manually crafted stable fake but is not
//     emitted by BuildEvent at runtime. Asserting it would always fail.
func assertEnvelopeFields(t *testing.T, evt contract.DaemonEvent, eventType string) {
	t.Helper()
	assert.Equalf(t, eventType, evt.Type,
		"[contract-gate] envelope.type expected %q; corpus fixture: testdata/corpus/daemon-emit/%s.json; parser source: services/daemon/internal/daemon/service.go; update protocol: see ADR-042 Layer 2 refresh protocol",
		eventType, eventType)
	assert.NotEmptyf(t, evt.AccountID,
		"[contract-gate] envelope.account_id must be non-empty; corpus fixture: testdata/corpus/daemon-emit/%s.json",
		eventType)
	assert.NotEmptyf(t, evt.SessionID,
		"[contract-gate] envelope.session_id must be non-empty; corpus fixture: testdata/corpus/daemon-emit/%s.json",
		eventType)
	assert.Falsef(t, evt.OccurredAt.IsZero(),
		"[contract-gate] envelope.occurred_at must be non-zero; corpus fixture: testdata/corpus/daemon-emit/%s.json",
		eventType)
	assert.NotEmptyf(t, evt.Payload,
		"[contract-gate] envelope.payload must be non-empty; corpus fixture: testdata/corpus/daemon-emit/%s.json",
		eventType)
}

// errorf formats a detailed [contract-gate] failure message naming the fixture,
// field, source file, and update protocol.
func contractError(t *testing.T, fixture, field string, expected, got interface{}, sourceFile string) {
	t.Helper()
	t.Errorf(
		"[contract-gate] daemon emitted unexpected shape for corpus fixture %s:\n"+
			"  field %s expected %v, got %v\n"+
			"  corpus fixture: testdata/corpus/daemon-emit/%s\n"+
			"  parser source: services/daemon/internal/logreader/%s\n"+
			"  If the MTGA log format changed: update the corpus (see ADR-042 Layer 2 refresh protocol).\n"+
			"  If the daemon assembly changed: fix the parser to match the corpus OR update the corpus with a new MTGA version.",
		fixture, field, expected, got, fixture, sourceFile,
	)
}

// ---------------------------------------------------------------------------
// Positive path: round-trip parse → BuildEvent → assert semantic fields
// ---------------------------------------------------------------------------

// TestContractEmit_MatchCompleted round-trips the match-completed corpus
// player-log fixture through ParseMatchCompletedEntry + BuildEvent and asserts
// that the resulting DaemonEvent payload carries the expected semantic fields.
func TestContractEmit_MatchCompleted(t *testing.T) {
	entry := loadCorpusLogEntry(t, "player-log/match-completed.log")

	require.Truef(t, logreader.IsMatchCompletedEntry(entry),
		"[contract-gate] corpus player-log/match-completed.log must classify as match.completed")

	p, err := logreader.ParseMatchCompletedEntry(entry, localPlayerID)
	require.NoErrorf(t, err,
		"[contract-gate] ParseMatchCompletedEntry failed for corpus player-log/match-completed.log")

	evt, err := dispatch.BuildEvent("match.completed", testAccountID, testSessionID, p)
	require.NoErrorf(t, err,
		"[contract-gate] dispatch.BuildEvent failed for match.completed")

	assertEnvelopeFields(t, evt, "match.completed")

	var payload contract.MatchCompletedPayload
	require.NoErrorf(t, json.Unmarshal(evt.Payload, &payload),
		"[contract-gate] unmarshal match.completed payload into MatchCompletedPayload")

	// Load corpus golden shape to confirm field alignment.
	corpusEvt := loadCorpusDaemonEvent(t, "daemon-emit/match-completed.json")
	var corpusPayload contract.MatchCompletedPayload
	require.NoErrorf(t, json.Unmarshal(corpusEvt.Payload, &corpusPayload),
		"[contract-gate] unmarshal corpus match-completed.json payload")

	// Assert semantic fields against corpus shape.
	if payload.MatchID != corpusPayload.MatchID {
		contractError(t, "match-completed.json", "match_id", corpusPayload.MatchID, payload.MatchID, "match.go")
	}
	if payload.WinningTeamID != corpusPayload.WinningTeamID {
		contractError(t, "match-completed.json", "winning_team_id", corpusPayload.WinningTeamID, payload.WinningTeamID, "match.go")
	}
	if payload.Format != corpusPayload.Format {
		contractError(t, "match-completed.json", "format", corpusPayload.Format, payload.Format, "match.go")
	}
	if payload.Result != corpusPayload.Result {
		contractError(t, "match-completed.json", "result", corpusPayload.Result, payload.Result, "match.go")
	}
	if payload.PlayerTeamID != corpusPayload.PlayerTeamID {
		contractError(t, "match-completed.json", "player_team_id", corpusPayload.PlayerTeamID, payload.PlayerTeamID, "match.go")
	}
	if payload.PlayerWins != corpusPayload.PlayerWins {
		contractError(t, "match-completed.json", "player_wins", corpusPayload.PlayerWins, payload.PlayerWins, "match.go")
	}
	if payload.OpponentWins != corpusPayload.OpponentWins {
		contractError(t, "match-completed.json", "opponent_wins", corpusPayload.OpponentWins, payload.OpponentWins, "match.go")
	}
	assert.GreaterOrEqualf(t, len(payload.ResultList), 1,
		"[contract-gate] match-completed result_list must have at least 1 entry; corpus fixture: daemon-emit/match-completed.json; parser source: match.go")
}

// TestContractEmit_QuestProgress round-trips the quest-progress corpus
// player-log fixture through ParseQuestProgressEntry + BuildEvent.
func TestContractEmit_QuestProgress(t *testing.T) {
	entry := loadCorpusLogEntry(t, "player-log/quest-progress.log")

	require.Truef(t, logreader.IsQuestProgressEntry(entry),
		"[contract-gate] corpus player-log/quest-progress.log must classify as quest.progress")

	p, err := logreader.ParseQuestProgressEntry(entry)
	require.NoErrorf(t, err,
		"[contract-gate] ParseQuestProgressEntry failed for corpus player-log/quest-progress.log")

	evt, err := dispatch.BuildEvent("quest.progress", testAccountID, testSessionID, p)
	require.NoErrorf(t, err,
		"[contract-gate] dispatch.BuildEvent failed for quest.progress")

	assertEnvelopeFields(t, evt, "quest.progress")

	var payload contract.QuestProgressPayload
	require.NoErrorf(t, json.Unmarshal(evt.Payload, &payload),
		"[contract-gate] unmarshal quest.progress payload into QuestProgressPayload")

	assert.GreaterOrEqualf(t, len(payload.Quests), 1,
		"[contract-gate] quest.progress quests array must have at least 1 entry; corpus fixture: daemon-emit/quest-progress.json; parser source: quests.go")

	for i, q := range payload.Quests {
		assert.NotEmptyf(t, q.QuestID,
			"[contract-gate] quest.progress quests[%d].quest_id must be non-empty; corpus fixture: daemon-emit/quest-progress.json; parser source: quests.go; If the MTGA log format changed: update the corpus (see ADR-042 Layer 2 refresh protocol). If the daemon assembly changed: fix the parser to match the corpus OR update the corpus with a new MTGA version.",
			i)
	}

	// Load corpus golden shape to confirm quest IDs and progress match.
	corpusEvt := loadCorpusDaemonEvent(t, "daemon-emit/quest-progress.json")
	var corpusPayload contract.QuestProgressPayload
	require.NoErrorf(t, json.Unmarshal(corpusEvt.Payload, &corpusPayload),
		"[contract-gate] unmarshal corpus quest-progress.json payload")

	if len(payload.Quests) == len(corpusPayload.Quests) {
		for i := range payload.Quests {
			if payload.Quests[i].QuestID != corpusPayload.Quests[i].QuestID {
				contractError(t, "quest-progress.json", "quests["+string(rune('0'+i))+"`.quest_id", corpusPayload.Quests[i].QuestID, payload.Quests[i].QuestID, "quests.go")
			}
		}
	} else {
		t.Errorf("[contract-gate] quest.progress quests array length mismatch: corpus has %d, daemon emitted %d; corpus fixture: daemon-emit/quest-progress.json; parser source: quests.go",
			len(corpusPayload.Quests), len(payload.Quests))
	}
}

// TestContractEmit_DeckUpdated round-trips the deck-updated corpus player-log
// fixture through ParseDeckEntry + BuildEvent.
func TestContractEmit_DeckUpdated(t *testing.T) {
	entry := loadCorpusLogEntry(t, "player-log/deck-updated.log")

	require.Truef(t, logreader.IsDeckEntry(entry),
		"[contract-gate] corpus player-log/deck-updated.log must classify as deck.updated")

	p, err := logreader.ParseDeckEntry(entry)
	require.NoErrorf(t, err,
		"[contract-gate] ParseDeckEntry failed for corpus player-log/deck-updated.log")

	evt, err := dispatch.BuildEvent("deck.updated", testAccountID, testSessionID, p)
	require.NoErrorf(t, err,
		"[contract-gate] dispatch.BuildEvent failed for deck.updated")

	assertEnvelopeFields(t, evt, "deck.updated")

	var payload contract.DeckUpdatedPayload
	require.NoErrorf(t, json.Unmarshal(evt.Payload, &payload),
		"[contract-gate] unmarshal deck.updated payload into DeckUpdatedPayload")

	corpusEvt := loadCorpusDaemonEvent(t, "daemon-emit/deck-updated.json")
	var corpusPayload contract.DeckUpdatedPayload
	require.NoErrorf(t, json.Unmarshal(corpusEvt.Payload, &corpusPayload),
		"[contract-gate] unmarshal corpus deck-updated.json payload")

	if payload.DeckID != corpusPayload.DeckID {
		contractError(t, "deck-updated.json", "deck_id", corpusPayload.DeckID, payload.DeckID, "deck.go")
	}
	if payload.Format != corpusPayload.Format {
		contractError(t, "deck-updated.json", "format", corpusPayload.Format, payload.Format, "deck.go")
	}
	assert.NotEmptyf(t, payload.DeckID,
		"[contract-gate] deck.updated deck_id must be non-empty; corpus fixture: daemon-emit/deck-updated.json; parser source: deck.go")
	assert.NotEmptyf(t, payload.Format,
		"[contract-gate] deck.updated format must be non-empty; corpus fixture: daemon-emit/deck-updated.json; parser source: deck.go")
	assert.GreaterOrEqualf(t, len(payload.Cards), 1,
		"[contract-gate] deck.updated cards array must have at least 1 entry; corpus fixture: daemon-emit/deck-updated.json; parser source: deck.go")
	for i, c := range payload.Cards {
		assert.Greaterf(t, c.ArenaID, 0,
			"[contract-gate] deck.updated cards[%d].arena_id must be > 0; corpus fixture: daemon-emit/deck-updated.json; parser source: deck.go",
			i)
	}
}

// TestContractEmit_DraftPack round-trips the draft-pack corpus player-log
// fixture through ParseDraftPack + BuildEvent and asserts semantic properties
// on the emitted payload.
func TestContractEmit_DraftPack(t *testing.T) {
	entry := loadCorpusLogEntry(t, "player-log/draft-pack.log")

	p, err := logreader.ParseDraftPack(entry)
	require.NoErrorf(t, err,
		"[contract-gate] ParseDraftPack failed for corpus player-log/draft-pack.log")

	evt, err := dispatch.BuildEvent("draft.pack", testAccountID, testSessionID, p)
	require.NoErrorf(t, err,
		"[contract-gate] dispatch.BuildEvent failed for draft.pack")

	assertEnvelopeFields(t, evt, "draft.pack")

	// The daemon currently serialises DraftPackPayload (logreader type) into the
	// payload. Assert the fields that the logreader type produces.
	var rawPayload map[string]interface{}
	require.NoErrorf(t, json.Unmarshal(evt.Payload, &rawPayload),
		"[contract-gate] unmarshal draft.pack payload")

	courseName, _ := rawPayload["CourseName"].(string)
	assert.NotEmptyf(t, courseName,
		"[contract-gate] draft.pack CourseName must be non-empty; corpus fixture: daemon-emit/draft-pack.json; parser source: draft_pick.go; "+
			"If the MTGA log format changed: update the corpus (see ADR-042 Layer 2 refresh protocol). "+
			"If the daemon assembly changed: fix the parser to match the corpus OR update the corpus with a new MTGA version.")

	draftPack, _ := rawPayload["draftPack"].(map[string]interface{})
	assert.NotNilf(t, draftPack,
		"[contract-gate] draft.pack payload must contain draftPack object; corpus fixture: daemon-emit/draft-pack.json; parser source: draft_pick.go")

	if draftPack != nil {
		packCards, _ := draftPack["PackCards"].([]interface{})
		assert.GreaterOrEqualf(t, len(packCards), 1,
			"[contract-gate] draft.pack draftPack.PackCards must have at least 1 card; corpus fixture: daemon-emit/draft-pack.json; parser source: draft_pick.go")
	}
}

// TestContractEmit_DraftPick round-trips the draft-pick corpus player-log
// fixture through ParseDraftPick + BuildEvent and asserts semantic properties
// on the emitted payload.
func TestContractEmit_DraftPick(t *testing.T) {
	entry := loadCorpusLogEntry(t, "player-log/draft-pick.log")

	p, err := logreader.ParseDraftPick(entry)
	require.NoErrorf(t, err,
		"[contract-gate] ParseDraftPick failed for corpus player-log/draft-pick.log")

	evt, err := dispatch.BuildEvent("draft.pick", testAccountID, testSessionID, p)
	require.NoErrorf(t, err,
		"[contract-gate] dispatch.BuildEvent failed for draft.pick")

	assertEnvelopeFields(t, evt, "draft.pick")

	// The daemon currently serialises DraftPickPayload (logreader type) into the
	// payload. Assert the fields that the logreader type produces.
	var rawPayload map[string]interface{}
	require.NoErrorf(t, json.Unmarshal(evt.Payload, &rawPayload),
		"[contract-gate] unmarshal draft.pick payload")

	courseName, _ := rawPayload["CourseName"].(string)
	assert.NotEmptyf(t, courseName,
		"[contract-gate] draft.pick CourseName must be non-empty; corpus fixture: daemon-emit/draft-pick.json; parser source: draft_pick.go; "+
			"If the MTGA log format changed: update the corpus (see ADR-042 Layer 2 refresh protocol). "+
			"If the daemon assembly changed: fix the parser to match the corpus OR update the corpus with a new MTGA version.")

	pickedCards, _ := rawPayload["pickedCards"].([]interface{})
	assert.GreaterOrEqualf(t, len(pickedCards), 1,
		"[contract-gate] draft.pick pickedCards must have at least 1 entry; corpus fixture: daemon-emit/draft-pick.json; parser source: draft_pick.go")
}

// TestContractEmit_DraftPack_Premier round-trips the Premier draft-pack corpus
// fixture (Draft.Notify wire format, #338) through ParsePremierDraftNotify +
// BuildEvent and asserts the emitted payload carries the DraftID and a non-empty
// PackCards slice. CourseName is intentionally empty for Premier.
func TestContractEmit_DraftPack_Premier(t *testing.T) {
	entry := loadCorpusLogEntry(t, "player-log/premier-draft-pack.log")

	p, err := logreader.ParsePremierDraftNotify(entry)
	require.NoErrorf(t, err,
		"[contract-gate] ParsePremierDraftNotify failed for corpus player-log/premier-draft-pack.log")

	evt, err := dispatch.BuildEvent("draft.pack", testAccountID, testSessionID, p)
	require.NoErrorf(t, err,
		"[contract-gate] dispatch.BuildEvent failed for draft.pack (Premier)")

	assertEnvelopeFields(t, evt, "draft.pack")

	var rawPayload map[string]interface{}
	require.NoErrorf(t, json.Unmarshal(evt.Payload, &rawPayload),
		"[contract-gate] unmarshal Premier draft.pack payload")

	draftID, _ := rawPayload["draft_id"].(string)
	assert.NotEmptyf(t, draftID,
		"[contract-gate] Premier draft.pack draft_id must be non-empty; corpus fixture: player-log/premier-draft-pack.log; parser source: draft_pick.go")

	draftPack, _ := rawPayload["draftPack"].(map[string]interface{})
	require.NotNilf(t, draftPack,
		"[contract-gate] Premier draft.pack payload must contain draftPack object; corpus fixture: player-log/premier-draft-pack.log; parser source: draft_pick.go")
	packCards, _ := draftPack["PackCards"].([]interface{})
	assert.GreaterOrEqualf(t, len(packCards), 1,
		"[contract-gate] Premier draft.pack draftPack.PackCards must have at least 1 card; corpus fixture: player-log/premier-draft-pack.log; parser source: draft_pick.go")
}

// TestContractEmit_DraftPick_Premier round-trips the Premier draft-pick corpus
// fixture (EventPlayerDraftMakePick wire format, #338) through
// ParsePremierDraftMakePick + BuildEvent and asserts the emitted payload carries
// the DraftID and a non-empty pickedCards slice.
func TestContractEmit_DraftPick_Premier(t *testing.T) {
	entry := loadCorpusLogEntry(t, "player-log/premier-draft-pick.log")

	p, err := logreader.ParsePremierDraftMakePick(entry)
	require.NoErrorf(t, err,
		"[contract-gate] ParsePremierDraftMakePick failed for corpus player-log/premier-draft-pick.log")

	evt, err := dispatch.BuildEvent("draft.pick", testAccountID, testSessionID, p)
	require.NoErrorf(t, err,
		"[contract-gate] dispatch.BuildEvent failed for draft.pick (Premier)")

	assertEnvelopeFields(t, evt, "draft.pick")

	var rawPayload map[string]interface{}
	require.NoErrorf(t, json.Unmarshal(evt.Payload, &rawPayload),
		"[contract-gate] unmarshal Premier draft.pick payload")

	draftID, _ := rawPayload["draft_id"].(string)
	assert.NotEmptyf(t, draftID,
		"[contract-gate] Premier draft.pick draft_id must be non-empty; corpus fixture: player-log/premier-draft-pick.log; parser source: draft_pick.go")

	pickedCards, _ := rawPayload["pickedCards"].([]interface{})
	assert.GreaterOrEqualf(t, len(pickedCards), 1,
		"[contract-gate] Premier draft.pick pickedCards must have at least 1 entry; corpus fixture: player-log/premier-draft-pick.log; parser source: draft_pick.go")
}

// TestContractEmit_CollectionUpdated round-trips the collection-updated corpus
// player-log fixture through ParseCollectionEntry + BuildEvent.
func TestContractEmit_CollectionUpdated(t *testing.T) {
	entry := loadCorpusLogEntry(t, "player-log/collection-updated.log")

	require.Truef(t, logreader.IsCollectionEntry(entry),
		"[contract-gate] corpus player-log/collection-updated.log must classify as collection.updated")

	p, err := logreader.ParseCollectionEntry(entry)
	require.NoErrorf(t, err,
		"[contract-gate] ParseCollectionEntry failed for corpus player-log/collection-updated.log")

	evt, err := dispatch.BuildEvent("collection.updated", testAccountID, testSessionID, p)
	require.NoErrorf(t, err,
		"[contract-gate] dispatch.BuildEvent failed for collection.updated")

	assertEnvelopeFields(t, evt, "collection.updated")

	var payload contract.CollectionUpdatedPayload
	require.NoErrorf(t, json.Unmarshal(evt.Payload, &payload),
		"[contract-gate] unmarshal collection.updated payload into CollectionUpdatedPayload")

	assert.GreaterOrEqualf(t, len(payload.Cards), 1,
		"[contract-gate] collection.updated cards array must have at least 1 entry; corpus fixture: daemon-emit/collection-updated.json; parser source: collection.go")
	for i, c := range payload.Cards {
		assert.Greaterf(t, c.ArenaID, 0,
			"[contract-gate] collection.updated cards[%d].arena_id must be > 0; corpus fixture: daemon-emit/collection-updated.json; parser source: collection.go",
			i)
		assert.GreaterOrEqualf(t, c.Count, 0,
			"[contract-gate] collection.updated cards[%d].count must be >= 0; corpus fixture: daemon-emit/collection-updated.json; parser source: collection.go",
			i)
	}
	assert.Falsef(t, payload.IsDelta,
		"[contract-gate] collection.updated is_delta must be false for a full snapshot; corpus fixture: daemon-emit/collection-updated.json; parser source: collection.go")
}

// TestContractEmit_InventoryUpdated round-trips the inventory-updated corpus
// player-log fixture through ParseInventoryEntry + BuildEvent.
func TestContractEmit_InventoryUpdated(t *testing.T) {
	entry := loadCorpusLogEntry(t, "player-log/inventory-updated.log")

	require.Truef(t, logreader.IsInventoryEntry(entry),
		"[contract-gate] corpus player-log/inventory-updated.log must classify as inventory.updated")

	p, err := logreader.ParseInventoryEntry(entry)
	require.NoErrorf(t, err,
		"[contract-gate] ParseInventoryEntry failed for corpus player-log/inventory-updated.log")

	evt, err := dispatch.BuildEvent("inventory.updated", testAccountID, testSessionID, p)
	require.NoErrorf(t, err,
		"[contract-gate] dispatch.BuildEvent failed for inventory.updated")

	assertEnvelopeFields(t, evt, "inventory.updated")

	var payload contract.InventoryUpdatedPayload
	require.NoErrorf(t, json.Unmarshal(evt.Payload, &payload),
		"[contract-gate] unmarshal inventory.updated payload into InventoryUpdatedPayload")

	corpusEvt := loadCorpusDaemonEvent(t, "daemon-emit/inventory-updated.json")
	var corpusPayload contract.InventoryUpdatedPayload
	require.NoErrorf(t, json.Unmarshal(corpusEvt.Payload, &corpusPayload),
		"[contract-gate] unmarshal corpus inventory-updated.json payload")

	if payload.Gems != corpusPayload.Gems {
		contractError(t, "inventory-updated.json", "gems", corpusPayload.Gems, payload.Gems, "inventory.go")
	}
	if payload.Gold != corpusPayload.Gold {
		contractError(t, "inventory-updated.json", "gold", corpusPayload.Gold, payload.Gold, "inventory.go")
	}
	if payload.WildCardRares != corpusPayload.WildCardRares {
		contractError(t, "inventory-updated.json", "wild_card_rares", corpusPayload.WildCardRares, payload.WildCardRares, "inventory.go")
	}

	// Non-negative counts are invariants for all currency fields.
	assert.GreaterOrEqualf(t, payload.Gems, 0,
		"[contract-gate] inventory.updated gems must be >= 0; corpus fixture: daemon-emit/inventory-updated.json; parser source: inventory.go")
	assert.GreaterOrEqualf(t, payload.Gold, 0,
		"[contract-gate] inventory.updated gold must be >= 0; corpus fixture: daemon-emit/inventory-updated.json; parser source: inventory.go")
	assert.GreaterOrEqualf(t, payload.WildCardRares, 0,
		"[contract-gate] inventory.updated wild_card_rares must be >= 0; corpus fixture: daemon-emit/inventory-updated.json; parser source: inventory.go")
}

// ---------------------------------------------------------------------------
// Regression variants: corpus-shape validation (daemon-emit fixtures only)
// ---------------------------------------------------------------------------

// TestContractEmit_EmptyFormatVariant asserts that the match-completed-empty-format
// corpus fixture carries format == "" — representing the #201 failure class where
// the daemon legitimately emits empty format when eventId is absent from
// reservedPlayers entries. The BFF projection layer is responsible for mapping
// empty format to "Unknown"; the daemon must not supply a default.
func TestContractEmit_EmptyFormatVariant(t *testing.T) {
	evt := loadCorpusDaemonEvent(t, "daemon-emit/match-completed-empty-format.json")

	var payload contract.MatchCompletedPayload
	require.NoErrorf(t, json.Unmarshal(evt.Payload, &payload),
		"[contract-gate] unmarshal match-completed-empty-format.json payload")

	if payload.Format != "" {
		t.Errorf(
			"[contract-gate] match-completed-empty-format variant: expected payload.format == \"\" (the #201 empty-format class),\n"+
				"  got %q — the empty-format corpus variant must represent a match where eventId was absent from all\n"+
				"  reservedPlayers entries. Check corpus or parser.\n"+
				"  corpus fixture: testdata/corpus/daemon-emit/match-completed-empty-format.json\n"+
				"  parser source: services/daemon/internal/logreader/match.go\n"+
				"  If the MTGA log format changed: update the corpus (see ADR-042 Layer 2 refresh protocol).\n"+
				"  If the daemon assembly changed: fix the parser to match the corpus OR update the corpus with a new MTGA version.",
			payload.Format,
		)
	}

	// The envelope type must still be correct.
	assert.Equalf(t, "match.completed", evt.Type,
		"[contract-gate] match-completed-empty-format.json envelope.type must be match.completed")
}

// TestContractEmit_MissingIDVariant asserts that the match-completed-missing-id
// corpus fixture carries match_id == "" — representing the missing-PK failure class.
// The BFF must handle an empty match_id gracefully (e.g. discard or log).
func TestContractEmit_MissingIDVariant(t *testing.T) {
	evt := loadCorpusDaemonEvent(t, "daemon-emit/match-completed-missing-id.json")

	var payload contract.MatchCompletedPayload
	require.NoErrorf(t, json.Unmarshal(evt.Payload, &payload),
		"[contract-gate] unmarshal match-completed-missing-id.json payload")

	if payload.MatchID != "" {
		t.Errorf(
			"[contract-gate] match-completed-missing-id variant: expected payload.match_id == \"\" (the missing-PK failure class),\n"+
				"  got %q — this corpus variant must represent a match where matchId was absent from finalMatchResult.\n"+
				"  corpus fixture: testdata/corpus/daemon-emit/match-completed-missing-id.json\n"+
				"  parser source: services/daemon/internal/logreader/match.go\n"+
				"  If the MTGA log format changed: update the corpus (see ADR-042 Layer 2 refresh protocol).\n"+
				"  If the daemon assembly changed: fix the parser to match the corpus OR update the corpus with a new MTGA version.",
			payload.MatchID,
		)
	}

	assert.Equalf(t, "match.completed", evt.Type,
		"[contract-gate] match-completed-missing-id.json envelope.type must be match.completed")
}

// TestContractEmit_DuplicateQuestVariant asserts that the quest-progress-duplicate
// corpus fixture shares the same first QuestID as quest-progress.json and has
// progress >= the original — confirming it represents "same quest, later state".
func TestContractEmit_DuplicateQuestVariant(t *testing.T) {
	origEvt := loadCorpusDaemonEvent(t, "daemon-emit/quest-progress.json")
	dupEvt := loadCorpusDaemonEvent(t, "daemon-emit/quest-progress-duplicate.json")

	var origPayload, dupPayload contract.QuestProgressPayload
	require.NoErrorf(t, json.Unmarshal(origEvt.Payload, &origPayload),
		"[contract-gate] unmarshal quest-progress.json payload")
	require.NoErrorf(t, json.Unmarshal(dupEvt.Payload, &dupPayload),
		"[contract-gate] unmarshal quest-progress-duplicate.json payload")

	require.GreaterOrEqualf(t, len(origPayload.Quests), 1,
		"[contract-gate] quest-progress.json must have at least 1 quest")
	require.GreaterOrEqualf(t, len(dupPayload.Quests), 1,
		"[contract-gate] quest-progress-duplicate.json must have at least 1 quest")

	if origPayload.Quests[0].QuestID != dupPayload.Quests[0].QuestID {
		t.Errorf(
			"[contract-gate] quest-progress-duplicate variant: first quest_id must match quest-progress.json,\n"+
				"  got orig=%q dup=%q — the duplicate variant must represent the same quest at a later progress point.\n"+
				"  corpus fixture: testdata/corpus/daemon-emit/quest-progress-duplicate.json\n"+
				"  parser source: services/daemon/internal/logreader/quests.go\n"+
				"  If the MTGA log format changed: update the corpus (see ADR-042 Layer 2 refresh protocol).\n"+
				"  If the daemon assembly changed: fix the parser to match the corpus OR update the corpus with a new MTGA version.",
			origPayload.Quests[0].QuestID, dupPayload.Quests[0].QuestID,
		)
	}

	if dupPayload.Quests[0].Progress < origPayload.Quests[0].Progress {
		t.Errorf(
			"[contract-gate] quest-progress-duplicate variant: progress (%d) must be >= original (%d);\n"+
				"  corpus fixture: testdata/corpus/daemon-emit/quest-progress-duplicate.json\n"+
				"  parser source: services/daemon/internal/logreader/quests.go",
			dupPayload.Quests[0].Progress, origPayload.Quests[0].Progress,
		)
	}
}

// ---------------------------------------------------------------------------
// Corpus freshness fitness function (ADR-042 Layer 2)
// ---------------------------------------------------------------------------

// TestContractEmit_CorpusFreshness warns (non-failing) if the corpus was not
// updated within 120 days. This implements the ADR-042 staleness fitness
// function: a stale corpus risks not catching new MTGA wire format changes.
func TestContractEmit_CorpusFreshness(t *testing.T) {
	path := filepath.Join(corpusDir, "mtga-version.txt")
	info, err := os.Stat(path)
	if err != nil {
		t.Logf("[contract-gate] WARNING: cannot stat %s: %v — corpus freshness unknown", path, err)
		return
	}

	age := time.Since(info.ModTime())
	const maxAge = 120 * 24 * time.Hour
	if age > maxAge {
		t.Logf(
			"[contract-gate] WARNING: corpus mtga-version.txt last modified %s (%.0f days ago) — exceeds 120-day freshness threshold. "+
				"Consider recapturing corpus fixtures against the current MTGA client version. "+
				"See ADR-042 Layer 2 refresh protocol.",
			info.ModTime().Format("2006-01-02"), age.Hours()/24,
		)
	}
	// Non-failing: log only. Do not call t.Fail() or t.Error().
}
