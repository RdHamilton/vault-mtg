package seventeenlands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const defaultBaseURL = "https://www.17lands.com"

// Client fetches card rating data from the 17Lands API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient returns a Client using the default 17Lands base URL.
func NewClient() *Client {
	return &Client{
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// NewClientWithBase returns a Client pointed at a custom base URL (useful for tests).
func NewClientWithBase(baseURL string, httpClient *http.Client) *Client {
	return &Client{baseURL: baseURL, httpClient: httpClient}
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

	resp, err := c.httpClient.Do(req)
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

	resp, err := c.httpClient.Do(req)
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
