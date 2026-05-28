package middleware_test

import (
	"crypto"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/middleware"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/observability"
	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/clerktest"
	"github.com/getsentry/sentry-go"
)

// jwksFromPublicKey serialises an *rsa.PublicKey into a minimal JWKS JSON
// document suitable for the Clerk SDK's JWKS endpoint.
func jwksFromPublicKey(kid string, pub crypto.PublicKey) string {
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		panic("jwksFromPublicKey: not an *rsa.PublicKey")
	}

	// Encode modulus and exponent in base64url (no padding) as required by JWK.
	n := base64.RawURLEncoding.EncodeToString(rsaPub.N.Bytes())

	e := big.NewInt(int64(rsaPub.E))
	eBytes := e.Bytes()
	eEncoded := base64.RawURLEncoding.EncodeToString(eBytes)

	return fmt.Sprintf(
		`{"keys":[{"use":"sig","kty":"RSA","kid":"%s","alg":"RS256","n":"%s","e":"%s"}]}`,
		kid, n, eEncoded,
	)
}

// withClerkBackend creates a mock Clerk JWKS server and points the SDK at it.
// If pub is nil the server always returns 404 (used for "no valid key" cases).
func withClerkBackend(t *testing.T, kid string, pub crypto.PublicKey) {
	t.Helper()

	var jwksBody string
	if pub != nil {
		jwksBody = jwksFromPublicKey(kid, pub)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jwks" && r.Method == http.MethodGet && jwksBody != "" {
			_, _ = w.Write([]byte(jwksBody))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	clerk.SetBackend(clerk.NewBackend(&clerk.BackendConfig{
		HTTPClient: srv.Client(),
		URL:        &srv.URL,
	}))
}

// clerkOKHandler is a sentinel handler that writes the Clerk subject claim
// when RequireClerkAuth passes the request through.
var clerkOKHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.ClerkUserIDFromContext(r)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("no clerk claims"))
		return
	}

	_, _ = fmt.Fprint(w, userID)
})

// TestRequireClerkAuth_MissingToken verifies that requests with no
// Authorization header are rejected with 401.
func TestRequireClerkAuth_MissingToken(t *testing.T) {
	withClerkBackend(t, "kid-missing", nil)

	handler := middleware.RequireClerkAuth("sk_test_dummy")(clerkOKHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing token: want 401, got %d", rr.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body["error"] != "unauthorized" {
		t.Errorf("body error field: want \"unauthorized\", got %q", body["error"])
	}
}

// TestRequireClerkAuth_MalformedToken verifies that a token that is not a
// valid JWT is rejected with 401.
func TestRequireClerkAuth_MalformedToken(t *testing.T) {
	withClerkBackend(t, "kid-malformed", nil)

	handler := middleware.RequireClerkAuth("sk_test_dummy")(clerkOKHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.jwt.at.all")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("malformed token: want 401, got %d", rr.Code)
	}
}

// TestRequireClerkAuth_UnknownKID verifies that a structurally valid JWT
// signed with an unknown key (not in the served JWKS) is rejected with 401.
func TestRequireClerkAuth_UnknownKID(t *testing.T) {
	kid := "test-kid-unknown"

	// Serve a JWKS that has no matching key.
	withClerkBackend(t, kid, nil)

	now := time.Now()
	claims := map[string]any{
		"sub": "user_unknown",
		"sid": "sess_unknown",
		"iss": "https://clerk.test",
		"iat": now.Add(-1 * time.Minute).Unix(),
		"nbf": now.Add(-1 * time.Minute).Unix(),
		"exp": now.Add(1 * time.Hour).Unix(),
	}

	token, _ := clerktest.GenerateJWT(t, claims, kid)

	handler := middleware.RequireClerkAuth("sk_test_dummy")(clerkOKHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unknown KID: want 401, got %d", rr.Code)
	}
}

// TestRequireClerkAuth_ValidToken verifies that a structurally valid,
// unexpired JWT whose key is present in the JWKS passes with 200 and the
// subject claim is accessible via ClerkUserIDFromContext.
//
// The issuer must match the Clerk issuer pattern ("https://clerk.*") as
// enforced by the Clerk SDK's jwt.Verify.
func TestRequireClerkAuth_ValidToken(t *testing.T) {
	kid := "test-kid-valid"

	now := time.Now()
	claims := map[string]any{
		"sub": "user_abc123",
		"sid": "sess_xyz",
		"iss": "https://clerk.test",
		"iat": now.Add(-1 * time.Minute).Unix(),
		"nbf": now.Add(-1 * time.Minute).Unix(),
		"exp": now.Add(1 * time.Hour).Unix(),
	}

	token, pubKey := clerktest.GenerateJWT(t, claims, kid)

	// Serve a JWKS that contains the matching public key.
	withClerkBackend(t, kid, pubKey)

	handler := middleware.RequireClerkAuth("sk_test_dummy")(clerkOKHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("valid token: want 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	if rr.Body.String() != "user_abc123" {
		t.Errorf("subject: want \"user_abc123\", got %q", rr.Body.String())
	}
}

// TestRequireClerkAuth_Response401HasJSONBody verifies the 401 body is valid
// JSON with error=unauthorized so callers can parse it programmatically.
func TestRequireClerkAuth_Response401HasJSONBody(t *testing.T) {
	withClerkBackend(t, "kid-json-body", nil)

	handler := middleware.RequireClerkAuth("sk_test_dummy")(clerkOKHandler)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	// No Authorization header.
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("401 body is not valid JSON: %v", err)
	}

	if body["error"] != "unauthorized" {
		t.Errorf("body[\"error\"]: want \"unauthorized\", got %q", body["error"])
	}
}

// TestClerkUserIDFromContext_NoClaims verifies that ClerkUserIDFromContext
// returns ("", false) when no Clerk session claims are on the context.
func TestClerkUserIDFromContext_NoClaims(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	uid, ok := middleware.ClerkUserIDFromContext(req)

	if ok || uid != "" {
		t.Errorf("no claims: want (\"\", false), got (%q, %v)", uid, ok)
	}
}

// ── RequireClerkAuthForSSE tests ─────────────────────────────────────────────
//
// RequireClerkAuthForSSE accepts the Clerk session cookie ("__session") as a
// fallback token source in addition to the standard Authorization: Bearer
// header.  These tests cover the SSE auth path required by ticket #1387.

// TestRequireClerkAuthForSSE_UnauthenticatedReturns401 verifies that a request
// with neither an Authorization header nor an __session cookie is rejected with
// 401 Unauthorized.
func TestRequireClerkAuthForSSE_UnauthenticatedReturns401(t *testing.T) {
	withClerkBackend(t, "kid-sse-unauth", nil)

	handler := middleware.RequireClerkAuthForSSE("sk_test_dummy")(clerkOKHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated SSE: want 401, got %d", rr.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("401 body is not valid JSON: %v", err)
	}

	if body["error"] != "unauthorized" {
		t.Errorf("body error field: want \"unauthorized\", got %q", body["error"])
	}
}

// TestRequireClerkAuthForSSE_ValidBearerHeader verifies that the standard
// Authorization: Bearer <token> path is unaffected — existing non-SSE callers
// and the existing test suite continue to work unchanged.
func TestRequireClerkAuthForSSE_ValidBearerHeader(t *testing.T) {
	kid := "kid-sse-bearer"

	now := time.Now()
	claims := map[string]any{
		"sub": "user_bearer",
		"sid": "sess_bearer",
		"iss": "https://clerk.test",
		"iat": now.Add(-1 * time.Minute).Unix(),
		"nbf": now.Add(-1 * time.Minute).Unix(),
		"exp": now.Add(1 * time.Hour).Unix(),
	}

	token, pubKey := clerktest.GenerateJWT(t, claims, kid)
	withClerkBackend(t, kid, pubKey)

	handler := middleware.RequireClerkAuthForSSE("sk_test_dummy")(clerkOKHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("SSE bearer: want 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	if rr.Body.String() != "user_bearer" {
		t.Errorf("subject: want \"user_bearer\", got %q", rr.Body.String())
	}
}

// TestRequireClerkAuthForSSE_ValidSessionCookie verifies that an EventSource
// connection authenticated via the Clerk "__session" cookie (no Authorization
// header) is accepted and the user ID is correctly extracted from the JWT.
func TestRequireClerkAuthForSSE_ValidSessionCookie(t *testing.T) {
	kid := "kid-sse-cookie"

	now := time.Now()
	claims := map[string]any{
		"sub": "user_cookie",
		"sid": "sess_cookie",
		"iss": "https://clerk.test",
		"iat": now.Add(-1 * time.Minute).Unix(),
		"nbf": now.Add(-1 * time.Minute).Unix(),
		"exp": now.Add(1 * time.Hour).Unix(),
	}

	token, pubKey := clerktest.GenerateJWT(t, claims, kid)
	withClerkBackend(t, kid, pubKey)

	handler := middleware.RequireClerkAuthForSSE("sk_test_dummy")(clerkOKHandler)

	// No Authorization header — only the __session cookie is present.
	// This mirrors how the browser EventSource API authenticates SSE connections.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	req.AddCookie(&http.Cookie{Name: "__session", Value: token})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("SSE cookie: want 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	if rr.Body.String() != "user_cookie" {
		t.Errorf("subject: want \"user_cookie\", got %q", rr.Body.String())
	}
}

// TestRequireClerkAuthForSSE_InvalidCookieReturns401 verifies that a request
// carrying a malformed or expired __session cookie value is rejected with 401.
func TestRequireClerkAuthForSSE_InvalidCookieReturns401(t *testing.T) {
	withClerkBackend(t, "kid-sse-bad-cookie", nil) // no valid key in JWKS

	handler := middleware.RequireClerkAuthForSSE("sk_test_dummy")(clerkOKHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	req.AddCookie(&http.Cookie{Name: "__session", Value: "not.a.valid.jwt"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("invalid cookie: want 401, got %d", rr.Code)
	}
}

// TestRequireClerkAuthForSSE_ValidQueryToken verifies that an EventSource
// connection authenticated via the ?token=<jwt> query parameter is accepted.
// This is the cross-domain fallback used when the Clerk Frontend API is on a
// different parent domain than the BFF — e.g. staging's Dev Clerk instance at
// *.clerk.accounts.dev talking to staging-api.vaultmtg.app.  Issue #1904.
func TestRequireClerkAuthForSSE_ValidQueryToken(t *testing.T) {
	kid := "kid-sse-query"

	now := time.Now()
	claims := map[string]any{
		"sub": "user_query",
		"sid": "sess_query",
		"iss": "https://clerk.test",
		"iat": now.Add(-1 * time.Minute).Unix(),
		"nbf": now.Add(-1 * time.Minute).Unix(),
		"exp": now.Add(1 * time.Hour).Unix(),
	}

	token, pubKey := clerktest.GenerateJWT(t, claims, kid)
	withClerkBackend(t, kid, pubKey)

	handler := middleware.RequireClerkAuthForSSE("sk_test_dummy")(clerkOKHandler)

	// No Authorization header, no __session cookie — only ?token=.
	// This mirrors the staging SPA's cross-domain SSE connection where the
	// Clerk session cookie lives on *.clerk.accounts.dev and never reaches
	// staging-api.vaultmtg.app.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events?token="+token, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("SSE query token: want 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	if rr.Body.String() != "user_query" {
		t.Errorf("subject: want \"user_query\", got %q", rr.Body.String())
	}
}

// TestRequireClerkAuthForSSE_InvalidQueryToken verifies that a request carrying
// a malformed or expired ?token= value is rejected with 401.
func TestRequireClerkAuthForSSE_InvalidQueryToken(t *testing.T) {
	withClerkBackend(t, "kid-sse-bad-query", nil) // no valid key in JWKS

	handler := middleware.RequireClerkAuthForSSE("sk_test_dummy")(clerkOKHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events?token=not.a.valid.jwt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("invalid query token: want 401, got %d", rr.Code)
	}
}

// TestRequireClerkAuthForSSE_HeaderPreferredOverQuery verifies the source
// precedence: when a Bearer header AND a ?token= are both present, the header
// wins.  The query-string fallback is intentionally last in the chain so a
// stale token cached in a URL bar can never override a fresh Bearer header.
func TestRequireClerkAuthForSSE_HeaderPreferredOverQuery(t *testing.T) {
	kid := "kid-sse-prefer-header-over-query"

	now := time.Now()
	claims := map[string]any{
		"sub": "user_prefer_header",
		"sid": "sess_prefer_header",
		"iss": "https://clerk.test",
		"iat": now.Add(-1 * time.Minute).Unix(),
		"nbf": now.Add(-1 * time.Minute).Unix(),
		"exp": now.Add(1 * time.Hour).Unix(),
	}

	token, pubKey := clerktest.GenerateJWT(t, claims, kid)
	withClerkBackend(t, kid, pubKey)

	handler := middleware.RequireClerkAuthForSSE("sk_test_dummy")(clerkOKHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events?token=stale.query.token", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("prefer header: want 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	if rr.Body.String() != "user_prefer_header" {
		t.Errorf("subject: want \"user_prefer_header\", got %q", rr.Body.String())
	}
}

// TestRequireClerkAuthForSSE_CookiePreferredOverQuery verifies the source
// precedence: when an __session cookie AND a ?token= are both present, the
// cookie wins.  Same reasoning as the header path — the query-string is the
// last resort.
func TestRequireClerkAuthForSSE_CookiePreferredOverQuery(t *testing.T) {
	kid := "kid-sse-prefer-cookie-over-query"

	now := time.Now()
	claims := map[string]any{
		"sub": "user_prefer_cookie",
		"sid": "sess_prefer_cookie",
		"iss": "https://clerk.test",
		"iat": now.Add(-1 * time.Minute).Unix(),
		"nbf": now.Add(-1 * time.Minute).Unix(),
		"exp": now.Add(1 * time.Hour).Unix(),
	}

	token, pubKey := clerktest.GenerateJWT(t, claims, kid)
	withClerkBackend(t, kid, pubKey)

	handler := middleware.RequireClerkAuthForSSE("sk_test_dummy")(clerkOKHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events?token=stale.query.token", nil)
	req.AddCookie(&http.Cookie{Name: "__session", Value: token})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("prefer cookie: want 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	if rr.Body.String() != "user_prefer_cookie" {
		t.Errorf("subject: want \"user_prefer_cookie\", got %q", rr.Body.String())
	}
}

// TestRequireClerkAuthForSSE_CookieIgnoredWhenBearerPresent verifies that when
// both an Authorization header and an __session cookie are present, the Bearer
// header takes precedence (header is checked first in the extractor).
func TestRequireClerkAuthForSSE_CookieIgnoredWhenBearerPresent(t *testing.T) {
	kid := "kid-sse-prefer-bearer"

	now := time.Now()
	claims := map[string]any{
		"sub": "user_prefer_bearer",
		"sid": "sess_prefer_bearer",
		"iss": "https://clerk.test",
		"iat": now.Add(-1 * time.Minute).Unix(),
		"nbf": now.Add(-1 * time.Minute).Unix(),
		"exp": now.Add(1 * time.Hour).Unix(),
	}

	token, pubKey := clerktest.GenerateJWT(t, claims, kid)
	withClerkBackend(t, kid, pubKey)

	handler := middleware.RequireClerkAuthForSSE("sk_test_dummy")(clerkOKHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	// Also attach a cookie with a bad token to confirm it is not used.
	req.AddCookie(&http.Cookie{Name: "__session", Value: "stale.cookie.token"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("prefer bearer: want 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	if rr.Body.String() != "user_prefer_bearer" {
		t.Errorf("subject: want \"user_prefer_bearer\", got %q", rr.Body.String())
	}
}

// TestRequireClerkAuth_SentryEventOnRejectedToken verifies that when
// RequireClerkAuth rejects an invalid token it sends a Sentry event with
// the component=auth tag.
func TestRequireClerkAuth_SentryEventOnRejectedToken(t *testing.T) {
	// Wire up a Sentry mock transport so we can capture events without
	// making real network calls.
	transport := &sentry.MockTransport{}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:       "https://key@o0.ingest.sentry.io/0",
		Transport: transport,
	}); err != nil {
		t.Fatalf("sentry.Init: %v", err)
	}
	// Reset the rate-limiter and restore global Sentry state in cleanup.
	observability.ResetRateLimiter()
	t.Cleanup(func() {
		_ = sentry.Init(sentry.ClientOptions{})
		observability.ResetRateLimiter()
	})

	// No valid key in JWKS so the SDK will reject any token.
	withClerkBackend(t, "kid-sentry-auth", nil)

	handler := middleware.RequireClerkAuth("sk_test_dummy")(clerkOKHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.jwt")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}

	sentry.Flush(200 * time.Millisecond)

	events := transport.Events()
	if len(events) == 0 {
		t.Fatal("expected a Sentry event for rejected Clerk token, got none")
	}
	ev := events[0]
	if ev.Tags["component"] != "auth" {
		t.Errorf("tag component: want %q, got %q", "auth", ev.Tags["component"])
	}
}
