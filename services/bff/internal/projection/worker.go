// Package projection provides a background worker that fans daemon_events rows
// into destination tables (matches, draft_sessions, card_inventory, inventory,
// quests, quest_session_tracking, decks, game_plays).
package projection

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/RdHamilton/vault-mtg/services/contract"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
)

const (
	batchSize    = 100
	tickInterval = 30 * time.Second
)

// daemonEventStore is the subset of DaemonEventsRepository the worker uses.
type daemonEventStore interface {
	ListPendingProjection(ctx context.Context, limit int) ([]repository.DaemonEventRow, error)
	MarkProjected(ctx context.Context, id int64) error
}

// accountStore resolves accounts.id from accounts.client_id (the raw MTGA string).
type accountStore interface {
	GetOrCreateByClientID(ctx context.Context, clientID string, userID int64) (int64, error)
}

// matchStore writes to the matches table.
type matchStore interface {
	UpsertMatch(ctx context.Context, m repository.MatchUpsert) error
}

// draftStore writes to the draft_sessions table.
type draftStore interface {
	UpsertDraftSession(ctx context.Context, s repository.DraftSessionUpsert) error
}

// collectionStore writes card counts to the card_inventory table.
type collectionStore interface {
	UpsertDelta(ctx context.Context, u repository.CardInventoryUpsert) error
}

// inventoryStore writes player inventory snapshots to the inventory table.
type inventoryStore interface {
	UpsertInventory(ctx context.Context, u repository.InventoryUpsert) error
}

// questStore writes quest progress and completion records to the quests and
// quest_session_tracking tables.
type questStore interface {
	UpsertQuestProgress(ctx context.Context, u repository.QuestProgressUpsert) error
	InsertQuestCompleted(ctx context.Context, ins repository.QuestCompletedInsert) error
}

// deckStore writes deck snapshots to the decks and deck_cards tables.
type deckStore interface {
	UpsertDeck(ctx context.Context, u repository.DeckUpsert) error
}

// gamePlayStore writes individual game records and life-change rows.
type gamePlayStore interface {
	InsertGamePlay(ctx context.Context, ins repository.GamePlayInsert) (int64, error)
	InsertLifeChanges(ctx context.Context, changes []repository.LifeChangeInsert) error
}

// Worker projects pending daemon_events rows into their destination tables.
type Worker struct {
	events     daemonEventStore
	accounts   accountStore
	matches    matchStore
	drafts     draftStore
	collection collectionStore
	inventory  inventoryStore
	quests     questStore
	decks      deckStore
	gamePlays  gamePlayStore
}

// NewWorker returns a Worker wired with the provided stores.
func NewWorker(
	events daemonEventStore,
	accounts accountStore,
	matches matchStore,
	drafts draftStore,
	collection collectionStore,
	inventory inventoryStore,
	quests questStore,
	decks deckStore,
	gamePlays gamePlayStore,
) *Worker {
	return &Worker{
		events:     events,
		accounts:   accounts,
		matches:    matches,
		drafts:     drafts,
		collection: collection,
		inventory:  inventory,
		quests:     quests,
		decks:      decks,
		gamePlays:  gamePlays,
	}
}

// Run starts the projection loop.  It performs an immediate drain on startup,
// then ticks every 30 seconds.  The loop exits when ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	log.Println("[projection] worker started")

	w.runOnce(ctx)

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[projection] worker stopped")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

// RunOnce is exported for integration tests.
func (w *Worker) RunOnce(ctx context.Context) {
	w.runOnce(ctx)
}

// runOnce fetches up to batchSize pending events and projects each one.
func (w *Worker) runOnce(ctx context.Context) {
	start := time.Now()

	var projected, skippedUnknown, skippedMalformed, errored int

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[projection] runOnce PANIC recovered: %v", r)
		}

		log.Printf(
			"[projection] runOnce completed pending=%d projected=%d skipped_unknown=%d skipped_malformed=%d errored=%d duration_ms=%d",
			projected+skippedUnknown+skippedMalformed+errored,
			projected, skippedUnknown, skippedMalformed, errored,
			time.Since(start).Milliseconds(),
		)
	}()

	rows, err := w.events.ListPendingProjection(ctx, batchSize)
	if err != nil {
		log.Printf("[projection] ListPendingProjection: %v", err)
		errored++
		return
	}

	for i := range rows {
		row := rows[i]

		outcome := w.projectRow(ctx, &row)

		switch outcome {
		case outcomeProjected:
			projected++
		case outcomeSkippedUnknown:
			skippedUnknown++
		case outcomeSkippedMalformed:
			skippedMalformed++
		case outcomeErrored:
			errored++
		}
	}
}

type projectionOutcome int

const (
	outcomeProjected projectionOutcome = iota
	outcomeSkippedUnknown
	outcomeSkippedMalformed
	outcomeErrored
)

// projectRow processes a single daemon_events row.
// It always attempts to mark the row as projected (even on skip/error) so
// malformed rows don't block the queue.
func (w *Worker) projectRow(ctx context.Context, row *repository.DaemonEventRow) projectionOutcome {
	var writeErr error

	outcome := outcomeProjected

	switch row.EventType {
	case "match.completed":
		writeErr = w.projectMatch(ctx, row)
		if writeErr != nil {
			log.Printf("[projection] projectMatch id=%d: %v", row.ID, writeErr)
			outcome = outcomeSkippedMalformed
		}

	case "draft.started", "draft.completed":
		writeErr = w.projectDraftSession(ctx, row)
		if writeErr != nil {
			log.Printf("[projection] projectDraftSession id=%d type=%s: %v", row.ID, row.EventType, writeErr)
			outcome = outcomeSkippedMalformed
		}

	case "draft.pick":
		// v0.2.0: increment total_picks on the session.
		writeErr = w.projectDraftPick(ctx, row)
		if writeErr != nil {
			log.Printf("[projection] projectDraftPick id=%d: %v", row.ID, writeErr)
			outcome = outcomeSkippedMalformed
		}

	case "collection.updated":
		writeErr = w.projectCollectionUpdated(ctx, row)
		if writeErr != nil {
			log.Printf("[projection] projectCollectionUpdated id=%d: %v", row.ID, writeErr)
			outcome = outcomeSkippedMalformed
		}

	case "inventory.updated":
		writeErr = w.projectInventoryUpdated(ctx, row)
		if writeErr != nil {
			log.Printf("[projection] projectInventoryUpdated id=%d: %v", row.ID, writeErr)
			outcome = outcomeSkippedMalformed
		}

	case "quest.progress":
		writeErr = w.projectQuestProgress(ctx, row)
		if writeErr != nil {
			log.Printf("[projection] projectQuestProgress id=%d: %v", row.ID, writeErr)
			outcome = outcomeSkippedMalformed
		}

	case "quest.completed":
		writeErr = w.projectQuestCompleted(ctx, row)
		if writeErr != nil {
			log.Printf("[projection] projectQuestCompleted id=%d: %v", row.ID, writeErr)
			outcome = outcomeSkippedMalformed
		}

	case "deck.updated":
		writeErr = w.projectDeckUpdated(ctx, row)
		if writeErr != nil {
			log.Printf("[projection] projectDeckUpdated id=%d: %v", row.ID, writeErr)
			outcome = outcomeSkippedMalformed
		}

	case "match.game_ended":
		writeErr = w.projectGamePlayEvent(ctx, row)
		if writeErr != nil {
			log.Printf("[projection] projectGamePlayEvent id=%d: %v", row.ID, writeErr)
			outcome = outcomeSkippedMalformed
		}

	default:
		log.Printf("[projection] unknown event_type=%q id=%d — marking projected", row.EventType, row.ID)
		outcome = outcomeSkippedUnknown
	}

	// Always mark projected so we don't re-scan this row.
	if err := w.events.MarkProjected(ctx, row.ID); err != nil {
		log.Printf("[projection] MarkProjected id=%d: %v", row.ID, err)
		return outcomeErrored
	}

	return outcome
}

// --- payload shapes ---

type matchPayload struct {
	MatchID         string  `json:"match_id"`
	EventID         string  `json:"event_id"`
	EventName       string  `json:"event_name"`
	Format          string  `json:"format"`
	Result          string  `json:"result"`
	ResultReason    *string `json:"result_reason"`
	PlayerWins      int     `json:"player_wins"`
	OpponentWins    int     `json:"opponent_wins"`
	PlayerTeamID    int     `json:"player_team_id"`
	DeckID          *string `json:"deck_id"`
	RankBefore      *string `json:"rank_before"`
	RankAfter       *string `json:"rank_after"`
	DurationSeconds *int    `json:"duration_seconds"`
	OpponentName    *string `json:"opponent_name"`
	OpponentID      *string `json:"opponent_id"`
	// WinningTeamID is included so the projection can derive Result when the
	// daemon did not pre-compute it (e.g. player.authenticated not yet seen).
	WinningTeamID int `json:"winning_team_id"`
}

type draftPayload struct {
	SessionID string `json:"session_id"`
	EventName string `json:"event_name"`
	SetCode   string `json:"set_code"`
	DraftType string `json:"draft_type"`
	Status    string `json:"status"`
}

func (w *Worker) projectMatch(ctx context.Context, row *repository.DaemonEventRow) error {
	var p matchPayload
	if err := json.Unmarshal(row.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal match payload: %w", err)
	}

	if p.MatchID == "" {
		return fmt.Errorf("match payload missing match_id")
	}

	if p.Format == "" {
		return fmt.Errorf("match payload missing format")
	}

	result := normaliseResult(p.Result)
	// Fallback: derive result from winning_team_id + player_team_id when the
	// daemon did not pre-compute the result string (player.authenticated not
	// yet observed in that daemon session).
	if result == "" && p.PlayerTeamID > 0 && p.WinningTeamID > 0 {
		if p.WinningTeamID == p.PlayerTeamID {
			result = "win"
		} else {
			result = "loss"
		}
	}
	if result == "" {
		return fmt.Errorf("match payload: result indeterminate (result=%q winning_team_id=%d player_team_id=%d)", p.Result, p.WinningTeamID, p.PlayerTeamID)
	}

	accountID, err := w.accounts.GetOrCreateByClientID(ctx, row.AccountID, row.UserID)
	if err != nil {
		return fmt.Errorf("resolve account: %w", err)
	}

	eventID := p.EventID
	if eventID == "" && row.EventID != nil {
		eventID = *row.EventID
	}

	return w.matches.UpsertMatch(ctx, repository.MatchUpsert{
		ID:              p.MatchID,
		AccountID:       accountID,
		EventID:         eventID,
		EventName:       p.EventName,
		Timestamp:       row.OccurredAt,
		DurationSeconds: p.DurationSeconds,
		PlayerWins:      p.PlayerWins,
		OpponentWins:    p.OpponentWins,
		PlayerTeamID:    p.PlayerTeamID,
		DeckID:          p.DeckID,
		RankBefore:      p.RankBefore,
		RankAfter:       p.RankAfter,
		Format:          p.Format,
		Result:          result,
		ResultReason:    p.ResultReason,
		OpponentName:    p.OpponentName,
		OpponentID:      p.OpponentID,
	})
}

func (w *Worker) projectDraftSession(ctx context.Context, row *repository.DaemonEventRow) error {
	var p draftPayload
	if err := json.Unmarshal(row.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal draft payload: %w", err)
	}

	if p.SessionID == "" {
		return fmt.Errorf("draft payload missing session_id")
	}

	accountID, err := w.accounts.GetOrCreateByClientID(ctx, row.AccountID, row.UserID)
	if err != nil {
		return fmt.Errorf("resolve account: %w", err)
	}

	status := p.Status
	if status == "" {
		if row.EventType == "draft.completed" {
			status = "completed"
		} else {
			status = "in_progress"
		}
	}

	var endTime *time.Time
	if row.EventType == "draft.completed" {
		t := row.OccurredAt
		endTime = &t
	}

	return w.drafts.UpsertDraftSession(ctx, repository.DraftSessionUpsert{
		ID:        p.SessionID,
		AccountID: accountID,
		EventName: p.EventName,
		SetCode:   p.SetCode,
		DraftType: p.DraftType,
		StartTime: row.OccurredAt,
		EndTime:   endTime,
		Status:    status,
	})
}

type draftPickPayload struct {
	SessionID string `json:"session_id"`
}

func (w *Worker) projectDraftPick(ctx context.Context, row *repository.DaemonEventRow) error {
	var p draftPickPayload
	if err := json.Unmarshal(row.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal draft pick payload: %w", err)
	}

	if p.SessionID == "" {
		return fmt.Errorf("draft.pick payload missing session_id")
	}

	accountID, err := w.accounts.GetOrCreateByClientID(ctx, row.AccountID, row.UserID)
	if err != nil {
		return fmt.Errorf("resolve account: %w", err)
	}

	// Upsert the session with a bumped total_picks counter via GREATEST.
	return w.drafts.UpsertDraftSession(ctx, repository.DraftSessionUpsert{
		ID:         p.SessionID,
		AccountID:  accountID,
		StartTime:  row.OccurredAt,
		Status:     "in_progress",
		TotalPicks: 1, // GREATEST(1, current) effectively increments when used in the ON CONFLICT clause
	})
}

// projectCollectionUpdated applies the delta from a collection.updated event
// to card_inventory.  Each card entry is upserted independently so a partial
// delta (IsDelta=true) only touches the cards that changed.
//
// Idempotency: the snapshot_hash is derived from the raw payload bytes so
// replaying the exact same event produces no new writes.
func (w *Worker) projectCollectionUpdated(ctx context.Context, row *repository.DaemonEventRow) error {
	var p contract.CollectionUpdatedPayload
	if err := json.Unmarshal(row.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal collection.updated payload: %w", err)
	}

	if len(p.Cards) == 0 {
		// Empty delta is a no-op; not an error.
		return nil
	}

	accountID, err := w.accounts.GetOrCreateByClientID(ctx, row.AccountID, row.UserID)
	if err != nil {
		return fmt.Errorf("resolve account: %w", err)
	}

	// Snapshot hash is computed from the raw payload bytes so it is stable
	// across re-sends of the same event.
	h := sha256.Sum256(row.Payload)
	snapshotHash := fmt.Sprintf("%x", h)

	for _, card := range p.Cards {
		if err := w.collection.UpsertDelta(ctx, repository.CardInventoryUpsert{
			AccountID:    accountID,
			CardID:       card.ArenaID,
			Count:        card.Count,
			SnapshotHash: snapshotHash,
		}); err != nil {
			return fmt.Errorf("UpsertDelta card_id=%d: %w", card.ArenaID, err)
		}
	}

	return nil
}

// --- inventory.updated projector ---

func (w *Worker) projectInventoryUpdated(ctx context.Context, row *repository.DaemonEventRow) error {
	var p contract.InventoryUpdatedPayload
	if err := json.Unmarshal(row.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal inventory.updated payload: %w", err)
	}

	if row.AccountID == "" {
		return fmt.Errorf("inventory.updated payload missing account_id")
	}

	accountID, err := w.accounts.GetOrCreateByClientID(ctx, row.AccountID, row.UserID)
	if err != nil {
		return fmt.Errorf("resolve account: %w", err)
	}

	return w.inventory.UpsertInventory(ctx, repository.InventoryUpsert{
		AccountID:          accountID,
		Gems:               p.Gems,
		Gold:               p.Gold,
		TotalVaultProgress: p.TotalVaultProgress,
		WildCardCommons:    p.WildCardCommons,
		WildCardUncommons:  p.WildCardUncommons,
		WildCardRares:      p.WildCardRares,
		WildCardMythics:    p.WildCardMythics,
		UpdatedAt:          row.OccurredAt,
	})
}

// --- quest.progress projector ---

func (w *Worker) projectQuestProgress(ctx context.Context, row *repository.DaemonEventRow) error {
	var p contract.QuestProgressPayload
	if err := json.Unmarshal(row.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal quest.progress payload: %w", err)
	}

	accountID, err := w.accounts.GetOrCreateByClientID(ctx, row.AccountID, row.UserID)
	if err != nil {
		return fmt.Errorf("projectQuestProgress resolve account client_id=%s: %w", row.AccountID, err)
	}

	for _, q := range p.Quests {
		if q.QuestID == "" {
			continue
		}

		if err := w.quests.UpsertQuestProgress(ctx, repository.QuestProgressUpsert{
			AccountID: accountID,
			QuestID:   q.QuestID,
			QuestName: q.QuestName,
			Progress:  q.Progress,
			Goal:      q.Goal,
			CanSwap:   q.CanSwap,
			SeenAt:    row.OccurredAt,
		}); err != nil {
			return fmt.Errorf("UpsertQuestProgress quest_id=%s: %w", q.QuestID, err)
		}
	}

	return nil
}

// --- quest.completed projector ---

func (w *Worker) projectQuestCompleted(ctx context.Context, row *repository.DaemonEventRow) error {
	var p contract.QuestCompletedPayload
	if err := json.Unmarshal(row.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal quest.completed payload: %w", err)
	}

	if p.QuestID == "" {
		return fmt.Errorf("quest.completed payload missing quest_id")
	}

	accountID, err := w.accounts.GetOrCreateByClientID(ctx, row.AccountID, row.UserID)
	if err != nil {
		return fmt.Errorf("resolve account: %w", err)
	}

	return w.quests.InsertQuestCompleted(ctx, repository.QuestCompletedInsert{
		AccountID:        accountID,
		QuestID:          p.QuestID,
		QuestName:        p.QuestName,
		Progress:         p.Progress,
		Goal:             p.Goal,
		XPReward:         p.XPReward,
		CompletionSource: p.CompletionSource,
		OccurredAt:       row.OccurredAt,
	})
}

// --- deck.updated projector ---

func (w *Worker) projectDeckUpdated(ctx context.Context, row *repository.DaemonEventRow) error {
	var p contract.DeckUpdatedPayload
	if err := json.Unmarshal(row.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal deck.updated payload: %w", err)
	}

	if p.DeckID == "" {
		return fmt.Errorf("deck.updated payload missing deck_id")
	}

	accountID, err := w.accounts.GetOrCreateByClientID(ctx, row.AccountID, row.UserID)
	if err != nil {
		return fmt.Errorf("resolve account: %w", err)
	}

	cards := make([]repository.DeckCard, 0, len(p.Cards))
	for _, c := range p.Cards {
		cards = append(cards, repository.DeckCard{
			ArenaID:  c.ArenaID,
			Quantity: c.Quantity,
		})
	}

	return w.decks.UpsertDeck(ctx, repository.DeckUpsert{
		DeckID:    p.DeckID,
		AccountID: accountID,
		Name:      p.Name,
		Format:    p.Format,
		Cards:     cards,
		UpdatedAt: row.OccurredAt,
	})
}

// --- match.game_ended projector ---

// projectGamePlayEvent projects a match.game_ended event into game_plays and
// life_change_tracking.
//
// Ordering guarantee: the Sequence field from the DaemonEvent envelope is
// written to game_plays.sequence.  InsertGamePlay enforces a WHERE
// game_plays.sequence < EXCLUDED.sequence guard on conflict, ensuring that
// out-of-order retransmissions of the same (match_id, game_number) do not
// regress the stored state.
func (w *Worker) projectGamePlayEvent(ctx context.Context, row *repository.DaemonEventRow) error {
	var p contract.GamePlayPayload
	if err := json.Unmarshal(row.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal match.game_ended payload: %w", err)
	}

	// Partial events are GRE buffer flushes emitted before a game completes.
	// They may not yet carry a final match_id or game_number, so skip those
	// guards.  A follow-on ticket will add GRE entry parsing to populate these
	// fields once the GRE log schema is mapped.
	if !p.Partial {
		if p.MatchID == "" {
			return fmt.Errorf("match.game_ended payload missing match_id")
		}

		if p.GameNumber < 1 {
			return fmt.Errorf("match.game_ended payload invalid game_number %d", p.GameNumber)
		}
	}

	accountID, err := w.accounts.GetOrCreateByClientID(ctx, row.AccountID, row.UserID)
	if err != nil {
		return fmt.Errorf("resolve account: %w", err)
	}

	gamePlayID, err := w.gamePlays.InsertGamePlay(ctx, repository.GamePlayInsert{
		AccountID:     accountID,
		MatchID:       p.MatchID,
		GameNumber:    p.GameNumber,
		WinningTeamID: p.WinningTeamID,
		TurnCount:     p.TurnCount,
		DurationSecs:  p.DurationSecs,
		Sequence:      row.Sequence,
		OccurredAt:    row.OccurredAt,
		Partial:       p.Partial,
	})
	if err != nil {
		return fmt.Errorf("InsertGamePlay: %w", err)
	}

	if len(p.LifeChanges) == 0 {
		return nil
	}

	changes := make([]repository.LifeChangeInsert, 0, len(p.LifeChanges))
	for _, lc := range p.LifeChanges {
		changes = append(changes, repository.LifeChangeInsert{
			AccountID:  accountID,
			GamePlayID: gamePlayID,
			TeamID:     lc.TeamID,
			LifeTotal:  lc.LifeTotal,
			Delta:      lc.Delta,
			TurnNumber: lc.TurnNumber,
		})
	}

	if err := w.gamePlays.InsertLifeChanges(ctx, changes); err != nil {
		return fmt.Errorf("InsertLifeChanges: %w", err)
	}

	return nil
}

// normaliseResult maps win/loss variants to the canonical DB value.
func normaliseResult(s string) string {
	switch s {
	case "win", "Win", "WIN":
		return "win"
	case "loss", "Loss", "LOSS":
		return "loss"
	default:
		return ""
	}
}
