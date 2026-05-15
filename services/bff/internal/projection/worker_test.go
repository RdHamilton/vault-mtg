package projection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// --- fakes ---

type fakeEventStore struct {
	pending    []repository.DaemonEventRow
	projected  []int64
	projectErr error
}

func (f *fakeEventStore) ListPendingProjection(_ context.Context, limit int) ([]repository.DaemonEventRow, error) {
	if limit < len(f.pending) {
		return f.pending[:limit], nil
	}
	return f.pending, nil
}

func (f *fakeEventStore) MarkProjected(_ context.Context, id int64) error {
	if f.projectErr != nil {
		return f.projectErr
	}
	f.projected = append(f.projected, id)
	return nil
}

type fakeAccountStore struct {
	accountID int64
	err       error
}

func (f *fakeAccountStore) GetOrCreateByClientID(_ context.Context, _ string, _ int64) (int64, error) {
	return f.accountID, f.err
}

type fakeMatchStore struct {
	upserts []repository.MatchUpsert
	err     error
}

func (f *fakeMatchStore) UpsertMatch(_ context.Context, m repository.MatchUpsert) error {
	if f.err != nil {
		return f.err
	}
	f.upserts = append(f.upserts, m)
	return nil
}

type fakeDraftStore struct {
	upserts []repository.DraftSessionUpsert
	err     error
}

func (f *fakeDraftStore) UpsertDraftSession(_ context.Context, s repository.DraftSessionUpsert) error {
	if f.err != nil {
		return f.err
	}
	f.upserts = append(f.upserts, s)
	return nil
}

type fakeCollectionStore struct {
	upserts []repository.CardInventoryUpsert
	err     error
}

func (f *fakeCollectionStore) UpsertDelta(_ context.Context, u repository.CardInventoryUpsert) error {
	if f.err != nil {
		return f.err
	}
	f.upserts = append(f.upserts, u)
	return nil
}

type fakeInventoryStore struct {
	err error
}

func (f *fakeInventoryStore) UpsertInventory(_ context.Context, _ repository.InventoryUpsert) error {
	return f.err
}

type fakeQuestStore struct {
	err error
}

func (f *fakeQuestStore) UpsertQuestProgress(_ context.Context, _ repository.QuestProgressUpsert) error {
	return f.err
}

func (f *fakeQuestStore) InsertQuestCompleted(_ context.Context, _ repository.QuestCompletedInsert) error {
	return f.err
}

type fakeDeckStore struct {
	err error
}

func (f *fakeDeckStore) UpsertDeck(_ context.Context, _ repository.DeckUpsert) error {
	return f.err
}

// --- helpers ---

func makePayload(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

func newWorker(events *fakeEventStore, accounts *fakeAccountStore, matches *fakeMatchStore, drafts *fakeDraftStore) *Worker {
	return NewWorker(events, accounts, matches, drafts, &fakeCollectionStore{}, &fakeInventoryStore{}, &fakeQuestStore{}, &fakeDeckStore{}, &fakeGamePlayStore{})
}

func newWorkerWithCollection(events *fakeEventStore, accounts *fakeAccountStore, collection *fakeCollectionStore) *Worker {
	return NewWorker(events, accounts, &fakeMatchStore{}, &fakeDraftStore{}, collection, &fakeInventoryStore{}, &fakeQuestStore{}, &fakeDeckStore{}, &fakeGamePlayStore{})
}

// --- tests ---

func TestRunOnce_MatchCompleted_ProjectsToMatches(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"match_id":       "match-001",
		"event_id":       "evt_abc",
		"event_name":     "Standard_BO1",
		"format":         "Standard",
		"result":         "win",
		"player_wins":    2,
		"opponent_wins":  1,
		"player_team_id": 0,
	})

	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 1, UserID: 1, AccountID: "acct-1", EventType: "match.completed", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}
	matches := &fakeMatchStore{}
	drafts := &fakeDraftStore{}

	w := newWorker(events, accounts, matches, drafts)
	w.RunOnce(context.Background())

	if len(matches.upserts) != 1 {
		t.Fatalf("expected 1 match upsert, got %d", len(matches.upserts))
	}
	if matches.upserts[0].ID != "match-001" {
		t.Errorf("expected match ID match-001, got %q", matches.upserts[0].ID)
	}
	if len(events.projected) != 1 || events.projected[0] != 1 {
		t.Errorf("expected row 1 marked projected, got %v", events.projected)
	}
}

func TestRunOnce_DraftStarted_ProjectsToDraftSessions(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"session_id": "draft-001",
		"event_name": "QuickDraft_EOE",
		"set_code":   "EOE",
		"draft_type": "quick_draft",
		"status":     "in_progress",
	})

	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 2, UserID: 1, AccountID: "acct-1", EventType: "draft.started", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}
	matches := &fakeMatchStore{}
	drafts := &fakeDraftStore{}

	w := newWorker(events, accounts, matches, drafts)
	w.RunOnce(context.Background())

	if len(drafts.upserts) != 1 {
		t.Fatalf("expected 1 draft upsert, got %d", len(drafts.upserts))
	}
	if drafts.upserts[0].ID != "draft-001" {
		t.Errorf("expected session ID draft-001, got %q", drafts.upserts[0].ID)
	}
	if len(events.projected) != 1 {
		t.Errorf("expected 1 row marked projected")
	}
}

func TestRunOnce_MalformedPayload_MarkedProjectedNoDestinationRow(t *testing.T) {
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{
				ID: 3, UserID: 1, AccountID: "acct-1", EventType: "match.completed",
				Payload: json.RawMessage(`{"bad":"shape"}`), OccurredAt: time.Now(),
			},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}
	matches := &fakeMatchStore{}
	drafts := &fakeDraftStore{}

	w := newWorker(events, accounts, matches, drafts)
	w.RunOnce(context.Background())

	// Row must be marked projected even though payload was bad.
	if len(events.projected) != 1 || events.projected[0] != 3 {
		t.Errorf("malformed row must be marked projected; got %v", events.projected)
	}
	// No match must have been written.
	if len(matches.upserts) != 0 {
		t.Errorf("expected 0 match upserts for malformed payload, got %d", len(matches.upserts))
	}
}

func TestRunOnce_UnknownEventType_MarkedProjected(t *testing.T) {
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{
				ID: 4, UserID: 1, AccountID: "acct-1", EventType: "sync.collection",
				Payload: json.RawMessage(`{}`), OccurredAt: time.Now(),
			},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}
	matches := &fakeMatchStore{}
	drafts := &fakeDraftStore{}

	w := newWorker(events, accounts, matches, drafts)
	w.RunOnce(context.Background())

	if len(events.projected) != 1 || events.projected[0] != 4 {
		t.Errorf("unknown event must be marked projected; got %v", events.projected)
	}
}

func TestRunOnce_Idempotent_SecondRunNoNewRows(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"match_id":       "match-idem",
		"event_id":       "evt_idem",
		"event_name":     "Standard_BO1",
		"format":         "Standard",
		"result":         "win",
		"player_wins":    2,
		"opponent_wins":  1,
		"player_team_id": 0,
	})

	row := repository.DaemonEventRow{
		ID: 5, UserID: 1, AccountID: "acct-1", EventType: "match.completed",
		Payload: payload, OccurredAt: time.Now(),
	}

	events := &fakeEventStore{pending: []repository.DaemonEventRow{row}}
	accounts := &fakeAccountStore{accountID: 10}
	matches := &fakeMatchStore{}
	drafts := &fakeDraftStore{}

	w := newWorker(events, accounts, matches, drafts)

	// First run — projects the row.
	w.RunOnce(context.Background())
	firstCount := len(matches.upserts)

	// Clear pending so the second run sees nothing new (simulates projected_at being set).
	events.pending = nil

	// Second run — nothing pending, so no additional upserts.
	w.RunOnce(context.Background())

	if len(matches.upserts) != firstCount {
		t.Errorf("second runOnce produced additional upserts; first=%d total=%d", firstCount, len(matches.upserts))
	}
}

func TestRunOnce_MixedTypes_AllMarkedProjected(t *testing.T) {
	matchPayload := makePayload(t, map[string]interface{}{
		"match_id":       "m1",
		"event_id":       "e1",
		"event_name":     "Standard",
		"format":         "Standard",
		"result":         "loss",
		"player_wins":    1,
		"opponent_wins":  2,
		"player_team_id": 0,
	})
	draftPayload := makePayload(t, map[string]interface{}{
		"session_id": "d1",
		"event_name": "QuickDraft",
		"set_code":   "BRO",
		"draft_type": "quick_draft",
		"status":     "in_progress",
	})

	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 10, UserID: 1, AccountID: "a", EventType: "match.completed", Payload: matchPayload, OccurredAt: time.Now()},
			{ID: 11, UserID: 1, AccountID: "a", EventType: "draft.started", Payload: draftPayload, OccurredAt: time.Now()},
			{ID: 12, UserID: 1, AccountID: "a", EventType: "unknown.type", Payload: json.RawMessage(`{}`), OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}
	matches := &fakeMatchStore{}
	drafts := &fakeDraftStore{}

	w := newWorker(events, accounts, matches, drafts)
	w.RunOnce(context.Background())

	if len(events.projected) != 3 {
		t.Errorf("expected all 3 rows projected, got %d: %v", len(events.projected), events.projected)
	}
	if len(matches.upserts) != 1 {
		t.Errorf("expected 1 match upsert, got %d", len(matches.upserts))
	}
	if len(drafts.upserts) != 1 {
		t.Errorf("expected 1 draft upsert, got %d", len(drafts.upserts))
	}
}

// --- collection.updated tests ---

func TestRunOnce_CollectionUpdated_ProjectsToInventory(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"cards": []map[string]interface{}{
			{"arena_id": 100001, "count": 4},
			{"arena_id": 100002, "count": 2},
		},
		"is_delta": false,
	})

	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 20, UserID: 1, AccountID: "acct-col", EventType: "collection.updated", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 42}
	collection := &fakeCollectionStore{}

	w := newWorkerWithCollection(events, accounts, collection)
	w.RunOnce(context.Background())

	if len(collection.upserts) != 2 {
		t.Fatalf("expected 2 card upserts, got %d", len(collection.upserts))
	}
	if collection.upserts[0].CardID != 100001 || collection.upserts[0].Count != 4 {
		t.Errorf("unexpected first upsert: %+v", collection.upserts[0])
	}
	if collection.upserts[1].CardID != 100002 || collection.upserts[1].Count != 2 {
		t.Errorf("unexpected second upsert: %+v", collection.upserts[1])
	}
	// All upserts must carry the same snapshot_hash.
	if collection.upserts[0].SnapshotHash == "" {
		t.Error("snapshot_hash must not be empty")
	}
	if collection.upserts[0].SnapshotHash != collection.upserts[1].SnapshotHash {
		t.Errorf("snapshot_hash must be consistent across cards in one event; got %q vs %q",
			collection.upserts[0].SnapshotHash, collection.upserts[1].SnapshotHash)
	}
	// Row must be marked projected.
	if len(events.projected) != 1 || events.projected[0] != 20 {
		t.Errorf("expected row 20 marked projected, got %v", events.projected)
	}
}

func TestRunOnce_CollectionUpdated_AccountIDScoped(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"cards":    []map[string]interface{}{{"arena_id": 200001, "count": 1}},
		"is_delta": true,
	})

	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 21, UserID: 5, AccountID: "acct-scoped", EventType: "collection.updated", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 99}
	collection := &fakeCollectionStore{}

	w := newWorkerWithCollection(events, accounts, collection)
	w.RunOnce(context.Background())

	if len(collection.upserts) != 1 {
		t.Fatalf("expected 1 upsert, got %d", len(collection.upserts))
	}
	if collection.upserts[0].AccountID != 99 {
		t.Errorf("expected account_id=99, got %d", collection.upserts[0].AccountID)
	}
}

func TestRunOnce_CollectionUpdated_EmptyCards_NoUpsert(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"cards":    []map[string]interface{}{},
		"is_delta": true,
	})

	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 22, UserID: 1, AccountID: "acct-empty", EventType: "collection.updated", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}
	collection := &fakeCollectionStore{}

	w := newWorkerWithCollection(events, accounts, collection)
	w.RunOnce(context.Background())

	if len(collection.upserts) != 0 {
		t.Errorf("expected 0 upserts for empty cards, got %d", len(collection.upserts))
	}
	// Must still be marked projected.
	if len(events.projected) != 1 || events.projected[0] != 22 {
		t.Errorf("expected row 22 marked projected, got %v", events.projected)
	}
}

func TestRunOnce_CollectionUpdated_IdempotentSamePayload(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"cards":    []map[string]interface{}{{"arena_id": 300001, "count": 3}},
		"is_delta": false,
	})

	row := repository.DaemonEventRow{
		ID: 23, UserID: 1, AccountID: "acct-idem", EventType: "collection.updated",
		Payload: payload, OccurredAt: time.Now(),
	}

	events := &fakeEventStore{pending: []repository.DaemonEventRow{row}}
	accounts := &fakeAccountStore{accountID: 10}
	collection := &fakeCollectionStore{}

	w := newWorkerWithCollection(events, accounts, collection)

	// First run.
	w.RunOnce(context.Background())
	firstCount := len(collection.upserts)

	// Reset pending to simulate the same event being re-queued (e.g. daemon retry).
	events.pending = []repository.DaemonEventRow{row}
	events.projected = nil

	// Second run with the same payload.
	w.RunOnce(context.Background())

	// The fake store always accepts; idempotency is enforced by the DB ON CONFLICT.
	// Here we just verify the worker calls UpsertDelta again (DB handles dedup).
	if len(collection.upserts) != firstCount*2 {
		t.Errorf("expected %d total upserts after two runs, got %d", firstCount*2, len(collection.upserts))
	}
	// Snapshot hashes must be identical across both runs.
	if collection.upserts[0].SnapshotHash != collection.upserts[firstCount].SnapshotHash {
		t.Errorf("snapshot_hash must be deterministic; run1=%q run2=%q",
			collection.upserts[0].SnapshotHash, collection.upserts[firstCount].SnapshotHash)
	}
}

func TestRunOnce_CollectionUpdated_MalformedPayload_MarkedProjected(t *testing.T) {
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{
				ID: 24, UserID: 1, AccountID: "acct-bad", EventType: "collection.updated",
				Payload: json.RawMessage(`not-json`), OccurredAt: time.Now(),
			},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}
	collection := &fakeCollectionStore{}

	w := newWorkerWithCollection(events, accounts, collection)
	w.RunOnce(context.Background())

	if len(events.projected) != 1 || events.projected[0] != 24 {
		t.Errorf("malformed row must be marked projected; got %v", events.projected)
	}
	if len(collection.upserts) != 0 {
		t.Errorf("expected 0 upserts for malformed payload, got %d", len(collection.upserts))
	}
}

// --- inventory.updated tests ---

func newWorkerWithInventory(events *fakeEventStore, accounts *fakeAccountStore, inv *fakeInventoryStore) *Worker {
	return NewWorker(events, accounts, &fakeMatchStore{}, &fakeDraftStore{}, &fakeCollectionStore{}, inv, &fakeQuestStore{}, &fakeDeckStore{}, &fakeGamePlayStore{})
}

func TestRunOnce_InventoryUpdated_ProjectsToInventory(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"gems":                 1500,
		"gold":                 20000,
		"total_vault_progress": 47,
		"wild_card_commons":    12,
		"wild_card_uncommons":  5,
		"wild_card_rares":      2,
		"wild_card_mythics":    1,
	})

	inv := &fakeInventoryStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 30, UserID: 1, AccountID: "acct-inv", EventType: "inventory.updated", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}

	w := NewWorker(events, accounts, &fakeMatchStore{}, &fakeDraftStore{}, &fakeCollectionStore{}, inv, &fakeQuestStore{}, &fakeDeckStore{}, &fakeGamePlayStore{})
	w.RunOnce(context.Background())

	if len(inv.upserts) != 1 {
		t.Fatalf("expected 1 inventory upsert, got %d", len(inv.upserts))
	}
	u := inv.upserts[0]
	if u.AccountID != 10 {
		t.Errorf("account_id: want 10 (resolved accounts.id), got %d", u.AccountID)
	}
	if u.Gems != 1500 {
		t.Errorf("gems: want 1500, got %d", u.Gems)
	}
	if u.Gold != 20000 {
		t.Errorf("gold: want 20000, got %d", u.Gold)
	}
	if u.TotalVaultProgress != 47 {
		t.Errorf("vault_progress: want 47, got %d", u.TotalVaultProgress)
	}
	if len(events.projected) != 1 || events.projected[0] != 30 {
		t.Errorf("expected row 30 marked projected, got %v", events.projected)
	}
}

func TestRunOnce_InventoryUpdated_MissingAccountID_MarkedProjected(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"gems": 100,
		"gold": 500,
	})
	inv := &fakeInventoryStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 31, UserID: 1, AccountID: "", EventType: "inventory.updated", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}

	w := NewWorker(events, accounts, &fakeMatchStore{}, &fakeDraftStore{}, &fakeCollectionStore{}, inv, &fakeQuestStore{}, &fakeDeckStore{}, &fakeGamePlayStore{})
	w.RunOnce(context.Background())

	// Row marked projected even though payload was rejected.
	if len(events.projected) != 1 || events.projected[0] != 31 {
		t.Errorf("expected row 31 marked projected, got %v", events.projected)
	}
	if len(inv.upserts) != 0 {
		t.Errorf("expected 0 inventory upserts for missing account_id, got %d", len(inv.upserts))
	}
}

// fakeInventoryStoreCapturing captures upserts for assertion.
type fakeInventoryStoreCapturing struct {
	upserts []repository.InventoryUpsert
	err     error
}

func (f *fakeInventoryStoreCapturing) UpsertInventory(_ context.Context, u repository.InventoryUpsert) error {
	if f.err != nil {
		return f.err
	}
	f.upserts = append(f.upserts, u)
	return nil
}

// --- quest.progress tests ---

func newWorkerWithQuests(events *fakeEventStore, accounts *fakeAccountStore, quests *fakeQuestStoreCapturing) *Worker {
	return NewWorker(events, accounts, &fakeMatchStore{}, &fakeDraftStore{}, &fakeCollectionStore{}, &fakeInventoryStore{}, quests, &fakeDeckStore{}, &fakeGamePlayStore{})
}

// fakeQuestStoreCapturing captures calls for assertion.
type fakeQuestStoreCapturing struct {
	progressUpserts  []repository.QuestProgressUpsert
	completedInserts []repository.QuestCompletedInsert
	err              error
}

func (f *fakeQuestStoreCapturing) UpsertQuestProgress(_ context.Context, u repository.QuestProgressUpsert) error {
	if f.err != nil {
		return f.err
	}
	f.progressUpserts = append(f.progressUpserts, u)
	return nil
}

func (f *fakeQuestStoreCapturing) InsertQuestCompleted(_ context.Context, ins repository.QuestCompletedInsert) error {
	if f.err != nil {
		return f.err
	}
	f.completedInserts = append(f.completedInserts, ins)
	return nil
}

func TestRunOnce_QuestProgress_UpsertsAllQuests(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"quests": []map[string]interface{}{
			{"quest_id": "q-001", "quest_name": "Win 3 Games", "progress": 1, "goal": 3, "can_swap": true},
			{"quest_id": "q-002", "quest_name": "Cast 5 Spells", "progress": 4, "goal": 5, "can_swap": false},
		},
	})

	quests := &fakeQuestStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 40, UserID: 1, AccountID: "acct-q", EventType: "quest.progress", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}

	w := newWorkerWithQuests(events, accounts, quests)
	w.RunOnce(context.Background())

	if len(quests.progressUpserts) != 2 {
		t.Fatalf("expected 2 quest progress upserts, got %d", len(quests.progressUpserts))
	}
	if quests.progressUpserts[0].QuestID != "q-001" {
		t.Errorf("first quest_id: want q-001, got %q", quests.progressUpserts[0].QuestID)
	}
	if quests.progressUpserts[1].Progress != 4 {
		t.Errorf("second progress: want 4, got %d", quests.progressUpserts[1].Progress)
	}
	if len(events.projected) != 1 || events.projected[0] != 40 {
		t.Errorf("expected row 40 marked projected, got %v", events.projected)
	}
}

func TestRunOnce_QuestProgress_EmptyQuests_NoUpsert(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"quests": []map[string]interface{}{},
	})

	quests := &fakeQuestStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 41, UserID: 1, AccountID: "acct-q", EventType: "quest.progress", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}

	w := newWorkerWithQuests(events, accounts, quests)
	w.RunOnce(context.Background())

	if len(quests.progressUpserts) != 0 {
		t.Errorf("expected 0 upserts for empty quests, got %d", len(quests.progressUpserts))
	}
	if len(events.projected) != 1 || events.projected[0] != 41 {
		t.Errorf("expected row 41 marked projected, got %v", events.projected)
	}
}

// --- quest.completed tests ---

func TestRunOnce_QuestCompleted_InsertsToSessionTracking(t *testing.T) {
	now := time.Now().UTC()
	payload := makePayload(t, map[string]interface{}{
		"quest_id":          "q-done-001",
		"quest_name":        "Win 3 Games",
		"progress":          3,
		"goal":              3,
		"xp_reward":         500,
		"completion_source": "match",
	})

	quests := &fakeQuestStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 50, UserID: 1, AccountID: "acct-qc", EventType: "quest.completed", Payload: payload, OccurredAt: now},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}

	w := newWorkerWithQuests(events, accounts, quests)
	w.RunOnce(context.Background())

	if len(quests.completedInserts) != 1 {
		t.Fatalf("expected 1 quest completed insert, got %d", len(quests.completedInserts))
	}
	ins := quests.completedInserts[0]
	if ins.QuestID != "q-done-001" {
		t.Errorf("quest_id: want q-done-001, got %q", ins.QuestID)
	}
	if ins.XPReward != 500 {
		t.Errorf("xp_reward: want 500, got %d", ins.XPReward)
	}
	if ins.AccountID != 10 {
		t.Errorf("account_id: want 10 (resolved accounts.id), got %d", ins.AccountID)
	}
	if !ins.OccurredAt.Equal(now) {
		t.Errorf("occurred_at: want %v, got %v", now, ins.OccurredAt)
	}
	if len(events.projected) != 1 || events.projected[0] != 50 {
		t.Errorf("expected row 50 marked projected, got %v", events.projected)
	}
}

func TestRunOnce_QuestCompleted_MissingQuestID_MarkedProjected(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"quest_name": "Win 3 Games",
		"progress":   3,
		"goal":       3,
	})

	quests := &fakeQuestStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 51, UserID: 1, AccountID: "acct-qc", EventType: "quest.completed", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}

	w := newWorkerWithQuests(events, accounts, quests)
	w.RunOnce(context.Background())

	if len(quests.completedInserts) != 0 {
		t.Errorf("expected 0 inserts for missing quest_id, got %d", len(quests.completedInserts))
	}
	if len(events.projected) != 1 || events.projected[0] != 51 {
		t.Errorf("expected row 51 marked projected, got %v", events.projected)
	}
}

// --- deck.updated tests ---

func newWorkerWithDecks(events *fakeEventStore, accounts *fakeAccountStore, decks *fakeDeckStoreCapturing) *Worker {
	return NewWorker(events, accounts, &fakeMatchStore{}, &fakeDraftStore{}, &fakeCollectionStore{}, &fakeInventoryStore{}, &fakeQuestStore{}, decks, &fakeGamePlayStore{})
}

// fakeDeckStoreCapturing captures calls for assertion.
type fakeDeckStoreCapturing struct {
	upserts []repository.DeckUpsert
	err     error
}

func (f *fakeDeckStoreCapturing) UpsertDeck(_ context.Context, u repository.DeckUpsert) error {
	if f.err != nil {
		return f.err
	}
	f.upserts = append(f.upserts, u)
	return nil
}

func TestRunOnce_DeckUpdated_ProjectsToDeck(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"deck_id": "deck-abc-123",
		"name":    "Mono Red Aggro",
		"format":  "Standard",
		"cards": []map[string]interface{}{
			{"arena_id": 84738, "quantity": 4},
			{"arena_id": 84739, "quantity": 4},
		},
	})

	decks := &fakeDeckStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 60, UserID: 1, AccountID: "acct-dk", EventType: "deck.updated", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 7}

	w := newWorkerWithDecks(events, accounts, decks)
	w.RunOnce(context.Background())

	if len(decks.upserts) != 1 {
		t.Fatalf("expected 1 deck upsert, got %d", len(decks.upserts))
	}
	u := decks.upserts[0]
	if u.DeckID != "deck-abc-123" {
		t.Errorf("deck_id: want deck-abc-123, got %q", u.DeckID)
	}
	if u.AccountID != 7 {
		t.Errorf("account_id: want 7, got %d", u.AccountID)
	}
	if len(u.Cards) != 2 {
		t.Errorf("cards: want 2, got %d", len(u.Cards))
	}
	if len(events.projected) != 1 || events.projected[0] != 60 {
		t.Errorf("expected row 60 marked projected, got %v", events.projected)
	}
}

func TestRunOnce_DeckUpdated_MissingDeckID_MarkedProjected(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"name":   "Nameless Deck",
		"format": "Historic",
		"cards":  []map[string]interface{}{},
	})

	decks := &fakeDeckStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 61, UserID: 1, AccountID: "acct-dk", EventType: "deck.updated", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 7}

	w := newWorkerWithDecks(events, accounts, decks)
	w.RunOnce(context.Background())

	if len(decks.upserts) != 0 {
		t.Errorf("expected 0 upserts for missing deck_id, got %d", len(decks.upserts))
	}
	if len(events.projected) != 1 || events.projected[0] != 61 {
		t.Errorf("expected row 61 marked projected, got %v", events.projected)
	}
}

type fakeGamePlayStore struct {
	err error
}

func (f *fakeGamePlayStore) InsertGamePlay(_ context.Context, _ repository.GamePlayInsert) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return 1, nil
}

func (f *fakeGamePlayStore) InsertLifeChanges(_ context.Context, _ []repository.LifeChangeInsert) error {
	return f.err
}

// --- partial flag tests ---

func TestRunOnce_GamePlayEvent_PartialTrue_SetsPartialOnInsert(t *testing.T) {
	now := time.Now().UTC()
	payload := makePayload(t, map[string]interface{}{
		"match_id":        "match-partial-001",
		"game_number":     1,
		"winning_team_id": 0,
		"turn_count":      5,
		"duration_secs":   60,
		"life_changes":    []map[string]interface{}{},
		"partial":         true,
	})

	gp := &fakeGamePlayStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 200, UserID: 1, AccountID: "acct-partial", EventType: "match.game_ended", Payload: payload, OccurredAt: now, Sequence: 1},
		},
	}
	accounts := &fakeAccountStore{accountID: 50}

	w := newWorkerWithGamePlay(events, accounts, gp)
	w.RunOnce(context.Background())

	if len(gp.gamePlayInserts) != 1 {
		t.Fatalf("expected 1 game_play insert, got %d", len(gp.gamePlayInserts))
	}
	if !gp.gamePlayInserts[0].Partial {
		t.Errorf("Partial: want true, got false")
	}
	if len(events.projected) != 1 || events.projected[0] != 200 {
		t.Errorf("expected row 200 marked projected, got %v", events.projected)
	}
}

func TestRunOnce_GamePlayEvent_PartialFalse_SetsPartialFalseOnInsert(t *testing.T) {
	now := time.Now().UTC()
	payload := makePayload(t, map[string]interface{}{
		"match_id":        "match-npartial-001",
		"game_number":     1,
		"winning_team_id": 1,
		"turn_count":      10,
		"duration_secs":   120,
		"life_changes":    []map[string]interface{}{},
		"partial":         false,
	})

	gp := &fakeGamePlayStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 201, UserID: 1, AccountID: "acct-npartial", EventType: "match.game_ended", Payload: payload, OccurredAt: now, Sequence: 2},
		},
	}
	accounts := &fakeAccountStore{accountID: 51}

	w := newWorkerWithGamePlay(events, accounts, gp)
	w.RunOnce(context.Background())

	if len(gp.gamePlayInserts) != 1 {
		t.Fatalf("expected 1 game_play insert, got %d", len(gp.gamePlayInserts))
	}
	if gp.gamePlayInserts[0].Partial {
		t.Errorf("Partial: want false, got true")
	}
}

func TestRunOnce_GamePlayEvent_PartialOmitted_DefaultsFalse(t *testing.T) {
	now := time.Now().UTC()
	// payload without "partial" key at all — should default to false.
	payload := makePayload(t, map[string]interface{}{
		"match_id":     "match-omit-partial",
		"game_number":  1,
		"turn_count":   8,
		"life_changes": []map[string]interface{}{},
	})

	gp := &fakeGamePlayStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 202, UserID: 1, AccountID: "acct-omit", EventType: "match.game_ended", Payload: payload, OccurredAt: now, Sequence: 3},
		},
	}
	accounts := &fakeAccountStore{accountID: 52}

	w := newWorkerWithGamePlay(events, accounts, gp)
	w.RunOnce(context.Background())

	if len(gp.gamePlayInserts) != 1 {
		t.Fatalf("expected 1 game_play insert, got %d", len(gp.gamePlayInserts))
	}
	if gp.gamePlayInserts[0].Partial {
		t.Errorf("Partial: want false when key omitted, got true")
	}
}

func TestRunOnce_GamePlayEvent_PartialTrue_NoMatchIDNoGameNumber_Accepted(t *testing.T) {
	// Partial events (GRE buffer flushes) may have no match_id / game_number yet.
	// The projector must accept them without error.
	now := time.Now().UTC()
	payload := makePayload(t, map[string]interface{}{
		"partial":      true,
		"life_changes": []map[string]interface{}{},
	})

	gp := &fakeGamePlayStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 203, UserID: 1, AccountID: "acct-gre-flush", EventType: "match.game_ended", Payload: payload, OccurredAt: now, Sequence: 4},
		},
	}
	accounts := &fakeAccountStore{accountID: 53}

	w := newWorkerWithGamePlay(events, accounts, gp)
	w.RunOnce(context.Background())

	if len(gp.gamePlayInserts) != 1 {
		t.Fatalf("expected 1 game_play insert for partial GRE flush, got %d", len(gp.gamePlayInserts))
	}
	ins := gp.gamePlayInserts[0]
	if !ins.Partial {
		t.Errorf("Partial: want true, got false")
	}
	if ins.MatchID != "" {
		t.Errorf("MatchID: want empty for GRE flush, got %q", ins.MatchID)
	}
	if ins.GameNumber != 0 {
		t.Errorf("GameNumber: want 0 for GRE flush, got %d", ins.GameNumber)
	}
	if len(events.projected) != 1 || events.projected[0] != 203 {
		t.Errorf("expected row 203 marked projected, got %v", events.projected)
	}
}

// --- match.game_ended tests ---

// fakeGamePlayStoreCapturing captures calls for assertion.
type fakeGamePlayStoreCapturing struct {
	gamePlayInserts []repository.GamePlayInsert
	lifeChanges     []repository.LifeChangeInsert
	nextID          int64
	err             error
}

func (f *fakeGamePlayStoreCapturing) InsertGamePlay(_ context.Context, ins repository.GamePlayInsert) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.gamePlayInserts = append(f.gamePlayInserts, ins)
	f.nextID++
	return f.nextID, nil
}

func (f *fakeGamePlayStoreCapturing) InsertLifeChanges(_ context.Context, changes []repository.LifeChangeInsert) error {
	if f.err != nil {
		return f.err
	}
	f.lifeChanges = append(f.lifeChanges, changes...)
	return nil
}

func newWorkerWithGamePlay(events *fakeEventStore, accounts *fakeAccountStore, gp *fakeGamePlayStoreCapturing) *Worker {
	return NewWorker(events, accounts, &fakeMatchStore{}, &fakeDraftStore{}, &fakeCollectionStore{}, &fakeInventoryStore{}, &fakeQuestStore{}, &fakeDeckStore{}, gp)
}

func TestRunOnce_GamePlayEvent_SingleGame(t *testing.T) {
	now := time.Now().UTC()
	payload := makePayload(t, map[string]interface{}{
		"match_id":        "match-gre-001",
		"game_number":     1,
		"winning_team_id": 1,
		"turn_count":      12,
		"duration_secs":   300,
		"life_changes": []map[string]interface{}{
			{"team_id": 1, "life_total": 20, "delta": 0, "turn_number": 1},
			{"team_id": 2, "life_total": 17, "delta": -3, "turn_number": 2},
		},
	})

	gp := &fakeGamePlayStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 70, UserID: 1, AccountID: "acct-gp", EventType: "match.game_ended", Payload: payload, OccurredAt: now, Sequence: 5},
		},
	}
	accounts := &fakeAccountStore{accountID: 11}

	w := newWorkerWithGamePlay(events, accounts, gp)
	w.RunOnce(context.Background())

	if len(gp.gamePlayInserts) != 1 {
		t.Fatalf("expected 1 game_play insert, got %d", len(gp.gamePlayInserts))
	}
	ins := gp.gamePlayInserts[0]
	if ins.MatchID != "match-gre-001" {
		t.Errorf("match_id: want match-gre-001, got %q", ins.MatchID)
	}
	if ins.GameNumber != 1 {
		t.Errorf("game_number: want 1, got %d", ins.GameNumber)
	}
	if ins.WinningTeamID != 1 {
		t.Errorf("winning_team_id: want 1, got %d", ins.WinningTeamID)
	}
	if ins.TurnCount != 12 {
		t.Errorf("turn_count: want 12, got %d", ins.TurnCount)
	}
	if ins.DurationSecs != 300 {
		t.Errorf("duration_secs: want 300, got %d", ins.DurationSecs)
	}
	if ins.Sequence != 5 {
		t.Errorf("sequence: want 5, got %d", ins.Sequence)
	}
	if ins.AccountID != 11 {
		t.Errorf("account_id: want 11, got %d", ins.AccountID)
	}
	if len(gp.lifeChanges) != 2 {
		t.Errorf("expected 2 life_changes, got %d", len(gp.lifeChanges))
	}
	if len(events.projected) != 1 || events.projected[0] != 70 {
		t.Errorf("expected row 70 marked projected, got %v", events.projected)
	}
}

func TestRunOnce_GamePlayEvent_MultiGameSession(t *testing.T) {
	now := time.Now().UTC()

	game1 := makePayload(t, map[string]interface{}{
		"match_id":        "match-multi",
		"game_number":     1,
		"winning_team_id": 1,
		"turn_count":      8,
		"duration_secs":   180,
		"life_changes":    []map[string]interface{}{},
	})
	game2 := makePayload(t, map[string]interface{}{
		"match_id":        "match-multi",
		"game_number":     2,
		"winning_team_id": 2,
		"turn_count":      15,
		"duration_secs":   420,
		"life_changes":    []map[string]interface{}{},
	})

	gp := &fakeGamePlayStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 80, UserID: 1, AccountID: "acct-multi", EventType: "match.game_ended", Payload: game1, OccurredAt: now, Sequence: 10},
			{ID: 81, UserID: 1, AccountID: "acct-multi", EventType: "match.game_ended", Payload: game2, OccurredAt: now.Add(5 * time.Minute), Sequence: 11},
		},
	}
	accounts := &fakeAccountStore{accountID: 20}

	w := newWorkerWithGamePlay(events, accounts, gp)
	w.RunOnce(context.Background())

	if len(gp.gamePlayInserts) != 2 {
		t.Fatalf("expected 2 game_play inserts, got %d", len(gp.gamePlayInserts))
	}
	if gp.gamePlayInserts[0].GameNumber != 1 {
		t.Errorf("first game_number: want 1, got %d", gp.gamePlayInserts[0].GameNumber)
	}
	if gp.gamePlayInserts[1].GameNumber != 2 {
		t.Errorf("second game_number: want 2, got %d", gp.gamePlayInserts[1].GameNumber)
	}
	if len(events.projected) != 2 {
		t.Errorf("expected 2 rows projected, got %d: %v", len(events.projected), events.projected)
	}
}

func TestRunOnce_GamePlayEvent_OutOfOrderSequence(t *testing.T) {
	now := time.Now().UTC()

	payloadSeq20 := makePayload(t, map[string]interface{}{
		"match_id":    "match-ooo",
		"game_number": 1,
		"turn_count":  10,
	})
	payloadSeq10 := makePayload(t, map[string]interface{}{
		"match_id":    "match-ooo",
		"game_number": 1,
		"turn_count":  8,
	})

	gp := &fakeGamePlayStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 90, UserID: 1, AccountID: "acct-ooo", EventType: "match.game_ended", Payload: payloadSeq20, OccurredAt: now, Sequence: 20},
			{ID: 91, UserID: 1, AccountID: "acct-ooo", EventType: "match.game_ended", Payload: payloadSeq10, OccurredAt: now.Add(-time.Second), Sequence: 10},
		},
	}
	accounts := &fakeAccountStore{accountID: 30}

	w := newWorkerWithGamePlay(events, accounts, gp)
	w.RunOnce(context.Background())

	if len(gp.gamePlayInserts) != 2 {
		t.Fatalf("expected 2 InsertGamePlay calls, got %d", len(gp.gamePlayInserts))
	}
	if gp.gamePlayInserts[0].Sequence != 20 {
		t.Errorf("first insert sequence: want 20, got %d", gp.gamePlayInserts[0].Sequence)
	}
	if gp.gamePlayInserts[1].Sequence != 10 {
		t.Errorf("second insert sequence: want 10, got %d", gp.gamePlayInserts[1].Sequence)
	}
	if len(events.projected) != 2 {
		t.Errorf("expected 2 rows projected, got %v", events.projected)
	}
}

func TestRunOnce_GamePlayEvent_MissingMatchID_MarkedProjected(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"game_number": 1,
		"turn_count":  5,
	})

	gp := &fakeGamePlayStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 95, UserID: 1, AccountID: "acct-bad", EventType: "match.game_ended", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}

	w := newWorkerWithGamePlay(events, accounts, gp)
	w.RunOnce(context.Background())

	if len(gp.gamePlayInserts) != 0 {
		t.Errorf("expected 0 inserts for missing match_id, got %d", len(gp.gamePlayInserts))
	}
	if len(events.projected) != 1 || events.projected[0] != 95 {
		t.Errorf("expected row 95 marked projected, got %v", events.projected)
	}
}

func TestRunOnce_GamePlayEvent_InvalidGameNumber_MarkedProjected(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"match_id":    "match-badnum",
		"game_number": 0,
		"turn_count":  5,
	})

	gp := &fakeGamePlayStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 96, UserID: 1, AccountID: "acct-bad", EventType: "match.game_ended", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}

	w := newWorkerWithGamePlay(events, accounts, gp)
	w.RunOnce(context.Background())

	if len(gp.gamePlayInserts) != 0 {
		t.Errorf("expected 0 inserts for invalid game_number, got %d", len(gp.gamePlayInserts))
	}
	if len(events.projected) != 1 || events.projected[0] != 96 {
		t.Errorf("expected row 96 marked projected, got %v", events.projected)
	}
}

func TestRunOnce_GamePlayEvent_NoLifeChanges_OnlyGamePlayInserted(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"match_id":     "match-nolife",
		"game_number":  1,
		"turn_count":   6,
		"life_changes": []map[string]interface{}{},
	})

	gp := &fakeGamePlayStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 97, UserID: 1, AccountID: "acct-nolife", EventType: "match.game_ended", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}

	w := newWorkerWithGamePlay(events, accounts, gp)
	w.RunOnce(context.Background())

	if len(gp.gamePlayInserts) != 1 {
		t.Fatalf("expected 1 game_play insert, got %d", len(gp.gamePlayInserts))
	}
	if len(gp.lifeChanges) != 0 {
		t.Errorf("expected 0 life_changes, got %d", len(gp.lifeChanges))
	}
	if len(events.projected) != 1 || events.projected[0] != 97 {
		t.Errorf("expected row 97 marked projected, got %v", events.projected)
	}
}

// TestRunOnce_MatchCompleted_FallbackResultFromWinningTeamID verifies that when
// the daemon sends winning_team_id + player_team_id but no pre-computed result
// string, the projection worker derives "win" or "loss" correctly.
func TestRunOnce_MatchCompleted_FallbackResultFromWinningTeamID(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"match_id":        "match-fallback",
		"format":          "Ladder",
		"winning_team_id": 2,
		"player_team_id":  2, // player is team 2 → should resolve to "win"
		// result is intentionally absent to test the fallback path
	})

	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 200, UserID: 1, AccountID: "acct-1", EventType: "match.completed", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}
	matches := &fakeMatchStore{}

	w := newWorker(events, accounts, matches, &fakeDraftStore{})
	w.RunOnce(context.Background())

	if len(matches.upserts) != 1 {
		t.Fatalf("expected 1 match upsert, got %d", len(matches.upserts))
	}
	if matches.upserts[0].Result != "win" {
		t.Errorf("expected result=win, got %q", matches.upserts[0].Result)
	}
	if len(events.projected) != 1 || events.projected[0] != 200 {
		t.Errorf("expected row 200 marked projected, got %v", events.projected)
	}
}

// TestRunOnce_MatchCompleted_FallbackResultLoss verifies the loss case of the
// winning_team_id + player_team_id fallback.
func TestRunOnce_MatchCompleted_FallbackResultLoss(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"match_id":        "match-fallback-loss",
		"format":          "Ladder",
		"winning_team_id": 2,
		"player_team_id":  1, // player is team 1, opponent (team 2) won → loss
	})

	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 201, UserID: 1, AccountID: "acct-1", EventType: "match.completed", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}
	matches := &fakeMatchStore{}

	w := newWorker(events, accounts, matches, &fakeDraftStore{})
	w.RunOnce(context.Background())

	if len(matches.upserts) != 1 {
		t.Fatalf("expected 1 match upsert, got %d", len(matches.upserts))
	}
	if matches.upserts[0].Result != "loss" {
		t.Errorf("expected result=loss, got %q", matches.upserts[0].Result)
	}
}

// --- cross-tenant security tests (AC1, AC2, AC5) ---

// fakeAccountStoreCrossTenant simulates the case where the daemon-supplied
// client_id is registered under a different user_id — GetOrCreateByClientID
// returns ErrCrosstenantAccount.
type fakeAccountStoreCrossTenant struct{}

func (f *fakeAccountStoreCrossTenant) GetOrCreateByClientID(_ context.Context, _ string, _ int64) (int64, error) {
	return 0, fmt.Errorf("resolve account: %w", repository.ErrCrosstenantAccount)
}

// TestRunOnce_CrossTenantAccount_MatchCompleted_Rejected verifies that a
// match.completed event whose client_id belongs to a different user is skipped
// and no match row is written (AC1, AC5).
func TestRunOnce_CrossTenantAccount_MatchCompleted_Rejected(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"match_id":       "match-cross",
		"format":         "Standard",
		"result":         "win",
		"player_wins":    2,
		"opponent_wins":  1,
		"player_team_id": 1,
	})

	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 300, UserID: 1, AccountID: "acct-user-b", EventType: "match.completed", Payload: payload, OccurredAt: time.Now()},
		},
	}
	matches := &fakeMatchStore{}

	w := NewWorker(events, &fakeAccountStoreCrossTenant{}, matches, &fakeDraftStore{}, &fakeCollectionStore{}, &fakeInventoryStore{}, &fakeQuestStore{}, &fakeDeckStore{}, &fakeGamePlayStore{})
	w.RunOnce(context.Background())

	// The event must be marked projected so it does not block the queue.
	if len(events.projected) != 1 || events.projected[0] != 300 {
		t.Errorf("cross-tenant event must be marked projected; got %v", events.projected)
	}
	// No match must have been written.
	if len(matches.upserts) != 0 {
		t.Errorf("cross-tenant event must not write a match row; got %d upserts", len(matches.upserts))
	}
}

// TestRunOnce_CrossTenantAccount_CollectionUpdated_Rejected verifies that a
// collection.updated event whose client_id belongs to a different user is
// skipped and no card_inventory row is written (AC1, AC5).
func TestRunOnce_CrossTenantAccount_CollectionUpdated_Rejected(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"cards":    []map[string]interface{}{{"arena_id": 99999, "count": 4}},
		"is_delta": false,
	})

	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 301, UserID: 1, AccountID: "acct-user-b", EventType: "collection.updated", Payload: payload, OccurredAt: time.Now()},
		},
	}
	collection := &fakeCollectionStore{}

	w := NewWorker(events, &fakeAccountStoreCrossTenant{}, &fakeMatchStore{}, &fakeDraftStore{}, collection, &fakeInventoryStore{}, &fakeQuestStore{}, &fakeDeckStore{}, &fakeGamePlayStore{})
	w.RunOnce(context.Background())

	if len(events.projected) != 1 || events.projected[0] != 301 {
		t.Errorf("cross-tenant event must be marked projected; got %v", events.projected)
	}
	if len(collection.upserts) != 0 {
		t.Errorf("cross-tenant event must not write a card_inventory row; got %d upserts", len(collection.upserts))
	}
}

// TestRunOnce_CrossTenantAccount_InventoryUpdated_Rejected verifies that an
// inventory.updated event whose client_id belongs to a different user is
// skipped and no inventory row is written (AC1, AC5).
func TestRunOnce_CrossTenantAccount_InventoryUpdated_Rejected(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"gems": 9999,
		"gold": 99999,
	})

	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 302, UserID: 1, AccountID: "acct-user-b", EventType: "inventory.updated", Payload: payload, OccurredAt: time.Now()},
		},
	}
	inv := &fakeInventoryStoreCapturing{}

	w := NewWorker(events, &fakeAccountStoreCrossTenant{}, &fakeMatchStore{}, &fakeDraftStore{}, &fakeCollectionStore{}, inv, &fakeQuestStore{}, &fakeDeckStore{}, &fakeGamePlayStore{})
	w.RunOnce(context.Background())

	if len(events.projected) != 1 || events.projected[0] != 302 {
		t.Errorf("cross-tenant event must be marked projected; got %v", events.projected)
	}
	if len(inv.upserts) != 0 {
		t.Errorf("cross-tenant event must not write an inventory row; got %d upserts", len(inv.upserts))
	}
}

// TestRunOnce_CrossTenantAccount_QuestCompleted_Rejected verifies that a
// quest.completed event whose client_id belongs to a different user is
// skipped and no quest_session_tracking row is written (AC1, AC5).
func TestRunOnce_CrossTenantAccount_QuestCompleted_Rejected(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"quest_id":   "q-cross-001",
		"quest_name": "Win 3 Games",
		"progress":   3,
		"goal":       3,
		"xp_reward":  500,
	})

	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 303, UserID: 1, AccountID: "acct-user-b", EventType: "quest.completed", Payload: payload, OccurredAt: time.Now()},
		},
	}
	quests := &fakeQuestStoreCapturing{}

	w := NewWorker(events, &fakeAccountStoreCrossTenant{}, &fakeMatchStore{}, &fakeDraftStore{}, &fakeCollectionStore{}, &fakeInventoryStore{}, quests, &fakeDeckStore{}, &fakeGamePlayStore{})
	w.RunOnce(context.Background())

	if len(events.projected) != 1 || events.projected[0] != 303 {
		t.Errorf("cross-tenant event must be marked projected; got %v", events.projected)
	}
	if len(quests.completedInserts) != 0 {
		t.Errorf("cross-tenant event must not write a quest_session_tracking row; got %d inserts", len(quests.completedInserts))
	}
}

// TestRunOnce_CrossTenantAccount_DeckUpdated_Rejected verifies that a
// deck.updated event whose client_id belongs to a different user is
// skipped and no deck row is written (AC1, AC5).
func TestRunOnce_CrossTenantAccount_DeckUpdated_Rejected(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"deck_id": "deck-cross-001",
		"name":    "Evil Deck",
		"format":  "Standard",
		"cards":   []map[string]interface{}{},
	})

	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 304, UserID: 1, AccountID: "acct-user-b", EventType: "deck.updated", Payload: payload, OccurredAt: time.Now()},
		},
	}
	decks := &fakeDeckStoreCapturing{}

	w := NewWorker(events, &fakeAccountStoreCrossTenant{}, &fakeMatchStore{}, &fakeDraftStore{}, &fakeCollectionStore{}, &fakeInventoryStore{}, &fakeQuestStore{}, decks, &fakeGamePlayStore{})
	w.RunOnce(context.Background())

	if len(events.projected) != 1 || events.projected[0] != 304 {
		t.Errorf("cross-tenant event must be marked projected; got %v", events.projected)
	}
	if len(decks.upserts) != 0 {
		t.Errorf("cross-tenant event must not write a deck row; got %d upserts", len(decks.upserts))
	}
}

// TestRunOnce_CrossTenantAccount_ErrIsSentinel verifies that ErrCrosstenantAccount
// is exported from the repository package and is unwrappable via errors.Is.
func TestRunOnce_CrossTenantAccount_ErrIsSentinel(t *testing.T) {
	wrapped := fmt.Errorf("resolve account: %w", repository.ErrCrosstenantAccount)
	if !errors.Is(wrapped, repository.ErrCrosstenantAccount) {
		t.Error("ErrCrosstenantAccount must be unwrappable via errors.Is")
	}
}
