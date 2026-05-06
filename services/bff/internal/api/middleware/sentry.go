// Package middleware provides HTTP middleware for the BFF service.
package middleware

import (
	"fmt"
	"net/http"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
)

// NewSentryMiddleware returns a chi-compatible middleware that:
//   - Recovers from panics, reports them to Sentry, and re-panics so that the
//     outer chi Recoverer middleware can write the 500 response.
//   - Attaches Sentry user context (DB user ID — no PII, no email, no name) to
//     every event when an authenticated user ID is present in the request context.
//
// If Sentry has not been initialised (no DSN configured) the middleware is a
// no-op passthrough — calls on an uninitialised client are safe and silently
// dropped by the SDK.
//
// The DSN is never touched inside this function.
func NewSentryMiddleware() func(http.Handler) http.Handler {
	h := sentryhttp.New(sentryhttp.Options{
		// Repanic=true lets chi's built-in Recoverer write the HTTP 500 response.
		// Without this the sentry middleware would swallow the panic and the
		// client would receive no response body.
		Repanic: true,
		// WaitForDelivery=false (default): flush happens asynchronously so that
		// individual requests are not blocked on Sentry network I/O.
		WaitForDelivery: false,
	})

	return func(next http.Handler) http.Handler {
		// Wrap the inner handler with the sentry-go HTTP handler.  This installs
		// a per-request hub clone and panic recovery on the context.
		wrapped := h.Handle(next)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Attach the authenticated DB user ID as Sentry user context so
			// events are searchable per user without storing PII.
			// UserIDFromContext reads the int64 user ID set by APIKeyAuth or
			// ClerkUserResolver middleware.  Returns (0, false) when no user is
			// authenticated (e.g. public routes).
			if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
				if userID, ok := UserIDFromContext(r.Context()); ok && userID != 0 {
					hub.Scope().SetUser(sentry.User{
						ID: fmt.Sprintf("%d", userID),
					})
				}
			}

			wrapped.ServeHTTP(w, r)
		})
	}
}
