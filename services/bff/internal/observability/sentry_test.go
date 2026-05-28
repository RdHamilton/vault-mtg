package observability_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/observability"
	"github.com/getsentry/sentry-go"
)

// newTestTransport initialises a Sentry client backed by MockTransport and
// installs it as the current hub so observability.ReportError picks it up.
// A Cleanup restores an empty client so subsequent tests are isolated.
func newTestTransport(t *testing.T) *sentry.MockTransport {
	t.Helper()
	transport := &sentry.MockTransport{}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:       "https://key@o0.ingest.sentry.io/0",
		Transport: transport,
	})
	if err != nil {
		t.Fatalf("sentry.Init: %v", err)
	}
	// Reset the rate-limiter so tests are independent of each other.
	observability.ResetRateLimiter()
	t.Cleanup(func() {
		// Replace the global client with a no-op so subsequent test runs
		// don't accidentally ship to the previous transport.
		_ = sentry.Init(sentry.ClientOptions{})
		observability.ResetRateLimiter()
	})
	return transport
}

// flush ensures events buffered by the SDK are delivered to the transport.
func flush() { sentry.Flush(100 * time.Millisecond) }

// TestReportError_NilErrorIsNoop verifies that passing a nil error does not
// send any event to Sentry.
func TestReportError_NilErrorIsNoop(t *testing.T) {
	transport := newTestTransport(t)

	observability.ReportError(context.Background(), nil)
	flush()

	if got := len(transport.Events()); got != 0 {
		t.Errorf("nil error: want 0 events, got %d", got)
	}
}

// TestReportError_CapturesEvent verifies that a non-nil error produces exactly
// one Sentry event.
func TestReportError_CapturesEvent(t *testing.T) {
	transport := newTestTransport(t)

	observability.ReportError(context.Background(), errors.New("boom"))
	flush()

	if got := len(transport.Events()); got != 1 {
		t.Fatalf("want 1 event, got %d", got)
	}
}

// TestReportError_TagsMergeCorrectly verifies that caller-supplied tags are
// attached as Sentry tags on the event.
func TestReportError_TagsMergeCorrectly(t *testing.T) {
	transport := newTestTransport(t)

	observability.ReportError(
		context.Background(),
		errors.New("db error"),
		map[string]string{"component": "db", "table": "accounts"},
	)
	flush()

	events := transport.Events()
	if len(events) == 0 {
		t.Fatal("want 1 event, got 0")
	}
	ev := events[0]
	if ev.Tags["component"] != "db" {
		t.Errorf("tag component: want %q, got %q", "db", ev.Tags["component"])
	}
	if ev.Tags["table"] != "accounts" {
		t.Errorf("tag table: want %q, got %q", "accounts", ev.Tags["table"])
	}
}

// TestReportError_MultipleTagMapsAreMerged verifies that when multiple tag maps
// are supplied they are all merged onto the event.
func TestReportError_MultipleTagMapsAreMerged(t *testing.T) {
	transport := newTestTransport(t)

	observability.ReportError(
		context.Background(),
		errors.New("multi"),
		map[string]string{"component": "outbound"},
		map[string]string{"target": "scryfall"},
	)
	flush()

	events := transport.Events()
	if len(events) == 0 {
		t.Fatal("want 1 event, got 0")
	}
	ev := events[0]
	if ev.Tags["component"] != "outbound" {
		t.Errorf("tag component: want %q, got %q", "outbound", ev.Tags["component"])
	}
	if ev.Tags["target"] != "scryfall" {
		t.Errorf("tag target: want %q, got %q", "scryfall", ev.Tags["target"])
	}
}

// TestReportError_MissingContextFieldsDoNotPanic verifies that a context
// carrying neither a user_id nor a request_id does not cause a panic.
func TestReportError_MissingContextFieldsDoNotPanic(t *testing.T) {
	newTestTransport(t)

	// Should not panic.
	observability.ReportError(context.Background(), errors.New("no ctx fields"))
	flush()
}

// TestReportError_AttachesUserIDFromContext verifies that when a user_id is
// present in the context via observability.WithUserID it is attached to the
// Sentry event's User.ID field.
func TestReportError_AttachesUserIDFromContext(t *testing.T) {
	transport := newTestTransport(t)

	ctx := observability.WithUserID(context.Background(), 99)
	observability.ReportError(ctx, errors.New("user ctx"))
	flush()

	events := transport.Events()
	if len(events) == 0 {
		t.Fatal("want 1 event, got 0")
	}
	if events[0].User.ID != "99" {
		t.Errorf("user ID: want %q, got %q", "99", events[0].User.ID)
	}
}

// TestReportError_RateLimiterSuppressesFlood verifies that the 1-error/sec
// rate limiter discards bursts: the first call goes through, subsequent rapid
// calls in the same second are dropped.
func TestReportError_RateLimiterSuppressesFlood(t *testing.T) {
	transport := newTestTransport(t)

	// Fire several errors in rapid succession.
	for i := 0; i < 5; i++ {
		observability.ReportError(context.Background(), errors.New("flood"))
	}
	flush()

	events := transport.Events()
	if len(events) != 1 {
		t.Errorf("rate limiter: want 1 event, got %d", len(events))
	}
}
