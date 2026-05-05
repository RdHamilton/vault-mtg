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

// isDraftableSetType reports whether the Scryfall set_type is eligible for Arena
// draft rating sync.  We include the five types that can appear in Arena draft
// queues:
//
//   - "expansion"        — normal booster sets (standard-legal)
//   - "core"             — core sets
//   - "masters"          — reprint / Masters sets released on Arena
//   - "draft_innovation" — chaos draft and special draft experiences
//   - "alchemy"          — Alchemy-specific supplemental sets
func isDraftableSetType(t string) bool {
	switch t {
	case "expansion", "core", "masters", "draft_innovation", "alchemy":
		return true
	}
	return false
}

// FetchSets retrieves all Arena-playable draft sets from Scryfall.
// It filters to sets where digital is true AND set_type is one of the
// draft-eligible types (expansion, core, masters, draft_innovation, alchemy).
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
		if s.Digital && isDraftableSetType(s.SetType) {
			result = append(result, s)
		}
	}

	return result, nil
}
