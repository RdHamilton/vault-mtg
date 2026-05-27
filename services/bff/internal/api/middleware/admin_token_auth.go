package middleware

import (
	"crypto/subtle"
	"net/http"
)

// AdminTokenAuth returns middleware that requires a static high-entropy Bearer
// token on the Authorization header. The comparison uses
// crypto/subtle.ConstantTimeCompare to prevent timing-based token enumeration.
//
// When the configured token is empty, ALL requests are rejected — this guards
// against a misconfigured BFF inadvertently serving the admin endpoint to
// unauthenticated callers.
//
// Use this middleware exclusively on internal/admin routes (e.g.
// GET /api/v1/admin/daemons/fleet-health). User-facing routes use Clerk JWT
// auth (RequireClerkAuth / RequireClerkAuthForSSE).
func AdminTokenAuth(configuredToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// An empty configured token is a mis-boot: fail closed.
			if configuredToken == "" {
				writeUnauthorized(w)
				return
			}

			incoming, ok := bearerToken(r)
			if !ok {
				writeUnauthorized(w)
				return
			}

			// ConstantTimeCompare requires equal-length inputs to be truly
			// constant-time. To prevent length-based short-circuits we always
			// compare the full slices; the function internally handles
			// different lengths by returning 0 without revealing which byte
			// differed — that is the correct constant-time behaviour.
			if subtle.ConstantTimeCompare([]byte(configuredToken), []byte(incoming)) != 1 {
				writeUnauthorized(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
