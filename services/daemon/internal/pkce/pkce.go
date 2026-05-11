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

	// codeVerifierBytes is the length of the random code_verifier in bytes.
	// base64url(32 bytes) = 43 characters, well within OAuth 43–128 char limit.
	codeVerifierBytes = 32

	// callbackTimeout is how long to wait for the browser to complete OAuth.
	callbackTimeout = 5 * time.Minute

	// serverShutdownTimeout is the graceful shutdown window for the callback server.
	serverShutdownTimeout = 5 * time.Second
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
	// AccessToken is the Clerk session JWT returned after token exchange.
	AccessToken string

	// RefreshToken may be present on some Clerk configurations.
	RefreshToken string
}

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

	port, listener, err := bindCallbackPort()
	if err != nil {
		return nil, fmt.Errorf("pkce: bind callback port: %w", err)
	}

	redirectURI := fmt.Sprintf("http://localhost:%d%s", port, CallbackPath)

	authURL, err := buildAuthURL(cfg, challenge, redirectURI)
	if err != nil {
		_ = listener.Close()
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
	code, err := waitForCode(ctx, listener)
	if err != nil {
		return nil, fmt.Errorf("pkce: wait for callback: %w", err)
	}

	// Exchange code + verifier for a token.
	tok, err := exchangeCode(ctx, cfg.TokenEndpoint, cfg.ClientID, code, verifier, redirectURI)
	if err != nil {
		return nil, fmt.Errorf("pkce: token exchange: %w", err)
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
func bindCallbackPort() (port int, l net.Listener, err error) {
	for _, p := range []int{PrimaryPort, FallbackPort} {
		l, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			return p, l, nil
		}
	}
	return 0, nil, fmt.Errorf("could not bind ports %d or %d: %w", PrimaryPort, FallbackPort, err)
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

// waitForCode starts the one-shot callback server and waits until it receives
// the OAuth authorization code, ctx is cancelled, or callbackTimeout elapses.
func waitForCode(ctx context.Context, l net.Listener) (string, error) {
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

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, "<html><body><h2>Authentication successful!</h2><p>You may close this tab and return to the app.</p></body></html>")
		select {
		case codeCh <- code:
		default:
		}
	})

	srv := &http.Server{Handler: mux}

	go func() {
		if serveErr := srv.Serve(l); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			select {
			case errCh <- fmt.Errorf("callback server: %w", serveErr):
			default:
			}
		}
	}()

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
