package mtgjson

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	// baseURL is the MTGJSON API base URL.
	baseURL = "https://mtgjson.com/api/v5"

	// rateLimitDelay is the minimum delay between requests.
	// MTGJSON is generous but we still want to be respectful.
	rateLimitDelay = 200 * time.Millisecond

	// requestTimeout is the maximum time to wait for a response.
	requestTimeout = 60 * time.Second

	// maxRetries is the number of retry attempts for failed requests.
	maxRetries = 3

	// initialBackoff is the initial backoff duration for retries.
	initialBackoff = 1 * time.Second

	// maxBackoff is the maximum backoff duration for retries.
	maxBackoff = 16 * time.Second
)

// Client is an HTTP client for the MTGJSON API.
type Client struct {
	httpClient  *http.Client
	rateLimiter *rate.Limiter
	userAgent   string
	baseURL     string // configurable for testing, defaults to baseURL constant
}

// NewClient creates a new MTGJSON API client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
		// Rate limiter: 1 request per 200ms = 5 req/sec
		rateLimiter: rate.NewLimiter(rate.Every(rateLimitDelay), 1),
		userAgent:   "MTGA-Companion/1.0",
		baseURL:     baseURL,
	}
}

// getBaseURL returns the base URL, using the default if not set.
func (c *Client) getBaseURL() string {
	if c.baseURL != "" {
		return c.baseURL
	}
	return baseURL
}

// GetSet fetches a complete set file from MTGJSON.
// Example URL: https://mtgjson.com/api/v5/ECL.json
func (c *Client) GetSet(ctx context.Context, setCode string) (*SetFile, error) {
	// MTGJSON uses uppercase set codes
	setCode = strings.ToUpper(setCode)
	url := fmt.Sprintf("%s/%s.json", c.getBaseURL(), setCode)

	log.Printf("[MTGJSON] Fetching set: %s", setCode)

	var setFile SetFile
	if err := c.doRequest(ctx, url, &setFile); err != nil {
		return nil, fmt.Errorf("failed to get set %s: %w", setCode, err)
	}

	log.Printf("[MTGJSON] Fetched set %s: %d cards", setCode, len(setFile.Data.Cards))
	return &setFile, nil
}

// GetSetCards fetches only the cards for a set (convenience method).
func (c *Client) GetSetCards(ctx context.Context, setCode string) ([]Card, error) {
	setFile, err := c.GetSet(ctx, setCode)
	if err != nil {
		return nil, err
	}
	return setFile.Data.Cards, nil
}

// GetSetCardsWithArenaIDs fetches cards that have MTG Arena IDs.
func (c *Client) GetSetCardsWithArenaIDs(ctx context.Context, setCode string) ([]Card, error) {
	cards, err := c.GetSetCards(ctx, setCode)
	if err != nil {
		return nil, err
	}

	// Filter to only cards with Arena IDs
	arenaCards := make([]Card, 0, len(cards))
	for _, card := range cards {
		if card.HasArenaID() {
			arenaCards = append(arenaCards, card)
		}
	}

	log.Printf("[MTGJSON] Set %s: %d/%d cards have Arena IDs", setCode, len(arenaCards), len(cards))
	return arenaCards, nil
}

// doRequest performs an HTTP request with rate limiting and retry logic.
func (c *Client) doRequest(ctx context.Context, url string, result interface{}) error {
	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Wait for rate limiter
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limiter error: %w", err)
		}

		// Create request
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		// Set headers
		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Accept", "application/json")

		// Execute request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)

			// Retry on network errors
			if attempt < maxRetries {
				log.Printf("[MTGJSON] Request failed (attempt %d/%d): %v, retrying in %v",
					attempt+1, maxRetries+1, err, backoff)
				time.Sleep(backoff)
				backoff = min(backoff*2, maxBackoff)
				continue
			}
			return lastErr
		}

		// Check status code
		switch resp.StatusCode {
		case http.StatusOK:
			// Success - parse response
			body, err := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if err != nil {
				return fmt.Errorf("failed to read response body: %w", err)
			}

			if err := json.Unmarshal(body, result); err != nil {
				return fmt.Errorf("failed to parse JSON response: %w", err)
			}

			return nil

		case http.StatusTooManyRequests:
			// Rate limited - exponential backoff
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("rate limited (HTTP 429)")

			if attempt < maxRetries {
				log.Printf("[MTGJSON] Rate limited (attempt %d/%d), retrying in %v",
					attempt+1, maxRetries+1, backoff)
				time.Sleep(backoff)
				backoff = min(backoff*2, maxBackoff)
				continue
			}
			return lastErr

		case http.StatusNotFound:
			_ = resp.Body.Close()
			return &NotFoundError{SetCode: url}

		default:
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// min returns the minimum of two durations.
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// NotFoundError represents a 404 error from the API.
type NotFoundError struct {
	SetCode string
}

// Error implements the error interface.
func (e *NotFoundError) Error() string {
	return fmt.Sprintf("set not found: %s", e.SetCode)
}

// IsNotFound returns true if the error is a NotFoundError.
func IsNotFound(err error) bool {
	_, ok := err.(*NotFoundError)
	return ok
}
