package middleware

import (
	"crypto/subtle"
	"log"
	"net/http"
	"strings"
)

// AdminTokenAuth returns middleware that requires a static high-entropy Bearer
// token on the Authorization header. The comparison uses
// crypto/subtle.ConstantTimeCompare to prevent timing-based token enumeration.
//
// When the configured token is empty, ALL requests are rejected — this guards
// against a misconfigured BFF inadvertently serving the admin endpoint to
// unauthenticated callers.
//
// Every auth attempt (success or failure) is written to the standard logger
// in the format: [admin_auth] outcome=ok|fail reason=<reason> path=<path> remote=<ip>
// The token value is NEVER included in the log line (I-10 / AC2).
//
// Use this middleware exclusively on internal/admin routes (e.g.
// GET /api/v1/admin/daemons/fleet-health). User-facing routes use Clerk JWT
// auth (RequireClerkAuth / RequireClerkAuthForSSE).
func AdminTokenAuth(configuredToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// An empty configured token is a mis-boot: fail closed.
			if configuredToken == "" {
				log.Printf("[admin_auth] outcome=fail reason=token_not_configured path=%s remote=%s", r.URL.Path, adminRemoteAddr(r))
				writeUnauthorized(w)
				return
			}

			incoming, ok := bearerToken(r)
			if !ok {
				log.Printf("[admin_auth] outcome=fail reason=missing_bearer path=%s remote=%s", r.URL.Path, adminRemoteAddr(r))
				writeUnauthorized(w)
				return
			}

			// ConstantTimeCompare requires equal-length inputs to be truly
			// constant-time. To prevent length-based short-circuits we always
			// compare the full slices; the function internally handles
			// different lengths by returning 0 without revealing which byte
			// differed — that is the correct constant-time behaviour.
			if subtle.ConstantTimeCompare([]byte(configuredToken), []byte(incoming)) != 1 {
				log.Printf("[admin_auth] outcome=fail reason=bad_token path=%s remote=%s", r.URL.Path, adminRemoteAddr(r))
				writeUnauthorized(w)
				return
			}

			log.Printf("[admin_auth] outcome=ok path=%s remote=%s", r.URL.Path, adminRemoteAddr(r))
			next.ServeHTTP(w, r)
		})
	}
}

// adminRemoteAddr returns X-Forwarded-For (first value, nginx-proxied path) or
// r.RemoteAddr as a fallback. The IP address is forensic context — it is not a
// secret. The token value is never included in any log line.
func adminRemoteAddr(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For may be a comma-separated list; take the first entry.
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	return r.RemoteAddr
}
