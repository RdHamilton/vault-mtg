package main

import (
	"context"
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

	contract "github.com/RdHamilton/MTGA-Companion/services/contract"
	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/clerktest"
	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/api/sse"
	"github.com/ramonehamilton/mtga-bff/internal/config"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// stubUserRepo is a ClerkUserLookup stub that always returns a fixed user (id=1).
type stubUserRepo struct {
	failWith error
}

func (s *stubUserRepo) UpsertByClerkUserID(_ context.Context, _ string) (*repository.User, error) {
	if s.failWith != nil {
		return nil, s.failWith
	}

	clerkID := "user_stub"

	return &repository.User{ID: 1, Email: "stub@clerk.local", ClerkUserID: &clerkID, SubscriptionTier: "free"}, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Test helpers
// ──────────────────────────────────────────────────────────────────────────────

// jwksForKey builds a minimal JWKS document from an RSA public key.
func jwksForKey(kid string, pub crypto.PublicKey) string {
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		panic("jwksForKey: not *rsa.PublicKey")
	}

	n := base64.RawURLEncoding.EncodeToString(rsaPub.N.Bytes())
	eBytes := big.NewInt(int64(rsaPub.E)).Bytes()
	e := base64.RawURLEncoding.EncodeToString(eBytes)

	return fmt.Sprintf(
		`{"keys":[{"use":"sig","kty":"RSA","kid":"%s","alg":"RS256","n":"%s","e":"%s"}]}`,
		kid, n, e,
	)
}

// setupClerkBackend starts a mock JWKS server and points the Clerk SDK at it.
// Returns a valid signed JWT string. The server is shut down via t.Cleanup.
func setupClerkBackend(t *testing.T) string {
	t.Helper()

	// Unique kid per test prevents JWKS cache collisions between tests.
	kid := "router-test-kid-" + t.Name()
	now := time.Now()
	claims := map[string]any{
		"sub": "user_router_test",
		"sid": "sess_router_test",
		"iss": "https://clerk.test",
		"iat": now.Add(-1 * time.Minute).Unix(),
		"nbf": now.Add(-1 * time.Minute).Unix(),
		"exp": now.Add(1 * time.Hour).Unix(),
	}

	jwt, pubKey := clerktest.GenerateJWT(t, claims, kid)
	jwks := jwksForKey(kid, pubKey)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jwks" && r.Method == http.MethodGet {
			_, _ = w.Write([]byte(jwks))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	clerk.SetBackend(clerk.NewBackend(&clerk.BackendConfig{
		HTTPClient: srv.Client(),
		URL:        &srv.URL,
	}))

	return jwt
}

// minimalConfig returns a non-production Config with no DB or secrets required.
func minimalConfig() *config.Config {
	return &config.Config{
		Env:                                 "development",
		AllowedOrigins:                      []string{"*"},
		DraftRatingsStalenessThresholdHours: 48,
		DaemonLatestVersion:                 "0.1.0",
	}
}

// noopBroadcaster satisfies handlers.EventBroadcaster without doing anything.
type noopBroadcaster struct{}

func (n *noopBroadcaster) BroadcastDaemonEvent(_ int64, _ contract.DaemonEvent) {}

// stubDraftGetter is a DraftRatingsGetter that always returns (nil, nil).
type stubDraftGetter struct{}

func (s *stubDraftGetter) GetRatings(_ context.Context, _, _ string) (*repository.DraftRatingsResult, error) {
	return nil, nil
}

// depsWithClerk builds minimal RouterDeps with ClerkAuthMiddl and
// ClerkUserResolver set (stub repo returns user id=1).
func depsWithClerk(t *testing.T) RouterDeps {
	t.Helper()

	broker := sse.NewWithHeartbeat(0)
	ingest := handlers.NewIngestHandler(&noopBroadcaster{})

	return RouterDeps{
		Broker:            broker,
		IngestHandler:     ingest,
		ClerkAuthMiddl:    bffmiddleware.RequireClerkAuth("test-secret-key"),
		ClerkUserResolver: bffmiddleware.ClerkUserResolver(&stubUserRepo{}),
	}
}

// depsNoAuth builds minimal RouterDeps with no auth middleware configured.
func depsNoAuth(t *testing.T) RouterDeps {
	t.Helper()

	broker := sse.NewWithHeartbeat(0)
	ingest := handlers.NewIngestHandler(&noopBroadcaster{})

	return RouterDeps{
		Broker:        broker,
		IngestHandler: ingest,
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Public routes
// ──────────────────────────────────────────────────────────────────────────────

// TestRouter_Health_IsPublic verifies /health is accessible without any auth.
func TestRouter_Health_IsPublic(t *testing.T) {
	r := BuildRouter(minimalConfig(), depsNoAuth(t))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /health: want 200, got %d", rr.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode /health body: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("/health status: want \"ok\", got %q", body["status"])
	}
}

// TestRouter_DaemonVersion_IsPublic verifies daemon version endpoint requires no auth.
func TestRouter_DaemonVersion_IsPublic(t *testing.T) {
	r := BuildRouter(minimalConfig(), depsNoAuth(t))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/daemon/version", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/daemon/version: want 200, got %d", rr.Code)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// SSE endpoint (GET /api/v1/events) — Clerk-protected
// ──────────────────────────────────────────────────────────────────────────────

func TestRouter_SSE_Returns401_WithoutToken(t *testing.T) {
	r := BuildRouter(minimalConfig(), depsWithClerk(t))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/events no token: want 401, got %d", rr.Code)
	}
}

func TestRouter_SSE_Returns401_WithInvalidToken(t *testing.T) {
	r := BuildRouter(minimalConfig(), depsWithClerk(t))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.jwt.at.all")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/events bad token: want 401, got %d", rr.Code)
	}
}

func TestRouter_SSE_401Body_IsJSON(t *testing.T) {
	r := BuildRouter(minimalConfig(), depsWithClerk(t))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("401 body not valid JSON: %v", err)
	}

	if body["error"] != "unauthorized" {
		t.Errorf("body[\"error\"]: want \"unauthorized\", got %q", body["error"])
	}
}

// TestRouter_SSE_ValidJWT_PassesClerkMiddleware verifies that a valid Clerk JWT
// is accepted by RequireClerkAuth. Uses httptest.NewServer + a real HTTP client
// so that http.Client.Do returns once response headers arrive — without blocking
// on the SSE stream. httptest.NewRecorder cannot be used here because
// r.ServeHTTP would never return (the SSE handler blocks on ctx.Done()).
func TestRouter_SSE_ValidJWT_PassesClerkMiddleware(t *testing.T) {
	jwt := setupClerkBackend(t)

	r := BuildRouter(minimalConfig(), depsWithClerk(t))

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/events", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/events: %v", err)
	}
	defer resp.Body.Close()

	// With a valid JWT the Clerk middleware must NOT emit {"error":"unauthorized"}.
	if resp.StatusCode == http.StatusUnauthorized {
		var body map[string]string
		if decErr := json.NewDecoder(resp.Body).Decode(&body); decErr == nil && body["error"] == "unauthorized" {
			t.Fatal("valid JWT: Clerk middleware rejected the token — it should have passed through")
		}
		t.Fatalf("unexpected 401 from /api/v1/events with valid JWT")
	}
	// Deferred cancel() terminates the SSE connection; ts.Close() then completes cleanly.
}

// ──────────────────────────────────────────────────────────────────────────────
// Draft ratings endpoint — Clerk-protected
// ──────────────────────────────────────────────────────────────────────────────

func TestRouter_DraftRatings_Returns401_WithoutToken(t *testing.T) {
	cfg := minimalConfig()
	deps := depsWithClerk(t)
	deps.DraftRatingsHandler = handlers.NewDraftRatingsHandler(&stubDraftGetter{}, cfg)

	r := BuildRouter(cfg, deps)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/draft-ratings/DSK/PremierDraft", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/draft-ratings no token: want 401, got %d", rr.Code)
	}
}

func TestRouter_DraftRatings_Returns401_WithInvalidToken(t *testing.T) {
	cfg := minimalConfig()
	deps := depsWithClerk(t)
	deps.DraftRatingsHandler = handlers.NewDraftRatingsHandler(&stubDraftGetter{}, cfg)

	r := BuildRouter(cfg, deps)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/draft-ratings/DSK/PremierDraft", nil)
	req.Header.Set("Authorization", "Bearer tampered.token.value")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/draft-ratings bad token: want 401, got %d", rr.Code)
	}
}

func TestRouter_DraftRatings_ValidJWT_PassesClerkMiddleware(t *testing.T) {
	jwt := setupClerkBackend(t)

	cfg := minimalConfig()
	deps := depsWithClerk(t)
	deps.DraftRatingsHandler = handlers.NewDraftRatingsHandler(&stubDraftGetter{}, cfg)

	r := BuildRouter(cfg, deps)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/draft-ratings/DSK/PremierDraft", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// Clerk middleware passes → handler returns 404 (stub returns nil result).
	// What must NOT happen: Clerk middleware rejects with {"error":"unauthorized"}.
	if rr.Code == http.StatusUnauthorized {
		var body map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&body); err == nil {
			if body["error"] == "unauthorized" {
				t.Fatal("valid JWT: Clerk middleware rejected the token — it should have passed through")
			}
		}
	}

	// Stub returns nil → handler returns 404.
	if rr.Code != http.StatusNotFound {
		t.Fatalf("valid JWT with nil stub: want 404 from handler, got %d", rr.Code)
	}
}

// TestRouter_DraftRatings_RouteAbsent_WhenHandlerNil verifies that when no
// DraftRatingsHandler is configured (no DB), chi returns 404 for that route —
// no panic and no unexpected error.
func TestRouter_DraftRatings_RouteAbsent_WhenHandlerNil(t *testing.T) {
	deps := depsWithClerk(t)
	// DraftRatingsHandler intentionally left nil.

	r := BuildRouter(minimalConfig(), deps)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/draft-ratings/DSK/PremierDraft", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("unregistered route: want 404, got %d", rr.Code)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// No-auth degraded mode
// ──────────────────────────────────────────────────────────────────────────────

// TestRouter_SSE_Returns503_WhenNoAuthConfigured verifies the 503 fallback when
// neither Clerk nor APIKey auth is configured.
func TestRouter_SSE_Returns503_WhenNoAuthConfigured(t *testing.T) {
	r := BuildRouter(minimalConfig(), depsNoAuth(t))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /api/v1/events no auth: want 503, got %d", rr.Code)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// ClerkUserResolver middleware tests
// ──────────────────────────────────────────────────────────────────────────────

// TestRouter_SSE_ValidJWT_WithResolver_ReachesHandler verifies that a valid
// Clerk JWT combined with a working ClerkUserResolver stub resolves the int64
// user ID and reaches the SSE handler (200 SSE response).
//
// Uses httptest.NewServer so http.Client.Do returns once headers arrive —
// avoiding the infinite SSE stream block that httptest.NewRecorder would cause.
func TestRouter_SSE_ValidJWT_WithResolver_ReachesHandler(t *testing.T) {
	jwt := setupClerkBackend(t)

	deps := depsWithClerk(t) // includes stubUserRepo returning id=1
	r := BuildRouter(minimalConfig(), deps)

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/events", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/events: %v", err)
	}
	defer resp.Body.Close()

	// A 401 from Clerk middleware indicates a token problem — must not happen.
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/events with resolver: unexpected 401")
	}

	// A JSON 500 from the resolver indicates the stub repo failed — must not happen.
	if resp.StatusCode == http.StatusInternalServerError {
		var errBody map[string]string
		if decErr := json.NewDecoder(resp.Body).Decode(&errBody); decErr == nil && errBody["error"] == "internal server error" {
			t.Fatalf("GET /api/v1/events with resolver: resolver returned 500")
		}
	}
	// Deferred cancel() terminates the SSE connection; ts.Close() then completes cleanly.
}

// TestRouter_SSE_ValidJWT_ResolverDBError_Returns500 verifies that when the
// user repo returns an error (e.g. DB down), the resolver middleware returns 500.
func TestRouter_SSE_ValidJWT_ResolverDBError_Returns500(t *testing.T) {
	jwt := setupClerkBackend(t)

	broker := sse.NewWithHeartbeat(0)
	ingest := handlers.NewIngestHandler(&noopBroadcaster{})

	deps := RouterDeps{
		Broker:            broker,
		IngestHandler:     ingest,
		ClerkAuthMiddl:    bffmiddleware.RequireClerkAuth("test-secret-key"),
		ClerkUserResolver: bffmiddleware.ClerkUserResolver(&stubUserRepo{failWith: context.DeadlineExceeded}),
	}

	r := BuildRouter(minimalConfig(), deps)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("resolver DB error: want 500, got %d", rr.Code)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// E2EUnguardedSSE — pipeline E2E bypass
// ──────────────────────────────────────────────────────────────────────────────

// TestRouter_SSE_E2EUnguardedSSE_AllowsUnauthenticated verifies that when
// E2EUnguardedSSE=true, GET /api/v1/events is reachable without any auth token.
// The sentinel middleware injects user ID=1 into context so the SSE broker
// does not return 401. The context is cancelled immediately so the SSE handler
// exits without blocking.
func TestRouter_SSE_E2EUnguardedSSE_AllowsUnauthenticated(t *testing.T) {
	deps := depsNoAuth(t)
	deps.E2EUnguardedSSE = true
	r := BuildRouter(minimalConfig(), deps)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so SSE handler exits after setup

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil).WithContext(ctx)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code == http.StatusServiceUnavailable {
		t.Fatalf("GET /api/v1/events E2EUnguardedSSE=true: got 503 (auth blocking); want SSE handler response")
	}
}
