package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/handlers"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/middleware"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
	contract "github.com/RdHamilton/vault-mtg/services/contract"
	"github.com/posthog/posthog-go"
)

// mockPostHogClient records Enqueue calls for test assertions.
type mockPostHogClient struct {
	calls []posthog.Message
}

func (m *mockPostHogClient) Enqueue(msg posthog.Message) error {
	m.calls = append(m.calls, msg)
	return nil
}

// broadcastedCall records a single BroadcastDaemonEvent invocation.
type broadcastedCall struct {
	userID int64
	event  contract.DaemonEvent
}

// mockBroadcaster records the events broadcast for assertions.
type mockBroadcaster struct {
	calls []broadcastedCall
}

// insertCall records a single Insert invocation on the mock repo.
type insertCall struct {
	userID     int64
	accountID  string
	eventType  string
	payload    json.RawMessage
	occurredAt time.Time
	eventID    string
	sequence   uint64
}

// mockDaemonEventsRepo is a test double for DaemonEventInserter.
type mockDaemonEventsRepo struct {
	calls []insertCall
	err   error // if non-nil, Insert returns this error
}

func (m *mockDaemonEventsRepo) Insert(
	_ context.Context,
	userID int64,
	accountID string,
	eventType string,
	payload json.RawMessage,
	occurredAt time.Time,
	eventID string,
	sequence uint64,
) error {
	m.calls = append(m.calls, insertCall{
		userID:     userID,
		accountID:  accountID,
		eventType:  eventType,
		payload:    payload,
		occurredAt: occurredAt,
		eventID:    eventID,
		sequence:   sequence,
	})

	return m.err
}

func (m *mockBroadcaster) BroadcastDaemonEvent(userID int64, event contract.DaemonEvent) {
	m.calls = append(m.calls, broadcastedCall{userID: userID, event: event})
}

// mockKeyLister satisfies the activeKeyLister interface used by middleware.APIKeyAuth.
type mockKeyLister struct {
	keys []repository.APIKey
}

func (m *mockKeyLister) ListAllActive(_ context.Context) ([]repository.APIKey, error) {
	return m.keys, nil
}

func (m *mockKeyLister) UpdateLastUsedAt(_ context.Context, _ int64) error { return nil }

// mustHash returns a bcrypt hash of token, fataling the test on error.
func mustHash(t *testing.T, token string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return string(hash)
}

// buildHandler wraps IngestHandler with APIKeyAuth middleware backed by repo.
func buildHandler(broadcaster handlers.EventBroadcaster, repo *mockKeyLister) http.Handler {
	ih := handlers.NewIngestHandler(broadcaster)
	return middleware.APIKeyAuth(repo)(http.HandlerFunc(ih.IngestEvent))
}

func ingestRequest(t *testing.T, token string, event contract.DaemonEvent) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()

	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req, httptest.NewRecorder()
}

func makeEvent(eventType string) contract.DaemonEvent {
	payload, _ := json.Marshal(map[string]string{"key": "value"})

	return contract.DaemonEvent{
		Type:       eventType,
		AccountID:  "acct_test",
		SessionID:  "sess_test",
		OccurredAt: time.Now().UTC(),
		Payload:    payload,
	}
}

func TestIngestEvent_Accepted(t *testing.T) {
	const token = "valid-test-token"

	repo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 1, KeyHash: mustHash(t, token), UserID: 42},
	}}

	broadcaster := &mockBroadcaster{}
	handler := buildHandler(broadcaster, repo)

	event := makeEvent("sync:ratings")
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", rr.Code)
	}

	if len(broadcaster.calls) != 1 {
		t.Fatalf("expected 1 broadcast call, got %d", len(broadcaster.calls))
	}

	if broadcaster.calls[0].event.Type != "sync:ratings" {
		t.Errorf("expected type 'sync:ratings', got %q", broadcaster.calls[0].event.Type)
	}
}

// TestIngestEvent_BroadcastCarriesAuthenticatedUserID verifies that the ingest
// handler passes the userID extracted from the auth middleware context to the
// broadcaster — not a caller-supplied value.  This is the key security property
// that prevents a compromised daemon from pushing events to another user's SSE
// stream.
func TestIngestEvent_BroadcastCarriesAuthenticatedUserID(t *testing.T) {
	const token = "daemon-token"
	const wantUserID int64 = 99

	repo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 5, KeyHash: mustHash(t, token), UserID: wantUserID},
	}}

	broadcaster := &mockBroadcaster{}
	handler := buildHandler(broadcaster, repo)

	event := makeEvent("draft:pick")
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}

	if len(broadcaster.calls) != 1 {
		t.Fatalf("expected 1 broadcast call, got %d", len(broadcaster.calls))
	}

	if got := broadcaster.calls[0].userID; got != wantUserID {
		t.Errorf("broadcaster received userID=%d, want %d — event would be routed to wrong SSE subscribers", got, wantUserID)
	}
}

func TestIngestEvent_Unauthorized_NoKeysInDB(t *testing.T) {
	// Empty key list — no valid tokens registered.
	repo := &mockKeyLister{keys: nil}

	handler := buildHandler(&mockBroadcaster{}, repo)

	event := makeEvent("sync:ratings")
	req, rr := ingestRequest(t, "anything", event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestIngestEvent_Unauthorized_WrongToken(t *testing.T) {
	repo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 1, KeyHash: mustHash(t, "correct-secret"), UserID: 42},
	}}

	handler := buildHandler(&mockBroadcaster{}, repo)

	event := makeEvent("sync:ratings")
	req, rr := ingestRequest(t, "wrong-secret", event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestIngestEvent_Unauthorized_NoHeader(t *testing.T) {
	repo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 1, KeyHash: mustHash(t, "correct-secret"), UserID: 42},
	}}

	handler := buildHandler(&mockBroadcaster{}, repo)

	event := makeEvent("sync:ratings")
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/events", bytes.NewReader(body))
	// No Authorization header.
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestIngestEvent_BadRequest_EmptyType(t *testing.T) {
	const token = "valid-test-token"

	repo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 1, KeyHash: mustHash(t, token), UserID: 42},
	}}

	handler := buildHandler(&mockBroadcaster{}, repo)

	event := contract.DaemonEvent{
		AccountID:  "acct_abc",
		OccurredAt: time.Now().UTC(),
		// Type intentionally empty.
	}

	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestIngestEvent_BadRequest_InvalidJSON(t *testing.T) {
	const token = "valid-test-token"

	repo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 1, KeyHash: mustHash(t, token), UserID: 42},
	}}

	handler := buildHandler(&mockBroadcaster{}, repo)

	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/events",
		bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestIngestEvent_NilBroadcaster(t *testing.T) {
	const token = "valid-test-token"

	repo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 1, KeyHash: mustHash(t, token), UserID: 42},
	}}

	handler := buildHandler(nil, repo)

	event := makeEvent("sync:ratings")
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	// nil broadcaster must not panic; event is accepted.
	if rr.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", rr.Code)
	}
}

// TestIngestEvent_PersistsEventWhenRepoWired verifies that IngestEvent calls
// Insert on the repository with the correct parameters when a repo is wired.
func TestIngestEvent_PersistsEventWhenRepoWired(t *testing.T) {
	const token = "persist-token"
	const wantUserID int64 = 55

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 3, KeyHash: mustHash(t, token), UserID: wantUserID},
	}}

	broadcaster := &mockBroadcaster{}
	eventsRepo := &mockDaemonEventsRepo{}

	ih := handlers.NewIngestHandler(broadcaster).WithRepository(eventsRepo)
	handler := middleware.APIKeyAuth(keyRepo)(http.HandlerFunc(ih.IngestEvent))

	event := makeEvent("draft:pick")
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}

	if len(eventsRepo.calls) != 1 {
		t.Fatalf("expected 1 Insert call, got %d", len(eventsRepo.calls))
	}

	call := eventsRepo.calls[0]
	if call.userID != wantUserID {
		t.Errorf("Insert userID=%d, want %d", call.userID, wantUserID)
	}
	if call.eventType != "draft:pick" {
		t.Errorf("Insert eventType=%q, want %q", call.eventType, "draft:pick")
	}

	// Broadcast must still have happened.
	if len(broadcaster.calls) != 1 {
		t.Fatalf("expected 1 broadcast call, got %d", len(broadcaster.calls))
	}
}

// TestIngestEvent_BroadcastsEvenWhenInsertFails verifies that a persistence
// failure does not prevent the live SSE broadcast — the frontend must still
// receive the event even when the database is degraded.
func TestIngestEvent_BroadcastsEvenWhenInsertFails(t *testing.T) {
	const token = "fail-persist-token"
	const wantUserID int64 = 77

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 7, KeyHash: mustHash(t, token), UserID: wantUserID},
	}}

	broadcaster := &mockBroadcaster{}
	eventsRepo := &mockDaemonEventsRepo{err: fmt.Errorf("db connection refused")}

	ih := handlers.NewIngestHandler(broadcaster).WithRepository(eventsRepo)
	handler := middleware.APIKeyAuth(keyRepo)(http.HandlerFunc(ih.IngestEvent))

	event := makeEvent("match:result")
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	// Handler must still return 202 despite the insert error.
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}

	// Broadcast must have proceeded despite persistence failure.
	if len(broadcaster.calls) != 1 {
		t.Fatalf("expected 1 broadcast call even after insert failure, got %d", len(broadcaster.calls))
	}

	// Insert was called — the error happened, it didn't silently skip.
	if len(eventsRepo.calls) != 1 {
		t.Fatalf("expected 1 Insert call, got %d", len(eventsRepo.calls))
	}
}

// TestIngestEvent_NilRepo_BroadcastOnly verifies that IngestEvent behaves
// exactly as before when no repository is wired — broadcast proceeds, no panic.
func TestIngestEvent_NilRepo_BroadcastOnly(t *testing.T) {
	const token = "nil-repo-token"
	const wantUserID int64 = 33

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 9, KeyHash: mustHash(t, token), UserID: wantUserID},
	}}

	broadcaster := &mockBroadcaster{}

	// No WithRepository call — repo is nil.
	ih := handlers.NewIngestHandler(broadcaster)
	handler := middleware.APIKeyAuth(keyRepo)(http.HandlerFunc(ih.IngestEvent))

	event := makeEvent("sync:collection")
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}

	if len(broadcaster.calls) != 1 {
		t.Fatalf("expected 1 broadcast call, got %d", len(broadcaster.calls))
	}

	if broadcaster.calls[0].userID != wantUserID {
		t.Errorf("broadcast userID=%d, want %d", broadcaster.calls[0].userID, wantUserID)
	}
}

// TestIngestEvent_SequenceFieldAccepted verifies that the ingest handler accepts
// a DaemonEvent carrying a non-zero Sequence value (ADR-013) and that the
// sequence is preserved on the broadcasted event.
func TestIngestEvent_SequenceFieldAccepted(t *testing.T) {
	const token = "seq-token"
	const wantSequence uint64 = 42

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 13, KeyHash: mustHash(t, token), UserID: 50},
	}}

	broadcaster := &mockBroadcaster{}
	ih := handlers.NewIngestHandler(broadcaster)
	handler := middleware.APIKeyAuth(keyRepo)(http.HandlerFunc(ih.IngestEvent))

	payload, _ := json.Marshal(map[string]string{"k": "v"})
	event := contract.DaemonEvent{
		Type:       "match.completed",
		AccountID:  "acct_seq",
		SessionID:  "sess_seq",
		Sequence:   wantSequence,
		OccurredAt: time.Now().UTC(),
		Payload:    payload,
	}
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}

	if len(broadcaster.calls) != 1 {
		t.Fatalf("expected 1 broadcast call, got %d", len(broadcaster.calls))
	}

	if got := broadcaster.calls[0].event.Sequence; got != wantSequence {
		t.Errorf("broadcast event Sequence=%d, want %d", got, wantSequence)
	}
}

// TestIngestEvent_SequencePropagatedToRepo verifies that the Sequence value from
// the DaemonEvent contract is forwarded to the repository Insert call so it is
// persisted in the daemon_events.sequence column (ADR-013, ticket #1521).
func TestIngestEvent_SequencePropagatedToRepo(t *testing.T) {
	const token = "seq-repo-token"
	const wantSequence uint64 = 77

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 15, KeyHash: mustHash(t, token), UserID: 60},
	}}

	eventsRepo := &mockDaemonEventsRepo{}
	ih := handlers.NewIngestHandler(&mockBroadcaster{}).WithRepository(eventsRepo)
	handler := middleware.APIKeyAuth(keyRepo)(http.HandlerFunc(ih.IngestEvent))

	payload, _ := json.Marshal(map[string]string{"key": "value"})
	event := contract.DaemonEvent{
		Type:       "match.completed",
		AccountID:  "acct_seq_repo",
		EventID:    "evt_seq_01",
		Sequence:   wantSequence,
		OccurredAt: time.Now().UTC(),
		Payload:    payload,
	}
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}

	if len(eventsRepo.calls) != 1 {
		t.Fatalf("expected 1 Insert call, got %d", len(eventsRepo.calls))
	}

	if got := eventsRepo.calls[0].sequence; got != wantSequence {
		t.Errorf("Insert sequence=%d, want %d", got, wantSequence)
	}
}

// TestIngestEvent_EventIDPropagatedToRepo verifies that the event_id from the
// contract is forwarded to the repository Insert call so it can be persisted in
// the daemon_events.event_id column for idempotency (ticket #1405).
func TestIngestEvent_EventIDPropagatedToRepo(t *testing.T) {
	const token = "eventid-token"
	const wantEventID = "evt_01HXYZ"

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 11, KeyHash: mustHash(t, token), UserID: 20},
	}}

	eventsRepo := &mockDaemonEventsRepo{}
	ih := handlers.NewIngestHandler(&mockBroadcaster{}).WithRepository(eventsRepo)
	handler := middleware.APIKeyAuth(keyRepo)(http.HandlerFunc(ih.IngestEvent))

	payload, _ := json.Marshal(map[string]string{"key": "value"})
	event := contract.DaemonEvent{
		Type:       "match.completed",
		AccountID:  "acct_test",
		EventID:    wantEventID,
		OccurredAt: time.Now().UTC(),
		Payload:    payload,
	}
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}

	if len(eventsRepo.calls) != 1 {
		t.Fatalf("expected 1 Insert call, got %d", len(eventsRepo.calls))
	}

	if got := eventsRepo.calls[0].eventID; got != wantEventID {
		t.Errorf("Insert eventID=%q, want %q", got, wantEventID)
	}
}

// buildHandlerWithPostHog constructs an IngestHandler with the given PostHog
// client and APIKey middleware wired — helper shared across gap detection tests.
func buildHandlerWithPostHog(broadcaster handlers.EventBroadcaster, keyRepo *mockKeyLister, phClient handlers.PostHogClient) http.Handler {
	ih := handlers.NewIngestHandler(broadcaster).WithPostHogClient(phClient)
	return middleware.APIKeyAuth(keyRepo)(http.HandlerFunc(ih.IngestEvent))
}

// TestIngestHandler_GapDetected_LogsAndCaptures verifies that when two
// consecutive events arrive with a sequence gap, the PostHog client receives a
// daemon_event_gap_detected capture call with the correct properties.
func TestIngestHandler_GapDetected_LogsAndCaptures(t *testing.T) {
	const token = "gap-token"

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 20, KeyHash: mustHash(t, token), UserID: 100},
	}}

	phClient := &mockPostHogClient{}
	broadcaster := &mockBroadcaster{}
	handler := buildHandlerWithPostHog(broadcaster, keyRepo, phClient)

	// First event — establishes baseline at seq=1.
	firstEvent := contract.DaemonEvent{
		Type:       "match.completed",
		AccountID:  "acct_gap_test",
		SessionID:  "sess_gap_test",
		Sequence:   1,
		OccurredAt: time.Now().UTC(),
		Payload:    json.RawMessage(`{}`),
	}

	req, rr := ingestRequest(t, token, firstEvent)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("first event: expected 202, got %d", rr.Code)
	}

	if len(phClient.calls) != 0 {
		t.Fatalf("first event should not trigger PostHog capture, got %d calls", len(phClient.calls))
	}

	// Second event — jumps to seq=5, skipping 2, 3, 4.
	gapEvent := contract.DaemonEvent{
		Type:       "match.completed",
		AccountID:  "acct_gap_test",
		SessionID:  "sess_gap_test",
		Sequence:   5,
		OccurredAt: time.Now().UTC(),
		Payload:    json.RawMessage(`{}`),
	}

	req2, rr2 := ingestRequest(t, token, gapEvent)
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusAccepted {
		t.Fatalf("gap event: expected 202 (gap detection must not block), got %d", rr2.Code)
	}

	if len(phClient.calls) != 1 {
		t.Fatalf("expected 1 PostHog capture call after gap, got %d", len(phClient.calls))
	}

	capture, ok := phClient.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}

	if capture.Event != "daemon_event_gap_detected" {
		t.Errorf("expected event=%q, got %q", "daemon_event_gap_detected", capture.Event)
	}

	// account_id must be hashed — verify it is not the raw value.
	if capture.DistinctId == "acct_gap_test" {
		t.Error("DistinctId must be hashed, got raw account_id")
	}

	if len(capture.DistinctId) != 16 {
		t.Errorf("DistinctId hash should be 16 chars, got %d: %q", len(capture.DistinctId), capture.DistinctId)
	}

	// Verify expected_sequence property.
	if v, ok := capture.Properties["expected_sequence"]; !ok {
		t.Error("expected_sequence property missing from PostHog capture")
	} else if v != uint64(2) {
		t.Errorf("expected_sequence=%v, want 2", v)
	}

	if v, ok := capture.Properties["received_sequence"]; !ok {
		t.Error("received_sequence property missing from PostHog capture")
	} else if v != uint64(5) {
		t.Errorf("received_sequence=%v, want 5", v)
	}
}

// TestIngestHandler_NoGap_NoCapture verifies that sequential events do not
// trigger a PostHog capture.
func TestIngestHandler_NoGap_NoCapture(t *testing.T) {
	const token = "nogap-token"

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 21, KeyHash: mustHash(t, token), UserID: 101},
	}}

	phClient := &mockPostHogClient{}
	broadcaster := &mockBroadcaster{}
	handler := buildHandlerWithPostHog(broadcaster, keyRepo, phClient)

	for seq := uint64(1); seq <= 5; seq++ {
		evt := contract.DaemonEvent{
			Type:       "draft.pick",
			AccountID:  "acct_nogap",
			SessionID:  "sess_nogap",
			Sequence:   seq,
			OccurredAt: time.Now().UTC(),
			Payload:    json.RawMessage(`{}`),
		}

		req, rr := ingestRequest(t, token, evt)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusAccepted {
			t.Fatalf("seq=%d: expected 202, got %d", seq, rr.Code)
		}
	}

	if len(phClient.calls) != 0 {
		t.Errorf("sequential events must not trigger PostHog captures, got %d", len(phClient.calls))
	}
}

// TestIngestEvent_DraftPackBroadcastToSSE verifies that a draft.pack event
// received by IngestHandler is forwarded to the EventBroadcaster (SSE broker).
// This confirms the live draft viewer receives pack-state updates in real time.
func TestIngestEvent_DraftPackBroadcastToSSE(t *testing.T) {
	const token = "draft-pack-token"
	const wantUserID int64 = 111

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 30, KeyHash: mustHash(t, token), UserID: wantUserID},
	}}

	broadcaster := &mockBroadcaster{}
	handler := buildHandler(broadcaster, keyRepo)

	event := makeEvent("draft.pack")
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}

	if len(broadcaster.calls) != 1 {
		t.Fatalf("expected 1 broadcast call for draft.pack, got %d", len(broadcaster.calls))
	}

	if got := broadcaster.calls[0].event.Type; got != "draft.pack" {
		t.Errorf("broadcast event type=%q, want %q", got, "draft.pack")
	}

	if got := broadcaster.calls[0].userID; got != wantUserID {
		t.Errorf("broadcast userID=%d, want %d — wrong SSE subscriber targeted", got, wantUserID)
	}
}

// TestIngestEvent_DraftPickBroadcastToSSE verifies that a draft.pick event
// received by IngestHandler is forwarded to the EventBroadcaster (SSE broker).
// This confirms the live draft viewer receives pick confirmations in real time.
func TestIngestEvent_DraftPickBroadcastToSSE(t *testing.T) {
	const token = "draft-pick-token"
	const wantUserID int64 = 222

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 31, KeyHash: mustHash(t, token), UserID: wantUserID},
	}}

	broadcaster := &mockBroadcaster{}
	handler := buildHandler(broadcaster, keyRepo)

	event := makeEvent("draft.pick")
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}

	if len(broadcaster.calls) != 1 {
		t.Fatalf("expected 1 broadcast call for draft.pick, got %d", len(broadcaster.calls))
	}

	if got := broadcaster.calls[0].event.Type; got != "draft.pick" {
		t.Errorf("broadcast event type=%q, want %q", got, "draft.pick")
	}

	if got := broadcaster.calls[0].userID; got != wantUserID {
		t.Errorf("broadcast userID=%d, want %d — wrong SSE subscriber targeted", got, wantUserID)
	}
}

// TestIngestHandler_SequenceReset_NotAGap_NoCapture verifies that a sequence
// reset (daemon restart) is not emitted as a gap to PostHog.
func TestIngestHandler_SequenceReset_NotAGap_NoCapture(t *testing.T) {
	const token = "reset-token"

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 22, KeyHash: mustHash(t, token), UserID: 102},
	}}

	phClient := &mockPostHogClient{}
	broadcaster := &mockBroadcaster{}
	handler := buildHandlerWithPostHog(broadcaster, keyRepo, phClient)

	// Build up to seq=50.
	for seq := uint64(1); seq <= 50; seq++ {
		evt := contract.DaemonEvent{
			Type:       "match.completed",
			AccountID:  "acct_reset",
			SessionID:  "sess_reset",
			Sequence:   seq,
			OccurredAt: time.Now().UTC(),
			Payload:    json.RawMessage(`{}`),
		}

		req, rr := ingestRequest(t, token, evt)
		handler.ServeHTTP(rr, req)
	}

	// Reset: daemon restarts, seq goes back to 1.
	resetEvt := contract.DaemonEvent{
		Type:       "match.completed",
		AccountID:  "acct_reset",
		SessionID:  "sess_reset",
		Sequence:   1,
		OccurredAt: time.Now().UTC(),
		Payload:    json.RawMessage(`{}`),
	}

	req, rr := ingestRequest(t, token, resetEvt)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("reset event: expected 202, got %d", rr.Code)
	}

	if len(phClient.calls) != 0 {
		t.Errorf("sequence reset must not trigger PostHog gap capture, got %d calls", len(phClient.calls))
	}
}

// ---------------------------------------------------------------------------
// Heartbeat drift-detection tests (#2569)
//
// These verify the BFF-side PostHog emit logic for daemon.log_format_drift.
// ---------------------------------------------------------------------------

// buildHeartbeatPayload marshals a heartbeat payload with the given drift
// fields and returns it as a json.RawMessage, ready to embed in a DaemonEvent.
func buildHeartbeatPayload(t *testing.T, parseFailureCount uint32, sampleLineHash string, failedEventTypes []string) json.RawMessage {
	t.Helper()
	type hb struct {
		ParseFailureCount uint32   `json:"parse_failure_count"`
		SampleLineHash    string   `json:"sample_line_hash,omitempty"`
		FailedEventTypes  []string `json:"failed_event_types,omitempty"`
	}
	raw, err := json.Marshal(hb{
		ParseFailureCount: parseFailureCount,
		SampleLineHash:    sampleLineHash,
		FailedEventTypes:  failedEventTypes,
	})
	if err != nil {
		t.Fatalf("marshal heartbeat payload: %v", err)
	}
	return raw
}

// TestIngestHandler_HeartbeatWithDriftEmitsPostHog verifies that a
// daemon.heartbeat with parse_failure_count > 0 causes the BFF to emit a
// daemon.log_format_drift event to PostHog with the correct fields.
func TestIngestHandler_HeartbeatWithDriftEmitsPostHog(t *testing.T) {
	const token = "hb-drift-token"

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 50, KeyHash: mustHash(t, token), UserID: 200},
	}}

	phClient := &mockPostHogClient{}
	ih := handlers.NewIngestHandler(&mockBroadcaster{}).WithPostHogClient(phClient)
	handler := middleware.APIKeyAuth(keyRepo)(http.HandlerFunc(ih.IngestEvent))

	payload := buildHeartbeatPayload(t, 3, "abc1234567890abc", []string{"draft.pack", "match.completed"})
	event := contract.DaemonEvent{
		Type:       "daemon.heartbeat",
		AccountID:  "acct_hb_drift",
		SessionID:  "sess_hb_drift",
		Sequence:   1,
		OccurredAt: time.Now().UTC(),
		Payload:    payload,
	}
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}

	// Exactly one PostHog call must have been made.
	if len(phClient.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(phClient.calls))
	}

	capture, ok := phClient.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}

	if capture.Event != "daemon.log_format_drift" {
		t.Errorf("expected PostHog event=%q, got %q", "daemon.log_format_drift", capture.Event)
	}

	// distinct_id must be the hash of the account ID, not the raw value.
	if capture.DistinctId == "acct_hb_drift" {
		t.Error("DistinctId must be hashed, got raw account_id")
	}
	if len(capture.DistinctId) != 16 {
		t.Errorf("DistinctId hash must be 16 chars, got %d: %q", len(capture.DistinctId), capture.DistinctId)
	}

	// parse_failure_count must be present.
	if v, ok := capture.Properties["parse_failure_count"]; !ok {
		t.Error("parse_failure_count property missing")
	} else if v != uint32(3) {
		t.Errorf("parse_failure_count=%v, want 3", v)
	}

	// sample_line_hash must be present.
	if _, ok := capture.Properties["sample_line_hash"]; !ok {
		t.Error("sample_line_hash property missing")
	}

	// failed_event_types must be present.
	if _, ok := capture.Properties["failed_event_types"]; !ok {
		t.Error("failed_event_types property missing")
	}
}

// TestIngestHandler_HeartbeatZeroDriftNoEmit verifies that a daemon.heartbeat
// with parse_failure_count == 0 does NOT emit a daemon.log_format_drift event.
func TestIngestHandler_HeartbeatZeroDriftNoEmit(t *testing.T) {
	const token = "hb-nodrift-token"

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 51, KeyHash: mustHash(t, token), UserID: 201},
	}}

	phClient := &mockPostHogClient{}
	ih := handlers.NewIngestHandler(&mockBroadcaster{}).WithPostHogClient(phClient)
	handler := middleware.APIKeyAuth(keyRepo)(http.HandlerFunc(ih.IngestEvent))

	payload := buildHeartbeatPayload(t, 0, "", nil)
	event := contract.DaemonEvent{
		Type:       "daemon.heartbeat",
		AccountID:  "acct_hb_nodrift",
		SessionID:  "sess_hb_nodrift",
		Sequence:   1,
		OccurredAt: time.Now().UTC(),
		Payload:    payload,
	}
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}

	if len(phClient.calls) != 0 {
		t.Errorf("expected 0 PostHog calls for zero drift, got %d", len(phClient.calls))
	}
}

// ---------------------------------------------------------------------------
// Structured error telemetry tests (#2139)
// ---------------------------------------------------------------------------

// buildHeartbeatPayloadFull marshals a full heartbeat payload including both
// #2569 drift fields and #2139 BFF-failure fields.
func buildHeartbeatPayloadFull(t *testing.T, parseFailureCount uint32, consecutiveBFFFailures uint32, lastBFFStatusCode int) json.RawMessage {
	t.Helper()
	type hb struct {
		ParseFailureCount      uint32 `json:"parse_failure_count"`
		ConsecutiveBFFFailures uint32 `json:"consecutive_bff_failures,omitempty"`
		LastBFFStatusCode      int    `json:"last_bff_status_code,omitempty"`
	}
	raw, err := json.Marshal(hb{
		ParseFailureCount:      parseFailureCount,
		ConsecutiveBFFFailures: consecutiveBFFFailures,
		LastBFFStatusCode:      lastBFFStatusCode,
	})
	if err != nil {
		t.Fatalf("marshal heartbeat payload: %v", err)
	}
	return raw
}

// TestIngestHandler_HeartbeatWith3ConsecutiveFailures_EmitsDispatchDegraded
// verifies that a daemon.heartbeat with consecutive_bff_failures >= 3 causes
// the BFF to emit daemon.dispatch_degraded to PostHog.
func TestIngestHandler_HeartbeatWith3ConsecutiveFailures_EmitsDispatchDegraded(t *testing.T) {
	const token = "degraded-token"

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 60, KeyHash: mustHash(t, token), UserID: 300},
	}}

	phClient := &mockPostHogClient{}
	ih := handlers.NewIngestHandler(&mockBroadcaster{}).WithPostHogClient(phClient)
	handler := middleware.APIKeyAuth(keyRepo)(http.HandlerFunc(ih.IngestEvent))

	payload := buildHeartbeatPayloadFull(t, 0, 3, 503)
	event := contract.DaemonEvent{
		Type:       "daemon.heartbeat",
		AccountID:  "acct_degraded",
		SessionID:  "sess_degraded",
		Sequence:   1,
		OccurredAt: time.Now().UTC(),
		Payload:    payload,
	}
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}

	// Exactly one PostHog call for dispatch_degraded.
	if len(phClient.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(phClient.calls))
	}

	capture, ok := phClient.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}

	if capture.Event != "daemon.dispatch_degraded" {
		t.Errorf("expected event=%q, got %q", "daemon.dispatch_degraded", capture.Event)
	}
	if capture.DistinctId == "acct_degraded" {
		t.Error("distinct_id must be hashed, got raw account_id")
	}
	if v, ok := capture.Properties["consecutive_failures"]; !ok {
		t.Error("consecutive_failures property missing")
	} else if v != uint32(3) {
		t.Errorf("consecutive_failures=%v, want 3", v)
	}
	if v, ok := capture.Properties["status_code"]; !ok {
		t.Error("status_code property missing")
	} else if v != 503 {
		t.Errorf("status_code=%v, want 503", v)
	}
}

// TestIngestHandler_HeartbeatWith2ConsecutiveFailures_NoEmit verifies that a
// daemon.heartbeat with consecutive_bff_failures < 3 does NOT emit
// daemon.dispatch_degraded (threshold not yet reached).
func TestIngestHandler_HeartbeatWith2ConsecutiveFailures_NoEmit(t *testing.T) {
	const token = "nodegraded-token"

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 61, KeyHash: mustHash(t, token), UserID: 301},
	}}

	phClient := &mockPostHogClient{}
	ih := handlers.NewIngestHandler(&mockBroadcaster{}).WithPostHogClient(phClient)
	handler := middleware.APIKeyAuth(keyRepo)(http.HandlerFunc(ih.IngestEvent))

	payload := buildHeartbeatPayloadFull(t, 0, 2, 503)
	event := contract.DaemonEvent{
		Type:       "daemon.heartbeat",
		AccountID:  "acct_nodegraded",
		SessionID:  "sess_nodegraded",
		Sequence:   1,
		OccurredAt: time.Now().UTC(),
		Payload:    payload,
	}
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}

	if len(phClient.calls) != 0 {
		t.Errorf("expected 0 PostHog calls for count<3, got %d", len(phClient.calls))
	}
}

// TestIngestHandler_AuthFailedEmitsPostHog verifies that a daemon.auth_failed
// event causes the BFF to emit daemon.auth_failed to PostHog with the correct
// properties, and that distinct_id is the hashed account ID.
func TestIngestHandler_AuthFailedEmitsPostHog(t *testing.T) {
	const token = "auth-failed-token"

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 62, KeyHash: mustHash(t, token), UserID: 302},
	}}

	phClient := &mockPostHogClient{}
	ih := handlers.NewIngestHandler(&mockBroadcaster{}).WithPostHogClient(phClient)
	handler := middleware.APIKeyAuth(keyRepo)(http.HandlerFunc(ih.IngestEvent))

	type authFailedPayload struct {
		Reason        string `json:"reason"`
		BFFStatusCode int    `json:"bff_status_code,omitempty"`
		Platform      string `json:"platform"`
		DaemonVersion string `json:"daemon_version"`
	}
	raw, _ := json.Marshal(authFailedPayload{
		Reason:        "bff_rejected",
		BFFStatusCode: 401,
		Platform:      "darwin",
		DaemonVersion: "0.3.3",
	})

	event := contract.DaemonEvent{
		Type:       "daemon.auth_failed",
		AccountID:  "acct_auth_fail",
		SessionID:  "sess_auth_fail",
		Sequence:   1,
		OccurredAt: time.Now().UTC(),
		Payload:    json.RawMessage(raw),
	}
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}

	if len(phClient.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(phClient.calls))
	}

	capture, ok := phClient.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}

	if capture.Event != "daemon.auth_failed" {
		t.Errorf("expected event=%q, got %q", "daemon.auth_failed", capture.Event)
	}
	if capture.DistinctId == "acct_auth_fail" {
		t.Error("distinct_id must be hashed, got raw account_id")
	}
	if len(capture.DistinctId) != 16 {
		t.Errorf("distinct_id must be 16-char hash, got %d", len(capture.DistinctId))
	}
	if v, ok := capture.Properties["reason"]; !ok {
		t.Error("reason property missing")
	} else if v != "bff_rejected" {
		t.Errorf("reason=%v, want bff_rejected", v)
	}
	if v, ok := capture.Properties["bff_status_code"]; !ok {
		t.Error("bff_status_code property missing for bff_rejected reason")
	} else if v != 401 {
		t.Errorf("bff_status_code=%v, want 401", v)
	}
	if v, ok := capture.Properties["platform"]; !ok {
		t.Error("platform property missing")
	} else if v != "darwin" {
		t.Errorf("platform=%v, want darwin", v)
	}
}

// TestIngestHandler_KeychainErrorEmitsPostHog verifies that a
// daemon.keychain_error event causes the BFF to emit daemon.keychain_error to
// PostHog with the correct properties.
func TestIngestHandler_KeychainErrorEmitsPostHog(t *testing.T) {
	const token = "keychain-error-token"

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 63, KeyHash: mustHash(t, token), UserID: 303},
	}}

	phClient := &mockPostHogClient{}
	ih := handlers.NewIngestHandler(&mockBroadcaster{}).WithPostHogClient(phClient)
	handler := middleware.APIKeyAuth(keyRepo)(http.HandlerFunc(ih.IngestEvent))

	type keychainErrorPayload struct {
		ErrorType     string `json:"error_type"`
		Platform      string `json:"platform"`
		DaemonVersion string `json:"daemon_version"`
	}
	raw, _ := json.Marshal(keychainErrorPayload{
		ErrorType:     "not_found",
		Platform:      "windows",
		DaemonVersion: "0.3.3",
	})

	event := contract.DaemonEvent{
		Type:       "daemon.keychain_error",
		AccountID:  "acct_keychain_err",
		SessionID:  "sess_keychain_err",
		Sequence:   1,
		OccurredAt: time.Now().UTC(),
		Payload:    json.RawMessage(raw),
	}
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}

	if len(phClient.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(phClient.calls))
	}

	capture, ok := phClient.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}

	if capture.Event != "daemon.keychain_error" {
		t.Errorf("expected event=%q, got %q", "daemon.keychain_error", capture.Event)
	}
	if capture.DistinctId == "acct_keychain_err" {
		t.Error("distinct_id must be hashed, got raw account_id")
	}
	if v, ok := capture.Properties["error_type"]; !ok {
		t.Error("error_type property missing")
	} else if v != "not_found" {
		t.Errorf("error_type=%v, want not_found", v)
	}
	if v, ok := capture.Properties["platform"]; !ok {
		t.Error("platform property missing")
	} else if v != "windows" {
		t.Errorf("platform=%v, want windows", v)
	}
}

// TestIngestHandler_AllEvents_DistinctIdIsHashed verifies that for all three
// new error telemetry event types, the PostHog distinct_id is always the hashed
// account ID — never the raw account_id string.
func TestIngestHandler_AllEvents_DistinctIdIsHashed(t *testing.T) {
	const rawAccountID = "raw_pii_account_id"
	const token = "pii-check-token"

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 64, KeyHash: mustHash(t, token), UserID: 304},
	}}

	tests := []struct {
		eventType string
		payload   json.RawMessage
	}{
		{
			eventType: "daemon.auth_failed",
			payload: mustMarshal(t, map[string]interface{}{
				"reason": "pkce_timeout", "platform": "darwin", "daemon_version": "0.3.3",
			}),
		},
		{
			eventType: "daemon.keychain_error",
			payload: mustMarshal(t, map[string]interface{}{
				"error_type": "os_error", "platform": "darwin", "daemon_version": "0.3.3",
			}),
		},
		{
			eventType: "daemon.heartbeat",
			payload:   buildHeartbeatPayloadFull(t, 0, 5, 503),
		},
	}

	for _, tc := range tests {
		t.Run(tc.eventType, func(t *testing.T) {
			phClient := &mockPostHogClient{}
			ih := handlers.NewIngestHandler(&mockBroadcaster{}).WithPostHogClient(phClient)
			handler := middleware.APIKeyAuth(keyRepo)(http.HandlerFunc(ih.IngestEvent))

			event := contract.DaemonEvent{
				Type:       tc.eventType,
				AccountID:  rawAccountID,
				SessionID:  "sess_pii_check",
				Sequence:   1,
				OccurredAt: time.Now().UTC(),
				Payload:    tc.payload,
			}
			req, rr := ingestRequest(t, token, event)
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusAccepted {
				t.Fatalf("expected 202, got %d", rr.Code)
			}
			if len(phClient.calls) == 0 {
				t.Fatalf("expected at least 1 PostHog call for %s", tc.eventType)
			}
			for _, msg := range phClient.calls {
				capture, ok := msg.(posthog.Capture)
				if !ok {
					continue
				}
				if capture.DistinctId == rawAccountID {
					t.Errorf("%s: distinct_id must not be raw account_id", tc.eventType)
				}
			}
		})
	}
}

// mustMarshal is a test helper that marshals v to JSON and fatals on error.
func mustMarshal(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("mustMarshal: %v", err)
	}
	return json.RawMessage(raw)
}

// TestIngestHandler_KeychainError_NonEmptyAccountID_HashesDistinctId verifies
// that a daemon.keychain_error event with a non-empty AccountID uses the hashed
// account ID as distinct_id (post-auth case B per Ray's OQ-1 verdict).
func TestIngestHandler_KeychainError_NonEmptyAccountID_HashesDistinctId(t *testing.T) {
	const token = "kc-hashid-token"
	const accountID = "post_auth_account_id"

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 65, KeyHash: mustHash(t, token), UserID: 305},
	}}

	phClient := &mockPostHogClient{}
	ih := handlers.NewIngestHandler(&mockBroadcaster{}).WithPostHogClient(phClient)
	handler := middleware.APIKeyAuth(keyRepo)(http.HandlerFunc(ih.IngestEvent))

	raw, _ := json.Marshal(map[string]string{
		"error_type": "not_found", "platform": "darwin", "daemon_version": "0.3.3",
	})
	event := contract.DaemonEvent{
		Type:       "daemon.keychain_error",
		AccountID:  accountID,
		SessionID:  "sess_kc_hash",
		Sequence:   1,
		OccurredAt: time.Now().UTC(),
		Payload:    json.RawMessage(raw),
	}
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
	if len(phClient.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(phClient.calls))
	}
	capture := phClient.calls[0].(posthog.Capture)
	if capture.DistinctId == accountID {
		t.Error("distinct_id must not be raw account_id for keychain_error")
	}
	if len(capture.DistinctId) != 16 {
		t.Errorf("distinct_id must be 16-char hash, got %d", len(capture.DistinctId))
	}
}

// TestIngestHandler_DriftEvent_DistinctIdIsHashed verifies that the distinct_id
// in the PostHog drift capture is the hashed account ID, never the raw value.
func TestIngestHandler_DriftEvent_DistinctIdIsHashed(t *testing.T) {
	const token = "hb-hashcheck-token"
	const rawAccountID = "acct_pii_check"

	keyRepo := &mockKeyLister{keys: []repository.APIKey{
		{ID: 52, KeyHash: mustHash(t, token), UserID: 202},
	}}

	phClient := &mockPostHogClient{}
	ih := handlers.NewIngestHandler(&mockBroadcaster{}).WithPostHogClient(phClient)
	handler := middleware.APIKeyAuth(keyRepo)(http.HandlerFunc(ih.IngestEvent))

	payload := buildHeartbeatPayload(t, 1, "deadbeef12345678", []string{"deck.updated"})
	event := contract.DaemonEvent{
		Type:       "daemon.heartbeat",
		AccountID:  rawAccountID,
		SessionID:  "sess_hashcheck",
		Sequence:   1,
		OccurredAt: time.Now().UTC(),
		Payload:    payload,
	}
	req, rr := ingestRequest(t, token, event)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}

	if len(phClient.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(phClient.calls))
	}

	capture, ok := phClient.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not posthog.Capture")
	}

	// The raw account ID must never appear as distinct_id.
	if capture.DistinctId == rawAccountID {
		t.Errorf("distinct_id must be hashed, got raw %q", rawAccountID)
	}
	if len(capture.DistinctId) != 16 {
		t.Errorf("distinct_id must be 16-char hash, got len=%d", len(capture.DistinctId))
	}
}
