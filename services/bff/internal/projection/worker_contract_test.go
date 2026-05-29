package projection

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
	"github.com/RdHamilton/vault-mtg/services/contract"
)

// TestProjectCollectionUpdated_UsesContractType verifies that the projector
// correctly decodes a contract.CollectionUpdatedPayload from the event row and
// passes each CollectionCard's ArenaID and Count to the collection store.
func TestProjectCollectionUpdated_UsesContractType(t *testing.T) {
	payload, _ := json.Marshal(contract.CollectionUpdatedPayload{
		Cards: []contract.CollectionCard{
			{ArenaID: 111111, Count: 4},
			{ArenaID: 222222, Count: 1},
		},
		IsDelta: false,
	})

	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 500, UserID: 1, AccountID: "acct-contract-col", EventType: "collection.updated", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 77}
	collection := &fakeCollectionStore{}

	w := newWorkerWithCollection(events, accounts, collection)
	w.RunOnce(context.Background())

	if len(collection.upserts) != 2 {
		t.Fatalf("expected 2 card upserts, got %d", len(collection.upserts))
	}
	if collection.upserts[0].CardID != 111111 {
		t.Errorf("card 0 ArenaID: want 111111, got %d", collection.upserts[0].CardID)
	}
	if collection.upserts[0].Count != 4 {
		t.Errorf("card 0 Count: want 4, got %d", collection.upserts[0].Count)
	}
	if collection.upserts[1].CardID != 222222 {
		t.Errorf("card 1 ArenaID: want 222222, got %d", collection.upserts[1].CardID)
	}
	if collection.upserts[1].Count != 1 {
		t.Errorf("card 1 Count: want 1, got %d", collection.upserts[1].Count)
	}
	if len(events.projected) != 1 || events.projected[0] != 500 {
		t.Errorf("expected row 500 marked projected, got %v", events.projected)
	}
}

// TestProjectInventoryUpdated_UsesContractType verifies that the projector uses
// contract.InventoryUpdatedPayload and that the additive Boosters field
// zero-values cleanly on old payloads that don't carry it.
func TestProjectInventoryUpdated_UsesContractType(t *testing.T) {
	// Old payload without the Boosters field — must decode cleanly.
	payload, _ := json.Marshal(map[string]interface{}{
		"gems":                 750,
		"gold":                 5000,
		"total_vault_progress": 12,
		"wild_card_commons":    3,
		"wild_card_uncommons":  1,
		"wild_card_rares":      0,
		"wild_card_mythics":    0,
		// boosters intentionally absent
	})

	inv := &fakeInventoryStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 501, UserID: 1, AccountID: "acct-contract-inv", EventType: "inventory.updated", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 78}

	w := NewWorker(events, accounts, &fakeMatchStore{}, &fakeDraftStore{}, &fakeCollectionStore{}, inv, &fakeQuestStore{}, &fakeDeckStore{}, &fakeGamePlayStore{})
	w.RunOnce(context.Background())

	if len(inv.upserts) != 1 {
		t.Fatalf("expected 1 inventory upsert, got %d", len(inv.upserts))
	}
	u := inv.upserts[0]
	if u.Gems != 750 {
		t.Errorf("gems: want 750, got %d", u.Gems)
	}
	if u.Gold != 5000 {
		t.Errorf("gold: want 5000, got %d", u.Gold)
	}
	// Boosters field being absent in the payload must not cause an error.
	if len(events.projected) != 1 || events.projected[0] != 501 {
		t.Errorf("expected row 501 marked projected, got %v", events.projected)
	}
}

// TestProjectQuestProgress_UsesContractType verifies that the projector uses
// contract.QuestProgressPayload / contract.QuestEntry for the quest.progress event.
func TestProjectQuestProgress_UsesContractType(t *testing.T) {
	payload, _ := json.Marshal(contract.QuestProgressPayload{
		Quests: []contract.QuestEntry{
			{QuestID: "qct-001", QuestName: "Contract Quest", Progress: 2, Goal: 5, CanSwap: true},
		},
	})

	quests := &fakeQuestStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 502, UserID: 1, AccountID: "acct-contract-qp", EventType: "quest.progress", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 79}

	w := newWorkerWithQuests(events, accounts, quests)
	w.RunOnce(context.Background())

	if len(quests.progressUpserts) != 1 {
		t.Fatalf("expected 1 quest progress upsert, got %d", len(quests.progressUpserts))
	}
	u := quests.progressUpserts[0]
	if u.QuestID != "qct-001" {
		t.Errorf("quest_id: want qct-001, got %q", u.QuestID)
	}
	if u.Progress != 2 {
		t.Errorf("progress: want 2, got %d", u.Progress)
	}
	if !u.CanSwap {
		t.Errorf("can_swap: want true, got false")
	}
}

// TestProjectQuestCompleted_UsesContractType verifies that the projector uses
// contract.QuestCompletedPayload for the quest.completed event.
func TestProjectQuestCompleted_UsesContractType(t *testing.T) {
	now := time.Now().UTC()
	payload, _ := json.Marshal(contract.QuestCompletedPayload{
		QuestID:          "qct-done-001",
		QuestName:        "Contract Complete",
		Progress:         5,
		Goal:             5,
		XPReward:         750,
		CompletionSource: "match",
	})

	quests := &fakeQuestStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 503, UserID: 1, AccountID: "acct-contract-qc", EventType: "quest.completed", Payload: payload, OccurredAt: now},
		},
	}
	accounts := &fakeAccountStore{accountID: 80}

	w := newWorkerWithQuests(events, accounts, quests)
	w.RunOnce(context.Background())

	if len(quests.completedInserts) != 1 {
		t.Fatalf("expected 1 quest completed insert, got %d", len(quests.completedInserts))
	}
	ins := quests.completedInserts[0]
	if ins.QuestID != "qct-done-001" {
		t.Errorf("quest_id: want qct-done-001, got %q", ins.QuestID)
	}
	if ins.XPReward != 750 {
		t.Errorf("xp_reward: want 750, got %d", ins.XPReward)
	}
	if ins.CompletionSource != "match" {
		t.Errorf("completion_source: want match, got %q", ins.CompletionSource)
	}
}

// TestProjectDeckUpdated_UsesContractType verifies that the projector uses
// contract.DeckUpdatedPayload / contract.DeckCard for the deck.updated event.
func TestProjectDeckUpdated_UsesContractType(t *testing.T) {
	payload, _ := json.Marshal(contract.DeckUpdatedPayload{
		DeckID: "deck-contract-001",
		Name:   "Contract Deck",
		Format: "Historic",
		Cards: []contract.DeckCard{
			{ArenaID: 50001, Quantity: 4},
			{ArenaID: 50002, Quantity: 2},
			{ArenaID: 50003, Quantity: 1},
		},
	})

	decks := &fakeDeckStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 504, UserID: 1, AccountID: "acct-contract-dk", EventType: "deck.updated", Payload: payload, OccurredAt: time.Now()},
		},
	}
	accounts := &fakeAccountStore{accountID: 81}

	w := newWorkerWithDecks(events, accounts, decks)
	w.RunOnce(context.Background())

	if len(decks.upserts) != 1 {
		t.Fatalf("expected 1 deck upsert, got %d", len(decks.upserts))
	}
	u := decks.upserts[0]
	if u.DeckID != "deck-contract-001" {
		t.Errorf("deck_id: want deck-contract-001, got %q", u.DeckID)
	}
	if len(u.Cards) != 3 {
		t.Errorf("cards: want 3, got %d", len(u.Cards))
	}
	if u.Cards[0].ArenaID != 50001 || u.Cards[0].Quantity != 4 {
		t.Errorf("card 0: want {50001 4}, got {%d %d}", u.Cards[0].ArenaID, u.Cards[0].Quantity)
	}
}

// TestProjectGamePlayEvent_UsesContractType verifies that the projector uses
// contract.GamePlayPayload / contract.LifeChangeEntry for the match.game_ended event.
func TestProjectGamePlayEvent_UsesContractType(t *testing.T) {
	now := time.Now().UTC()
	payload, _ := json.Marshal(contract.GamePlayPayload{
		MatchID:       "match-contract-001",
		GameNumber:    1,
		WinningTeamID: 2,
		TurnCount:     14,
		DurationSecs:  400,
		LifeChanges: []contract.LifeChangeEntry{
			{TeamID: 1, LifeTotal: 20, Delta: 0, TurnNumber: 1},
			{TeamID: 2, LifeTotal: 15, Delta: -5, TurnNumber: 3},
		},
		Partial: false,
	})

	gp := &fakeGamePlayStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 505, UserID: 1, AccountID: "acct-contract-gp", EventType: "match.game_ended", Payload: payload, OccurredAt: now, Sequence: 7},
		},
	}
	accounts := &fakeAccountStore{accountID: 82}

	w := newWorkerWithGamePlay(events, accounts, gp)
	w.RunOnce(context.Background())

	if len(gp.gamePlayInserts) != 1 {
		t.Fatalf("expected 1 game_play insert, got %d", len(gp.gamePlayInserts))
	}
	ins := gp.gamePlayInserts[0]
	if ins.MatchID != "match-contract-001" {
		t.Errorf("match_id: want match-contract-001, got %q", ins.MatchID)
	}
	if ins.TurnCount != 14 {
		t.Errorf("turn_count: want 14, got %d", ins.TurnCount)
	}
	if ins.Sequence != 7 {
		t.Errorf("sequence: want 7, got %d", ins.Sequence)
	}
	if len(gp.lifeChanges) != 2 {
		t.Errorf("life_changes: want 2, got %d", len(gp.lifeChanges))
	}
	if gp.lifeChanges[0].TeamID != 1 || gp.lifeChanges[0].LifeTotal != 20 {
		t.Errorf("life change 0: want {team=1 life=20}, got {%d %d}", gp.lifeChanges[0].TeamID, gp.lifeChanges[0].LifeTotal)
	}
	if gp.lifeChanges[1].Delta != -5 {
		t.Errorf("life change 1 delta: want -5, got %d", gp.lifeChanges[1].Delta)
	}
}
