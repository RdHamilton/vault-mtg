package websocket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	contract "github.com/RdHamilton/MTGA-Companion/services/contract"
)

func makeDaemonEvent(eventType string) contract.DaemonEvent {
	payload, _ := json.Marshal(map[string]interface{}{"key": "value"})

	return contract.DaemonEvent{
		Type:       eventType,
		AccountID:  "acct_test",
		SessionID:  "sess_test",
		OccurredAt: time.Now().UTC(),
		Payload:    payload,
	}
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

	event := makeDaemonEvent("quest:updated")

	// Should not panic when no clients are connected.
	forwarder.ForwardEvent(event)

	time.Sleep(10 * time.Millisecond)
}

func TestDaemonEventForwarder_ForwardEvent_AllDaemonEventTypes(t *testing.T) {
	eventTypes := []string{
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

	for _, eventType := range eventTypes {
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
			forwarder.ForwardEvent(makeDaemonEvent(eventType))

			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
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

			if received.Data == nil {
				t.Error("Expected Data to not be nil")
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

	for i := range 3 {
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
	forwarder.ForwardEvent(makeDaemonEvent("quest:updated"))

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

	// DaemonEvent with empty Type must be dropped without panic.
	event := contract.DaemonEvent{
		AccountID:  "acct_test",
		OccurredAt: time.Now().UTC(),
	}

	forwarder.ForwardEvent(event)
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

	event := makeDaemonEvent("stats:updated")
	forwarder.ForwardEvent(&event)

	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
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

func TestDaemonEventForwarder_ForwardEvent_NilPointer(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	forwarder := NewDaemonEventForwarder(hub)

	// nil *contract.DaemonEvent must not panic.
	var nilEvent *contract.DaemonEvent
	forwarder.ForwardEvent(nilEvent)
}

func TestDaemonEventForwarder_ForwardEvent_JSONFallback(t *testing.T) {
	// Passing an unrecognised type falls back to JSON round-trip.
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

	// Anonymous struct whose fields match DaemonEvent JSON tags.
	legacyEvent := struct {
		Type      string `json:"type"`
		AccountID string `json:"account_id"`
		SessionID string `json:"session_id"`
	}{
		Type:      "legacy:event",
		AccountID: "acct_legacy",
		SessionID: "sess_legacy",
	}
	forwarder.ForwardEvent(legacyEvent)

	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	var received Event
	if err := json.Unmarshal(message, &received); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if received.Type != "legacy:event" {
		t.Errorf("Expected type 'legacy:event', got %q", received.Type)
	}
}

func TestDaemonEventForwarder_ForwardEvent_NonMarshalableType(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	forwarder := NewDaemonEventForwarder(hub)

	// Channel cannot be marshaled; forwarder must log and return without panic.
	forwarder.ForwardEvent(make(chan int))
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

	eventTypes := []string{"stats:updated", "quest:updated", "deck:updated"}

	for _, et := range eventTypes {
		forwarder.ForwardEvent(makeDaemonEvent(et))
		time.Sleep(10 * time.Millisecond)
	}

	for i, et := range eventTypes {
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read message %d: %v", i, err)
		}

		var received Event
		if err := json.Unmarshal(message, &received); err != nil {
			t.Fatalf("Failed to unmarshal message %d: %v", i, err)
		}

		if received.Type != et {
			t.Errorf("Event %d: expected type %q, got %q", i, et, received.Type)
		}
	}
}
