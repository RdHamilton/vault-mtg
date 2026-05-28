// Package observability provides shared Sentry helpers for the BFF service.
package observability

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// ctxKeyObsUserID is the context key used by observability to store the DB
// user ID.  Middleware packages should call WithUserID to populate it so that
// ReportError can attach the user scope without importing the middleware
// package (which would create an import cycle).
type ctxKey string

const ctxKeyObsUserID ctxKey = "obs_user_id"

// WithUserID returns a copy of ctx with the given DB user ID stored for use
// by ReportError.  Call this from middleware after resolving the user ID.
func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, ctxKeyObsUserID, userID)
}

// userIDFromContext extracts the user ID stored by WithUserID.
func userIDFromContext(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(ctxKeyObsUserID).(int64)
	return v, ok && v != 0
}

// rateMu protects the rate-limiter state.
var (
	rateMu    sync.Mutex
	lastError time.Time
)

// minInterval is the minimum time between Sentry CaptureException calls.
// Keeps the helper within Sentry's free-tier quota when a downstream service
// goes down and every request fails.
const minInterval = time.Second

// ResetRateLimiter resets the rate-limiter state.  Only for use in tests.
func ResetRateLimiter() {
	rateMu.Lock()
	lastError = time.Time{}
	rateMu.Unlock()
}

// ReportError captures err to Sentry with any caller-supplied tags merged in.
// It also attaches:
//   - user_id (int64 DB user ID) from the request context when set via WithUserID
//   - request_id from the chi RequestID middleware context when available
//
// A simple 1-error/sec rate limiter prevents quota blowback when many
// requests fail simultaneously.  All existing log calls at the call site
// are preserved — this helper is purely additive.
//
// Nil err is a no-op.
func ReportError(ctx context.Context, err error, tags ...map[string]string) {
	if err == nil {
		return
	}

	// Rate-limit: at most one event per second.
	rateMu.Lock()
	now := time.Now()
	if now.Sub(lastError) < minInterval {
		rateMu.Unlock()
		return
	}
	lastError = now
	rateMu.Unlock()

	hub := sentry.CurrentHub().Clone()
	hub.WithScope(func(scope *sentry.Scope) {
		// Attach tags from all supplied maps.
		for _, m := range tags {
			for k, v := range m {
				scope.SetTag(k, v)
			}
		}

		// Attach DB user ID as Sentry user — no PII (no email, no name).
		if userID, ok := userIDFromContext(ctx); ok {
			scope.SetUser(sentry.User{ID: fmt.Sprintf("%d", userID)})
		}

		// Attach request_id when the chi RequestID middleware has run.
		if reqID := chimiddleware.GetReqID(ctx); reqID != "" {
			scope.SetTag("request_id", reqID)
		}

		hub.CaptureException(err)
	})
}
