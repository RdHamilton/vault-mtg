package sse_test

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	contract "github.com/RdHamilton/MTGA-Companion/services/contract"
	"github.com/ramonehamilton/mtga-bff/internal/api/sse"
)

// makeEvent returns a minimal DaemonEvent for use in tests.
func makeEvent(eventType string) contract.DaemonEvent {
	payload, _ := json.Marshal(map[string]string{"key": "value"})

	return contract.DaemonEvent{
		Type:      eventType,
		AccountID: "acct_test",
		SessionID: "sess_test",
		Payload:   payload,
	}
}

// --- unit tests ----------------------------------------------------------

func TestBroker_SubscriberCount_InitiallyZero(t *testing.T) {
	b := sse.New()
	if n := b.SubscriberCount(); n != 0 {
		t.Errorf("expected 0 subscribers, got %d", n)
	}
}

func TestBroker_Publish_NoSubscribersNoPanic(t *testing.T) {
	b := sse.New()

	// Must not panic or deadlock when there are no subscribers.
	b.Publish(makeEvent("test:event"))
}

// --- handler / SSE frame tests -------------------------------------------

// connectSSE starts a test server backed by the broker and connects to it via
// an *httptest.Server.  It returns the response body scanner and a cancel func
// that closes the client connection.
func connectSSE(t *testing.T, b *sse.Broker, userID string) (*httptest.Server, *bufio.Scanner, func()) {
	t.Helper()

	srv := httptest.NewServer(b)

	req, err := http.NewRequest(http.MethodGet, srv.URL, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	req.Header.Set("X-User-ID", userID)

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}

	scanner := bufio.NewScanner(resp.Body)
	cancel := func() {
		resp.Body.Close()
		srv.Close()
	}

	return srv, scanner, cancel
}

func TestBrokerServeHTTP_ConnectedHeaderSet(t *testing.T) {
	b := sse.New()
	_, _, cancel := connectSSE(t, b, "user1")
	defer cancel()

	// Brief pause so the goroutine has time to register.
	time.Sleep(20 * time.Millisecond)

	if n := b.SubscriberCount(); n != 1 {
		t.Errorf("expected 1 subscriber, got %d", n)
	}
}

func TestBrokerServeHTTP_DisconnectDecreasesCount(t *testing.T) {
	b := sse.New()
	_, _, cancel := connectSSE(t, b, "user1")

	time.Sleep(20 * time.Millisecond)

	if n := b.SubscriberCount(); n != 1 {
		t.Errorf("expected 1 subscriber before disconnect, got %d", n)
	}

	cancel() // close connection

	// Allow the server goroutine to process the disconnect.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if b.SubscriberCount() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if n := b.SubscriberCount(); n != 0 {
		t.Errorf("expected 0 subscribers after disconnect, got %d", n)
	}
}

func TestBrokerServeHTTP_PublishDeliveredToClient(t *testing.T) {
	b := sse.New()
	_, scanner, cancel := connectSSE(t, b, "user1")
	defer cancel()

	// Wait for the subscriber to register.
	time.Sleep(30 * time.Millisecond)

	ev := makeEvent("draft:pick")
	b.Publish(ev)

	// Read lines until we find the expected SSE data line.
	lines := make(chan string, 20)
	go func() {
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	timeout := time.After(2 * time.Second)
	var dataLine string
	for {
		select {
		case <-timeout:
			t.Fatal("timed out waiting for SSE data line")
		case line := <-lines:
			if strings.HasPrefix(line, "data: ") {
				dataLine = strings.TrimPrefix(line, "data: ")
				goto found
			}
		}
	}

found:
	var received contract.DaemonEvent
	if err := json.Unmarshal([]byte(dataLine), &received); err != nil {
		t.Fatalf("unmarshal SSE data: %v", err)
	}

	if received.Type != "draft:pick" {
		t.Errorf("expected type 'draft:pick', got %q", received.Type)
	}

	if received.AccountID != "acct_test" {
		t.Errorf("expected account_id 'acct_test', got %q", received.AccountID)
	}
}

func TestBrokerServeHTTP_EventNameInFrame(t *testing.T) {
	b := sse.New()
	_, scanner, cancel := connectSSE(t, b, "user1")
	defer cancel()

	time.Sleep(30 * time.Millisecond)

	b.Publish(makeEvent("draft:pick"))

	lines := make(chan string, 20)
	go func() {
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	timeout := time.After(2 * time.Second)
	foundEvent := false
	for {
		select {
		case <-timeout:
			if !foundEvent {
				t.Error("timed out waiting for 'event:' line")
			}
			return
		case line := <-lines:
			if line == "event: draft:pick" {
				foundEvent = true
				return
			}
		}
	}
}

func TestBrokerServeHTTP_MultipleSubscribers(t *testing.T) {
	b := sse.New()
	_, scannerA, cancelA := connectSSE(t, b, "userA")
	_, scannerB, cancelB := connectSSE(t, b, "userB")
	defer cancelA()
	defer cancelB()

	time.Sleep(30 * time.Millisecond)

	if n := b.SubscriberCount(); n != 2 {
		t.Fatalf("expected 2 subscribers, got %d", n)
	}

	b.Publish(makeEvent("sync:ratings"))

	readData := func(scanner *bufio.Scanner, label string) {
		lines := make(chan string, 20)
		go func() {
			for scanner.Scan() {
				lines <- scanner.Text()
			}
		}()

		timeout := time.After(2 * time.Second)
		for {
			select {
			case <-timeout:
				t.Errorf("[%s] timed out waiting for data line", label)
				return
			case line := <-lines:
				if strings.HasPrefix(line, "data: ") {
					return // received
				}
			}
		}
	}

	readData(scannerA, "A")
	readData(scannerB, "B")
}

func TestBrokerServeHTTP_NonFlushableWriter(t *testing.T) {
	b := sse.New()

	// Use a plain non-flushing writer — must not embed ResponseRecorder because
	// ResponseRecorder itself implements http.Flusher and would promote the method.
	w := &plainWriter{header: make(http.Header)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", http.NoBody)

	b.ServeHTTP(w, req)

	if w.code != http.StatusInternalServerError {
		t.Errorf("expected 500 for non-flushing writer, got %d", w.code)
	}
}

// plainWriter is a minimal http.ResponseWriter that intentionally does NOT
// implement http.Flusher so we can test the guard in Broker.ServeHTTP.
type plainWriter struct {
	header http.Header
	code   int
	buf    strings.Builder
}

func (p *plainWriter) Header() http.Header         { return p.header }
func (p *plainWriter) WriteHeader(code int)        { p.code = code }
func (p *plainWriter) Write(b []byte) (int, error) { return p.buf.Write(b) }

// Ensure plainWriter does NOT implement http.Flusher at compile time.
var _ http.ResponseWriter = (*plainWriter)(nil)
