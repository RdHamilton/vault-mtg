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

	contract "github.com/RdHamilton/MTGA-Companion/services/contract"
	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	"github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

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
) error {
	m.calls = append(m.calls, insertCall{
		userID:     userID,
		accountID:  accountID,
		eventType:  eventType,
		payload:    payload,
		occurredAt: occurredAt,
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

// TestIngestEvent_JWTUserIDOverridesAPIKeyUserID verifies that when DaemonJWT
// context is present (JWT auth path), the JWT-derived userID is used for
// broadcasting — not the API-key userID that UserIDFromContext would return.
// This tests the bug-fix: inner-scope := no longer shadows the outer userID.
func TestIngestEvent_JWTUserIDOverridesAPIKeyUserID(t *testing.T) {
	const apiKeyUserID int64 = 42 // from API key context
	const jwtUserID int64 = 77    // from daemon JWT context — must win

	broadcaster := &mockBroadcaster{}
	ih := handlers.NewIngestHandler(broadcaster)

	event := makeEvent("draft:pick")
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Inject both API-key userID and JWT userID into context.
	ctx := middleware.WithUserID(req.Context(), apiKeyUserID)
	ctx = middleware.WithDaemonUserID(ctx, jwtUserID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	ih.IngestEvent(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}

	if len(broadcaster.calls) != 1 {
		t.Fatalf("expected 1 broadcast call, got %d", len(broadcaster.calls))
	}

	// The JWT userID must be used, not the API key userID.
	if got := broadcaster.calls[0].userID; got != jwtUserID {
		t.Errorf("broadcaster received userID=%d, want JWT userID=%d", got, jwtUserID)
	}

	// AccountID must also be scoped to JWT userID.
	wantAccount := fmt.Sprintf("user:%d", jwtUserID)
	if got := broadcaster.calls[0].event.AccountID; got != wantAccount {
		t.Errorf("event.AccountID=%q, want %q", got, wantAccount)
	}
}

// TestIngestEvent_JWTRouteChain mounts the real DaemonJWTAuth middleware around
// IngestHandler (no manual WithDaemonUserID seeding), signs a real JWT, and
// verifies the handler returns 202 with the correct userID scoping. This
// catches regressions in the middleware-to-handler wiring.
func TestIngestEvent_JWTRouteChain(t *testing.T) {
	const secret = "ingest-jwt-chain-secret"
	const wantUserID int64 = 123

	// Issue a real token via the same function the register handler uses.
	token, err := middleware.IssueDaemonJWT(secret, wantUserID, "daemon-abc")
	if err != nil {
		t.Fatalf("IssueDaemonJWT: %v", err)
	}

	broadcaster := &mockBroadcaster{}
	ih := handlers.NewIngestHandler(broadcaster)

	// Wrap the handler with the real middleware — no manual context seeding.
	handler := middleware.DaemonJWTAuth(secret)(http.HandlerFunc(ih.IngestEvent))

	event := makeEvent("draft:pick")
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}

	if len(broadcaster.calls) != 1 {
		t.Fatalf("expected 1 broadcast call, got %d", len(broadcaster.calls))
	}

	if got := broadcaster.calls[0].userID; got != wantUserID {
		t.Errorf("broadcaster userID=%d, want %d", got, wantUserID)
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
