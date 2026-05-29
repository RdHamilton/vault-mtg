package scryfall

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const defaultBaseURL = "https://api.scryfall.com"

// ScryfallCard represents a single card object from the Scryfall bulk-data
// default-cards JSONL file. Only fields needed for the cards and set_cards
// upserts are mapped; unmapped fields are silently ignored during decode.
type ScryfallCard struct {
	ScryfallID      string   `json:"id"`
	ArenaID         *int     `json:"arena_id"`
	Name            string   `json:"name"`
	ManaCost        string   `json:"mana_cost"`
	CMC             float64  `json:"cmc"`
	TypeLine        string   `json:"type_line"`
	OracleText      string   `json:"oracle_text"`
	Colors          []string `json:"colors"`
	ColorIdentity   []string `json:"color_identity"`
	Rarity          string   `json:"rarity"`
	SetCode         string   `json:"set"`
	CollectorNumber string   `json:"collector_number"`
	Power           string   `json:"power"`
	Toughness       string   `json:"toughness"`
	Loyalty         string   `json:"loyalty"`
	Layout          string   `json:"layout"`
	ImageURIs       any      `json:"image_uris"`
	CardFaces       any      `json:"card_faces"`
	Legalities      any      `json:"legalities"`
	ReleasedAt      string   `json:"released_at"`
}

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

// bulkDownloadTimeout is applied via context.WithTimeout for the 150 MB
// bulk-data stream. The global *http.Client timeout of 30 s is too short
// to stream the full file, so a per-request context deadline is used instead
// without touching the shared client timeout. 900 s matches the Lambda max.
const bulkDownloadTimeout = 900 * time.Second

// FetchBulkDefaultCards fetches the Scryfall default-cards bulk-data file and
// returns all cards that carry a non-null arena_id. The bulk file is a JSONL
// stream (~150 MB) so the response body is scanned line-by-line rather than
// fully buffered.
//
// A per-request context.WithTimeout of 900 s is applied so the stream has
// enough wall-clock time to download without altering the shared *http.Client
// timeout (which remains 30 s and governs normal API calls).
func (c *Client) FetchBulkDefaultCards(ctx context.Context) ([]ScryfallCard, error) {
	dlCtx, cancel := context.WithTimeout(ctx, bulkDownloadTimeout)
	defer cancel()

	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	u.Path += "/bulk-data/default-cards"

	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build bulk-data request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch bulk-data: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // network close errors on response bodies are not actionable

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scryfall returned %d for /bulk-data/default-cards", resp.StatusCode)
	}

	var cards []ScryfallCard
	scanner := bufio.NewScanner(resp.Body)
	// Default scanner buffer is 64 KB; bulk-data lines can be several KB each.
	// 1 MB per line is more than sufficient.
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var card ScryfallCard
		if err := json.Unmarshal(line, &card); err != nil {
			return nil, fmt.Errorf("decode bulk-data card: %w", err)
		}

		// Skip paper-only cards; only write Arena-tagged cards.
		if card.ArenaID == nil {
			continue
		}

		cards = append(cards, card)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan bulk-data: %w", err)
	}

	return cards, nil
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
	defer resp.Body.Close() //nolint:errcheck // network close errors on response bodies are not actionable

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
