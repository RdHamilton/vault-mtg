// Package sentryhook initialises the Sentry SDK for the daemon and provides a
// BeforeSend hook that strips secrets from event metadata before transmission.
//
// The DSN is injected at build time via -ldflags -X main.DefaultSentryDSN=<dsn>;
// when empty (local `go build`, `go run`, or any non-release build) Sentry is
// disabled and all SDK calls are no-ops. The DSN is never logged.
//
// PII safety mirrors the BFF pattern (services/bff/cmd/main.go and
// services/bff/internal/api/middleware/sentry.go):
//   - SendDefaultPII is left at false (the SDK default).
//   - ServerName is suppressed so the host machine name never reaches Sentry.
//   - BeforeSend strips bearer tokens, Clerk publishable keys, and explicit
//     secret patterns from event Message, Exception values, and Extra map.
//
// Environment is derived from the configured cloud_api_url:
//   - api.vaultmtg.app          → "production"
//   - staging-api.vaultmtg.app  → "staging"
//   - anything else (localhost) → "development"
package sentryhook

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
)

// FlushTimeout caps how long sentry.Flush waits during graceful shutdown.
// Matches the BFF (services/bff/cmd/main.go).
const FlushTimeout = 2 * time.Second

// ErrDisabled is the sentinel returned by Init when DSN is empty. Callers may
// branch on it to log "Sentry disabled" without treating it as an error.
var ErrDisabled = errors.New("sentry disabled: empty DSN")

// Init configures the global Sentry hub. When dsn is empty, Init returns
// ErrDisabled and the SDK is left uninitialised — all subsequent sentry.* calls
// become safe no-ops per the sentry-go contract.
//
// release is the daemon Version string and is sent as the Sentry release tag
// so events correlate to a deployed daemon build.
//
// cloudAPIURL is used to derive the Sentry environment ("production",
// "staging", "development").
func Init(dsn, release, cloudAPIURL string) error {
	if dsn == "" {
		return ErrDisabled
	}
	env := environmentFromCloudAPIURL(cloudAPIURL)
	return sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Release:          release,
		Environment:      env,
		ServerName:       "redacted", // suppress hostname leakage
		AttachStacktrace: true,
		// SendDefaultPII defaults to false; we leave it explicit for clarity.
		SendDefaultPII: false,
		// Cap transmission to 100 events / hour per release via Sentry-side
		// rate-limit rules (set in the Sentry UI). The Go SDK does not expose a
		// client-side hourly limiter; rely on the server-side rate limiter as
		// noted in #1832's Risks section.
		BeforeSend: scrubEvent,
	})
}

// SetUser attaches a hashed Clerk user_id to the current hub scope so events
// emitted after PKCE auth completes are searchable per user without storing
// raw PII. Mirrors hashAccountID in services/bff/internal/api/handlers/posthog.go.
//
// Safe to call before Sentry is initialised — the SDK contract guarantees
// configure-scope on a nil client is a no-op.
func SetUser(clerkUserID string) {
	if clerkUserID == "" {
		return
	}
	sum := sha256.Sum256([]byte(clerkUserID))
	hashed := fmt.Sprintf("%x", sum)[:16]
	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetUser(sentry.User{ID: hashed})
	})
}

// Flush blocks until queued events are sent or FlushTimeout elapses. Call on
// graceful shutdown.
func Flush() {
	sentry.Flush(FlushTimeout)
}

// environmentFromCloudAPIURL returns the Sentry environment string given the
// daemon's configured cloud_api_url. The match is substring-based so the
// resolution survives small URL changes (path suffix, port).
func environmentFromCloudAPIURL(url string) string {
	switch {
	case strings.Contains(url, "staging-api"):
		return "staging"
	case strings.Contains(url, "api.vaultmtg.app"):
		return "production"
	default:
		return "development"
	}
}

// secretPatterns is the central list of regexes used to redact secret strings
// from any event field. Add new patterns here so all sites (Message, Extra,
// Exception values) share one source of truth.
var secretPatterns = []*regexp.Regexp{
	// "Bearer <opaque-token>" — common in HTTP Authorization headers and log lines.
	regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._\-+/=]+`),
	// Clerk publishable keys (pk_live_*, pk_test_*) and secret keys (sk_*).
	regexp.MustCompile(`pk_(live|test)_[A-Za-z0-9_\-]+`),
	regexp.MustCompile(`sk_(live|test)_[A-Za-z0-9_\-]+`),
	// Sentry DSNs themselves (https://<key>@<org>.ingest.sentry.io/<id>).
	regexp.MustCompile(`https?://[a-f0-9]+@[a-z0-9.\-]*sentry\.io/\d+`),
	// Generic "api_key=<value>" or "api-key: <value>" patterns.
	regexp.MustCompile(`(?i)api[_\-]?key["':=\s]+["']?[A-Za-z0-9_\-]{8,}["']?`),
	// JWT-shaped strings (three dot-separated base64url segments).
	regexp.MustCompile(`eyJ[A-Za-z0-9_\-]+\.eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+`),
}

// scrub returns s with every match of every secretPatterns regex replaced by
// "[REDACTED]". Exported so other packages (localapi diagnostics) can reuse
// the same scrubber without duplicating patterns.
func Scrub(s string) string {
	for _, p := range secretPatterns {
		s = p.ReplaceAllString(s, "[REDACTED]")
	}
	return s
}

// scrubEvent is the Sentry BeforeSend hook. It walks the structured fields
// most likely to contain secrets and replaces matches in place. The Sentry
// SDK calls this synchronously on the goroutine emitting the event, so the
// work must be bounded — a fixed set of regexes is fine; do NOT add an
// unbounded JSON walk here.
func scrubEvent(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	if event == nil {
		return nil
	}
	event.Message = Scrub(event.Message)
	for i := range event.Exception {
		event.Exception[i].Value = Scrub(event.Exception[i].Value)
	}
	for i := range event.Breadcrumbs {
		event.Breadcrumbs[i].Message = Scrub(event.Breadcrumbs[i].Message)
	}
	// Authorization headers in Request.Headers — only present for HTTP-style
	// events; daemon panics rarely produce these but the BFF middleware test
	// pattern strips them defensively.
	if event.Request != nil && event.Request.Headers != nil {
		for k := range event.Request.Headers {
			if strings.EqualFold(k, "Authorization") || strings.EqualFold(k, "Cookie") {
				event.Request.Headers[k] = "[REDACTED]"
			} else {
				event.Request.Headers[k] = Scrub(event.Request.Headers[k])
			}
		}
	}
	// Contexts is the v0.46+ replacement for the older Extra map. Each Context
	// is a map[string]interface{}; walk string values and scrub them in place.
	for _, ctx := range event.Contexts {
		for k, v := range ctx {
			if s, ok := v.(string); ok {
				ctx[k] = Scrub(s)
			}
		}
	}
	for k, v := range event.Tags {
		event.Tags[k] = Scrub(v)
	}
	return event
}
