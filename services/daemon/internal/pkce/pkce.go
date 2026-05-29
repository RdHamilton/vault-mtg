// Package pkce implements the PKCE (Proof Key for Code Exchange) OAuth browser-redirect
// login flow for the daemon.
//
// Flow summary (per ADR-020 §Decision):
//  1. Generate a random code_verifier and derive code_challenge (S256).
//  2. Bind a one-shot HTTP server on port 51423 (retry 51424 on failure).
//  3. Open the Clerk OAuth authorization URL in the system browser.
//  4. Receive the auth code on the callback server.
//  5. Exchange code + verifier for a Clerk session JWT.
//  6. Return the JWT to the caller for BFF registration.
//
// # Consent-loop prevention
//
// The callback handler responds with a 302 redirect to a VaultMTG success page
// instead of writing an HTML body. A redirect header (< 300 bytes) is fully
// flushed to the browser before the TCP connection closes, so the browser
// navigates away and never retries the consent request. Writing a full HTML body
// and then calling srv.Shutdown immediately caused the connection to be torn
// down mid-transfer, which made macOS browsers treat the response as incomplete
// and re-submit the OAuth request — creating the observed consent loop.
//
// The package is CGO-free and cross-compiles cleanly for darwin/amd64,
// darwin/arm64, and windows/amd64.
package pkce

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	// PrimaryPort is the fixed PKCE callback port registered in Clerk.
	PrimaryPort = 51423

	// FallbackPort is the retry port when PrimaryPort is busy.
	FallbackPort = 51424

	// CallbackPath is the URI path that Clerk redirects to after auth.
	CallbackPath = "/oauth/callback"

	// SuccessRedirectURL is where the browser is sent after a successful OAuth
	// callback. Using a redirect (302) instead of an inline HTML body ensures
	// the short redirect response is fully flushed before the callback server
	// shuts down, preventing the browser from treating a mid-transfer TCP close
	// as a request failure and re-submitting the OAuth consent request.
	SuccessRedirectURL = "https://vaultmtg.app/auth/success"

	// codeVerifierBytes is the length of the random code_verifier in bytes.
	// base64url(32 bytes) = 43 characters, well within OAuth 43–128 char limit.
	codeVerifierBytes = 32

	// callbackTimeout is how long to wait for the browser to complete OAuth.
	callbackTimeout = 5 * time.Minute

	// serverShutdownTimeout is the graceful shutdown window for the callback server.
	// Must be long enough for any in-flight HTTP response to be fully flushed.
	serverShutdownTimeout = 5 * time.Second

	// responseFlushDelay is the time the callback handler waits after writing
	// its response before signalling codeCh. This gives the OS network stack
	// time to flush the response to the browser before srv.Shutdown tears down
	// the listener. Without this delay, a 302 redirect can be lost when the
	// server closes the connection immediately after the handler returns.
	responseFlushDelay = 100 * time.Millisecond
)

// Config holds the Clerk OAuth parameters needed to build the authorization URL.
type Config struct {
	// ClerkFrontendAPI is the Clerk frontend API base URL,
	// e.g. "https://accounts.your-app.clerk.accounts.dev".
	ClerkFrontendAPI string

	// ClientID is the Clerk OAuth client_id (publishable key).
	ClientID string

	// Scopes is the list of OAuth scopes to request.
	// Defaults to ["profile", "email"] when nil.
	Scopes []string

	// TokenEndpoint is the Clerk token exchange endpoint.
	// e.g. ClerkFrontendAPI + "/oauth/token"
	TokenEndpoint string
}

// TokenResponse is the result of a successful PKCE flow.
type TokenResponse struct {
	// AccessToken is the Clerk OAuth access token returned after token exchange.
	AccessToken string

	// IDToken is the OIDC id_token returned alongside the access token.
	// Used for proving identity to the BFF when the BFF middleware expects
	// an OIDC-shaped JWT rather than an OAuth access token.
	IDToken string

	// RefreshToken may be present on some Clerk configurations.
	RefreshToken string
}

// ErrTokenExchange is returned (wrapped) when the Clerk token endpoint rejects the
// authorization code exchange (e.g. HTTP 4xx "invalid_grant"). Callers in the daemon
// package detect this via errors.Is and map it to the "pkce_token_exchange_failed"
// reason code for daemon.auth_failed PostHog events. Using a sentinel instead of a
// strings.Contains check makes the taxonomy contract explicit and immune to error-string
// drift across Clerk API versions.
//
// Commit cb4a4c15 [#88] established the reason-code taxonomy (pkce_cancelled,
// pkce_timeout); this sentinel extends it with a third code.
var ErrTokenExchange = errors.New("pkce: token exchange failed")

// Run executes the full PKCE browser-redirect flow and returns the Clerk session JWT.
// It opens the system browser, waits for the OAuth callback, and exchanges the
// auth code for a token.
//
// ctx governs the overall flow — cancelling it aborts the browser wait.
// headless controls whether the browser is opened (false) or only the URL is printed (true).
func Run(ctx context.Context, cfg Config, headless bool) (*TokenResponse, error) {
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, fmt.Errorf("pkce: generate verifier: %w", err)
	}

	port, listeners, err := bindCallbackPort()
	if err != nil {
		return nil, fmt.Errorf("pkce: bind callback port: %w", err)
	}

	redirectURI := fmt.Sprintf("http://localhost:%d%s", port, CallbackPath)

	authURL, err := buildAuthURL(cfg, challenge, redirectURI)
	if err != nil {
		for _, l := range listeners {
			_ = l.Close()
		}
		return nil, fmt.Errorf("pkce: build auth URL: %w", err)
	}

	if headless {
		fmt.Printf("[mtga-daemon] Open this URL to authenticate: %s\n", authURL)
	} else {
		if err := OpenBrowser(authURL); err != nil {
			// Non-fatal: print URL as fallback.
			log.Printf("[pkce] warn: could not open browser (%v); open this URL manually: %s", err, authURL)
			fmt.Printf("[mtga-daemon] Open this URL to authenticate: %s\n", authURL)
		}
	}

	// Wait for the callback server to receive the auth code.
	code, err := waitForCode(ctx, listeners)
	if err != nil {
		return nil, fmt.Errorf("pkce: wait for callback: %w", err)
	}

	// Exchange code + verifier for a token.
	// Wrap ErrTokenExchange as the first %w so callers can detect token-exchange
	// failures via errors.Is(err, pkce.ErrTokenExchange) regardless of wrapping
	// depth. The original exchange error is preserved as the second %w for context.
	tok, err := exchangeCode(ctx, cfg.TokenEndpoint, cfg.ClientID, code, verifier, redirectURI)
	if err != nil {
		return nil, fmt.Errorf("pkce: token exchange: %w: %w", ErrTokenExchange, err)
	}

	return tok, nil
}

// generatePKCE creates a random code_verifier and derives the S256 code_challenge.
func generatePKCE() (verifier, challenge string, err error) {
	buf := make([]byte, codeVerifierBytes)
	if _, err = rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("rand: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

// bindCallbackPort tries PrimaryPort then FallbackPort.
// For each candidate port it attempts to bind both tcp4 (127.0.0.1) and tcp6
// ([::1]) so the callback server accepts connections from browsers regardless
// of which loopback address the OS resolves "localhost" to.  macOS resolves
// "localhost" to ::1 first; browsers do not fall back to IPv4 on
// Connection Refused, so an IPv4-only bind causes auth to time out in the browser.
func bindCallbackPort() (port int, listeners []net.Listener, err error) {
	for _, p := range []int{PrimaryPort, FallbackPort} {
		var bound []net.Listener
		if l4, e := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", p)); e == nil {
			bound = append(bound, l4)
		}
		if l6, e := net.Listen("tcp6", fmt.Sprintf("[::1]:%d", p)); e == nil {
			bound = append(bound, l6)
		}
		if len(bound) > 0 {
			return p, bound, nil
		}
	}
	return 0, nil, fmt.Errorf("could not bind on ports %d or %d (tried tcp4+tcp6)", PrimaryPort, FallbackPort)
}

// buildAuthURL constructs the Clerk OAuth authorization URL.
func buildAuthURL(cfg Config, challenge, redirectURI string) (string, error) {
	if cfg.ClerkFrontendAPI == "" {
		return "", errors.New("ClerkFrontendAPI is required")
	}
	if cfg.ClientID == "" {
		return "", errors.New("ClientID is required")
	}

	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{"profile", "email"}
	}

	base := strings.TrimRight(cfg.ClerkFrontendAPI, "/") + "/oauth/authorize"
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {cfg.ClientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {strings.Join(scopes, " ")},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	return base + "?" + params.Encode(), nil
}

// waitForCode starts the one-shot callback server on every listener (IPv4 and
// IPv6) and waits until it receives the OAuth authorization code, ctx is
// cancelled, or callbackTimeout elapses.  A single http.Server is shared
// across all listeners so the first arriving code wins regardless of which
// network path the browser used.
func waitForCode(ctx context.Context, listeners []net.Listener) (string, error) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(CallbackPath, func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		errParam := r.URL.Query().Get("error")

		if errParam != "" {
			desc := r.URL.Query().Get("error_description")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, "<html><body><h2>Authentication failed</h2><p>%s: %s</p><p>You may close this tab.</p></body></html>",
				errParam, desc)
			select {
			case errCh <- fmt.Errorf("oauth error %s: %s", errParam, desc):
			default:
			}
			return
		}
		if code == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, "<html><body><h2>Authentication failed</h2><p>No code received.</p></body></html>")
			select {
			case errCh <- errors.New("oauth callback: missing code parameter"):
			default:
			}
			return
		}

		// Respond with a 302 redirect to the VaultMTG success page. A redirect
		// consists only of response headers (no body), which the OS flushes
		// immediately. The browser navigates away before we call srv.Shutdown,
		// so it never retries the OAuth request and the consent loop cannot occur.
		http.Redirect(w, r, SuccessRedirectURL, http.StatusFound)

		// Give the network stack a moment to deliver the redirect response to the
		// browser before signalling codeCh, which causes the deferred srv.Shutdown
		// to fire.  100 ms is imperceptible to the user and is orders of magnitude
		// longer than a loopback RTT.
		time.Sleep(responseFlushDelay)

		select {
		case codeCh <- code:
		default:
		}
	})

	srv := &http.Server{Handler: mux}

	// Start one Serve goroutine per listener. Both IPv4 and IPv6 listeners
	// share the same mux and codeCh — whichever the browser hits first wins.
	for _, l := range listeners {
		go func(l net.Listener) {
			if serveErr := srv.Serve(l); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				select {
				case errCh <- fmt.Errorf("callback server: %w", serveErr):
				default:
				}
			}
		}(l)
	}

	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	timeoutCtx, cancel := context.WithTimeout(ctx, callbackTimeout)
	defer cancel()

	select {
	case code := <-codeCh:
		return code, nil
	case err := <-errCh:
		return "", err
	case <-timeoutCtx.Done():
		if errors.Is(ctx.Err(), context.Canceled) {
			return "", context.Canceled
		}
		return "", fmt.Errorf("pkce: timed out waiting for OAuth callback (5 min)")
	}
}

// exchangeCode exchanges the authorization code for a Clerk session token.
func exchangeCode(ctx context.Context, tokenEndpoint, clientID, code, verifier, redirectURI string) (*TokenResponse, error) {
	if tokenEndpoint == "" {
		return nil, errors.New("tokenEndpoint is required")
	}

	body := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {redirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint,
		strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(respBody))
	}

	tok, err := parseTokenResponse(respBody)
	if err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}
	return tok, nil
}

// tokenResponseJSON is the raw token endpoint JSON structure.
type tokenResponseJSON struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// parseTokenResponse parses the JSON token response from Clerk.
func parseTokenResponse(body []byte) (*TokenResponse, error) {
	var raw tokenResponseJSON
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	if raw.Error != "" {
		return nil, fmt.Errorf("token error %s: %s", raw.Error, raw.ErrorDesc)
	}

	token := raw.AccessToken
	if token == "" {
		token = raw.IDToken // some Clerk configs return id_token
	}
	if token == "" {
		return nil, errors.New("no access_token in token response")
	}

	return &TokenResponse{
		AccessToken:  token,
		IDToken:      raw.IDToken,
		RefreshToken: raw.RefreshToken,
	}, nil
}

// OpenBrowser opens the given URL in the platform default browser.
// Uses os-level commands: "open" on macOS, "start" on Windows, "xdg-open" on Linux.
// Exported for testing.
func OpenBrowser(urlStr string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", urlStr)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", urlStr)
	default:
		// Linux / other — try xdg-open.
		cmd = exec.Command("xdg-open", urlStr)
	}
	return cmd.Start()
}
