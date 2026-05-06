// Package middleware provides HTTP middleware for the BFF service.
package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
)

// ctxKeyClerkUserID is the context key used to store the Clerk user ID (sub claim).
const ctxKeyClerkUserID ctxKey = "clerk_user_id"

// RequireClerkAuth returns middleware that verifies a Clerk session JWT from
// the "Authorization: Bearer <token>" header.
//
// If the token is missing or invalid the middleware responds with 401
// Unauthorized and the request does not reach the handler.  On success the
// verified session claims are available via clerk.SessionClaimsFromContext(r.Context()).
//
// The secretKey must be the Clerk backend API secret (CLERK_SECRET_KEY).
// Calling this function also initialises the Clerk SDK package-level key so
// that the SDK can fetch the JWKS from Clerk's servers for verification.
func RequireClerkAuth(secretKey string) func(http.Handler) http.Handler {
	// Initialise the Clerk SDK with the provided secret key.  This configures
	// the package-level HTTP client and JWKS cache used by WithHeaderAuthorization.
	clerk.SetKey(secretKey)

	return func(next http.Handler) http.Handler {
		// clerkhttp.RequireHeaderAuthorization returns 403 on a missing/invalid
		// token.  We wrap it to normalise the status code to 401, which is the
		// correct code for missing/invalid credentials (403 means authenticated
		// but forbidden).
		inner := clerkhttp.RequireHeaderAuthorization()(next)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &statusCapture{ResponseWriter: w}
			inner.ServeHTTP(rw, r)

			// Rewrite 403 → 401 for the "no valid token" case.
			if rw.status == http.StatusForbidden {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				body, _ := json.Marshal(map[string]string{"error": "unauthorized"})
				_, _ = w.Write(body)
			}
		})
	}
}

// ClerkUserIDFromContext returns the Clerk user ID (JWT "sub" claim) that
// RequireClerkAuth attached to the context.  Returns ("", false) when no
// Clerk claims are present.
func ClerkUserIDFromContext(r *http.Request) (string, bool) {
	claims, ok := clerk.SessionClaimsFromContext(r.Context())
	if !ok || claims == nil {
		return "", false
	}

	return claims.Subject, true
}

// statusCapture is a minimal ResponseWriter wrapper that records the first
// status code written so the outer handler can inspect it.
type statusCapture struct {
	http.ResponseWriter
	status  int
	written bool
}

func (s *statusCapture) WriteHeader(code int) {
	if !s.written {
		s.status = code
		s.written = true
		// Only forward non-403 codes; the outer handler rewrites 403 → 401
		// and writes its own headers.
		if code != http.StatusForbidden {
			s.ResponseWriter.WriteHeader(code)
		}
	}
}

func (s *statusCapture) Write(b []byte) (int, error) {
	if s.status == http.StatusForbidden {
		// Swallow the body; the outer handler writes its own 401 body.
		return len(b), nil
	}

	if !s.written {
		s.WriteHeader(http.StatusOK)
	}

	return s.ResponseWriter.Write(b)
}
