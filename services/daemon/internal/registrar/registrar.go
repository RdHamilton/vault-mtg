// Package registrar handles daemon registration with the BFF.
//
// On first run (or when the stored JWT is missing/near-expiry) the daemon calls
// POST /api/daemon/register using the user API key as the Bearer token.  The BFF
// returns a signed JWT and a daemon UUID that are then persisted to the local
// config file and used for all subsequent ingest calls.
package registrar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	// registerPath is the BFF endpoint that issues daemon JWTs.
	registerPath = "/api/daemon/register"

	// defaultTimeout is the HTTP timeout for the registration call.
	defaultTimeout = 15 * time.Second
)

// RegisterRequest is the body sent to POST /api/daemon/register.
type RegisterRequest struct {
	UserID int `json:"user_id"`
}

// RegisterResponse is the body returned by POST /api/daemon/register on success.
type RegisterResponse struct {
	Token    string `json:"token"`
	DaemonID string `json:"daemon_id"`
}

// ErrHTTP is returned when the BFF responds with a non-2xx status code.
type ErrHTTP struct {
	StatusCode int
	Body       string
}

func (e *ErrHTTP) Error() string {
	return fmt.Sprintf("BFF registration returned HTTP %d: %s", e.StatusCode, e.Body)
}

// Client performs daemon registration against the BFF.
type Client struct {
	bffBaseURL string
	httpClient *http.Client
}

// NewClient creates a Client that calls bffBaseURL for registration.
// bffBaseURL must not end with a trailing slash.
func NewClient(bffBaseURL string) *Client {
	return &Client{
		bffBaseURL: bffBaseURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// NewClientWithHTTP creates a Client using the provided *http.Client.
// Intended for testing — production code should use NewClient.
func NewClientWithHTTP(bffBaseURL string, hc *http.Client) *Client {
	return &Client{
		bffBaseURL: bffBaseURL,
		httpClient: hc,
	}
}

// Register calls POST /api/daemon/register with userID as the payload.
// apiKey is sent as the Bearer token (user API key, not daemon JWT).
// Returns the RegisterResponse containing the daemon JWT and UUID on success.
func (c *Client) Register(ctx context.Context, apiKey string, userID int) (*RegisterResponse, error) {
	reqBody, err := json.Marshal(RegisterRequest{UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("registrar: marshal request: %w", err)
	}

	url := c.bffBaseURL + registerPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("registrar: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("registrar: network error: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read body for both success and error paths.
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("registrar: read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &ErrHTTP{
			StatusCode: resp.StatusCode,
			Body:       buf.String(),
		}
	}

	var result RegisterResponse
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("registrar: decode response: %w", err)
	}
	if result.Token == "" {
		return nil, fmt.Errorf("registrar: BFF returned empty token")
	}
	if result.DaemonID == "" {
		return nil, fmt.Errorf("registrar: BFF returned empty daemon_id")
	}

	return &result, nil
}
