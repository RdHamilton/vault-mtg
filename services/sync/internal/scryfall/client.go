package scryfall

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const defaultBaseURL = "https://api.scryfall.com"

// ScryfallSet represents a single set returned by the Scryfall /sets endpoint.
type ScryfallSet struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	ReleasedAt string `json:"released_at"`
	SetType    string `json:"set_type"`
	Digital    bool   `json:"digital"`
	CardCount  int    `json:"card_count"`
}

// setsResponse is the envelope returned by GET /sets.
type setsResponse struct {
	Data []ScryfallSet `json:"data"`
}

// Client fetches set metadata from the Scryfall API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient returns a Client using the default Scryfall base URL.
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

// FetchSets retrieves all Arena-playable expansion and core sets from Scryfall.
// It filters to sets where set_type is "expansion" or "core" AND digital is true.
func (c *Client) FetchSets(ctx context.Context) ([]ScryfallSet, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	u.Path += "/sets"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch sets: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scryfall returned %d for /sets", resp.StatusCode)
	}

	var envelope setsResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode sets: %w", err)
	}

	var result []ScryfallSet
	for _, s := range envelope.Data {
		if s.Digital && (s.SetType == "expansion" || s.SetType == "core") {
			result = append(result, s)
		}
	}

	return result, nil
}
