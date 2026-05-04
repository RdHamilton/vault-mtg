package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	"github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
	contract "github.com/RdHamilton/MTGA-Companion/services/contract"
)

// mockBroadcaster records the events broadcast for assertions.
type mockBroadcaster struct {
	events []contract.DaemonEvent
}

func (m *mockBroadcaster) BroadcastDaemonEvent(event contract.DaemonEvent) {
	m.events = append(m.events, event)
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

	if len(broadcaster.events) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(broadcaster.events))
	}

	if broadcaster.events[0].Type != "sync:ratings" {
		t.Errorf("expected type 'sync:ratings', got %q", broadcaster.events[0].Type)
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
