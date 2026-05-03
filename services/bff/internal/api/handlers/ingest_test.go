package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	contract "github.com/ramonehamilton/mtga-contract"
)

// mockBroadcaster records the events broadcast for assertions.
type mockBroadcaster struct {
	events []contract.DaemonEvent
}

func (m *mockBroadcaster) BroadcastDaemonEvent(event contract.DaemonEvent) {
	m.events = append(m.events, event)
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
	t.Setenv("DAEMON_SECRET", "test-secret")

	broadcaster := &mockBroadcaster{}
	handler := handlers.NewIngestHandler(broadcaster)

	event := makeEvent("sync:ratings")
	req, rr := ingestRequest(t, "test-secret", event)
	handler.IngestEvent(rr, req)

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

func TestIngestEvent_Unauthorized_EnvSecretUnset(t *testing.T) {
	t.Setenv("DAEMON_SECRET", "")

	handler := handlers.NewIngestHandler(&mockBroadcaster{})

	event := makeEvent("sync:ratings")
	req, rr := ingestRequest(t, "anything", event)
	handler.IngestEvent(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestIngestEvent_Unauthorized_WrongToken(t *testing.T) {
	t.Setenv("DAEMON_SECRET", "correct-secret")

	handler := handlers.NewIngestHandler(&mockBroadcaster{})

	event := makeEvent("sync:ratings")
	req, rr := ingestRequest(t, "wrong-secret", event)
	handler.IngestEvent(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestIngestEvent_Unauthorized_NoHeader(t *testing.T) {
	t.Setenv("DAEMON_SECRET", "correct-secret")

	handler := handlers.NewIngestHandler(&mockBroadcaster{})

	event := makeEvent("sync:ratings")
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/events", bytes.NewReader(body))
	// No Authorization header.
	rr := httptest.NewRecorder()

	handler.IngestEvent(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestIngestEvent_BadRequest_EmptyType(t *testing.T) {
	t.Setenv("DAEMON_SECRET", "test-secret")

	handler := handlers.NewIngestHandler(&mockBroadcaster{})

	event := contract.DaemonEvent{
		AccountID:  "acct_abc",
		OccurredAt: time.Now().UTC(),
		// Type intentionally empty.
	}

	req, rr := ingestRequest(t, "test-secret", event)
	handler.IngestEvent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestIngestEvent_BadRequest_InvalidJSON(t *testing.T) {
	t.Setenv("DAEMON_SECRET", "test-secret")

	handler := handlers.NewIngestHandler(&mockBroadcaster{})

	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/events",
		bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-secret")

	rr := httptest.NewRecorder()
	handler.IngestEvent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestIngestEvent_NilBroadcaster(t *testing.T) {
	t.Setenv("DAEMON_SECRET", "test-secret")

	handler := handlers.NewIngestHandler(nil)

	event := makeEvent("sync:ratings")
	req, rr := ingestRequest(t, "test-secret", event)
	handler.IngestEvent(rr, req)

	// nil broadcaster must not panic; event is accepted.
	if rr.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", rr.Code)
	}
}
