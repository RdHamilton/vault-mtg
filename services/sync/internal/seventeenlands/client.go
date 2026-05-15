package seventeenlands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://www.17lands.com"

// Default retry/backoff configuration for the client-level rate-limit retry loop.
// These are intentionally smaller than the handler-level retries (2s→4s→8s) because
// they target 429/5xx transients specifically — not permanent data-fetch failures.
const (
	defaultClientMaxRetries  = 3   // total attempts (1 initial + 2 retries)
	defaultClientBaseBackoff = 500 // milliseconds for attempt 1
)

// Client fetches card rating data from the 17Lands API.
type Client struct {
	baseURL     string
	httpClient  *http.Client
	maxRetries  int
	baseBackoff time.Duration // backoff for attempt 1; doubles each subsequent attempt
}

// retryableStatus returns true for HTTP status codes that should be retried:
// 429 Too Many Requests and any 5xx server error.
func retryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

// NewClient returns a Client using the default 17Lands base URL.
// Retry limits are read from CLIENT_MAX_RETRIES and CLIENT_BASE_BACKOFF_MS env vars;
// defaults are 3 attempts and 500 ms base backoff respectively.
func NewClient() *Client {
	maxRetries := defaultClientMaxRetries
	if v := os.Getenv("CLIENT_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			maxRetries = n
		}
	}

	baseBackoff := time.Duration(defaultClientBaseBackoff) * time.Millisecond
	if v := os.Getenv("CLIENT_BASE_BACKOFF_MS"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			baseBackoff = time.Duration(n) * time.Millisecond
		}
	}

	return &Client{
		baseURL:     defaultBaseURL,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		maxRetries:  maxRetries,
		baseBackoff: baseBackoff,
	}
}

// NewClientWithBase returns a Client pointed at a custom base URL (useful for tests).
// maxRetries and baseBackoff are left at their zero values — callers that need retry
// control in tests should use NewClientWithOptions.
func NewClientWithBase(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:     baseURL,
		httpClient:  httpClient,
		maxRetries:  0,
		baseBackoff: 0,
	}
}

// NewClientWithOptions returns a fully configurable Client. Intended for tests.
func NewClientWithOptions(baseURL string, httpClient *http.Client, maxRetries int, baseBackoff time.Duration) *Client {
	return &Client{
		baseURL:     baseURL,
		httpClient:  httpClient,
		maxRetries:  maxRetries,
		baseBackoff: baseBackoff,
	}
}

// doWithRetry executes req, retrying on 429 or 5xx up to c.maxRetries total attempts.
// The response body is returned only on a successful (non-retryable) status. The caller
// is responsible for closing the body.
func (c *Client) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
	)

	for attempt := 0; attempt < c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := c.baseBackoff * (1 << uint(attempt-1)) //nolint:gomnd
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		// Clone request for each attempt so body (if any) can be re-read.
		// GET requests have no body, so this is safe here.
		resp, err = c.httpClient.Do(req.Clone(req.Context()))
		if err != nil {
			// Network error: retry immediately without consuming body.
			continue
		}

		if !retryableStatus(resp.StatusCode) {
			// Terminal (success or client error): return as-is.
			return resp, nil
		}

		// Retryable status — drain and close body before the next attempt.
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}

	if err != nil {
		return nil, err
	}
	// Last attempt returned a retryable status — return it so the caller can log
	// the exact status code.
	return resp, nil
}

// FetchCardRatings retrieves card ratings for the given set and draft format.
func (c *Client) FetchCardRatings(ctx context.Context, setCode, format string) ([]CardRating, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	u.Path += "/card_ratings/data"
	q := u.Query()
	q.Set("expansion", setCode)
	q.Set("format", format)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("fetch ratings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("17lands returned %d for set %s/%s", resp.StatusCode, setCode, format)
	}

	var ratings []CardRating
	if err := json.NewDecoder(resp.Body).Decode(&ratings); err != nil {
		return nil, fmt.Errorf("decode ratings: %w", err)
	}

	return ratings, nil
}

// FetchColorRatings retrieves per-color-combination win-rate data for the given
// set and draft format from the 17Lands /color_ratings/data endpoint.
func (c *Client) FetchColorRatings(ctx context.Context, setCode, format string) ([]ColorRating, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	u.Path += "/color_ratings/data"
	q := u.Query()
	q.Set("expansion", setCode)
	q.Set("format", format)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("fetch color ratings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("17lands returned %d for color ratings %s/%s", resp.StatusCode, setCode, format)
	}

	var ratings []ColorRating
	if err := json.NewDecoder(resp.Body).Decode(&ratings); err != nil {
		return nil, fmt.Errorf("decode color ratings: %w", err)
	}

	return ratings, nil
}

// do dispatches through the retry wrapper when maxRetries > 0, otherwise falls back
// to a single httpClient.Do call. This preserves behaviour for NewClientWithBase
// (tests that don't want retry) while NewClient and NewClientWithOptions get retry.
func (c *Client) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if c.maxRetries > 0 {
		return c.doWithRetry(ctx, req)
	}
	return c.httpClient.Do(req)
}
