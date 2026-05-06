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

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/clerktest"

	"github.com/ramonehamilton/mtga-bff/internal/api/middleware"
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
