package websocket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	contract "github.com/ramonehamilton/mtga-contract"
)

// mockEvent simulates a legacy daemon.Event struct (struct with Type+Data fields)
// to verify the JSON-fallback path in ForwardEvent.
type mockEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func TestNewDaemonEventForwarder(t *testing.T) {
	hub := NewHub()
	forwarder := NewDaemonEventForwarder(hub)

	if forwarder == nil {
		t.Fatal("NewDaemonEventForwarder() returned nil")
	}

	if forwarder.hub != hub {
		t.Error("Forwarder hub reference is incorrect")
	}
}

func TestDaemonEventForwarder_ForwardEvent_NoClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	forwarder := NewDaemonEventForwarder(hub)

	event := makeDaemonEvent("quest:updated", nil)

	// Should not panic when no clients are connected.
	forwarder.ForwardEvent(event)

	// Give time for the goroutine to process.
	time.Sleep(10 * time.Millisecond)
}

func TestDaemonEventForwarder_ForwardEvent_AllDaemonEventTypes(t *testing.T) {
	daemonEventTypes := []string{
		"daemon:status",
		"daemon:error",
		"stats:updated",
		"deck:updated",
		"rank:updated",
		"quest:updated",
		"draft:updated",
		"replay:started",
		"replay:error",
		"replay:completed",
		"replay:progress",
		"replay:paused",
		"replay:resumed",
		"replay:draft_detected",
	}

	for _, eventType := range daemonEventTypes {
		t.Run(eventType, func(t *testing.T) {
			hub := NewHub()
			go hub.Run()

			server := httptest.NewServer(http.HandlerFunc(hub.ServeWs))
			defer server.Close()

			wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

			dialer := websocket.Dialer{}
			conn, _, err := dialer.Dial(wsURL, nil)
			if err != nil {
				t.Fatalf("Failed to connect: %v", err)
			}
			defer conn.Close()

			time.Sleep(50 * time.Millisecond)

			forwarder := NewDaemonEventForwarder(hub)
			forwarder.ForwardEvent(makeDaemonEvent(eventType, nil))

			conn.SetReadDeadline(time.Now().Add(time.Second))
			_, message, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("Failed to read message: %v", err)
			}

			var received Event
			if err := json.Unmarshal(message, &received); err != nil {
				t.Fatalf("Failed to unmarshal message: %v", err)
			}

			if received.Type != eventType {
				t.Errorf("Expected type %q, got %q", eventType, received.Type)
			}
		})
	}
}

func TestDaemonEventForwarder_ForwardEvent_MultipleClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(hub.ServeWs))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	dialer := websocket.Dialer{}
	var conns []*websocket.Conn

	for i := 0; i < 3; i++ {
		conn, _, err := dialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect client %d: %v", i, err)
		}
		conns = append(conns, conn)
	}

	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()

	time.Sleep(50 * time.Millisecond)

	if count := hub.ClientCount(); count != 3 {
		t.Errorf("Expected 3 clients, got %d", count)
	}

	forwarder := NewDaemonEventForwarder(hub)
	forwarder.ForwardEvent(makeDaemonEvent("quest:updated", nil))

	for i, conn := range conns {
		conn.SetReadDeadline(time.Now().Add(time.Second))
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("Client %d failed to read message: %v", i, err)
			continue
		}

		var received Event
		if err := json.Unmarshal(message, &received); err != nil {
			t.Errorf("Client %d failed to unmarshal message: %v", i, err)
			continue
		}

		if received.Type != "quest:updated" {
			t.Errorf("Client %d expected type 'quest:updated', got %q", i, received.Type)
		}
	}
}

func TestDaemonEventForwarder_ForwardEvent_EmptyType(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	forwarder := NewDaemonEventForwarder(hub)

	// Empty type — should log warning and not panic.
	forwarder.ForwardEvent(contract.DaemonEvent{Type: ""})
}

func TestDaemonEventForwarder_ForwardEvent_PointerEvent(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(hub.ServeWs))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	forwarder := NewDaemonEventForwarder(hub)

	ev := makeDaemonEvent("stats:updated", nil)
	forwarder.ForwardEvent(&ev) // Pass pointer — *contract.DaemonEvent path.

	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	var received Event
	if err := json.Unmarshal(message, &received); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if received.Type != "stats:updated" {
		t.Errorf("Expected type 'stats:updated', got %q", received.Type)
	}
}

func TestDaemonEventForwarder_ForwardEvent_NonStructType(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	forwarder := NewDaemonEventForwarder(hub)

	// Non-struct types should log a warning but not panic.
	forwarder.ForwardEvent("not a struct")
	forwarder.ForwardEvent(123)
	forwarder.ForwardEvent(nil)
}

func TestDaemonEventForwarder_ForwardEvent_LegacyFallback(t *testing.T) {
	// Verify the JSON-fallback path works for legacy callers that still pass
	// the old daemon.Event struct (a struct with Type and Data fields that do
	// NOT implement contract.DaemonEvent directly).
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(hub.ServeWs))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	forwarder := NewDaemonEventForwarder(hub)

	// mockEvent maps to contract.DaemonEvent.type via JSON.
	forwarder.ForwardEvent(mockEvent{Type: "quest:updated", Data: map[string]interface{}{"key": "val"}})

	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	var received Event
	if err := json.Unmarshal(message, &received); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if received.Type != "quest:updated" {
		t.Errorf("Expected type 'quest:updated', got %q", received.Type)
	}
}

func TestDaemonEventForwarder_ForwardContractDaemonEvent(t *testing.T) {
	// Verify that passing a contract.DaemonEvent value directly (no reflection,
	// no JSON fallback) works end-to-end.
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(hub.ServeWs))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	payload, _ := json.Marshal(contract.SyncRatingsPayload{SetCode: "BLB", CardsUpdated: 5, Source: "17lands"})
	ev := contract.DaemonEvent{
		Type:       "sync:ratings",
		AccountID:  "acct_test",
		SessionID:  "sess_test",
		OccurredAt: time.Now().UTC(),
		Payload:    payload,
	}

	forwarder := NewDaemonEventForwarder(hub)
	forwarder.ForwardEvent(ev) // contract.DaemonEvent value — no reflection used.

	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	var received Event
	if err := json.Unmarshal(message, &received); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if received.Type != "sync:ratings" {
		t.Errorf("Expected type 'sync:ratings', got %q", received.Type)
	}
}

func TestDaemonEventForwarder_ForwardEvent_SequentialEvents(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(hub.ServeWs))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	forwarder := NewDaemonEventForwarder(hub)

	types := []string{"stats:updated", "quest:updated", "deck:updated"}
	for _, typ := range types {
		forwarder.ForwardEvent(makeDaemonEvent(typ, nil))
		time.Sleep(10 * time.Millisecond)
	}

	for i, typ := range types {
		conn.SetReadDeadline(time.Now().Add(time.Second))
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read message %d: %v", i, err)
		}

		var received Event
		if err := json.Unmarshal(message, &received); err != nil {
			t.Fatalf("Failed to unmarshal message %d: %v", i, err)
		}

		if received.Type != typ {
			t.Errorf("Event %d: expected type %q, got %q", i, typ, received.Type)
		}
	}
}

// TestDaemonEventForwarder_IntegrationWithRealDaemonEventTypes validates the
// full flow from a contract.DaemonEvent to a WebSocket client.
func TestDaemonEventForwarder_IntegrationWithRealDaemonEventTypes(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(hub.ServeWs))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	forwarder := NewDaemonEventForwarder(hub)

	questPayload, _ := json.Marshal(map[string]interface{}{
		"active_quests": []map[string]interface{}{
			{"id": "quest1", "progress": 4, "goal": 30},
		},
		"daily_wins":  6,
		"weekly_wins": 15,
	})
	ev := contract.DaemonEvent{
		Type:       "quest:updated",
		AccountID:  "acct_q",
		SessionID:  "sess_q",
		OccurredAt: time.Now().UTC(),
		Payload:    questPayload,
	}
	forwarder.ForwardEvent(ev)

	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read quest:updated message: %v", err)
	}

	var received Event
	if err := json.Unmarshal(message, &received); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if received.Type != "quest:updated" {
		t.Errorf("Expected type 'quest:updated', got %q", received.Type)
	}
}

// makeDaemonEvent builds a contract.DaemonEvent for tests.
func makeDaemonEvent(eventType string, rawPayload []byte) contract.DaemonEvent {
	return contract.DaemonEvent{
		Type:       eventType,
		AccountID:  "acct_test",
		SessionID:  "sess_test",
		OccurredAt: time.Now().UTC(),
		Payload:    rawPayload,
	}
}
