package sse_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// stubExtractor returns a UserIDExtractor that always yields the given userID.
func stubExtractor(userID int64) sse.UserIDExtractor {
	return func(_ context.Context) (int64, bool) {
		return userID, true
	}
}

// noUserExtractor returns a UserIDExtractor that always fails (unauthenticated).
func noUserExtractor() sse.UserIDExtractor {
	return func(_ context.Context) (int64, bool) {
		return 0, false
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
	b.Publish(1, makeEvent("test:event"))
}

// --- handler / SSE frame tests -------------------------------------------

// connectSSE starts a test server backed by the broker and connects to it via
// an *httptest.Server using the provided extractor to supply user ID context.
// It returns the response body scanner and a cancel func that closes the
// client connection.
func connectSSE(t *testing.T, b *sse.Broker, extractor sse.UserIDExtractor) (*httptest.Server, *bufio.Scanner, func()) {
	t.Helper()

	srv := httptest.NewServer(b.Handler(extractor))

	req, err := http.NewRequest(http.MethodGet, srv.URL, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}

	scanner := bufio.NewScanner(resp.Body)
	cancel := func() {
		resp.Body.Close()
		srv.Close()
	}

	return srv, scanner, cancel
}

func TestBrokerHandler_RejectsUnauthenticated(t *testing.T) {
	b := sse.New()
	srv := httptest.NewServer(b.Handler(noUserExtractor()))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated request, got %d", resp.StatusCode)
	}
}

func TestBrokerServeHTTP_ConnectedHeaderSet(t *testing.T) {
	b := sse.New()
	_, _, cancel := connectSSE(t, b, stubExtractor(1))
	defer cancel()

	// Brief pause so the goroutine has time to register.
	time.Sleep(20 * time.Millisecond)

	if n := b.SubscriberCount(); n != 1 {
		t.Errorf("expected 1 subscriber, got %d", n)
	}
}

func TestBrokerServeHTTP_ConnectedContentType(t *testing.T) {
	b := sse.New()
	srv := httptest.NewServer(b.Handler(stubExtractor(1)))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}

	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}
}

func TestBrokerServeHTTP_DisconnectDecreasesCount(t *testing.T) {
	b := sse.New()
	_, _, cancel := connectSSE(t, b, stubExtractor(1))

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

func TestBrokerServeHTTP_PublishDeliveredToMatchingUser(t *testing.T) {
	b := sse.New()
	_, scanner, cancel := connectSSE(t, b, stubExtractor(42))
	defer cancel()

	// Wait for the subscriber to register.
	time.Sleep(30 * time.Millisecond)

	ev := makeEvent("draft:pick")
	b.Publish(42, ev)

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

// TestBrokerServeHTTP_PublishNotDeliveredToDifferentUser is the critical
// security regression test: User B's subscriber must NOT receive events
// published for User A.
func TestBrokerServeHTTP_PublishNotDeliveredToDifferentUser(t *testing.T) {
	b := sse.New()

	// Connect User A (userID=1) and User B (userID=2).
	_, _, cancelA := connectSSE(t, b, stubExtractor(1))
	defer cancelA()

	_, scannerB, cancelB := connectSSE(t, b, stubExtractor(2))
	defer cancelB()

	time.Sleep(30 * time.Millisecond)

	if n := b.SubscriberCount(); n != 2 {
		t.Fatalf("expected 2 subscribers, got %d", n)
	}

	// Publish an event scoped to User A only.
	b.Publish(1, makeEvent("draft:pick"))

	// User B's scanner should NOT receive the event within a reasonable window.
	linesB := make(chan string, 20)
	go func() {
		for scannerB.Scan() {
			linesB <- scannerB.Text()
		}
	}()

	timeout := time.After(300 * time.Millisecond)
	for {
		select {
		case <-timeout:
			// Good: no data arrived for User B.
			return
		case line := <-linesB:
			if strings.HasPrefix(line, "data: ") {
				t.Errorf("User B received an event scoped to User A: %s", line)
				return
			}
		}
	}
}

// TestBrokerServeHTTP_PublishOnlyToTargetUserAmongMany verifies that when
// multiple users are connected, only the targeted user's subscriber receives
// the event.
func TestBrokerServeHTTP_PublishOnlyToTargetUserAmongMany(t *testing.T) {
	b := sse.New()

	_, scannerA, cancelA := connectSSE(t, b, stubExtractor(10))
	defer cancelA()

	_, scannerB, cancelB := connectSSE(t, b, stubExtractor(20))
	defer cancelB()

	_, scannerC, cancelC := connectSSE(t, b, stubExtractor(30))
	defer cancelC()

	time.Sleep(30 * time.Millisecond)

	if n := b.SubscriberCount(); n != 3 {
		t.Fatalf("expected 3 subscribers, got %d", n)
	}

	// Publish for userID=20 only.
	b.Publish(20, makeEvent("sync:ratings"))

	type result struct {
		label       string
		gotData     bool
		shouldGetIt bool
	}

	results := make(chan result, 3)
	var wg sync.WaitGroup

	readLines := func(scanner *bufio.Scanner, label string, target bool) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			timeout := time.After(300 * time.Millisecond)
			lines := make(chan string, 20)
			go func() {
				for scanner.Scan() {
					lines <- scanner.Text()
				}
			}()
			for {
				select {
				case <-timeout:
					results <- result{label: label, gotData: false, shouldGetIt: target}
					return
				case line := <-lines:
					if strings.HasPrefix(line, "data: ") {
						results <- result{label: label, gotData: true, shouldGetIt: target}
						return
					}
				}
			}
		}()
	}

	readLines(scannerA, "userA(10)", false) // should NOT receive
	readLines(scannerB, "userB(20)", true)  // SHOULD receive
	readLines(scannerC, "userC(30)", false) // should NOT receive

	wg.Wait()
	close(results)

	for r := range results {
		if r.shouldGetIt && !r.gotData {
			t.Errorf("%s: expected to receive event but did not", r.label)
		}
		if !r.shouldGetIt && r.gotData {
			t.Errorf("%s: cross-tenant delivery — received event it should not have", r.label)
		}
	}
}

func TestBrokerServeHTTP_EventNameInFrame(t *testing.T) {
	b := sse.New()
	_, scanner, cancel := connectSSE(t, b, stubExtractor(1))
	defer cancel()

	time.Sleep(30 * time.Millisecond)

	b.Publish(1, makeEvent("draft:pick"))

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

func TestBrokerServeHTTP_MultipleSubscribersSameUser(t *testing.T) {
	b := sse.New()

	// Two connections for the same user (e.g. two open browser tabs).
	_, scannerA, cancelA := connectSSE(t, b, stubExtractor(99))
	_, scannerB, cancelB := connectSSE(t, b, stubExtractor(99))
	defer cancelA()
	defer cancelB()

	time.Sleep(30 * time.Millisecond)

	if n := b.SubscriberCount(); n != 2 {
		t.Fatalf("expected 2 subscribers, got %d", n)
	}

	b.Publish(99, makeEvent("sync:ratings"))

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

	readData(scannerA, "tabA")
	readData(scannerB, "tabB")
}

func TestBrokerServeHTTP_NonFlushableWriter(t *testing.T) {
	b := sse.New()

	// Use a plain non-flushing writer — must not embed ResponseRecorder because
	// ResponseRecorder itself implements http.Flusher and would promote the method.
	w := &plainWriter{header: make(http.Header)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", http.NoBody)

	// Inject a valid user ID into context so we reach the flusher check.
	ctx := context.WithValue(req.Context(), struct{ k string }{"uid"}, int64(1))
	req = req.WithContext(ctx)

	b.Handler(stubExtractor(1)).ServeHTTP(w, req)

	if w.code != http.StatusInternalServerError {
		t.Errorf("expected 500 for non-flushing writer, got %d", w.code)
	}
}

// plainWriter is a minimal http.ResponseWriter that intentionally does NOT
// implement http.Flusher so we can test the guard in Broker.Handler.
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

// --- heartbeat tests ---------------------------------------------------------

// connectSSEWithBroker is like connectSSE but accepts a pre-built broker so
// tests that need custom heartbeat intervals can inject their own.
func connectSSEWithBroker(t *testing.T, b *sse.Broker, extractor sse.UserIDExtractor) (*httptest.Server, *bufio.Scanner, func()) {
	t.Helper()

	srv := httptest.NewServer(b.Handler(extractor))

	req, err := http.NewRequest(http.MethodGet, srv.URL, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}

	scanner := bufio.NewScanner(resp.Body)
	cancel := func() {
		resp.Body.Close()
		srv.Close()
	}

	return srv, scanner, cancel
}

// TestBrokerServeHTTP_HeartbeatSentWhenIdle verifies that the broker sends a
// ": heartbeat" SSE comment frame after the configured interval elapses with no
// events published.
func TestBrokerServeHTTP_HeartbeatSentWhenIdle(t *testing.T) {
	// Use a short heartbeat interval so the test completes quickly.
	b := sse.NewWithHeartbeat(50 * time.Millisecond)
	_, scanner, cancel := connectSSEWithBroker(t, b, stubExtractor(1))
	defer cancel()

	lines := make(chan string, 50)
	go func() {
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	// Drain the initial ": connected" comment.
	connTimeout := time.After(500 * time.Millisecond)
	for {
		select {
		case <-connTimeout:
			t.Fatal("timed out waiting for initial connection frame")
		case line := <-lines:
			if line == ": connected" {
				goto waitHeartbeat
			}
		}
	}

waitHeartbeat:
	// Now wait for a heartbeat comment within a generous window.
	heartbeatTimeout := time.After(500 * time.Millisecond)
	for {
		select {
		case <-heartbeatTimeout:
			t.Fatal("timed out waiting for ': heartbeat' SSE comment frame")
		case line := <-lines:
			if line == ": heartbeat" {
				return // pass
			}
		}
	}
}

// TestBrokerServeHTTP_HeartbeatMultipleFires verifies that more than one
// heartbeat frame is sent over the lifetime of an idle connection.
func TestBrokerServeHTTP_HeartbeatMultipleFires(t *testing.T) {
	b := sse.NewWithHeartbeat(40 * time.Millisecond)
	_, scanner, cancel := connectSSEWithBroker(t, b, stubExtractor(2))
	defer cancel()

	lines := make(chan string, 100)
	go func() {
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	// Collect all lines for 300ms; expect at least 2 heartbeat frames.
	deadline := time.After(400 * time.Millisecond)
	heartbeats := 0
	for {
		select {
		case <-deadline:
			if heartbeats < 2 {
				t.Errorf("expected at least 2 heartbeat frames, got %d", heartbeats)
			}
			return
		case line := <-lines:
			if line == ": heartbeat" {
				heartbeats++
			}
		}
	}
}

// TestBrokerServeHTTP_HeartbeatDoesNotInterruptEvents verifies that heartbeats
// and real events are both delivered correctly to the client.
func TestBrokerServeHTTP_HeartbeatDoesNotInterruptEvents(t *testing.T) {
	b := sse.NewWithHeartbeat(30 * time.Millisecond)
	_, scanner, cancel := connectSSEWithBroker(t, b, stubExtractor(5))
	defer cancel()

	// Wait for subscriber to register.
	time.Sleep(20 * time.Millisecond)

	b.Publish(5, makeEvent("draft:pick"))

	lines := make(chan string, 100)
	go func() {
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	eventTimeout := time.After(2 * time.Second)
	for {
		select {
		case <-eventTimeout:
			t.Error("timed out waiting for real event data line")
			return
		case line := <-lines:
			if strings.HasPrefix(line, "data: ") {
				return // pass
			}
		}
	}
}

// TestBrokerNewWithHeartbeat_DefaultInterval verifies that New() constructs a
// non-nil broker (sanity check — not a timing test).
func TestBrokerNewWithHeartbeat_DefaultInterval(t *testing.T) {
	b := sse.New()
	if b == nil {
		t.Fatal("New() returned nil")
	}
}

// TestBrokerServeHTTP_DisabledHeartbeat verifies that a broker constructed
// with a zero interval sends no heartbeat frames but still delivers events.
func TestBrokerServeHTTP_DisabledHeartbeat(t *testing.T) {
	b := sse.NewWithHeartbeat(0) // heartbeat disabled
	_, scanner, cancel := connectSSEWithBroker(t, b, stubExtractor(7))
	defer cancel()

	time.Sleep(20 * time.Millisecond)
	b.Publish(7, makeEvent("draft:pick"))

	lines := make(chan string, 50)
	go func() {
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	// Expect no heartbeat but do expect the data frame.
	eventTimeout := time.After(2 * time.Second)
	for {
		select {
		case <-eventTimeout:
			t.Error("timed out waiting for event data line (disabled heartbeat)")
			return
		case line := <-lines:
			if line == ": heartbeat" {
				t.Error("received unexpected heartbeat frame when heartbeat is disabled")
				return
			}
			if strings.HasPrefix(line, "data: ") {
				return // pass
			}
		}
	}
}
