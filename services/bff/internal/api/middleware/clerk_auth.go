// Package middleware provides HTTP middleware for the BFF service.
package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/observability"
	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
)

// ctxKeyClerkUserID is reserved for future use if direct context-key storage
// of the Clerk user ID is needed alongside the Clerk SDK's session claims.
// Currently the Clerk SDK owns the context key; this constant is kept for
// documentation purposes.
//
//nolint:unused
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
				observability.ReportError(
					r.Context(),
					fmt.Errorf("clerk auth rejected: status %d", rw.status),
					map[string]string{"component": "auth"},
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				body, _ := json.Marshal(map[string]string{"error": "unauthorized"})
				_, _ = w.Write(body)
			}
		})
	}
}

// RequireClerkAuthForSSE returns middleware that verifies a Clerk session JWT
// using a token extractor that accepts ANY of:
//
//  1. "Authorization: Bearer <token>" header — the standard path used by all
//     non-SSE browser requests and the existing test suite.
//  2. "__session" cookie — the session cookie written by the Clerk frontend SDK
//     when the SPA + BFF share a parent domain (e.g. production:
//     stg-app.vaultmtg.app + staging-api.vaultmtg.app under .vaultmtg.app).
//  3. "?token=<jwt>" query parameter — fallback for cross-domain SSE when the
//     Clerk Frontend API is on a different parent domain (e.g. the Clerk Dev
//     instance at *.clerk.accounts.dev).  The browser EventSource API has no
//     header support, so the SPA appends a fresh Clerk JWT to the SSE URL on
//     every (re)connect.  Clerk JWTs are short-lived (60s default) so the
//     exposure window is bounded; nginx is configured to log_access off on the
//     /api/v1/events path to keep tokens out of proxy logs.
//
// Auth approach: Clerk JWT passthrough — never custom signing or secret
// sharing.  All three sources carry the same Clerk-issued JWT; only the
// transport differs.
//
// Security considerations:
//   - The cookie must be same-site (Strict or Lax) and httpOnly to resist CSRF
//     and XSS.  The Clerk frontend SDK sets these attributes automatically.
//   - Use this middleware ONLY on GET endpoints that establish long-lived
//     read-only streams.  Mutation endpoints must continue to use the Bearer
//     header path (RequireClerkAuth) to avoid CSRF risk.
//   - Disable nginx access_log on routes mounted under this middleware to
//     avoid leaking ?token= JWTs into long-lived log files.
//
// The secretKey must be the Clerk backend API secret (CLERK_SECRET_KEY).
func RequireClerkAuthForSSE(secretKey string) func(http.Handler) http.Handler {
	clerk.SetKey(secretKey)

	// jwtExtractor returns the first non-empty token found in:
	// header → cookie → query.  The Clerk SDK calls this function to obtain
	// the raw JWT string before signature verification.
	jwtExtractor := func(r *http.Request) string {
		// 1. Bearer header (existing path — non-SSE callers and tests).
		if authHeader := strings.TrimSpace(r.Header.Get("Authorization")); authHeader != "" {
			return strings.TrimPrefix(authHeader, "Bearer ")
		}

		// 2. Clerk session cookie (EventSource / browser SSE path on
		// same-parent-domain deployments — e.g. prod).
		if cookie, err := r.Cookie(clerkSessionCookieName); err == nil {
			return cookie.Value
		}

		// 3. ?token= query parameter (EventSource cross-domain fallback —
		// e.g. staging-api.vaultmtg.app talking to a SPA whose Clerk session
		// cookie lives on *.clerk.accounts.dev).  Issue #1904.
		if token := r.URL.Query().Get("token"); token != "" {
			return token
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
				observability.ReportError(
					r.Context(),
					fmt.Errorf("clerk auth rejected: status %d", rw.status),
					map[string]string{"component": "auth"},
				)
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

// WithClerkUserID returns a copy of r with a synthetic Clerk session context
// that contains the given userID as the "sub" claim.
//
// This helper is intended for use in tests that need to simulate the effect of
// RequireClerkAuth having verified a JWT — it avoids the need for a real Clerk
// backend in unit tests.
func WithClerkUserID(r *http.Request, userID string) *http.Request {
	claims := &clerk.SessionClaims{}
	claims.Subject = userID
	ctx := clerk.ContextWithSessionClaims(r.Context(), claims)
	return r.WithContext(ctx)
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

// Flush delegates to the underlying ResponseWriter's Flush method when it
// supports http.Flusher.  Without this, SSE handlers that do w.(http.Flusher)
// get (nil, false) and return 500 "streaming not supported".
func (s *statusCapture) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
