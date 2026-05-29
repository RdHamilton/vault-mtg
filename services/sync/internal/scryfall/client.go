package scryfall

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const defaultBaseURL = "https://api.scryfall.com"

// scryfallUserAgent is sent on every request. Scryfall's API policy requires a
// descriptive User-Agent and Accept header on all requests.
const scryfallUserAgent = "VaultMTG-SyncLambda/1.0 (contact: support@vaultmtg.app)"

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

// setScryfallHeaders sets the User-Agent and Accept headers required by the
// Scryfall API on every outbound request.
func setScryfallHeaders(req *http.Request) {
	req.Header.Set("User-Agent", scryfallUserAgent)
	req.Header.Set("Accept", "application/json;q=0.9,*/*;q=0.8")
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

// bulkDataMeta is the JSON object returned by GET /bulk-data/default-cards.
// Only the fields needed to locate and decompress the actual bulk file are
// mapped; all other fields are ignored.
type bulkDataMeta struct {
	DownloadURI     string `json:"download_uri"`
	ContentEncoding string `json:"content_encoding"`
}

// FetchBulkDefaultCards fetches the Scryfall default-cards bulk-data file and
// returns all cards that carry a non-null arena_id.
//
// The Scryfall bulk-data endpoint uses a two-hop flow:
//  1. GET /bulk-data/default-cards → JSON metadata object containing download_uri.
//  2. GET download_uri → JSON array of card objects (~150 MB, optionally
//     gzip-encoded at the HTTP transport layer).
//
// The metadata GET uses the normal 30 s client timeout. A separate
// context.WithTimeout of 900 s is applied to the bulk download so it has
// enough wall-clock time without altering the shared *http.Client timeout.
//
// The bulk array is decoded with a streaming json.Decoder so the full body is
// never buffered in memory. If the download server sends Content-Encoding:
// gzip (e.g. when the caller disables Go's transparent decompression), the body
// is wrapped in compress/gzip.NewReader before decoding.
func (c *Client) FetchBulkDefaultCards(ctx context.Context) ([]ScryfallCard, error) {
	// --- Step 1: fetch the metadata object ---
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	u.Path += "/bulk-data/default-cards"

	metaReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build bulk-data metadata request: %w", err)
	}
	setScryfallHeaders(metaReq)

	metaResp, err := c.httpClient.Do(metaReq)
	if err != nil {
		return nil, fmt.Errorf("fetch bulk-data metadata: %w", err)
	}
	defer metaResp.Body.Close() //nolint:errcheck // network close errors on response bodies are not actionable

	if metaResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scryfall returned %d for /bulk-data/default-cards", metaResp.StatusCode)
	}

	var meta bulkDataMeta
	if err := json.NewDecoder(metaResp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("decode bulk-data metadata: %w", err)
	}

	if meta.DownloadURI == "" {
		return nil, fmt.Errorf("bulk-data metadata missing download_uri")
	}

	// --- Step 2: stream the bulk JSON array from download_uri ---
	dlCtx, cancel := context.WithTimeout(ctx, bulkDownloadTimeout)
	defer cancel()

	dlReq, err := http.NewRequestWithContext(dlCtx, http.MethodGet, meta.DownloadURI, nil)
	if err != nil {
		return nil, fmt.Errorf("build bulk-data download request: %w", err)
	}
	setScryfallHeaders(dlReq)

	dlResp, err := c.httpClient.Do(dlReq)
	if err != nil {
		return nil, fmt.Errorf("fetch bulk-data download: %w", err)
	}
	defer dlResp.Body.Close() //nolint:errcheck // network close errors on response bodies are not actionable

	if dlResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scryfall returned %d for bulk-data download", dlResp.StatusCode)
	}

	// Decompress if the server signals gzip encoding via the Content-Encoding
	// header. Go's http.Transport transparently decompresses gzip when it adds
	// Accept-Encoding automatically (the common case); in that situation the
	// header is stripped and we read a plain stream. When callers set
	// DisableCompression: true on the transport (e.g. tests), the header
	// remains and we must decompress ourselves.
	var reader io.Reader = dlResp.Body
	if dlResp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(dlResp.Body)
		if err != nil {
			return nil, fmt.Errorf("init gzip reader for bulk-data: %w", err)
		}
		defer gzReader.Close() //nolint:errcheck
		reader = gzReader
	}

	// The bulk file is a JSON array: [ {...}, {...}, ... ].
	// We stream it with json.Decoder to avoid loading 150 MB into memory.
	dec := json.NewDecoder(reader)

	// Consume the opening '['.
	openTok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("read bulk-data array open token: %w", err)
	}
	if delim, ok := openTok.(json.Delim); !ok || delim != '[' {
		return nil, fmt.Errorf("bulk-data download did not begin with '[', got %v", openTok)
	}

	var cards []ScryfallCard
	for dec.More() {
		var card ScryfallCard
		if err := dec.Decode(&card); err != nil {
			return nil, fmt.Errorf("decode bulk-data card: %w", err)
		}

		// Skip paper-only cards; only write Arena-tagged cards.
		if card.ArenaID == nil {
			continue
		}

		cards = append(cards, card)
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
	setScryfallHeaders(req)

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
