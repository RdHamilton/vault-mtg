package mtgjson

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.httpClient == nil {
		t.Error("httpClient is nil")
	}
	if client.rateLimiter == nil {
		t.Error("rateLimiter is nil")
	}
	if client.userAgent == "" {
		t.Error("userAgent is empty")
	}
}

func TestClient_GetSet_Success(t *testing.T) {
	// Create mock server
	mockSetFile := SetFile{
		Data: SetData{
			Code:        "ECL",
			Name:        "Lorwyn Eclipsed",
			ReleaseDate: "2025-01-17",
			BaseSetSize: 286,
			Cards: []Card{
				{
					UUID: "test-uuid-1",
					Name: "Test Card 1",
					Identifiers: CardIdentifiers{
						MtgArenaId: "12345",
						ScryfallId: "abc-123",
					},
					Rarity:   "common",
					ManaCost: "{W}",
				},
				{
					UUID: "test-uuid-2",
					Name: "Test Card 2",
					Identifiers: CardIdentifiers{
						MtgArenaId: "12346",
						ScryfallId: "def-456",
					},
					Rarity:   "rare",
					ManaCost: "{2}{U}{U}",
				},
			},
		},
		Meta: Meta{
			Date:    "2025-01-17",
			Version: "5.2.0",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ECL.json" {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(mockSetFile); err != nil {
				t.Fatalf("failed to encode response: %v", err)
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// Create client with mock server
	client := &Client{
		httpClient:  server.Client(),
		rateLimiter: NewClient().rateLimiter,
		userAgent:   "test",
	}

	// Override base URL by using the mock server directly
	ctx := context.Background()
	var setFile SetFile
	err := client.doRequest(ctx, server.URL+"/ECL.json", &setFile)
	if err != nil {
		t.Fatalf("GetSet failed: %v", err)
	}

	if setFile.Data.Code != "ECL" {
		t.Errorf("Code = %q, want %q", setFile.Data.Code, "ECL")
	}
	if len(setFile.Data.Cards) != 2 {
		t.Errorf("len(Cards) = %d, want 2", len(setFile.Data.Cards))
	}
	if setFile.Data.Cards[0].Name != "Test Card 1" {
		t.Errorf("Cards[0].Name = %q, want %q", setFile.Data.Cards[0].Name, "Test Card 1")
	}
}

func TestClient_GetSet_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := &Client{
		httpClient:  server.Client(),
		rateLimiter: NewClient().rateLimiter,
		userAgent:   "test",
	}

	ctx := context.Background()
	var setFile SetFile
	err := client.doRequest(ctx, server.URL+"/INVALID.json", &setFile)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestClient_GetSet_RateLimited(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		// Success on second attempt
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"data": {"code": "ECL", "name": "Test", "cards": []}, "meta": {}}`)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client := &Client{
		httpClient:  server.Client(),
		rateLimiter: NewClient().rateLimiter,
		userAgent:   "test",
	}

	ctx := context.Background()
	var setFile SetFile
	err := client.doRequest(ctx, server.URL+"/ECL.json", &setFile)
	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}

	if attempts < 2 {
		t.Errorf("expected at least 2 attempts, got %d", attempts)
	}
}

func TestClient_GetSet_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create client with very short timeout
	client := &Client{
		httpClient: &http.Client{
			Timeout: 100 * time.Millisecond,
		},
		rateLimiter: NewClient().rateLimiter,
		userAgent:   "test",
	}

	ctx := context.Background()
	var setFile SetFile
	err := client.doRequest(ctx, server.URL+"/ECL.json", &setFile)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestClient_GetSetCardsWithArenaIDs(t *testing.T) {
	mockSetFile := SetFile{
		Data: SetData{
			Code: "ECL",
			Cards: []Card{
				{
					UUID: "uuid-1",
					Name: "Card With Arena ID",
					Identifiers: CardIdentifiers{
						MtgArenaId: "12345",
					},
				},
				{
					UUID: "uuid-2",
					Name: "Card Without Arena ID",
					Identifiers: CardIdentifiers{
						ScryfallId: "abc-123",
					},
				},
				{
					UUID: "uuid-3",
					Name: "Another Card With Arena ID",
					Identifiers: CardIdentifiers{
						MtgArenaId: "12346",
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(mockSetFile); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Test filtering by creating a custom client that hits the mock server
	client := &Client{
		httpClient:  server.Client(),
		rateLimiter: NewClient().rateLimiter,
		userAgent:   "test",
	}

	// Call doRequest directly to get all cards
	ctx := context.Background()
	var setFile SetFile
	err := client.doRequest(ctx, server.URL+"/ECL.json", &setFile)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	// Filter cards with Arena IDs manually (simulating GetSetCardsWithArenaIDs)
	arenaCards := make([]Card, 0)
	for _, card := range setFile.Data.Cards {
		if card.HasArenaID() {
			arenaCards = append(arenaCards, card)
		}
	}

	if len(arenaCards) != 2 {
		t.Errorf("expected 2 cards with Arena IDs, got %d", len(arenaCards))
	}
}

func TestNotFoundError(t *testing.T) {
	err := &NotFoundError{SetCode: "INVALID"}
	if err.Error() != "set not found: INVALID" {
		t.Errorf("Error() = %q, want %q", err.Error(), "set not found: INVALID")
	}

	if !IsNotFound(err) {
		t.Error("IsNotFound should return true for NotFoundError")
	}

	var regularErr error = err
	if !IsNotFound(regularErr) {
		t.Error("IsNotFound should work with error interface")
	}
}

func TestClient_GetSet_WrapperFunction(t *testing.T) {
	mockSetFile := SetFile{
		Data: SetData{
			Code:        "ECL",
			Name:        "Lorwyn Eclipsed",
			ReleaseDate: "2025-01-17",
			Cards: []Card{
				{
					UUID: "uuid-1",
					Name: "Test Card",
					Identifiers: CardIdentifiers{
						MtgArenaId: "12345",
					},
				},
			},
		},
		Meta: Meta{
			Date:    "2025-01-17",
			Version: "5.2.0",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request path is uppercase
		if r.URL.Path != "/ECL.json" {
			t.Errorf("Expected /ECL.json, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(mockSetFile); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := &Client{
		httpClient:  server.Client(),
		rateLimiter: NewClient().rateLimiter,
		userAgent:   "test",
		baseURL:     server.URL,
	}

	ctx := context.Background()
	setFile, err := client.GetSet(ctx, "ecl") // lowercase input
	if err != nil {
		t.Fatalf("GetSet failed: %v", err)
	}

	if setFile.Data.Code != "ECL" {
		t.Errorf("Code = %q, want %q", setFile.Data.Code, "ECL")
	}
	if len(setFile.Data.Cards) != 1 {
		t.Errorf("len(Cards) = %d, want 1", len(setFile.Data.Cards))
	}
}

func TestClient_GetSetCards_WrapperFunction(t *testing.T) {
	mockSetFile := SetFile{
		Data: SetData{
			Code: "ECL",
			Cards: []Card{
				{UUID: "uuid-1", Name: "Card 1"},
				{UUID: "uuid-2", Name: "Card 2"},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(mockSetFile); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := &Client{
		httpClient:  server.Client(),
		rateLimiter: NewClient().rateLimiter,
		userAgent:   "test",
		baseURL:     server.URL,
	}

	ctx := context.Background()
	cards, err := client.GetSetCards(ctx, "ECL")
	if err != nil {
		t.Fatalf("GetSetCards failed: %v", err)
	}

	if len(cards) != 2 {
		t.Errorf("len(cards) = %d, want 2", len(cards))
	}
	if cards[0].Name != "Card 1" {
		t.Errorf("cards[0].Name = %q, want %q", cards[0].Name, "Card 1")
	}
}

func TestClient_GetSetCardsWithArenaIDs_WrapperFunction(t *testing.T) {
	mockSetFile := SetFile{
		Data: SetData{
			Code: "ECL",
			Cards: []Card{
				{UUID: "uuid-1", Name: "With Arena ID", Identifiers: CardIdentifiers{MtgArenaId: "12345"}},
				{UUID: "uuid-2", Name: "Without Arena ID"},
				{UUID: "uuid-3", Name: "Another With Arena ID", Identifiers: CardIdentifiers{MtgArenaId: "12346"}},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(mockSetFile); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := &Client{
		httpClient:  server.Client(),
		rateLimiter: NewClient().rateLimiter,
		userAgent:   "test",
		baseURL:     server.URL,
	}

	ctx := context.Background()
	cards, err := client.GetSetCardsWithArenaIDs(ctx, "ECL")
	if err != nil {
		t.Fatalf("GetSetCardsWithArenaIDs failed: %v", err)
	}

	if len(cards) != 2 {
		t.Errorf("len(cards) = %d, want 2 (only cards with Arena IDs)", len(cards))
	}
	if cards[0].Name != "With Arena ID" {
		t.Errorf("cards[0].Name = %q, want %q", cards[0].Name, "With Arena ID")
	}
}

func TestClient_GetSetCards_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := &Client{
		httpClient:  server.Client(),
		rateLimiter: NewClient().rateLimiter,
		userAgent:   "test",
		baseURL:     server.URL,
	}

	ctx := context.Background()
	_, err := client.GetSetCards(ctx, "INVALID")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestClient_getBaseURL(t *testing.T) {
	// Test with empty baseURL (should use default)
	client := &Client{}
	if client.getBaseURL() != baseURL {
		t.Errorf("getBaseURL() = %q, want %q", client.getBaseURL(), baseURL)
	}

	// Test with custom baseURL
	client = &Client{baseURL: "http://custom.example.com"}
	if client.getBaseURL() != "http://custom.example.com" {
		t.Errorf("getBaseURL() = %q, want %q", client.getBaseURL(), "http://custom.example.com")
	}
}
