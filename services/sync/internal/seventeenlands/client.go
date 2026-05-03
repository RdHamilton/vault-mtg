package seventeenlands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
		httpClient: &http.Client{},
	}
}

// NewClientWithBase returns a Client pointed at a custom base URL (useful for tests).
func NewClientWithBase(baseURL string, httpClient *http.Client) *Client {
	return &Client{baseURL: baseURL, httpClient: httpClient}
}

// FetchCardRatings retrieves card ratings for the given set and draft format.
func (c *Client) FetchCardRatings(ctx context.Context, setCode, format string) ([]CardRating, error) {
	url := fmt.Sprintf("%s/card_ratings/data?expansion=%s&format=%s", c.baseURL, setCode, format)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
