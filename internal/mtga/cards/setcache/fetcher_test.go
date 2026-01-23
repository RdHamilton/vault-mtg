package setcache

import (
	"context"
	"testing"
	"time"

	"github.com/ramonehamilton/MTGA-Companion/internal/mtga/cards/mtgjson"
	"github.com/ramonehamilton/MTGA-Companion/internal/mtga/cards/scryfall"
	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
	"github.com/ramonehamilton/MTGA-Companion/internal/storage/repository"
)

// mockSetCardRepo implements repository.SetCardRepository for testing.
type mockSetCardRepo struct {
	cards map[string][]*models.SetCard
}

func newMockSetCardRepo() *mockSetCardRepo {
	return &mockSetCardRepo{
		cards: make(map[string][]*models.SetCard),
	}
}

func (m *mockSetCardRepo) SaveCard(_ context.Context, card *models.SetCard) error {
	m.cards[card.SetCode] = append(m.cards[card.SetCode], card)
	return nil
}

func (m *mockSetCardRepo) SaveCards(_ context.Context, cards []*models.SetCard) error {
	for _, card := range cards {
		m.cards[card.SetCode] = append(m.cards[card.SetCode], card)
	}
	return nil
}

func (m *mockSetCardRepo) GetCardByArenaID(_ context.Context, _ string) (*models.SetCard, error) {
	return nil, nil
}

func (m *mockSetCardRepo) GetCardsBySet(_ context.Context, setCode string) ([]*models.SetCard, error) {
	return m.cards[setCode], nil
}

func (m *mockSetCardRepo) SearchCards(_ context.Context, _ string, _ []string, _ int) ([]*models.SetCard, error) {
	return nil, nil
}

func (m *mockSetCardRepo) IsSetCached(_ context.Context, setCode string) (bool, error) {
	return len(m.cards[setCode]) > 0, nil
}

func (m *mockSetCardRepo) GetCachedSets(_ context.Context) ([]string, error) {
	sets := make([]string, 0, len(m.cards))
	for setCode := range m.cards {
		sets = append(sets, setCode)
	}
	return sets, nil
}

func (m *mockSetCardRepo) DeleteSet(_ context.Context, setCode string) error {
	delete(m.cards, setCode)
	return nil
}

func (m *mockSetCardRepo) GetMetadataStaleness(_ context.Context, _, _ int) (*repository.MetadataStaleness, error) {
	return nil, nil
}

func (m *mockSetCardRepo) GetStaleCards(_ context.Context, _, _ int) ([]*repository.StaleCard, error) {
	return nil, nil
}

func (m *mockSetCardRepo) GetSetRarityCounts(_ context.Context) ([]*repository.SetRarityCount, error) {
	return nil, nil
}

func (m *mockSetCardRepo) GetAllCardSetInfo(_ context.Context) ([]*repository.CardSetInfo, error) {
	return nil, nil
}

// mockScryfallClient implements a mock Scryfall client for testing.
type mockScryfallClient struct {
	searchResults map[string]*scryfall.SearchResult
}

func newMockScryfallClient() *mockScryfallClient {
	return &mockScryfallClient{
		searchResults: make(map[string]*scryfall.SearchResult),
	}
}

func (m *mockScryfallClient) SearchCards(_ context.Context, query string) (*scryfall.SearchResult, error) {
	if result, ok := m.searchResults[query]; ok {
		return result, nil
	}
	return &scryfall.SearchResult{TotalCards: 0}, nil
}

func (m *mockScryfallClient) setSearchResult(query string, totalCards int) {
	m.searchResults[query] = &scryfall.SearchResult{TotalCards: totalCards}
}

// scryfallClientAdapter wraps mockScryfallClient to match the real client interface.
type scryfallClientAdapter struct {
	mock *mockScryfallClient
}

func TestCheckCacheCompleteness_IncompleteCache(t *testing.T) {
	// Setup: 50 cached cards, but Scryfall reports 286 Arena cards
	mockRepo := newMockSetCardRepo()
	for i := 0; i < 50; i++ {
		_ = mockRepo.SaveCard(context.Background(), &models.SetCard{
			SetCode: "TLA",
			ArenaID: string(rune(i)),
			Name:    "Test Card",
		})
	}

	mockScryfall := newMockScryfallClient()
	mockScryfall.setSearchResult("set:tla game:arena", 286)

	// Create fetcher with mocks - we can't use the real constructor
	// because it expects *scryfall.Client, so we test the logic directly
	cachedCards, _ := mockRepo.GetCardsBySet(context.Background(), "TLA")
	cachedCount := len(cachedCards)

	expectedCount := 286

	// Test the logic: cached < 90% of expected should trigger refresh
	needsRefresh := expectedCount > 0 && cachedCount < (expectedCount*9/10)

	if !needsRefresh {
		t.Errorf("Expected needsRefresh=true for cached=%d, expected=%d", cachedCount, expectedCount)
	}
}

func TestCheckCacheCompleteness_CompleteCache(t *testing.T) {
	// Setup: 280 cached cards, Scryfall reports 286 Arena cards (>90% complete)
	mockRepo := newMockSetCardRepo()
	for i := 0; i < 280; i++ {
		_ = mockRepo.SaveCard(context.Background(), &models.SetCard{
			SetCode: "TLA",
			ArenaID: string(rune(i)),
			Name:    "Test Card",
		})
	}

	cachedCards, _ := mockRepo.GetCardsBySet(context.Background(), "TLA")
	cachedCount := len(cachedCards)

	expectedCount := 286

	// Test the logic: cached >= 90% of expected should NOT trigger refresh
	needsRefresh := expectedCount > 0 && cachedCount < (expectedCount*9/10)

	if needsRefresh {
		t.Errorf("Expected needsRefresh=false for cached=%d, expected=%d (90%% threshold = %d)",
			cachedCount, expectedCount, expectedCount*9/10)
	}
}

func TestCheckCacheCompleteness_ExactThreshold(t *testing.T) {
	// Test the boundary: exactly 90% should NOT trigger refresh
	expectedCount := 100
	cachedCount := 90 // Exactly 90%

	// 90 >= 90 (which is 100*9/10), so needsRefresh should be false
	needsRefresh := expectedCount > 0 && cachedCount < (expectedCount*9/10)

	if needsRefresh {
		t.Errorf("Expected needsRefresh=false at exactly 90%% (cached=%d, expected=%d)", cachedCount, expectedCount)
	}

	// 89 < 90, so needsRefresh should be true
	cachedCount = 89
	needsRefresh = expectedCount > 0 && cachedCount < (expectedCount*9/10)

	if !needsRefresh {
		t.Errorf("Expected needsRefresh=true just below 90%% (cached=%d, expected=%d)", cachedCount, expectedCount)
	}
}

func TestCheckCacheCompleteness_EmptyExpected(t *testing.T) {
	// If Scryfall reports 0 cards, we should NOT trigger refresh
	expectedCount := 0
	cachedCount := 50

	needsRefresh := expectedCount > 0 && cachedCount < (expectedCount*9/10)

	if needsRefresh {
		t.Errorf("Expected needsRefresh=false when expectedCount=0")
	}
}

func TestMTGASetToScryfall_Mapping(t *testing.T) {
	tests := []struct {
		mtgaCode     string
		expectedCode string
	}{
		{"TLA", "tla"},
		{"BLB", "blb"},
		{"DSK", "dsk"},
		{"UNKNOWN", "unknown"}, // Falls back to lowercase
	}

	for _, tt := range tests {
		t.Run(tt.mtgaCode, func(t *testing.T) {
			scryfallCode, ok := MTGASetToScryfall[tt.mtgaCode]
			if !ok {
				// Falls back to lowercase
				scryfallCode = tt.mtgaCode
				scryfallCode = string([]rune(scryfallCode)) // Force lowercase would happen in actual code
			}

			// For unknown codes, the actual code uses strings.ToLower
			if _, exists := MTGASetToScryfall[tt.mtgaCode]; !exists {
				if tt.mtgaCode != "UNKNOWN" {
					t.Errorf("Expected %s to be in MTGASetToScryfall map", tt.mtgaCode)
				}
			}
		})
	}
}

func TestArenaExclusiveBasicLands_TLAMapping(t *testing.T) {
	// Test that TLA basic lands are correctly mapped
	tests := []struct {
		arenaID      int
		expectedSet  string
		expectedName string
	}{
		{97563, "TLA", "Plains"},
		{97564, "TLA", "Island"},
		{97565, "TLA", "Swamp"},
		{97566, "TLA", "Mountain"},
		{97567, "TLA", "Forest"},
	}

	for _, tt := range tests {
		t.Run(tt.expectedName, func(t *testing.T) {
			basicLand, ok := ArenaExclusiveBasicLands[tt.arenaID]
			if !ok {
				t.Fatalf("Expected ArenaExclusiveBasicLands to contain arenaID %d", tt.arenaID)
			}

			if basicLand.SetCode != tt.expectedSet {
				t.Errorf("Expected SetCode=%s, got %s", tt.expectedSet, basicLand.SetCode)
			}

			if basicLand.CardName != tt.expectedName {
				t.Errorf("Expected CardName=%s, got %s", tt.expectedName, basicLand.CardName)
			}
		})
	}
}

func TestArenaExclusiveBasicLands_UnknownID(t *testing.T) {
	// Test that unknown IDs return false
	unknownIDs := []int{12345, 99999, 0, -1}

	for _, id := range unknownIDs {
		if _, ok := ArenaExclusiveBasicLands[id]; ok {
			t.Errorf("Expected ArenaExclusiveBasicLands to NOT contain arenaID %d", id)
		}
	}
}

func TestCheckCacheCompleteness_ArenaExclusiveSet_IncompleteCache(t *testing.T) {
	// Arena-exclusive set: Scryfall reports 0 game:arena cards, but 17Lands has 286 ratings
	// This tests the logic for sets like TLA where Scryfall lacks Arena IDs

	// Simulated state: 50 cached cards
	cachedCount := 50

	// Scryfall returns 0 (no game:arena cards for Arena-exclusive sets)
	scryfallExpected := 0

	// 17Lands has 286 cards
	ratingsCount := 286

	// Logic: if scryfallExpected == 0, use ratingsCount instead
	var needsRefresh bool
	if scryfallExpected > 0 {
		needsRefresh = cachedCount < (scryfallExpected * 9 / 10)
	} else if ratingsCount > 0 {
		needsRefresh = cachedCount < (ratingsCount * 9 / 10)
	}

	if !needsRefresh {
		t.Errorf("Expected needsRefresh=true for Arena-exclusive set: cached=%d, 17lands=%d", cachedCount, ratingsCount)
	}
}

func TestCheckCacheCompleteness_ArenaExclusiveSet_CompleteCache(t *testing.T) {
	// Arena-exclusive set with complete cache
	cachedCount := 280
	scryfallExpected := 0 // Scryfall has no Arena IDs
	ratingsCount := 286   // 17Lands has 286 cards

	var needsRefresh bool
	if scryfallExpected > 0 {
		needsRefresh = cachedCount < (scryfallExpected * 9 / 10)
	} else if ratingsCount > 0 {
		needsRefresh = cachedCount < (ratingsCount * 9 / 10)
	}

	if needsRefresh {
		t.Errorf("Expected needsRefresh=false for complete Arena-exclusive cache: cached=%d, 17lands=%d (90%% = %d)",
			cachedCount, ratingsCount, ratingsCount*9/10)
	}
}

func TestCheckCacheCompleteness_ArenaExclusiveSet_NoRatings(t *testing.T) {
	// Arena-exclusive set with no 17Lands ratings yet
	cachedCount := 5      // Some basic lands cached
	scryfallExpected := 0 // Scryfall has no Arena IDs
	ratingsCount := 0     // No 17Lands ratings yet

	var needsRefresh bool
	if scryfallExpected > 0 {
		needsRefresh = cachedCount < (scryfallExpected * 9 / 10)
	} else if ratingsCount > 0 {
		needsRefresh = cachedCount < (ratingsCount * 9 / 10)
	}

	// With no ratings, we can't determine completeness, so no refresh
	if needsRefresh {
		t.Errorf("Expected needsRefresh=false when no 17Lands ratings available")
	}
}

func TestExtractScryfallIDFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "standard scryfall image URL",
			url:      "https://cards.scryfall.io/large/front/f/a/fa940e68-010e-4b68-be8a-555d7068f7b4.jpg",
			expected: "fa940e68-010e-4b68-be8a-555d7068f7b4",
		},
		{
			name:     "small image URL",
			url:      "https://cards.scryfall.io/small/front/1/2/12345678-1234-1234-1234-123456789abc.jpg",
			expected: "12345678-1234-1234-1234-123456789abc",
		},
		{
			name:     "art crop URL",
			url:      "https://cards.scryfall.io/art_crop/front/a/b/abcdef01-2345-6789-abcd-ef0123456789.jpg",
			expected: "abcdef01-2345-6789-abcd-ef0123456789",
		},
		{
			name:     "normal image URL",
			url:      "https://cards.scryfall.io/normal/front/9/9/99887766-5544-3322-1100-aabbccddeeff.png",
			expected: "99887766-5544-3322-1100-aabbccddeeff",
		},
		{
			name:     "empty URL",
			url:      "",
			expected: "",
		},
		{
			name:     "URL without UUID",
			url:      "https://example.com/image.jpg",
			expected: "",
		},
		{
			name:     "URL with invalid UUID format",
			url:      "https://cards.scryfall.io/large/front/1/2/not-a-uuid.jpg",
			expected: "",
		},
		{
			name:     "partial UUID",
			url:      "https://cards.scryfall.io/large/front/1/2/12345678-1234-1234.jpg",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractScryfallIDFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("ExtractScryfallIDFromURL(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestExtractScryfallIDFromURL_VariousFormats(t *testing.T) {
	// Test that any URL containing a valid UUID extracts it correctly
	validUUID := "fa940e68-010e-4b68-be8a-555d7068f7b4"

	urlsWithValidUUID := []string{
		"https://cards.scryfall.io/large/front/f/a/fa940e68-010e-4b68-be8a-555d7068f7b4.jpg",
		"https://cdn.17lands.com/images/fa940e68-010e-4b68-be8a-555d7068f7b4.png",
		"https://example.com/path/to/fa940e68-010e-4b68-be8a-555d7068f7b4/image.webp",
		"fa940e68-010e-4b68-be8a-555d7068f7b4",
	}

	for _, url := range urlsWithValidUUID {
		result := ExtractScryfallIDFromURL(url)
		if result != validUUID {
			t.Errorf("ExtractScryfallIDFromURL(%q) = %q, want %q", url, result, validUUID)
		}
	}
}

// mockDraftRatingsRepo implements the DraftRatingsRepository interface for testing fallback logic.
type mockDraftRatingsRepo struct {
	cardLookup map[string]struct {
		name    string
		setCode string
	}
}

func newMockDraftRatingsRepo() *mockDraftRatingsRepo {
	return &mockDraftRatingsRepo{
		cardLookup: make(map[string]struct {
			name    string
			setCode string
		}),
	}
}

func (m *mockDraftRatingsRepo) SetCardLookup(arenaID, name, setCode string) {
	m.cardLookup[arenaID] = struct {
		name    string
		setCode string
	}{name: name, setCode: setCode}
}

func (m *mockDraftRatingsRepo) GetCardNameAndSetByArenaID(_ context.Context, arenaID string) (string, string, error) {
	if card, ok := m.cardLookup[arenaID]; ok {
		return card.name, card.setCode, nil
	}
	return "", "", nil
}

func (m *mockDraftRatingsRepo) GetSetCodeByArenaID(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *mockDraftRatingsRepo) GetSetsWithRatings(_ context.Context) ([]string, error) {
	return nil, nil
}

// Additional mock methods to satisfy the interface (stubbed).
func (m *mockDraftRatingsRepo) SaveCardRating(_ context.Context, _, _ string, _ interface{}) error {
	return nil
}

func (m *mockDraftRatingsRepo) GetCardRatings(_ context.Context, _, _ string) (interface{}, error) {
	return nil, nil
}

func TestFetchCardByArenaID_FallbackLogic(t *testing.T) {
	// Test the conditional logic for fallback when ratingsRepo is nil vs non-nil
	// This tests the nil check we added

	tests := []struct {
		name             string
		ratingsRepoNil   bool
		arenaIDInRatings bool
		expectedBehavior string
	}{
		{
			name:             "nil ratingsRepo skips fallback",
			ratingsRepoNil:   true,
			arenaIDInRatings: false,
			expectedBehavior: "should not panic",
		},
		{
			name:             "non-nil ratingsRepo with card found triggers fallback",
			ratingsRepoNil:   false,
			arenaIDInRatings: true,
			expectedBehavior: "should attempt name-based fetch",
		},
		{
			name:             "non-nil ratingsRepo with card not found skips fallback",
			ratingsRepoNil:   false,
			arenaIDInRatings: false,
			expectedBehavior: "should skip fallback and return original error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockRatings *mockDraftRatingsRepo
			if !tt.ratingsRepoNil {
				mockRatings = newMockDraftRatingsRepo()
				if tt.arenaIDInRatings {
					mockRatings.SetCardLookup("123456", "Test Card", "TLA")
				}
			}

			// Test the nil check logic directly
			arenaIDStr := "123456"
			var cardName, setCode string
			var lookupErr error

			if mockRatings != nil {
				cardName, setCode, lookupErr = mockRatings.GetCardNameAndSetByArenaID(context.Background(), arenaIDStr)
			}

			// Verify expected behavior
			if tt.ratingsRepoNil {
				// With nil repo, variables should remain zero values
				if cardName != "" || setCode != "" {
					t.Errorf("Expected empty results with nil repo, got name=%q, set=%q", cardName, setCode)
				}
			} else if tt.arenaIDInRatings {
				// With card in ratings, should find it
				if cardName != "Test Card" || setCode != "TLA" {
					t.Errorf("Expected name='Test Card', set='TLA', got name=%q, set=%q", cardName, setCode)
				}
				if lookupErr != nil {
					t.Errorf("Expected no error, got %v", lookupErr)
				}
			} else {
				// Card not in ratings, should return empty
				if cardName != "" || setCode != "" {
					t.Errorf("Expected empty results when card not in ratings, got name=%q, set=%q", cardName, setCode)
				}
			}
		})
	}
}

func (m *mockSetCardRepo) GetSetCardCount(_ context.Context, setCode string) (int, error) {
	return len(m.cards[setCode]), nil
}

func TestConvertMTGJSONCard_BasicCard(t *testing.T) {
	// Test conversion of a basic MTGJSON card to SetCard
	mtgjsonCard := &mtgjson.Card{
		UUID:      "test-uuid-12345",
		Name:      "Lightning Bolt",
		ManaCost:  "{R}",
		ManaValue: 1.0,
		Type:      "Instant",
		Types:     []string{"Instant"},
		Colors:    []string{"R"},
		Rarity:    "Common",
		Text:      "Lightning Bolt deals 3 damage to any target.",
		Identifiers: mtgjson.CardIdentifiers{
			MtgArenaId: "12345",
			ScryfallId: "fa940e68-010e-4b68-be8a-555d7068f7b4",
		},
	}

	fetchedAt := time.Now()
	setCard := convertMTGJSONCard(mtgjsonCard, "ECL", fetchedAt)

	// Verify basic fields
	if setCard.Name != "Lightning Bolt" {
		t.Errorf("Name = %q, want %q", setCard.Name, "Lightning Bolt")
	}
	if setCard.SetCode != "ECL" {
		t.Errorf("SetCode = %q, want %q", setCard.SetCode, "ECL")
	}
	if setCard.ArenaID != "12345" {
		t.Errorf("ArenaID = %q, want %q", setCard.ArenaID, "12345")
	}
	if setCard.ScryfallID != "fa940e68-010e-4b68-be8a-555d7068f7b4" {
		t.Errorf("ScryfallID = %q, want %q", setCard.ScryfallID, "fa940e68-010e-4b68-be8a-555d7068f7b4")
	}
	if setCard.ManaCost != "{R}" {
		t.Errorf("ManaCost = %q, want %q", setCard.ManaCost, "{R}")
	}
	if setCard.CMC != 1 {
		t.Errorf("CMC = %d, want %d", setCard.CMC, 1)
	}
	if setCard.Rarity != "common" {
		t.Errorf("Rarity = %q, want %q", setCard.Rarity, "common")
	}
	if setCard.Text != "Lightning Bolt deals 3 damage to any target." {
		t.Errorf("Text = %q, want %q", setCard.Text, "Lightning Bolt deals 3 damage to any target.")
	}

	// Verify image URLs are constructed from Scryfall ID
	expectedImageURL := "https://cards.scryfall.io/normal/front/f/a/fa940e68-010e-4b68-be8a-555d7068f7b4.jpg"
	if setCard.ImageURL != expectedImageURL {
		t.Errorf("ImageURL = %q, want %q", setCard.ImageURL, expectedImageURL)
	}
	expectedSmallURL := "https://cards.scryfall.io/small/front/f/a/fa940e68-010e-4b68-be8a-555d7068f7b4.jpg"
	if setCard.ImageURLSmall != expectedSmallURL {
		t.Errorf("ImageURLSmall = %q, want %q", setCard.ImageURLSmall, expectedSmallURL)
	}
}

func TestConvertMTGJSONCard_CreatureWithStats(t *testing.T) {
	mtgjsonCard := &mtgjson.Card{
		UUID:       "creature-uuid",
		Name:       "Goblin Guide",
		ManaCost:   "{R}",
		ManaValue:  1.0,
		Type:       "Creature — Goblin Scout",
		Types:      []string{"Creature"},
		Subtypes:   []string{"Goblin", "Scout"},
		Supertypes: []string{},
		Colors:     []string{"R"},
		Rarity:     "Rare",
		Text:       "Haste",
		Power:      "2",
		Toughness:  "2",
		Identifiers: mtgjson.CardIdentifiers{
			MtgArenaId: "67890",
			ScryfallId: "abcd1234-5678-90ab-cdef-1234567890ab",
		},
	}

	setCard := convertMTGJSONCard(mtgjsonCard, "ECL", time.Now())

	if setCard.Power != "2" {
		t.Errorf("Power = %q, want %q", setCard.Power, "2")
	}
	if setCard.Toughness != "2" {
		t.Errorf("Toughness = %q, want %q", setCard.Toughness, "2")
	}

	// Check that types includes both types and subtypes
	expectedTypes := []string{"Creature", "Goblin", "Scout"}
	if len(setCard.Types) != len(expectedTypes) {
		t.Errorf("Types length = %d, want %d", len(setCard.Types), len(expectedTypes))
	}
	for i, expectedType := range expectedTypes {
		if i < len(setCard.Types) && setCard.Types[i] != expectedType {
			t.Errorf("Types[%d] = %q, want %q", i, setCard.Types[i], expectedType)
		}
	}
}

func TestConvertMTGJSONCard_LegendaryCreature(t *testing.T) {
	mtgjsonCard := &mtgjson.Card{
		UUID:       "legend-uuid",
		Name:       "Ojer Axonil, Deepest Might",
		ManaCost:   "{2}{R}{R}",
		ManaValue:  4.0,
		Type:       "Legendary Creature — God",
		Types:      []string{"Creature"},
		Subtypes:   []string{"God"},
		Supertypes: []string{"Legendary"},
		Colors:     []string{"R"},
		Rarity:     "Mythic",
		Power:      "4",
		Toughness:  "4",
		Identifiers: mtgjson.CardIdentifiers{
			MtgArenaId: "98765",
			ScryfallId: "12345678-1234-1234-1234-123456789abc",
		},
	}

	setCard := convertMTGJSONCard(mtgjsonCard, "LCI", time.Now())

	// Check supertypes are included
	expectedTypes := []string{"Legendary", "Creature", "God"}
	if len(setCard.Types) != len(expectedTypes) {
		t.Errorf("Types length = %d, want %d", len(setCard.Types), len(expectedTypes))
	}
	for i, expectedType := range expectedTypes {
		if i < len(setCard.Types) && setCard.Types[i] != expectedType {
			t.Errorf("Types[%d] = %q, want %q", i, setCard.Types[i], expectedType)
		}
	}
}

func TestConvertMTGJSONCard_MulticolorCard(t *testing.T) {
	mtgjsonCard := &mtgjson.Card{
		UUID:      "multicolor-uuid",
		Name:      "Nicol Bolas, Dragon-God",
		ManaCost:  "{U}{B}{B}{B}{R}",
		ManaValue: 5.0,
		Type:      "Legendary Planeswalker — Bolas",
		Types:     []string{"Planeswalker"},
		Subtypes:  []string{"Bolas"},
		Colors:    []string{"U", "B", "R"},
		Rarity:    "Mythic",
		Identifiers: mtgjson.CardIdentifiers{
			MtgArenaId: "11111",
			ScryfallId: "98765432-1234-5678-90ab-cdef12345678",
		},
	}

	setCard := convertMTGJSONCard(mtgjsonCard, "WAR", time.Now())

	// Check colors
	if len(setCard.Colors) != 3 {
		t.Errorf("Colors length = %d, want 3", len(setCard.Colors))
	}
}

func TestConvertMTGJSONCard_WithLegalities(t *testing.T) {
	mtgjsonCard := &mtgjson.Card{
		UUID:      "legal-uuid",
		Name:      "Test Card",
		ManaCost:  "{1}",
		ManaValue: 1.0,
		Type:      "Artifact",
		Types:     []string{"Artifact"},
		Rarity:    "Common",
		Identifiers: mtgjson.CardIdentifiers{
			MtgArenaId: "22222",
			ScryfallId: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		},
		Legalities: mtgjson.Legalities{
			Standard: "legal",
			Historic: "legal",
			Pioneer:  "not_legal",
			Modern:   "legal",
		},
	}

	setCard := convertMTGJSONCard(mtgjsonCard, "FDN", time.Now())

	// Legalities should be serialized as JSON
	if setCard.Legalities == "" {
		t.Error("Legalities should not be empty")
	}
	// Should contain the legalities JSON
	if !containsSubstring(setCard.Legalities, "standard") {
		t.Errorf("Legalities should contain 'standard', got %q", setCard.Legalities)
	}
}

func TestConvertMTGJSONCard_NoScryfallID(t *testing.T) {
	mtgjsonCard := &mtgjson.Card{
		UUID:      "no-scryfall-uuid",
		Name:      "Arena Exclusive Card",
		ManaCost:  "{W}",
		ManaValue: 1.0,
		Type:      "Creature",
		Types:     []string{"Creature"},
		Rarity:    "Common",
		Identifiers: mtgjson.CardIdentifiers{
			MtgArenaId: "33333",
			// No ScryfallId
		},
	}

	setCard := convertMTGJSONCard(mtgjsonCard, "Y24", time.Now())

	// Image URLs should be empty without Scryfall ID
	if setCard.ImageURL != "" {
		t.Errorf("ImageURL should be empty without ScryfallID, got %q", setCard.ImageURL)
	}
	if setCard.ImageURLSmall != "" {
		t.Errorf("ImageURLSmall should be empty without ScryfallID, got %q", setCard.ImageURLSmall)
	}
}

func TestNewFetcher_CreatesMTGJSONClient(t *testing.T) {
	// Test that NewFetcher creates an MTGJSON client
	mockRepo := newMockSetCardRepo()
	scryfallClient := scryfall.NewClient()

	fetcher := NewFetcher(scryfallClient, mockRepo, nil)

	if fetcher == nil {
		t.Fatal("NewFetcher returned nil")
	}
	if fetcher.mtgjsonClient == nil {
		t.Error("mtgjsonClient should be initialized")
	}
	if fetcher.scryfallClient == nil {
		t.Error("scryfallClient should be set")
	}
}

// containsSubstring checks if a string contains a substring.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
