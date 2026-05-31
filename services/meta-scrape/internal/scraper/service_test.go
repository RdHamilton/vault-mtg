package scraper

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/meta-scrape/internal/store"
)

func createTestServers() (*httptest.Server, *httptest.Server) {
	goldfishServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
		<html>
		<body>
		<div class='archetype-tile' id='1'>
		<div class='archetype-tile-title'><a href="/archetype/mono-red">Mono Red Aggro</a></div>
		<div class='archetype-tile-statistic metagame-percentage'>
			<div class='archetype-tile-statistic-value'>15.5%</div>
		</div>
		</div>
		<div class='archetype-tile' id='2'>
		<div class='archetype-tile-title'><a href="/archetype/azorius">Azorius Control</a></div>
		<div class='archetype-tile-statistic metagame-percentage'>
			<div class='archetype-tile-statistic-value'>12.3%</div>
		</div>
		</div>
		<div class='archetype-tile' id='3'>
		<div class='archetype-tile-title'><a href="/archetype/golgari">Golgari Midrange</a></div>
		<div class='archetype-tile-statistic metagame-percentage'>
			<div class='archetype-tile-statistic-value'>3.7%</div>
		</div>
		</div>
		</body>
		</html>
		`
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))

	top8Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
		<html>
		<body>
		<table>
		<tr>
			<td><a href="/archetype?a=123">Mono Red Aggro</a></td>
			<td>25</td>
		</tr>
		<tr>
			<td><a href="/archetype?a=456">Azorius Control</a></td>
			<td>18</td>
		</tr>
		<tr>
			<td><a href="/archetype?a=789">Dimir Control</a></td>
			<td>12</td>
		</tr>
		</table>
		<div class="event_title">Grand Prix Test</div>
		<div class="event_title">Pro Tour Test</div>
		</body>
		</html>
		`
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))

	return goldfishServer, top8Server
}

func TestNewService(t *testing.T) {
	t.Run("with nil config", func(t *testing.T) {
		service := NewService(nil, nil)
		if service == nil {
			t.Fatal("expected non-nil service")
		}
		if service.goldfishClient == nil {
			t.Error("expected non-nil goldfish client")
		}
		if service.top8Client == nil {
			t.Error("expected non-nil top8 client")
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &ServiceConfig{
			GoldfishConfig: &GoldfishConfig{
				BaseURL:        "https://custom-goldfish.url",
				CacheTTL:       2 * time.Hour,
				RequestTimeout: 10 * time.Second,
				RateLimitMs:    500,
			},
			Top8Config: &Top8Config{
				BaseURL:        "https://custom-top8.url",
				CacheTTL:       3 * time.Hour,
				RequestTimeout: 15 * time.Second,
				RateLimitMs:    1000,
			},
		}
		service := NewService(config, nil)
		if service.goldfishClient.baseURL != "https://custom-goldfish.url" {
			t.Error("goldfish client not configured correctly")
		}
		if service.top8Client.baseURL != "https://custom-top8.url" {
			t.Error("top8 client not configured correctly")
		}
	})
}

func TestService_GetAggregatedMeta(t *testing.T) {
	goldfishServer, top8Server := createTestServers()
	defer goldfishServer.Close()
	defer top8Server.Close()

	config := &ServiceConfig{
		GoldfishConfig: &GoldfishConfig{
			BaseURL:        goldfishServer.URL,
			CacheTTL:       1 * time.Hour,
			RequestTimeout: 5 * time.Second,
			RateLimitMs:    10,
		},
		Top8Config: &Top8Config{
			BaseURL:        top8Server.URL,
			CacheTTL:       1 * time.Hour,
			RequestTimeout: 5 * time.Second,
			RateLimitMs:    10,
		},
	}
	service := NewService(config, nil)

	ctx := context.Background()
	meta, err := service.GetAggregatedMeta(ctx, "standard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta == nil {
		t.Fatal("expected non-nil meta")
	}
	if meta.Format != "standard" {
		t.Errorf("expected format 'standard', got %s", meta.Format)
	}
	if len(meta.Sources) == 0 {
		t.Error("expected at least one source")
	}
	if len(meta.Archetypes) == 0 {
		t.Error("expected at least one archetype")
	}
}

func TestService_GetAggregatedMeta_CombinesData(t *testing.T) {
	goldfishServer, top8Server := createTestServers()
	defer goldfishServer.Close()
	defer top8Server.Close()

	config := &ServiceConfig{
		GoldfishConfig: &GoldfishConfig{
			BaseURL:        goldfishServer.URL,
			CacheTTL:       1 * time.Hour,
			RequestTimeout: 5 * time.Second,
			RateLimitMs:    10,
		},
		Top8Config: &Top8Config{
			BaseURL:        top8Server.URL,
			CacheTTL:       1 * time.Hour,
			RequestTimeout: 5 * time.Second,
			RateLimitMs:    10,
		},
	}
	service := NewService(config, nil)

	ctx := context.Background()
	meta, err := service.GetAggregatedMeta(ctx, "standard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find Mono Red Aggro - should have data from both sources
	var monoRed *AggregatedArchetype
	for _, arch := range meta.Archetypes {
		if arch.NormalizedName == "mono red aggro" {
			monoRed = arch
			break
		}
	}

	if monoRed == nil {
		t.Fatal("expected to find Mono Red Aggro")
	}

	// Should have meta share from goldfish
	if monoRed.MetaShare == 0 {
		t.Error("expected non-zero meta share")
	}

	// Should have tournament data from top8
	if monoRed.TournamentTop8s == 0 {
		t.Error("expected non-zero tournament top8s")
	}
}

func TestService_GetTopArchetypes(t *testing.T) {
	goldfishServer, top8Server := createTestServers()
	defer goldfishServer.Close()
	defer top8Server.Close()

	config := &ServiceConfig{
		GoldfishConfig: &GoldfishConfig{
			BaseURL:        goldfishServer.URL,
			CacheTTL:       1 * time.Hour,
			RequestTimeout: 5 * time.Second,
			RateLimitMs:    10,
		},
		Top8Config: &Top8Config{
			BaseURL:        top8Server.URL,
			CacheTTL:       1 * time.Hour,
			RequestTimeout: 5 * time.Second,
			RateLimitMs:    10,
		},
	}
	service := NewService(config, nil)

	ctx := context.Background()
	archetypes, err := service.GetTopArchetypes(ctx, "standard", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(archetypes) != 2 {
		t.Errorf("expected 2 archetypes, got %d", len(archetypes))
	}
}

func TestService_GetArchetypeByName(t *testing.T) {
	goldfishServer, top8Server := createTestServers()
	defer goldfishServer.Close()
	defer top8Server.Close()

	config := &ServiceConfig{
		GoldfishConfig: &GoldfishConfig{
			BaseURL:        goldfishServer.URL,
			CacheTTL:       1 * time.Hour,
			RequestTimeout: 5 * time.Second,
			RateLimitMs:    10,
		},
		Top8Config: &Top8Config{
			BaseURL:        top8Server.URL,
			CacheTTL:       1 * time.Hour,
			RequestTimeout: 5 * time.Second,
			RateLimitMs:    10,
		},
	}
	service := NewService(config, nil)

	ctx := context.Background()

	t.Run("found archetype", func(t *testing.T) {
		arch, err := service.GetArchetypeByName(ctx, "standard", "mono red")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if arch == nil {
			t.Fatal("expected non-nil archetype")
		}
	})

	t.Run("not found archetype", func(t *testing.T) {
		_, err := service.GetArchetypeByName(ctx, "standard", "nonexistent deck xyz")
		if err == nil {
			t.Error("expected error for nonexistent archetype")
		}
	})
}

func TestService_GetTier1Archetypes(t *testing.T) {
	goldfishServer, top8Server := createTestServers()
	defer goldfishServer.Close()
	defer top8Server.Close()

	config := &ServiceConfig{
		GoldfishConfig: &GoldfishConfig{
			BaseURL:        goldfishServer.URL,
			CacheTTL:       1 * time.Hour,
			RequestTimeout: 5 * time.Second,
			RateLimitMs:    10,
		},
		Top8Config: &Top8Config{
			BaseURL:        top8Server.URL,
			CacheTTL:       1 * time.Hour,
			RequestTimeout: 5 * time.Second,
			RateLimitMs:    10,
		},
	}
	service := NewService(config, nil)

	ctx := context.Background()
	tier1, err := service.GetTier1Archetypes(ctx, "standard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All returned archetypes should be tier 1
	for _, arch := range tier1 {
		if arch.Tier != 1 {
			t.Errorf("expected tier 1, got tier %d for %s", arch.Tier, arch.Name)
		}
	}
}

func TestService_GetArchetypesByColors(t *testing.T) {
	goldfishServer, top8Server := createTestServers()
	defer goldfishServer.Close()
	defer top8Server.Close()

	config := &ServiceConfig{
		GoldfishConfig: &GoldfishConfig{
			BaseURL:        goldfishServer.URL,
			CacheTTL:       1 * time.Hour,
			RequestTimeout: 5 * time.Second,
			RateLimitMs:    10,
		},
		Top8Config: &Top8Config{
			BaseURL:        top8Server.URL,
			CacheTTL:       1 * time.Hour,
			RequestTimeout: 5 * time.Second,
			RateLimitMs:    10,
		},
	}
	service := NewService(config, nil)

	ctx := context.Background()

	t.Run("filter by red", func(t *testing.T) {
		archetypes, err := service.GetArchetypesByColors(ctx, "standard", []string{"R"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, arch := range archetypes {
			hasRed := false
			for _, c := range arch.Colors {
				if c == "R" {
					hasRed = true
					break
				}
			}
			if !hasRed {
				t.Errorf("archetype %s should have red", arch.Name)
			}
		}
	})

	t.Run("no color filter", func(t *testing.T) {
		archetypes, err := service.GetArchetypesByColors(ctx, "standard", []string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should return all archetypes
		if len(archetypes) == 0 {
			t.Error("expected some archetypes with no filter")
		}
	})
}

func TestService_GetRecentTournaments(t *testing.T) {
	goldfishServer, top8Server := createTestServers()
	defer goldfishServer.Close()
	defer top8Server.Close()

	config := &ServiceConfig{
		GoldfishConfig: &GoldfishConfig{
			BaseURL:        goldfishServer.URL,
			CacheTTL:       1 * time.Hour,
			RequestTimeout: 5 * time.Second,
			RateLimitMs:    10,
		},
		Top8Config: &Top8Config{
			BaseURL:        top8Server.URL,
			CacheTTL:       1 * time.Hour,
			RequestTimeout: 5 * time.Second,
			RateLimitMs:    10,
		},
	}
	service := NewService(config, nil)

	ctx := context.Background()
	tournaments, err := service.GetRecentTournaments(ctx, "standard", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tournaments) > 5 {
		t.Errorf("expected at most 5 tournaments, got %d", len(tournaments))
	}
}

func TestService_GetSupportedFormats(t *testing.T) {
	service := NewService(nil, nil)
	formats := service.GetSupportedFormats()

	if len(formats) == 0 {
		t.Error("expected at least one supported format")
	}

	// Check for common formats
	expectedFormats := []string{"standard", "modern", "pioneer", "historic"}
	for _, expected := range expectedFormats {
		found := false
		for _, f := range formats {
			if f == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s to be supported", expected)
		}
	}
}

func TestService_IsFormatSupported(t *testing.T) {
	service := NewService(nil, nil)

	tests := []struct {
		format   string
		expected bool
	}{
		{"standard", true},
		{"Standard", true},
		{"MODERN", true},
		{"pioneer", true},
		{"commander", false},
		{"brawl", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			result := service.IsFormatSupported(tt.format)
			if result != tt.expected {
				t.Errorf("IsFormatSupported(%s) = %v, expected %v", tt.format, result, tt.expected)
			}
		})
	}
}

func TestService_ColorsMatch(t *testing.T) {
	service := NewService(nil, nil)

	tests := []struct {
		name            string
		archetypeColors []string
		targetColors    []string
		expected        bool
	}{
		{"exact match", []string{"W", "U"}, []string{"W", "U"}, true},
		{"subset match", []string{"W", "U", "B"}, []string{"W", "U"}, true},
		{"no match", []string{"R", "G"}, []string{"W", "U"}, false},
		{"empty target", []string{"W", "U"}, []string{}, true},
		{"empty archetype", []string{}, []string{"W"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.colorsMatch(tt.archetypeColors, tt.targetColors)
			if result != tt.expected {
				t.Errorf("colorsMatch(%v, %v) = %v, expected %v",
					tt.archetypeColors, tt.targetColors, result, tt.expected)
			}
		})
	}
}

func TestService_CalculateConfidence(t *testing.T) {
	service := NewService(nil, nil)

	tests := []struct {
		name     string
		arch     *AggregatedArchetype
		minScore float64
	}{
		{
			"full data",
			&AggregatedArchetype{
				MetaShare:       15.0,
				TournamentTop8s: 25,
				Colors:          []string{"W", "U"},
			},
			0.9,
		},
		{
			"meta only",
			&AggregatedArchetype{
				MetaShare: 10.0,
			},
			0.4,
		},
		{
			"tournament only",
			&AggregatedArchetype{
				TournamentTop8s: 15,
			},
			0.4,
		},
		{
			"no data",
			&AggregatedArchetype{},
			0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := service.calculateConfidence(tt.arch)
			if confidence < tt.minScore {
				t.Errorf("calculateConfidence() = %v, expected >= %v", confidence, tt.minScore)
			}
		})
	}
}

func TestService_CalculateTier(t *testing.T) {
	service := NewService(nil, nil)

	tests := []struct {
		name         string
		arch         *AggregatedArchetype
		expectedTier int
	}{
		{
			"tier 1 by meta share",
			&AggregatedArchetype{MetaShare: 10.0},
			1,
		},
		{
			"tier 2 by meta share",
			&AggregatedArchetype{MetaShare: 3.0},
			2,
		},
		{
			"tier 3 by meta share",
			&AggregatedArchetype{MetaShare: 1.0},
			3,
		},
		{
			"tier 1 by tournament",
			&AggregatedArchetype{TournamentTop8s: 25},
			1,
		},
		{
			"tier 2 by tournament",
			&AggregatedArchetype{TournamentTop8s: 12},
			2,
		},
		{
			"tier 3 by tournament",
			&AggregatedArchetype{TournamentTop8s: 7},
			3,
		},
		{
			"tier 4 no data",
			&AggregatedArchetype{},
			4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier := service.calculateTier(tt.arch)
			if tier != tt.expectedTier {
				t.Errorf("calculateTier() = %d, expected %d", tier, tt.expectedTier)
			}
		})
	}
}

func TestAggregatedArchetype_Fields(t *testing.T) {
	arch := &AggregatedArchetype{
		Name:            "Test Archetype",
		NormalizedName:  "test archetype",
		Colors:          []string{"W", "U", "B"},
		MetaShare:       15.5,
		TournamentTop8s: 25,
		TournamentWins:  5,
		Tier:            1,
		ConfidenceScore: 0.95,
		TrendDirection:  "up",
		LastSeenInMeta:  time.Now(),
		LastSeenInTop8:  time.Now(),
	}

	if arch.Name != "Test Archetype" {
		t.Errorf("unexpected Name: %s", arch.Name)
	}
	if arch.MetaShare != 15.5 {
		t.Errorf("unexpected MetaShare: %f", arch.MetaShare)
	}
	if arch.Tier != 1 {
		t.Errorf("unexpected Tier: %d", arch.Tier)
	}
}

func TestAggregatedMeta_Fields(t *testing.T) {
	meta := &AggregatedMeta{
		Format:          "standard",
		Archetypes:      []*AggregatedArchetype{{Name: "Test"}},
		TopDecks:        []*MetaDeck{{Name: "Deck"}},
		Tournaments:     []*Tournament{{Name: "Tournament"}},
		TotalArchetypes: 1,
		LastUpdated:     time.Now(),
		Sources:         []string{"mtggoldfish", "mtgtop8"},
	}

	if meta.Format != "standard" {
		t.Errorf("unexpected Format: %s", meta.Format)
	}
	if len(meta.Sources) != 2 {
		t.Errorf("unexpected Sources length: %d", len(meta.Sources))
	}
}

// ---------------------------------------------------------------------------
// MetaStore integration tests (#175): PersistMeta maps AggregatedArchetype ->
// store.Archetype and calls UpsertArchetypes; RefreshAll persists after
// aggregation. A fakeUpserter stands in for *store.MetaStore so these run with
// zero DB and zero live network.
// ---------------------------------------------------------------------------

// fakeUpserter records the archetypes passed to UpsertArchetypes and can be
// configured to return an error.
type fakeUpserter struct {
	calls   int
	last    []store.Archetype
	failErr error
}

func (f *fakeUpserter) UpsertArchetypes(ctx context.Context, archetypes []store.Archetype) error {
	f.calls++
	f.last = archetypes
	return f.failErr
}

func TestService_PersistMeta_MapsFields(t *testing.T) {
	fake := &fakeUpserter{}
	service := NewServiceWithStore(nil, fake)

	tier := 1
	share := float32(15.5)
	top8s := 25
	wins := 5
	conf := float32(0.95)
	trend := "up"

	meta := &AggregatedMeta{
		Format: "standard",
		Archetypes: []*AggregatedArchetype{
			{
				Name:            "Mono Red Aggro",
				Tier:            tier,
				MetaShare:       15.5,
				TournamentTop8s: top8s,
				TournamentWins:  wins,
				ConfidenceScore: 0.95,
				TrendDirection:  trend,
			},
		},
	}

	if err := service.PersistMeta(context.Background(), "standard", meta); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.calls != 1 {
		t.Fatalf("expected UpsertArchetypes called once, got %d", fake.calls)
	}
	if len(fake.last) != 1 {
		t.Fatalf("expected 1 mapped archetype, got %d", len(fake.last))
	}
	got := fake.last[0]
	if got.Name != "Mono Red Aggro" {
		t.Errorf("Name: got %q", got.Name)
	}
	if got.Format != "standard" {
		t.Errorf("Format: got %q", got.Format)
	}
	if got.Tier == nil || *got.Tier != "1" {
		t.Errorf("Tier: expected ptr to \"1\", got %v", got.Tier)
	}
	if got.MetaShare == nil || *got.MetaShare != share {
		t.Errorf("MetaShare: expected ptr to %v, got %v", share, got.MetaShare)
	}
	if got.TournamentTop8s == nil || *got.TournamentTop8s != top8s {
		t.Errorf("TournamentTop8s: expected ptr to %d, got %v", top8s, got.TournamentTop8s)
	}
	if got.TournamentWins == nil || *got.TournamentWins != wins {
		t.Errorf("TournamentWins: expected ptr to %d, got %v", wins, got.TournamentWins)
	}
	if got.ConfidenceScore == nil || *got.ConfidenceScore != conf {
		t.Errorf("ConfidenceScore: expected ptr to %v, got %v", conf, got.ConfidenceScore)
	}
	if got.TrendDirection == nil || *got.TrendDirection != trend {
		t.Errorf("TrendDirection: expected ptr to %q, got %v", trend, got.TrendDirection)
	}
	// #177 owns these; PersistMeta must leave them nil.
	if got.Description != nil || got.PlayStyle != nil || got.SourceURL != nil {
		t.Errorf("expected Description/PlayStyle/SourceURL nil in #175, got %v/%v/%v",
			got.Description, got.PlayStyle, got.SourceURL)
	}
}

func TestService_PersistMeta_NilFieldsForZeroValues(t *testing.T) {
	fake := &fakeUpserter{}
	service := NewServiceWithStore(nil, fake)

	meta := &AggregatedMeta{
		Format: "standard",
		Archetypes: []*AggregatedArchetype{
			{Name: "Sparse Deck", Tier: 0}, // all zero values
		},
	}

	if err := service.PersistMeta(context.Background(), "standard", meta); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := fake.last[0]
	if got.Tier != nil {
		t.Errorf("expected nil Tier for zero value, got %v", *got.Tier)
	}
	if got.MetaShare != nil {
		t.Errorf("expected nil MetaShare for zero value, got %v", *got.MetaShare)
	}
	if got.TournamentTop8s != nil {
		t.Errorf("expected nil TournamentTop8s for zero value, got %v", *got.TournamentTop8s)
	}
	if got.TournamentWins != nil {
		t.Errorf("expected nil TournamentWins for zero value, got %v", *got.TournamentWins)
	}
	if got.TrendDirection != nil {
		t.Errorf("expected nil TrendDirection for empty value, got %v", *got.TrendDirection)
	}
}

func TestService_PersistMeta_EmptyIsNoOp(t *testing.T) {
	fake := &fakeUpserter{}
	service := NewServiceWithStore(nil, fake)

	// FH-1: empty archetype list must not call the store at all.
	meta := &AggregatedMeta{Format: "standard", Archetypes: nil}
	if err := service.PersistMeta(context.Background(), "standard", meta); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.calls != 0 {
		t.Errorf("expected no UpsertArchetypes call for empty meta, got %d", fake.calls)
	}
}

func TestService_PersistMeta_NilStoreIsNoOp(t *testing.T) {
	// A service constructed without a store (read-only use) must not panic.
	service := NewService(nil, nil)
	meta := &AggregatedMeta{
		Format:     "standard",
		Archetypes: []*AggregatedArchetype{{Name: "X", MetaShare: 5}},
	}
	if err := service.PersistMeta(context.Background(), "standard", meta); err != nil {
		t.Fatalf("unexpected error with nil store: %v", err)
	}
}

func TestService_PersistMeta_PropagatesStoreError(t *testing.T) {
	fake := &fakeUpserter{failErr: errors.New("db down")}
	service := NewServiceWithStore(nil, fake)
	meta := &AggregatedMeta{
		Format:     "standard",
		Archetypes: []*AggregatedArchetype{{Name: "X", MetaShare: 5}},
	}
	err := service.PersistMeta(context.Background(), "standard", meta)
	if err == nil {
		t.Fatal("expected error from store to propagate")
	}
}

func TestService_RefreshAll_PersistsAfterAggregation(t *testing.T) {
	goldfishServer, top8Server := createTestServers()
	defer goldfishServer.Close()
	defer top8Server.Close()

	fake := &fakeUpserter{}
	config := &ServiceConfig{
		GoldfishConfig: &GoldfishConfig{
			BaseURL:        goldfishServer.URL,
			CacheTTL:       1 * time.Hour,
			RequestTimeout: 5 * time.Second,
			RateLimitMs:    10,
		},
		Top8Config: &Top8Config{
			BaseURL:        top8Server.URL,
			CacheTTL:       1 * time.Hour,
			RequestTimeout: 5 * time.Second,
			RateLimitMs:    10,
		},
	}
	service := NewServiceWithStore(config, fake)

	meta, err := service.RefreshAll(context.Background(), "standard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(meta.Archetypes) == 0 {
		t.Fatal("expected aggregated archetypes")
	}
	if fake.calls != 1 {
		t.Errorf("expected RefreshAll to persist once, got %d calls", fake.calls)
	}
	if len(fake.last) != len(meta.Archetypes) {
		t.Errorf("expected %d persisted archetypes, got %d", len(meta.Archetypes), len(fake.last))
	}
}

func TestService_RefreshAll_NoStoreStillAggregates(t *testing.T) {
	goldfishServer, top8Server := createTestServers()
	defer goldfishServer.Close()
	defer top8Server.Close()

	config := &ServiceConfig{
		GoldfishConfig: &GoldfishConfig{
			BaseURL:        goldfishServer.URL,
			CacheTTL:       1 * time.Hour,
			RequestTimeout: 5 * time.Second,
			RateLimitMs:    10,
		},
		Top8Config: &Top8Config{
			BaseURL:        top8Server.URL,
			CacheTTL:       1 * time.Hour,
			RequestTimeout: 5 * time.Second,
			RateLimitMs:    10,
		},
	}
	service := NewService(config, nil)

	meta, err := service.RefreshAll(context.Background(), "standard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(meta.Archetypes) == 0 {
		t.Error("expected aggregated archetypes even without a store")
	}
}

// compile-time assertion that *store.MetaStore satisfies the archetypeUpserter
// interface PersistMeta depends on. If #176's signature drifts, this fails to
// compile rather than at runtime.
var _ archetypeUpserter = (*store.MetaStore)(nil)
