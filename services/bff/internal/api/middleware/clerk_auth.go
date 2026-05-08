// Package middleware provides HTTP middleware for the BFF service.
package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
)

// ctxKeyClerkUserID is the context key used to store the Clerk user ID (sub claim).
const ctxKeyClerkUserID ctxKey = "clerk_user_id"

// clerkSessionCookieName is the name of the session cookie set by the Clerk
// frontend SDK.  The browser sends this cookie automatically on same-origin
// requests, including EventSource connections which cannot set custom headers.
const clerkSessionCookieName = "__session"

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

// RequireClerkAuthForSSE returns middleware that verifies a Clerk session JWT
// using a token extractor that accepts EITHER:
//
//  1. "Authorization: Bearer <token>" header — the standard path used by all
//     non-SSE browser requests and the existing test suite.
//  2. "__session" cookie — the session cookie written by the Clerk frontend SDK.
//     The browser EventSource API cannot set custom request headers, so it sends
//     the Clerk session cookie automatically on same-origin connections instead.
//
// Auth approach: Clerk session cookie passthrough.
//
//   - The Clerk JS SDK stores the active session token in a cookie named
//     "__session" (httpOnly, Secure in production).
//   - On SSE connections the extractor reads the cookie value and treats it as
//     the Bearer token for Clerk JWT verification.  No custom signing or secret
//     sharing is required — it is the same Clerk-issued JWT, delivered via a
//     different transport.
//   - The Bearer header path is checked first; the cookie is only used when no
//     header is present.  This means non-SSE routes behind this middleware
//     continue to work identically to RequireClerkAuth.
//
// Security considerations:
//   - The cookie must be same-site (Strict or Lax) and httpOnly to resist CSRF
//     and XSS.  The Clerk frontend SDK sets these attributes automatically.
//   - Use this middleware ONLY on GET endpoints that establish long-lived
//     read-only streams.  Mutation endpoints must continue to use the Bearer
//     header path (RequireClerkAuth) to avoid CSRF risk.
//
// The secretKey must be the Clerk backend API secret (CLERK_SECRET_KEY).
func RequireClerkAuthForSSE(secretKey string) func(http.Handler) http.Handler {
	clerk.SetKey(secretKey)

	// jwtExtractor checks the Authorization header first, then falls back to
	// the Clerk session cookie.  The Clerk SDK calls this function to obtain the
	// raw JWT string before signature verification.
	jwtExtractor := func(r *http.Request) string {
		// 1. Bearer header (existing path — non-SSE callers and tests).
		if authHeader := strings.TrimSpace(r.Header.Get("Authorization")); authHeader != "" {
			return strings.TrimPrefix(authHeader, "Bearer ")
		}

		// 2. Clerk session cookie (EventSource / browser SSE path).
		if cookie, err := r.Cookie(clerkSessionCookieName); err == nil {
			return cookie.Value
		}

		return ""
	}

	return func(next http.Handler) http.Handler {
		inner := clerkhttp.RequireHeaderAuthorization(
			clerkhttp.AuthorizationJWTExtractor(jwtExtractor),
		)(next)

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
