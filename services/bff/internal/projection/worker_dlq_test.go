package projection

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/posthog/posthog-go"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
)

// --- DLQ fake ---

type fakeDLQStore struct {
	inserts []repository.ProjectionErrorInsert
	err     error
}

func (f *fakeDLQStore) Insert(_ context.Context, ins repository.ProjectionErrorInsert) error {
	if f.err != nil {
		return f.err
	}
	f.inserts = append(f.inserts, ins)
	return nil
}

// --- PostHog fake ---

type fakePostHogClient struct {
	captured []posthog.Message
}

func (f *fakePostHogClient) Enqueue(msg posthog.Message) error {
	f.captured = append(f.captured, msg)
	return nil
}

// --- helpers ---

func newWorkerWithDLQ(events *fakeEventStore, accounts accountStore, matches *fakeMatchStore, dlq *fakeDLQStore, ph *fakePostHogClient) *Worker {
	w := NewWorker(events, accounts, matches, &fakeDraftStore{}, &fakeCollectionStore{}, &fakeInventoryStore{}, &fakeQuestStore{}, &fakeDeckStore{}, &fakeGamePlayStore{})
	w.WithDLQ(dlq)
	w.WithPostHogClient(ph)
	return w
}

// --- permanentErr sentinel tests ---

func TestPermanent_WrapAndUnwrap(t *testing.T) {
	inner := fmt.Errorf("bad payload")
	err := permanent(inner)

	if !isPermanent(err) {
		t.Error("isPermanent: want true for wrapped error, got false")
	}

	// Unwrapping must expose the original error.
	if err.Error() != "bad payload" {
		t.Errorf("Error(): want %q, got %q", "bad payload", err.Error())
	}
}

func TestPermanent_NilIsNil(t *testing.T) {
	if permanent(nil) != nil {
		t.Error("permanent(nil) must return nil")
	}
}

func TestIsPermanent_PlainError(t *testing.T) {
	err := fmt.Errorf("transient")
	if isPermanent(err) {
		t.Error("isPermanent: want false for plain error, got true")
	}
}

// --- DLQ routing tests ---

// TestDLQ_PermanentError_WritesToDLQAndMarkProjected verifies that when a
// projector returns a permanent() error, the row is written to the DLQ,
// the source row is marked projected, and no destination row is written.
func TestDLQ_PermanentError_WritesToDLQAndMarkProjected(t *testing.T) {
	// match.completed with empty account_id triggers the permanent guard
	// added in Correction 2.
	payload := makePayload(t, map[string]interface{}{
		"match_id": "match-perm-001",
		"format":   "Standard",
		"result":   "win",
	})

	dlq := &fakeDLQStore{}
	ph := &fakePostHogClient{}
	matches := &fakeMatchStore{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{
				ID:         400,
				UserID:     1,
				AccountID:  "", // empty triggers permanent guard
				EventType:  "match.completed",
				Payload:    payload,
				OccurredAt: time.Now(),
			},
		},
	}
	accounts := &fakeAccountStore{accountID: 10}

	w := newWorkerWithDLQ(events, accounts, matches, dlq, ph)
	w.RunOnce(context.Background())

	// DLQ must have received exactly one row.
	if len(dlq.inserts) != 1 {
		t.Fatalf("expected 1 DLQ insert, got %d", len(dlq.inserts))
	}
	ins := dlq.inserts[0]
	if ins.DaemonEventID != 400 {
		t.Errorf("DaemonEventID: want 400, got %d", ins.DaemonEventID)
	}
	if ins.EventType != "match.completed" {
		t.Errorf("EventType: want match.completed, got %q", ins.EventType)
	}
	if ins.AccountID != "" {
		// account_id in the DLQ row is the raw row.AccountID (empty here).
		t.Errorf("AccountID: want empty, got %q", ins.AccountID)
	}

	// Source row must be marked projected so it does not block the queue.
	if len(events.projected) != 1 || events.projected[0] != 400 {
		t.Errorf("expected row 400 marked projected, got %v", events.projected)
	}

	// No match must have been written.
	if len(matches.upserts) != 0 {
		t.Errorf("expected 0 match upserts for permanent error, got %d", len(matches.upserts))
	}
}

// TestDLQ_PermanentError_EmitsPostHogMetric verifies that a permanent error
// emits exactly one projection.dead_letter PostHog event with a hashed
// account_id (never raw PII).
func TestDLQ_PermanentError_EmitsPostHogMetric(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"match_id": "match-ph-001",
		"format":   "Standard",
		"result":   "win",
	})

	dlq := &fakeDLQStore{}
	ph := &fakePostHogClient{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{
				ID:         401,
				UserID:     1,
				AccountID:  "",
				EventType:  "match.completed",
				Payload:    payload,
				OccurredAt: time.Now(),
			},
		},
	}
	w := newWorkerWithDLQ(events, &fakeAccountStore{accountID: 10}, &fakeMatchStore{}, dlq, ph)
	w.RunOnce(context.Background())

	if len(ph.captured) != 1 {
		t.Fatalf("expected 1 PostHog event, got %d", len(ph.captured))
	}
	cap, ok := ph.captured[0].(posthog.Capture)
	if !ok {
		t.Fatalf("expected posthog.Capture, got %T", ph.captured[0])
	}
	if cap.Event != "projection.dead_letter" {
		t.Errorf("Event: want projection.dead_letter, got %q", cap.Event)
	}
	// DistinctId must be a hash, not the raw account_id.
	if cap.DistinctId == "" {
		t.Errorf("DistinctId must not be empty (should be hash of empty string)")
	}
	// Properties is posthog.Properties (map[string]interface{}) — no type assertion needed.
	if _, has := cap.Properties["account_id_hash"]; !has {
		t.Error("Properties must include account_id_hash")
	}
	// Verify raw account_id is NOT present.
	if _, has := cap.Properties["account_id"]; has {
		t.Error("Properties must NOT include raw account_id")
	}
}

// TestDLQ_PermanentError_RawPayloadPreserved verifies that the raw bytes of
// the daemon payload are stored verbatim in the DLQ row — even when the bytes
// are not valid JSON (which is the key reason raw_payload is TEXT not JSONB).
func TestDLQ_PermanentError_RawPayloadPreserved(t *testing.T) {
	badBytes := json.RawMessage(`not-json-at-all`)

	dlq := &fakeDLQStore{}
	ph := &fakePostHogClient{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{
				ID:         402,
				UserID:     1,
				AccountID:  "", // triggers permanent guard
				EventType:  "match.completed",
				Payload:    badBytes,
				OccurredAt: time.Now(),
			},
		},
	}

	w := newWorkerWithDLQ(events, &fakeAccountStore{accountID: 10}, &fakeMatchStore{}, dlq, ph)
	w.RunOnce(context.Background())

	if len(dlq.inserts) != 1 {
		t.Fatalf("expected 1 DLQ insert, got %d", len(dlq.inserts))
	}
	if dlq.inserts[0].RawPayload != string(badBytes) {
		t.Errorf("RawPayload: want %q, got %q", string(badBytes), dlq.inserts[0].RawPayload)
	}
}

// TestDLQ_DLQInsertFailure_FallsBackToSkippedMalformed verifies that when the
// DLQ store itself fails, the row is still marked projected (not stuck) and the
// outcome is skipped rather than dead-lettered.
func TestDLQ_DLQInsertFailure_FallsBackToSkippedMalformed(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"match_id": "match-dlqfail",
		"format":   "Standard",
		"result":   "win",
	})

	dlq := &fakeDLQStore{err: fmt.Errorf("db down")}
	ph := &fakePostHogClient{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{
				ID:         403,
				UserID:     1,
				AccountID:  "",
				EventType:  "match.completed",
				Payload:    payload,
				OccurredAt: time.Now(),
			},
		},
	}

	w := newWorkerWithDLQ(events, &fakeAccountStore{accountID: 10}, &fakeMatchStore{}, dlq, ph)
	w.RunOnce(context.Background())

	// Row must still be marked projected.
	if len(events.projected) != 1 || events.projected[0] != 403 {
		t.Errorf("expected row 403 marked projected even on DLQ failure, got %v", events.projected)
	}
	// No PostHog event should be emitted (DLQ insert failed before that).
	if len(ph.captured) != 0 {
		t.Errorf("expected 0 PostHog events on DLQ failure, got %d", len(ph.captured))
	}
}

// TestDLQ_NilDLQ_FallsBackToSkippedMalformed verifies that when no DLQ store
// is wired (nil), a permanent error still marks the row projected and falls
// back to outcomeSkippedMalformed rather than panicking.
func TestDLQ_NilDLQ_FallsBackToSkippedMalformed(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"match_id": "match-nodlq",
		"format":   "Standard",
		"result":   "win",
	})

	matches := &fakeMatchStore{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{
				ID:         404,
				UserID:     1,
				AccountID:  "", // permanent guard fires
				EventType:  "match.completed",
				Payload:    payload,
				OccurredAt: time.Now(),
			},
		},
	}

	// Worker with no DLQ wired (default).
	w := NewWorker(events, &fakeAccountStore{accountID: 10}, matches, &fakeDraftStore{}, &fakeCollectionStore{}, &fakeInventoryStore{}, &fakeQuestStore{}, &fakeDeckStore{}, &fakeGamePlayStore{})
	w.RunOnce(context.Background())

	// Row must still be marked projected.
	if len(events.projected) != 1 || events.projected[0] != 404 {
		t.Errorf("expected row 404 marked projected, got %v", events.projected)
	}
	if len(matches.upserts) != 0 {
		t.Errorf("expected 0 match upserts, got %d", len(matches.upserts))
	}
}

// TestDLQ_MatchCompleted_MissingAccountID_PermanentError verifies Correction 2:
// the account_id == "" guard in projectMatch fires before GetOrCreateByClientID
// and produces a permanent error (DLQ write), not a transient one.
func TestDLQ_MatchCompleted_MissingAccountID_PermanentError(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"match_id": "match-noacct",
		"format":   "Standard",
		"result":   "win",
	})

	dlq := &fakeDLQStore{}
	ph := &fakePostHogClient{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 410, UserID: 1, AccountID: "", EventType: "match.completed", Payload: payload, OccurredAt: time.Now()},
		},
	}
	// accounts store should NOT be called when account_id is empty.
	accounts := &fakeAccountStoreTracking{}

	w := newWorkerWithDLQ(events, accounts, &fakeMatchStore{}, dlq, ph)
	w.RunOnce(context.Background())

	if len(dlq.inserts) != 1 {
		t.Fatalf("expected 1 DLQ insert for missing account_id, got %d", len(dlq.inserts))
	}
	if accounts.called {
		t.Error("GetOrCreateByClientID must NOT be called when account_id is empty")
	}
}

// fakeAccountStoreTracking tracks whether GetOrCreateByClientID was called.
type fakeAccountStoreTracking struct {
	called    bool
	accountID int64
	err       error
}

func (f *fakeAccountStoreTracking) GetOrCreateByClientID(_ context.Context, _ string, _ int64) (int64, error) {
	f.called = true
	return f.accountID, f.err
}

// TestDLQ_MatchCompleted_ResultIndeterminate_DefaultsAndProjects verifies Q2
// (vault-mtg-tickets#200): when result is indeterminate the event now projects
// with result="unknown" rather than being dropped.  The DLQ must NOT receive a
// row (indeterminate result is an enrichment miss, not a permanent structural
// error), and a projection.missing_field metric IS emitted.
func TestDLQ_MatchCompleted_ResultIndeterminate_DefaultsAndProjects(t *testing.T) {
	// Both winning_team_id and player_team_id are zero → result indeterminate.
	matches := &fakeMatchStore{}
	payload := makePayload(t, map[string]interface{}{
		"match_id":        "match-indet",
		"format":          "Standard",
		"winning_team_id": 0,
		"player_team_id":  0,
	})

	dlq := &fakeDLQStore{}
	ph := &fakePostHogClient{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 420, UserID: 1, AccountID: "acct-indet", EventType: "match.completed", Payload: payload, OccurredAt: time.Now()},
		},
	}

	w := NewWorker(events, &fakeAccountStore{accountID: 10}, matches, &fakeDraftStore{}, &fakeCollectionStore{}, &fakeInventoryStore{}, &fakeQuestStore{}, &fakeDeckStore{}, &fakeGamePlayStore{})
	w.WithDLQ(dlq)
	w.WithPostHogClient(ph)
	w.RunOnce(context.Background())

	// DLQ must NOT receive a row — indeterminate result is default-filled, not dead-lettered.
	if len(dlq.inserts) != 0 {
		t.Errorf("result indeterminate must NOT produce a DLQ row; got %d inserts", len(dlq.inserts))
	}
	// Match must be written with result="unknown".
	if len(matches.upserts) != 1 {
		t.Fatalf("expected 1 match upsert for indeterminate result, got %d", len(matches.upserts))
	}
	if matches.upserts[0].Result != "unknown" {
		t.Errorf("Result: want %q (default-fill), got %q", "unknown", matches.upserts[0].Result)
	}
	// projection.missing_field metric must be emitted (not projection.dead_letter).
	if len(ph.captured) != 1 {
		t.Errorf("expected 1 projection.missing_field metric, got %d", len(ph.captured))
	}
	// Row must be marked projected.
	if len(events.projected) != 1 || events.projected[0] != 420 {
		t.Errorf("expected row 420 marked projected, got %v", events.projected)
	}
}

// TestDLQ_InventoryUpdated_MissingAccountID_PermanentError verifies that the
// existing permanent() guard on inventory.updated missing account_id also
// routes to the DLQ now that the sentinel is respected.
func TestDLQ_InventoryUpdated_MissingAccountID_PermanentError(t *testing.T) {
	payload := makePayload(t, map[string]interface{}{
		"gems": 100,
		"gold": 500,
	})

	dlq := &fakeDLQStore{}
	ph := &fakePostHogClient{}
	inv := &fakeInventoryStoreCapturing{}
	events := &fakeEventStore{
		pending: []repository.DaemonEventRow{
			{ID: 430, UserID: 1, AccountID: "", EventType: "inventory.updated", Payload: payload, OccurredAt: time.Now()},
		},
	}

	w := NewWorker(events, &fakeAccountStore{accountID: 10}, &fakeMatchStore{}, &fakeDraftStore{}, &fakeCollectionStore{}, inv, &fakeQuestStore{}, &fakeDeckStore{}, &fakeGamePlayStore{})
	w.WithDLQ(dlq)
	w.WithPostHogClient(ph)
	w.RunOnce(context.Background())

	// DLQ must receive the row.
	if len(dlq.inserts) != 1 {
		t.Fatalf("expected 1 DLQ insert, got %d", len(dlq.inserts))
	}
	if dlq.inserts[0].EventType != "inventory.updated" {
		t.Errorf("EventType: want inventory.updated, got %q", dlq.inserts[0].EventType)
	}
	// Row must be marked projected.
	if len(events.projected) != 1 || events.projected[0] != 430 {
		t.Errorf("expected row 430 marked projected, got %v", events.projected)
	}
	// No inventory write.
	if len(inv.upserts) != 0 {
		t.Errorf("expected 0 inventory upserts, got %d", len(inv.upserts))
	}
	// PostHog event emitted.
	if len(ph.captured) != 1 {
		t.Errorf("expected 1 PostHog event, got %d", len(ph.captured))
	}
}

// TestHashAccountIDProjection verifies the projection-package hash helper
// produces the expected truncated SHA-256 hex output and never returns the
// raw input.
func TestHashAccountIDProjection(t *testing.T) {
	raw := "user_2abc123"
	h := hashAccountIDProjection(raw)

	if len(h) != 16 {
		t.Errorf("hash length: want 16, got %d", len(h))
	}
	if h == raw {
		t.Error("hash must not equal raw account_id")
	}
	// Deterministic.
	if hashAccountIDProjection(raw) != h {
		t.Error("hash must be deterministic")
	}
}
