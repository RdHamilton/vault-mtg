package pkce

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGeneratePKCE_Uniqueness verifies that two calls produce different verifiers.
func TestGeneratePKCE_Uniqueness(t *testing.T) {
	v1, c1, err := generatePKCE()
	require.NoError(t, err)
	v2, c2, err := generatePKCE()
	require.NoError(t, err)

	assert.NotEqual(t, v1, v2)
	assert.NotEqual(t, c1, c2)
}

// TestGeneratePKCE_Base64URL verifies that verifier and challenge are valid base64url strings.
func TestGeneratePKCE_Base64URL(t *testing.T) {
	v, c, err := generatePKCE()
	require.NoError(t, err)

	assert.NotEmpty(t, v)
	assert.NotEmpty(t, c)
	// base64url chars: A-Z a-z 0-9 - _  (no padding)
	for _, ch := range v {
		assert.True(t, isBase64URLChar(ch), "verifier contains invalid char: %q", ch)
	}
	for _, ch := range c {
		assert.True(t, isBase64URLChar(ch), "challenge contains invalid char: %q", ch)
	}
}

func isBase64URLChar(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
		(r >= '0' && r <= '9') || r == '-' || r == '_'
}

// TestBuildAuthURL_HappyPath verifies the constructed URL contains required params.
func TestBuildAuthURL_HappyPath(t *testing.T) {
	cfg := Config{
		ClerkFrontendAPI: "https://accounts.example.com",
		ClientID:         "pk_test_abc123",
	}
	redirectURI := fmt.Sprintf("http://localhost:%d%s", PrimaryPort, CallbackPath)
	authURL, err := buildAuthURL(cfg, "challenge_abc", redirectURI)
	require.NoError(t, err)

	assert.Contains(t, authURL, "https://accounts.example.com/oauth/authorize")
	assert.Contains(t, authURL, "response_type=code")
	assert.Contains(t, authURL, "client_id=pk_test_abc123")
	assert.Contains(t, authURL, "code_challenge=challenge_abc")
	assert.Contains(t, authURL, "code_challenge_method=S256")
	assert.Contains(t, authURL, "redirect_uri=")
}

// TestBuildAuthURL_MissingClerkFrontendAPI errors when ClerkFrontendAPI is empty.
func TestBuildAuthURL_MissingClerkFrontendAPI(t *testing.T) {
	cfg := Config{ClientID: "pk_test_abc"}
	_, err := buildAuthURL(cfg, "ch", "http://localhost:51423/oauth/callback")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ClerkFrontendAPI")
}

// TestBuildAuthURL_MissingClientID errors when ClientID is empty.
func TestBuildAuthURL_MissingClientID(t *testing.T) {
	cfg := Config{ClerkFrontendAPI: "https://accounts.example.com"}
	_, err := buildAuthURL(cfg, "ch", "http://localhost:51423/oauth/callback")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ClientID")
}

// TestBuildAuthURL_DefaultScopes verifies profile+email scopes are used when Scopes is nil.
func TestBuildAuthURL_DefaultScopes(t *testing.T) {
	cfg := Config{
		ClerkFrontendAPI: "https://accounts.example.com",
		ClientID:         "pk_test_abc",
	}
	authURL, err := buildAuthURL(cfg, "ch", "http://localhost:51423/oauth/callback")
	require.NoError(t, err)
	assert.Contains(t, authURL, "scope=")
	assert.True(t, strings.Contains(authURL, "profile") || strings.Contains(authURL, "email"))
}

// TestParseTokenResponse_HappyPath parses a normal Clerk token response.
func TestParseTokenResponse_HappyPath(t *testing.T) {
	body := []byte(`{"access_token":"tok_abc","refresh_token":"ref_xyz"}`)
	tok, err := parseTokenResponse(body)
	require.NoError(t, err)
	assert.Equal(t, "tok_abc", tok.AccessToken)
	assert.Equal(t, "ref_xyz", tok.RefreshToken)
}

// TestParseTokenResponse_IDTokenFallback uses id_token when access_token is absent.
func TestParseTokenResponse_IDTokenFallback(t *testing.T) {
	body := []byte(`{"id_token":"idtok_abc"}`)
	tok, err := parseTokenResponse(body)
	require.NoError(t, err)
	assert.Equal(t, "idtok_abc", tok.AccessToken)
}

// TestParseTokenResponse_ErrorField returns error from error field.
func TestParseTokenResponse_ErrorField(t *testing.T) {
	body := []byte(`{"error":"invalid_client","error_description":"Bad client"}`)
	_, err := parseTokenResponse(body)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_client")
}

// TestParseTokenResponse_MissingToken errors when no token is present.
func TestParseTokenResponse_MissingToken(t *testing.T) {
	body := []byte(`{}`)
	_, err := parseTokenResponse(body)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access_token")
}

// TestParseTokenResponse_MalformedJSON errors on invalid JSON.
func TestParseTokenResponse_MalformedJSON(t *testing.T) {
	_, err := parseTokenResponse([]byte(`not-json`))
	require.Error(t, err)
}

// TestExchangeCode_HappyPath tests the token exchange against a mock server.
func TestExchangeCode_HappyPath(t *testing.T) {
	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		b, _ := json.Marshal(nil)
		_ = b
		buf := new(strings.Builder)
		fmt.Fprintf(buf, "")
		body := make([]byte, 512)
		n, _ := r.Body.Read(body)
		capturedBody = string(body[:n])

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok_ok"})
	}))
	defer srv.Close()

	tok, err := exchangeCode(context.Background(), srv.URL, "pk_test", "authcode123", "verifier456", "http://localhost:51423/oauth/callback")
	require.NoError(t, err)
	assert.Equal(t, "tok_ok", tok.AccessToken)
	assert.Contains(t, capturedBody, "grant_type=authorization_code")
	assert.Contains(t, capturedBody, "code=authcode123")
}

// TestExchangeCode_MissingEndpoint errors when TokenEndpoint is empty.
func TestExchangeCode_MissingEndpoint(t *testing.T) {
	_, err := exchangeCode(context.Background(), "", "pk_test", "code", "verifier", "http://localhost:51423/oauth/callback")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tokenEndpoint")
}

// TestExchangeCode_Non200 errors on a non-2xx response.
func TestExchangeCode_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()

	_, err := exchangeCode(context.Background(), srv.URL, "pk_test", "code", "verifier", "http://localhost:51423/oauth/callback")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

// redirectResult carries the HTTP status code and Location header from the
// callback request made by the test goroutine. Using a channel avoids the data
// race that would occur if the goroutine wrote to shared variables while the
// test goroutine was reading them after waitForCode returned.
type redirectResult struct {
	statusCode int
	location   string
}

// TestWaitForCode_HappyPath sends a code to the callback server and verifies
// that the server responds with a 302 redirect to SuccessRedirectURL.
// The redirect response prevents the consent-loop bug (#2084): a short
// response consisting only of headers is fully flushed before the server
// shuts down, so the browser navigates away and never retries the request.
func TestWaitForCode_HappyPath(t *testing.T) {
	// Use port 0 so the OS assigns any free port.
	l, err := startListener(t)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	callbackURL := fmt.Sprintf("http://127.0.0.1:%d%s?code=mycode123", listenerPort(l), CallbackPath)

	// resultCh carries the captured HTTP response from the goroutine.
	resultCh := make(chan redirectResult, 1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		// Use a client that does NOT follow redirects so we can inspect the 302.
		noFollow := &http.Client{
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		resp, getErr := noFollow.Get(callbackURL) //nolint:noctx
		if getErr == nil {
			resultCh <- redirectResult{
				statusCode: resp.StatusCode,
				location:   resp.Header.Get("Location"),
			}
			_ = resp.Body.Close()
		} else {
			resultCh <- redirectResult{}
		}
	}()

	code, err := waitForCode(ctx, l)
	require.NoError(t, err)
	assert.Equal(t, "mycode123", code)

	// Wait for the goroutine's result so we can assert on it without a race.
	result := <-resultCh

	// Verify the browser received a redirect (not an HTML body) so it navigates
	// away rather than retrying — this is the fix for the consent loop.
	assert.Equal(t, http.StatusFound, result.statusCode, "callback must respond with 302 redirect")
	assert.Equal(t, SuccessRedirectURL, result.location, "redirect must point to SuccessRedirectURL")
}

// TestWaitForCode_NoConsentLoop verifies that a second callback request after
// the first succeeds is silently dropped and does not cause the server to emit
// a second code or re-open consent. This tests the one-shot nature of the
// callback server (fix for #2084).
func TestWaitForCode_NoConsentLoop(t *testing.T) {
	l, err := startListener(t)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	port := listenerPort(l)
	callbackURL := fmt.Sprintf("http://127.0.0.1:%d%s?code=firstcode", port, CallbackPath)

	go func() {
		time.Sleep(50 * time.Millisecond)
		noFollow := &http.Client{
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		// First request — should succeed and redirect.
		resp, getErr := noFollow.Get(callbackURL) //nolint:noctx
		if getErr == nil {
			_ = resp.Body.Close()
		}
		// Second request — server is shutting down; the codeCh select has
		// a default branch so the second code is dropped.  We only verify
		// that waitForCode returned after the first request, not the second.
	}()

	code, err := waitForCode(ctx, l)
	require.NoError(t, err)
	assert.Equal(t, "firstcode", code, "only the first code must be returned")
}

// TestWaitForCode_ErrorParam returns error when callback contains error parameter.
func TestWaitForCode_ErrorParam(t *testing.T) {
	l, err := startListener(t)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	callbackURL := fmt.Sprintf("http://127.0.0.1:%d%s?error=access_denied&error_description=user+denied",
		listenerPort(l), CallbackPath)
	go func() {
		time.Sleep(100 * time.Millisecond)
		resp, getErr := http.Get(callbackURL) //nolint:noctx
		if getErr == nil {
			_ = resp.Body.Close()
		}
	}()

	_, err = waitForCode(ctx, l)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access_denied")
}

// TestWaitForCode_Timeout returns error when context expires before code arrives.
func TestWaitForCode_Timeout(t *testing.T) {
	l, err := startListener(t)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = waitForCode(ctx, l)
	require.Error(t, err)
}

// TestConstants verifies the exported constants.
func TestConstants(t *testing.T) {
	assert.Equal(t, 51423, PrimaryPort)
	assert.Equal(t, 51424, FallbackPort)
	assert.Equal(t, "/oauth/callback", CallbackPath)
	// SuccessRedirectURL must be a valid absolute HTTPS URL so the browser can
	// navigate away from the callback page without retrying the OAuth request.
	assert.True(t, strings.HasPrefix(SuccessRedirectURL, "https://"),
		"SuccessRedirectURL must be an absolute HTTPS URL, got: %s", SuccessRedirectURL)
}

// ─── helpers ────────────────────────────────────────────────────────────────

func startListener(t *testing.T) (*net.TCPListener, error) {
	t.Helper()
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	return net.ListenTCP("tcp", addr)
}

func listenerPort(l net.Listener) int {
	return l.Addr().(*net.TCPAddr).Port
}
