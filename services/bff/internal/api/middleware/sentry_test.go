package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getsentry/sentry-go"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
)

// newTestHub returns a Sentry hub backed by an in-memory transport so that
// tests capture events without making any real network calls.
func newTestHub(t *testing.T) (*sentry.Hub, *sentry.MockTransport) {
	t.Helper()

	transport := &sentry.MockTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{
		// A valid-looking but non-functional DSN — the SDK validates the format
		// but MockTransport prevents any actual HTTP delivery.
		Dsn:       "https://key@o0.ingest.sentry.io/0",
		Transport: transport,
	})
	if err != nil {
		t.Fatalf("sentry.NewClient: %v", err)
	}

	hub := sentry.NewHub(client, sentry.NewScope())

	return hub, transport
}

// TestSentryMiddleware_CapturesPanicWithoutCrashingServer verifies that:
//   - A handler panic is captured by Sentry (event appears in the transport).
//   - The middleware re-panics (Repanic=true) — a recovery wrapper simulates
//     chi's Recoverer writing a 500 — and the server continues serving
//     subsequent requests without crashing.
func TestSentryMiddleware_CapturesPanicWithoutCrashingServer(t *testing.T) {
	hub, transport := newTestHub(t)

	// Build a minimal handler chain:
	//   recoverWrapper → SentryMiddleware → panicHandler
	//
	// recoverWrapper simulates chi's Recoverer: it catches the re-panic from
	// the Sentry middleware and writes a 500, mirroring production behaviour.
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		panic("test panic")
	})

	sentryMiddl := bffmiddleware.NewSentryMiddleware()

	recoverWrapper := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()
		sentryMiddl(panicHandler).ServeHTTP(w, r)
	})

	// Inject the test hub into the request context so the sentry-go HTTP
	// handler uses our in-memory transport instead of the global client.
	req := httptest.NewRequest(http.MethodGet, "/test-panic", nil)
	req = req.WithContext(sentry.SetHubOnContext(req.Context(), hub))
	rr := httptest.NewRecorder()

	recoverWrapper.ServeHTTP(rr, req)

	// The server must have survived and returned a 500.
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from recovered panic, got %d", rr.Code)
	}

	// Flush ensures any buffered events are delivered to the in-memory transport.
	hub.Flush(0)

	// The panic must have been captured as a Sentry event.
	events := transport.Events()
	if len(events) == 0 {
		t.Fatal("expected at least one Sentry event to be captured, got none")
	}
}

// TestSentryMiddleware_NoopWhenSentryUninitialised verifies that the middleware
// is transparent when the hub carries no client — simulating SENTRY_DSN unset.
// All SDK calls on a nil client are safe no-ops per the sentry-go contract.
func TestSentryMiddleware_NoopWhenSentryUninitialised(t *testing.T) {
	// A hub with no client simulates a process started without SENTRY_DSN.
	emptyHub := sentry.NewHub(nil, sentry.NewScope())

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	sentryMiddl := bffmiddleware.NewSentryMiddleware()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req = req.WithContext(sentry.SetHubOnContext(req.Context(), emptyHub))
	rr := httptest.NewRecorder()

	sentryMiddl(okHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("no-op middleware: expected 200, got %d", rr.Code)
	}
}

// TestSentryMiddleware_AttachesUserIDFromContext verifies that when an
// authenticated user ID is present in the request context the Sentry scope
// carries that ID so events are searchable per user without PII (no email, no
// name — only the opaque int64 DB user ID as a string).
func TestSentryMiddleware_AttachesUserIDFromContext(t *testing.T) {
	hub, transport := newTestHub(t)

	// A handler that deliberately captures a message so we can inspect the
	// Sentry scope user after the middleware has run.
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h := sentry.GetHubFromContext(r.Context()); h != nil {
			h.CaptureMessage("user scope test")
		}
		w.WriteHeader(http.StatusOK)
	})

	sentryMiddl := bffmiddleware.NewSentryMiddleware()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	// Inject both the test hub and an authenticated user ID (42).
	ctx := sentry.SetHubOnContext(req.Context(), hub)
	ctx = bffmiddleware.WithUserID(ctx, 42)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	sentryMiddl(captureHandler).ServeHTTP(rr, req)

	hub.Flush(0)

	events := transport.Events()
	if len(events) == 0 {
		t.Fatal("expected a captured Sentry event, got none")
	}

	// Verify the user ID was attached to the Sentry scope (not email, not name).
	ev := events[0]
	if ev.User.ID != "42" {
		t.Errorf("expected Sentry user ID '42', got %q", ev.User.ID)
	}
}
