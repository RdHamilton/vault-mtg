package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewGoldfishClient(t *testing.T) {
	t.Run("with nil config uses defaults", func(t *testing.T) {
		client := NewGoldfishClient(nil)
		if client == nil {
			t.Fatal("expected non-nil client")
		}
		if client.baseURL != "https://www.mtggoldfish.com" {
			t.Errorf("expected default base URL, got %s", client.baseURL)
		}
		if client.cacheTTL != 4*time.Hour {
			t.Errorf("expected 4 hour cache TTL, got %v", client.cacheTTL)
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &GoldfishConfig{
			BaseURL:        "https://custom.url",
			CacheTTL:       2 * time.Hour,
			RequestTimeout: 10 * time.Second,
			RateLimitMs:    500,
		}
		client := NewGoldfishClient(config)
		if client.baseURL != "https://custom.url" {
			t.Errorf("expected custom base URL, got %s", client.baseURL)
		}
		if client.cacheTTL != 2*time.Hour {
			t.Errorf("expected 2 hour cache TTL, got %v", client.cacheTTL)
		}
	})
}

func TestDefaultGoldfishConfig(t *testing.T) {
	config := DefaultGoldfishConfig()

	if config.BaseURL != "https://www.mtggoldfish.com" {
		t.Errorf("unexpected BaseURL: %s", config.BaseURL)
	}
	if config.CacheTTL != 4*time.Hour {
		t.Errorf("unexpected CacheTTL: %v", config.CacheTTL)
	}
	if config.RequestTimeout != 30*time.Second {
		t.Errorf("unexpected RequestTimeout: %v", config.RequestTimeout)
	}
	if config.RateLimitMs != 1000 {
		t.Errorf("unexpected RateLimitMs: %d", config.RateLimitMs)
	}
}

func TestGoldfishClient_GetMeta(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return mock HTML with archetype data (current MTGGoldfish format)
		html := `
		<html>
		<body>
		<div class='archetype-tile' id='1'>
		<div class='archetype-tile-title'>
			<a href="/archetype/mono-red">Mono Red Aggro</a>
		</div>
		<div class='archetype-tile-statistic metagame-percentage'>
			<div class='archetype-tile-statistic-value'>15.5%</div>
		</div>
		</div>
		<div class='archetype-tile' id='2'>
		<div class='archetype-tile-title'>
			<a href="/archetype/azorius">Azorius Control</a>
		</div>
		<div class='archetype-tile-statistic metagame-percentage'>
			<div class='archetype-tile-statistic-value'>12.3%</div>
		</div>
		</div>
		<div class='archetype-tile' id='3'>
		<div class='archetype-tile-title'>
			<a href="/archetype/golgari">Golgari Midrange</a>
		</div>
		<div class='archetype-tile-statistic metagame-percentage'>
			<div class='archetype-tile-statistic-value'>8.7%</div>
		</div>
		</div>
		</body>
		</html>
		`
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	config := &GoldfishConfig{
		BaseURL:        server.URL,
		CacheTTL:       1 * time.Hour,
		RequestTimeout: 5 * time.Second,
		RateLimitMs:    10, // Fast rate limit for tests
	}
	client := NewGoldfishClient(config)

	ctx := context.Background()
	meta, err := client.GetMeta(ctx, "standard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta == nil {
		t.Fatal("expected non-nil meta")
	}
	if meta.Format != "standard" {
		t.Errorf("expected format 'standard', got %s", meta.Format)
	}
	if meta.Source != "mtggoldfish" {
		t.Errorf("expected source 'mtggoldfish', got %s", meta.Source)
	}
	if len(meta.Decks) != 3 {
		t.Errorf("expected 3 decks, got %d", len(meta.Decks))
	}
}

func TestGoldfishClient_GetMeta_UnsupportedFormat(t *testing.T) {
	config := &GoldfishConfig{
		BaseURL:        "https://test.url",
		CacheTTL:       1 * time.Hour,
		RequestTimeout: 5 * time.Second,
		RateLimitMs:    10,
	}
	client := NewGoldfishClient(config)

	ctx := context.Background()
	_, err := client.GetMeta(ctx, "unsupported")
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestGoldfishClient_GetTopDecks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
		<html>
		<body>
		<div class='archetype-tile' id='1'>
		<div class='archetype-tile-title'><a href="/archetype/deck1">Deck One</a></div>
		<div class='archetype-tile-statistic metagame-percentage'>
			<div class='archetype-tile-statistic-value'>20.0%</div>
		</div>
		</div>
		<div class='archetype-tile' id='2'>
		<div class='archetype-tile-title'><a href="/archetype/deck2">Deck Two</a></div>
		<div class='archetype-tile-statistic metagame-percentage'>
			<div class='archetype-tile-statistic-value'>15.0%</div>
		</div>
		</div>
		<div class='archetype-tile' id='3'>
		<div class='archetype-tile-title'><a href="/archetype/deck3">Deck Three</a></div>
		<div class='archetype-tile-statistic metagame-percentage'>
			<div class='archetype-tile-statistic-value'>10.0%</div>
		</div>
		</div>
		</body>
		</html>
		`
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	config := &GoldfishConfig{
		BaseURL:        server.URL,
		CacheTTL:       1 * time.Hour,
		RequestTimeout: 5 * time.Second,
		RateLimitMs:    10,
	}
	client := NewGoldfishClient(config)

	ctx := context.Background()
	decks, err := client.GetTopDecks(ctx, "standard", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(decks) != 2 {
		t.Errorf("expected 2 decks, got %d", len(decks))
	}
}

func TestGoldfishClient_GetDeckByArchetype(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
		<html>
		<body>
		<div class='archetype-tile' id='1'>
		<div class='archetype-tile-title'><a href="/archetype/mono-red">Mono Red Aggro</a></div>
		<div class='archetype-tile-statistic metagame-percentage'>
			<div class='archetype-tile-statistic-value'>15.0%</div>
		</div>
		</div>
		<div class='archetype-tile' id='2'>
		<div class='archetype-tile-title'><a href="/archetype/azorius">Azorius Control</a></div>
		<div class='archetype-tile-statistic metagame-percentage'>
			<div class='archetype-tile-statistic-value'>10.0%</div>
		</div>
		</div>
		</body>
		</html>
		`
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	config := &GoldfishConfig{
		BaseURL:        server.URL,
		CacheTTL:       1 * time.Hour,
		RequestTimeout: 5 * time.Second,
		RateLimitMs:    10,
	}
	client := NewGoldfishClient(config)

	ctx := context.Background()

	t.Run("found archetype", func(t *testing.T) {
		deck, err := client.GetDeckByArchetype(ctx, "standard", "mono red aggro")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if deck.Name != "Mono Red Aggro" {
			t.Errorf("expected 'Mono Red Aggro', got %s", deck.Name)
		}
	})

	t.Run("not found archetype", func(t *testing.T) {
		_, err := client.GetDeckByArchetype(ctx, "standard", "nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent archetype")
		}
	})
}

func TestGoldfishClient_GetMetaShare(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
		<html>
		<body>
		<div class='archetype-tile' id='1'>
		<div class='archetype-tile-title'><a href="/archetype/test">Test Deck</a></div>
		<div class='archetype-tile-statistic metagame-percentage'>
			<div class='archetype-tile-statistic-value'>25.5%</div>
		</div>
		</div>
		</body>
		</html>
		`
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	config := &GoldfishConfig{
		BaseURL:        server.URL,
		CacheTTL:       1 * time.Hour,
		RequestTimeout: 5 * time.Second,
		RateLimitMs:    10,
	}
	client := NewGoldfishClient(config)

	ctx := context.Background()
	share, err := client.GetMetaShare(ctx, "standard", "test deck")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if share != 25.5 {
		t.Errorf("expected meta share 25.5, got %f", share)
	}
}

func TestGoldfishClient_Cache(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		html := `
		<html>
		<body>
		<div class='archetype-tile' id='1'>
		<div class='archetype-tile-title'><a href="/archetype/cached">Cached Deck</a></div>
		<div class='archetype-tile-statistic metagame-percentage'>
			<div class='archetype-tile-statistic-value'>10.0%</div>
		</div>
		</div>
		</body>
		</html>
		`
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	config := &GoldfishConfig{
		BaseURL:        server.URL,
		CacheTTL:       1 * time.Hour,
		RequestTimeout: 5 * time.Second,
		RateLimitMs:    10,
	}
	client := NewGoldfishClient(config)

	ctx := context.Background()

	// First request should hit server
	_, err := client.GetMeta(ctx, "standard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestCount != 1 {
		t.Errorf("expected 1 request, got %d", requestCount)
	}

	// Second request should use cache
	_, err = client.GetMeta(ctx, "standard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestCount != 1 {
		t.Errorf("expected still 1 request (cached), got %d", requestCount)
	}
}

func TestGoldfishClient_ClearCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
		<html>
		<body>
		<div class='archetype-tile' id='1'>
		<div class='archetype-tile-title'><a href="/archetype/test">Test</a></div>
		<div class='archetype-tile-statistic metagame-percentage'>
			<div class='archetype-tile-statistic-value'>10.0%</div>
		</div>
		</div>
		</body>
		</html>
		`
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	config := &GoldfishConfig{
		BaseURL:        server.URL,
		CacheTTL:       1 * time.Hour,
		RequestTimeout: 5 * time.Second,
		RateLimitMs:    10,
	}
	client := NewGoldfishClient(config)

	ctx := context.Background()

	// Populate cache
	_, _ = client.GetMeta(ctx, "standard")

	// Check cache status
	cached, _ := client.GetCacheStatus("standard")
	if !cached {
		t.Error("expected cache to be populated")
	}

	// Clear cache
	client.ClearCache()

	// Check cache status after clear
	cached, _ = client.GetCacheStatus("standard")
	if cached {
		t.Error("expected cache to be cleared")
	}
}

func TestGoldfishClient_RefreshMeta(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		html := `
		<html>
		<body>
		<div class='archetype-tile' id='1'>
		<div class='archetype-tile-title'><a href="/archetype/refresh">Refreshed Deck</a></div>
		<div class='archetype-tile-statistic metagame-percentage'>
			<div class='archetype-tile-statistic-value'>10.0%</div>
		</div>
		</div>
		</body>
		</html>
		`
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	config := &GoldfishConfig{
		BaseURL:        server.URL,
		CacheTTL:       1 * time.Hour,
		RequestTimeout: 5 * time.Second,
		RateLimitMs:    10,
	}
	client := NewGoldfishClient(config)

	ctx := context.Background()

	// First request
	_, _ = client.GetMeta(ctx, "standard")

	// Refresh should bypass cache
	_, err := client.RefreshMeta(ctx, "standard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestCount != 2 {
		t.Errorf("expected 2 requests after refresh, got %d", requestCount)
	}
}

func TestGoldfishClient_ExtractColorsFromName(t *testing.T) {
	client := NewGoldfishClient(nil)

	tests := []struct {
		name     string
		expected []string
	}{
		{"Mono Red Aggro", []string{"R"}},
		{"Azorius Control", []string{"W", "U"}},
		{"Golgari Midrange", []string{"B", "G"}},
		{"Esper Control", []string{"W", "U", "B"}},
		{"Jund Sacrifice", []string{"B", "R", "G"}},
		{"Unknown Deck", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			colors := client.extractColorsFromName(tt.name)
			if len(colors) != len(tt.expected) {
				t.Errorf("expected %d colors, got %d", len(tt.expected), len(colors))
				return
			}
			for i, c := range tt.expected {
				if colors[i] != c {
					t.Errorf("expected color %s at position %d, got %s", c, i, colors[i])
				}
			}
		})
	}
}

func TestGoldfishClient_NormalizeArchetypeName(t *testing.T) {
	client := NewGoldfishClient(nil)

	tests := []struct {
		input    string
		expected string
	}{
		{"Standard Mono Red", "mono red"},
		{"Historic Control", "control"},
		{"  Trimmed Name  ", "trimmed name"},
		{"Pioneer Aggro", "aggro"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := client.normalizeArchetypeName(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestFormatMeta_Serialize(t *testing.T) {
	meta := &FormatMeta{
		Format: "standard",
		Decks: []*MetaDeck{
			{
				Name:      "Test Deck",
				MetaShare: 15.5,
			},
		},
		TotalDecks:  1,
		LastUpdated: time.Now(),
		Source:      "mtggoldfish",
	}

	data, err := meta.Serialize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty serialized data")
	}

	// Deserialize and verify
	restored, err := DeserializeFormatMeta(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if restored.Format != meta.Format {
		t.Errorf("expected format %s, got %s", meta.Format, restored.Format)
	}
	if len(restored.Decks) != 1 {
		t.Errorf("expected 1 deck, got %d", len(restored.Decks))
	}
}

func TestDeserializeFormatMeta_InvalidJSON(t *testing.T) {
	_, err := DeserializeFormatMeta([]byte("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestGoldfishClient_ParseMetaPageTableFormat(t *testing.T) {
	client := NewGoldfishClient(nil)

	html := `
	<html>
	<body>
	<table>
	<tr>
		<td><a href="/archetype/mono-red#paper">Mono Red</a></td>
		<td>18.5%</td>
	</tr>
	<tr>
		<td><a href="/archetype/control#paper">Blue Control</a></td>
		<td>12.3%</td>
	</tr>
	</table>
	</body>
	</html>
	`

	meta := client.parseMetaPage(html, "standard")

	if len(meta.Decks) < 2 {
		t.Errorf("expected at least 2 decks from table format, got %d", len(meta.Decks))
	}
}

func TestGoldfishClient_GetCacheStatus_NotCached(t *testing.T) {
	client := NewGoldfishClient(nil)

	cached, expiresAt := client.GetCacheStatus("nonexistent")
	if cached {
		t.Error("expected not cached")
	}
	if !expiresAt.IsZero() {
		t.Error("expected zero time for nonexistent cache entry")
	}
}

func TestGoldfishClient_ParseMetaPageCurrentFormat(t *testing.T) {
	// Test with actual current MTGGoldfish HTML structure (as of 2024)
	client := NewGoldfishClient(nil)

	// This HTML matches the real MTGGoldfish structure with single quotes
	html := `
<div class='archetype-tile' id='28086'>
<div class='archetype-tile-image'>
<div aria-label='Image of Kaito' class='card-tile' role='img'>
</div>
</div>
<div class='archetype-tile-description-wrapper'>
<div class='archetype-tile-description'>
<div class='archetype-tile-title'>
<span class='deck-price-online'>
<a href="/archetype/standard-dimir-midrange-woe#online">Dimir Midrange</a>
</span>
</div>
</div>
<div class='archetype-tile-statistics'>
<div class='archetype-tile-statistics-left'>
<div class='archetype-tile-statistic metagame-percentage'>
<div class='archetype-tile-statistic-name'>META%</div>
<div class='archetype-tile-statistic-value'>
21.3%
<span class='archetype-tile-statistic-value-extra-data'>(385)</span>
</div>
</div>
</div>
</div>
</div>
</div>

<div class='archetype-tile' id='28249'>
<div class='archetype-tile-description-wrapper'>
<div class='archetype-tile-description'>
<div class='archetype-tile-title'>
<span class='deck-price-online'>
<a href="/archetype/standard-simic-aggro-woe#online">Simic Aggro</a>
</span>
</div>
</div>
<div class='archetype-tile-statistics'>
<div class='archetype-tile-statistics-left'>
<div class='archetype-tile-statistic metagame-percentage'>
<div class='archetype-tile-statistic-name'>META%</div>
<div class='archetype-tile-statistic-value'>
11.2%
</div>
</div>
</div>
</div>
</div>
</div>

<div class='archetype-tile' id='28080'>
<div class='archetype-tile-description-wrapper'>
<div class='archetype-tile-description'>
<div class='archetype-tile-title'>
<span class='deck-price-online'>
<a href="/archetype/standard-jeskai-control-woe#online">Jeskai Control</a>
</span>
</div>
</div>
<div class='archetype-tile-statistics'>
<div class='archetype-tile-statistics-left'>
<div class='archetype-tile-statistic metagame-percentage'>
<div class='archetype-tile-statistic-value'>
8.5%
</div>
</div>
</div>
</div>
</div>
</div>
`

	meta := client.parseMetaPage(html, "standard")

	if meta == nil {
		t.Fatal("Expected meta to not be nil")
	}

	if meta.Format != "standard" {
		t.Errorf("Expected format 'standard', got '%s'", meta.Format)
	}

	if len(meta.Decks) == 0 {
		t.Fatal("Expected at least one deck to be parsed from current HTML format")
	}

	t.Logf("Parsed %d decks from current format HTML", len(meta.Decks))
	for _, deck := range meta.Decks {
		t.Logf("  - %s: %.1f%%", deck.Name, deck.MetaShare)
	}

	// Verify we got the expected decks
	expectedDecks := map[string]float64{
		"Dimir Midrange": 21.3,
		"Simic Aggro":    11.2,
		"Jeskai Control": 8.5,
	}

	for _, deck := range meta.Decks {
		if expectedShare, ok := expectedDecks[deck.Name]; ok {
			if deck.MetaShare != expectedShare {
				t.Errorf("Expected %s meta share %.1f, got %.1f", deck.Name, expectedShare, deck.MetaShare)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// New parse-focused tests (#175 AC4: >=10 parse tests for the Goldfish scraper).
// All use parseMetaPage directly or httptest.Server -- zero live network.
// ---------------------------------------------------------------------------

// TestGoldfishParse_SingleArchetypeTile verifies a minimal single-tile page
// parses one deck with the correct name and share.
func TestGoldfishParse_SingleArchetypeTile(t *testing.T) {
	client := NewGoldfishClient(nil)
	html := `
	<div class='archetype-tile' id='1'>
	<div class='archetype-tile-title'><a href="/archetype/x">Boros Energy</a></div>
	<div class='archetype-tile-statistic metagame-percentage'>
	<div class='archetype-tile-statistic-value'>9.4%</div>
	</div>
	</div>`
	meta := client.parseMetaPage(html, "standard")
	if len(meta.Decks) != 1 {
		t.Fatalf("expected 1 deck, got %d", len(meta.Decks))
	}
	if meta.Decks[0].Name != "Boros Energy" {
		t.Errorf("expected name 'Boros Energy', got %q", meta.Decks[0].Name)
	}
	if meta.Decks[0].MetaShare != 9.4 {
		t.Errorf("expected share 9.4, got %v", meta.Decks[0].MetaShare)
	}
}

// TestGoldfishParse_TierAssignment confirms the >5/>2/>0.5 thresholds map to
// tiers 1/2/3/4 respectively.
func TestGoldfishParse_TierAssignment(t *testing.T) {
	client := NewGoldfishClient(nil)
	html := `
	<div class='archetype-tile' id='1'><div class='archetype-tile-title'><a href="/a">T1 Deck</a></div><div class='archetype-tile-statistic-value'>8.0%</div></div>
	<div class='archetype-tile' id='2'><div class='archetype-tile-title'><a href="/a">T2 Deck</a></div><div class='archetype-tile-statistic-value'>3.0%</div></div>
	<div class='archetype-tile' id='3'><div class='archetype-tile-title'><a href="/a">T3 Deck</a></div><div class='archetype-tile-statistic-value'>1.0%</div></div>
	<div class='archetype-tile' id='4'><div class='archetype-tile-title'><a href="/a">T4 Deck</a></div><div class='archetype-tile-statistic-value'>0.1%</div></div>`
	meta := client.parseMetaPage(html, "standard")
	byName := map[string]int{}
	for _, d := range meta.Decks {
		byName[d.Name] = d.Tier
	}
	cases := map[string]int{"T1 Deck": 1, "T2 Deck": 2, "T3 Deck": 3, "T4 Deck": 4}
	for name, wantTier := range cases {
		if byName[name] != wantTier {
			t.Errorf("%s: expected tier %d, got %d", name, wantTier, byName[name])
		}
	}
}

// TestGoldfishParse_DoubleQuotedClasses verifies the regex handles double-quoted
// class attributes (the ['\"] alternation).
func TestGoldfishParse_DoubleQuotedClasses(t *testing.T) {
	client := NewGoldfishClient(nil)
	html := `
	<div class="archetype-tile" id="1">
	<div class="archetype-tile-title"><a href="/archetype/y">Gruul Aggro</a></div>
	<div class="archetype-tile-statistic metagame-percentage">
	<div class="archetype-tile-statistic-value">7.2%</div>
	</div>
	</div>`
	meta := client.parseMetaPage(html, "standard")
	if len(meta.Decks) != 1 || meta.Decks[0].Name != "Gruul Aggro" {
		t.Fatalf("expected Gruul Aggro from double-quoted HTML, got %+v", meta.Decks)
	}
}

// TestGoldfishParse_IntegerPercentage verifies a share with no decimal point
// (e.g. "5%") parses correctly.
func TestGoldfishParse_IntegerPercentage(t *testing.T) {
	client := NewGoldfishClient(nil)
	html := `
	<div class='archetype-tile' id='1'><div class='archetype-tile-title'><a href="/a">Mono White</a></div><div class='archetype-tile-statistic-value'>5%</div></div>`
	meta := client.parseMetaPage(html, "standard")
	if len(meta.Decks) != 1 {
		t.Fatalf("expected 1 deck, got %d", len(meta.Decks))
	}
	if meta.Decks[0].MetaShare != 5.0 {
		t.Errorf("expected share 5.0, got %v", meta.Decks[0].MetaShare)
	}
}

// TestGoldfishParse_EmptyPage returns zero decks for HTML with no archetypes.
func TestGoldfishParse_EmptyPage(t *testing.T) {
	client := NewGoldfishClient(nil)
	meta := client.parseMetaPage(`<html><body><p>No data</p></body></html>`, "standard")
	if len(meta.Decks) != 0 {
		t.Errorf("expected 0 decks for empty page, got %d", len(meta.Decks))
	}
	if meta.TotalDecks != 0 {
		t.Errorf("expected TotalDecks 0, got %d", meta.TotalDecks)
	}
}

// TestGoldfishParse_TableFallbackShareAndColors verifies the table fallback
// pattern extracts the name and that colors are derived from the name.
func TestGoldfishParse_TableFallbackShareAndColors(t *testing.T) {
	client := NewGoldfishClient(nil)
	html := `
	<table>
	<tr><td><a href="/archetype/izzet-phoenix#paper">Izzet Phoenix</a></td><td>14.0%</td></tr>
	</table>`
	meta := client.parseMetaPage(html, "modern")
	if len(meta.Decks) != 1 {
		t.Fatalf("expected 1 deck from table fallback, got %d", len(meta.Decks))
	}
	d := meta.Decks[0]
	if d.Name != "Izzet Phoenix" {
		t.Errorf("expected 'Izzet Phoenix', got %q", d.Name)
	}
	// Izzet -> UR
	if len(d.Colors) != 2 || d.Colors[0] != "U" || d.Colors[1] != "R" {
		t.Errorf("expected colors [U R] for Izzet, got %v", d.Colors)
	}
}

// TestGoldfishParse_FieldsPopulated verifies Format and ArchetypeName are set
// on each parsed deck.
func TestGoldfishParse_FieldsPopulated(t *testing.T) {
	client := NewGoldfishClient(nil)
	html := `
	<div class='archetype-tile' id='1'><div class='archetype-tile-title'><a href="/a">Standard Dimir Control</a></div><div class='archetype-tile-statistic-value'>6.0%</div></div>`
	meta := client.parseMetaPage(html, "standard")
	if len(meta.Decks) != 1 {
		t.Fatalf("expected 1 deck, got %d", len(meta.Decks))
	}
	d := meta.Decks[0]
	if d.Format != "standard" {
		t.Errorf("expected format 'standard', got %q", d.Format)
	}
	// normalizeArchetypeName strips the "standard " prefix and lowercases.
	if d.ArchetypeName != "dimir control" {
		t.Errorf("expected normalized archetype 'dimir control', got %q", d.ArchetypeName)
	}
}

// TestGoldfishParse_DeckCap confirms the parser caps at 50 decks.
func TestGoldfishParse_DeckCap(t *testing.T) {
	client := NewGoldfishClient(nil)
	var sb strings.Builder
	for i := 0; i < 60; i++ {
		sb.WriteString(`<div class='archetype-tile' id='x'><div class='archetype-tile-title'><a href="/a">Deck</a></div><div class='archetype-tile-statistic-value'>1.0%</div></div>`)
	}
	meta := client.parseMetaPage(sb.String(), "standard")
	if len(meta.Decks) != 50 {
		t.Errorf("expected deck list capped at 50, got %d", len(meta.Decks))
	}
}

// TestGoldfishParse_WhitespaceAroundValue verifies leading/trailing whitespace
// around the percentage is tolerated.
func TestGoldfishParse_WhitespaceAroundValue(t *testing.T) {
	client := NewGoldfishClient(nil)
	html := `
	<div class='archetype-tile' id='1'><div class='archetype-tile-title'><a href="/a">Selesnya Tokens</a></div><div class='archetype-tile-statistic-value'>
	   4.5%
	</div></div>`
	meta := client.parseMetaPage(html, "standard")
	if len(meta.Decks) != 1 {
		t.Fatalf("expected 1 deck, got %d", len(meta.Decks))
	}
	if meta.Decks[0].MetaShare != 4.5 {
		t.Errorf("expected share 4.5, got %v", meta.Decks[0].MetaShare)
	}
}

// TestGoldfishParse_TotalDecksMatchesLength asserts TotalDecks tracks len(Decks).
func TestGoldfishParse_TotalDecksMatchesLength(t *testing.T) {
	client := NewGoldfishClient(nil)
	html := `
	<div class='archetype-tile' id='1'><div class='archetype-tile-title'><a href="/a">Deck A</a></div><div class='archetype-tile-statistic-value'>6.0%</div></div>
	<div class='archetype-tile' id='2'><div class='archetype-tile-title'><a href="/a">Deck B</a></div><div class='archetype-tile-statistic-value'>3.0%</div></div>`
	meta := client.parseMetaPage(html, "standard")
	if meta.TotalDecks != len(meta.Decks) {
		t.Errorf("TotalDecks (%d) != len(Decks) (%d)", meta.TotalDecks, len(meta.Decks))
	}
}

// TestGoldfishClient_GetMeta_Non200 verifies a non-200 response is an error and
// no source data is cached.
func TestGoldfishClient_GetMeta_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewGoldfishClient(&GoldfishConfig{
		BaseURL:        server.URL,
		CacheTTL:       time.Hour,
		RequestTimeout: 5 * time.Second,
		RateLimitMs:    10,
	})
	_, err := client.GetMeta(context.Background(), "standard")
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
	if cached, _ := client.GetCacheStatus("standard"); cached {
		t.Error("failed fetch should not populate cache")
	}
}

// TestGoldfishClient_ResponseSizeCap verifies the HP-1 io.LimitReader cap: a
// response body larger than the 10 MB cap is truncated before parsing, so the
// parser never sees the oversized tail. We assert the fetch still succeeds and
// the parser only sees content within the cap.
func TestGoldfishClient_ResponseSizeCap(t *testing.T) {
	// One valid tile, then padding past the 10 MB cap whose tile must NOT be seen.
	const valid = `<div class='archetype-tile' id='1'><div class='archetype-tile-title'><a href="/a">Within Cap</a></div><div class='archetype-tile-statistic-value'>9.0%</div></div>`
	const beyond = `<div class='archetype-tile' id='2'><div class='archetype-tile-title'><a href="/a">Beyond Cap</a></div><div class='archetype-tile-statistic-value'>9.0%</div></div>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(valid))
		_, _ = w.Write([]byte(strings.Repeat(" ", 11<<20))) // 11 MB of filler > 10 MB cap
		_, _ = w.Write([]byte(beyond))
	}))
	defer server.Close()

	client := NewGoldfishClient(&GoldfishConfig{
		BaseURL:        server.URL,
		CacheTTL:       time.Hour,
		RequestTimeout: 30 * time.Second,
		RateLimitMs:    10,
	})
	meta, err := client.GetMeta(context.Background(), "standard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, d := range meta.Decks {
		if d.Name == "Beyond Cap" {
			t.Fatal("parser saw content beyond the 10 MB size cap; io.LimitReader not applied")
		}
	}
	found := false
	for _, d := range meta.Decks {
		if d.Name == "Within Cap" {
			found = true
		}
	}
	if !found {
		t.Error("expected the within-cap deck to be parsed")
	}
}
