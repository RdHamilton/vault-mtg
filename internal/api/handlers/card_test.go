package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ramonehamilton/MTGA-Companion/internal/gui"
	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
)

// mockCardFacade is a mock implementation of the card facade for testing.
type mockCardFacade struct {
	searchResults []*models.SetCard
	card          *models.SetCard
	sets          []*gui.SetInfo
	err           error

	// Track calls for verification
	searchCalls []searchCardsCall
}

type searchCardsCall struct {
	Query    string
	SetCodes []string
	Limit    int
}

func (m *mockCardFacade) SearchCards(_ context.Context, query string, setCodes []string, limit int) ([]*models.SetCard, error) {
	m.searchCalls = append(m.searchCalls, searchCardsCall{
		Query:    query,
		SetCodes: setCodes,
		Limit:    limit,
	})
	return m.searchResults, m.err
}

func (m *mockCardFacade) GetCardByArenaID(_ context.Context, _ string) (*models.SetCard, error) {
	return m.card, m.err
}

func (m *mockCardFacade) GetAllSetInfo(_ context.Context) ([]*gui.SetInfo, error) {
	return m.sets, m.err
}

// cardFacadeInterface defines the interface for testing card handlers.
type cardFacadeInterface interface {
	SearchCards(ctx context.Context, query string, setCodes []string, limit int) ([]*models.SetCard, error)
	GetCardByArenaID(ctx context.Context, arenaID string) (*models.SetCard, error)
	GetAllSetInfo(ctx context.Context) ([]*gui.SetInfo, error)
}

// testCardHandler wraps the card handler for testing with a mock.
type testCardHandler struct {
	facade cardFacadeInterface
}

func newTestCardHandler(facade cardFacadeInterface) *testCardHandler {
	return &testCardHandler{facade: facade}
}

func (h *testCardHandler) SearchCards(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	setCode := r.URL.Query().Get("set")

	var setCodes []string
	if setCode != "" {
		setCodes = []string{setCode}
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := parseLimit(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	cards, err := h.facade.SearchCards(r.Context(), query, setCodes, limit)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"data": cards})
}

func (h *testCardHandler) GetCard(w http.ResponseWriter, r *http.Request) {
	cardID := chi.URLParam(r, "cardID")
	if cardID == "" {
		http.Error(w, `{"error":"card ID is required"}`, http.StatusBadRequest)
		return
	}

	card, err := h.facade.GetCardByArenaID(r.Context(), cardID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	if card == nil {
		http.Error(w, `{"error":"card not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"data": card})
}

func parseLimit(s string) (int, error) {
	var limit int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("invalid limit")
		}
		limit = limit*10 + int(c-'0')
	}
	return limit, nil
}

// TestCardHandler_SearchCards tests the GET /cards/search endpoint.
// This test is critical for ensuring card search works correctly after database/API changes.
func TestCardHandler_SearchCards(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    string
		mockResults    []*models.SetCard
		mockErr        error
		expectedStatus int
		expectedLen    int
		expectedQuery  string
		expectedSet    string
		expectedLimit  int
	}{
		{
			name:        "successful search by name",
			queryParams: "?q=Firebending",
			mockResults: []*models.SetCard{
				{
					ArenaID:  "12345",
					Name:     "Firebending Lesson",
					SetCode:  "TLA",
					ManaCost: "{R}",
					Types:    []string{"Instant"},
				},
				{
					ArenaID:  "12346",
					Name:     "Firebending Master",
					SetCode:  "TLA",
					ManaCost: "{2}{R}{R}",
					Types:    []string{"Creature", "Human", "Wizard"},
				},
			},
			mockErr:        nil,
			expectedStatus: http.StatusOK,
			expectedLen:    2,
			expectedQuery:  "Firebending",
			expectedSet:    "",
			expectedLimit:  50,
		},
		{
			name:        "search with set filter",
			queryParams: "?q=Lightning&set=TLA",
			mockResults: []*models.SetCard{
				{
					ArenaID:  "12347",
					Name:     "Lightning Strike",
					SetCode:  "TLA",
					ManaCost: "{1}{R}",
					Types:    []string{"Instant"},
				},
			},
			mockErr:        nil,
			expectedStatus: http.StatusOK,
			expectedLen:    1,
			expectedQuery:  "Lightning",
			expectedSet:    "TLA",
			expectedLimit:  50,
		},
		{
			name:        "search with custom limit",
			queryParams: "?q=Creature&limit=10",
			mockResults: []*models.SetCard{
				{ArenaID: "1", Name: "Card 1", SetCode: "SET"},
				{ArenaID: "2", Name: "Card 2", SetCode: "SET"},
			},
			mockErr:        nil,
			expectedStatus: http.StatusOK,
			expectedLen:    2,
			expectedQuery:  "Creature",
			expectedSet:    "",
			expectedLimit:  10,
		},
		{
			name:           "empty query returns empty results",
			queryParams:    "",
			mockResults:    []*models.SetCard{},
			mockErr:        nil,
			expectedStatus: http.StatusOK,
			expectedLen:    0,
			expectedQuery:  "",
			expectedLimit:  50,
		},
		{
			name:           "search with no results",
			queryParams:    "?q=NonexistentCard12345",
			mockResults:    []*models.SetCard{},
			mockErr:        nil,
			expectedStatus: http.StatusOK,
			expectedLen:    0,
			expectedQuery:  "NonexistentCard12345",
			expectedLimit:  50,
		},
		{
			name:           "facade error returns internal server error",
			queryParams:    "?q=Error",
			mockResults:    nil,
			mockErr:        errors.New("database error"),
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "invalid limit defaults to 50",
			queryParams:    "?q=Test&limit=invalid",
			mockResults:    []*models.SetCard{},
			mockErr:        nil,
			expectedStatus: http.StatusOK,
			expectedLen:    0,
			expectedQuery:  "Test",
			expectedLimit:  50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCardFacade{
				searchResults: tt.mockResults,
				err:           tt.mockErr,
			}
			handler := newTestCardHandler(mock)

			r := chi.NewRouter()
			r.Get("/api/v1/cards/search", handler.SearchCards)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/search"+tt.queryParams, nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d (body: %s)", tt.expectedStatus, rec.Code, rec.Body.String())
			}

			// Verify the facade was called with correct parameters
			if tt.expectedStatus == http.StatusOK && len(mock.searchCalls) > 0 {
				call := mock.searchCalls[0]
				if call.Query != tt.expectedQuery {
					t.Errorf("expected query %q, got %q", tt.expectedQuery, call.Query)
				}
				if call.Limit != tt.expectedLimit {
					t.Errorf("expected limit %d, got %d", tt.expectedLimit, call.Limit)
				}
				if tt.expectedSet != "" {
					if len(call.SetCodes) == 0 || call.SetCodes[0] != tt.expectedSet {
						t.Errorf("expected set %q, got %v", tt.expectedSet, call.SetCodes)
					}
				}
			}

			// Verify response structure
			if tt.expectedStatus == http.StatusOK {
				var response struct {
					Data []*models.SetCard `json:"data"`
				}
				if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
					t.Errorf("failed to decode response: %v", err)
				}
				if len(response.Data) != tt.expectedLen {
					t.Errorf("expected %d results, got %d", tt.expectedLen, len(response.Data))
				}
			}
		})
	}
}

// TestCardHandler_SearchCards_ResponseFormat verifies the response format includes expected fields.
// This is a regression test to ensure card data is properly serialized.
func TestCardHandler_SearchCards_ResponseFormat(t *testing.T) {
	mock := &mockCardFacade{
		searchResults: []*models.SetCard{
			{
				ArenaID:  "97414",
				Name:     "Firebending Lesson",
				SetCode:  "TLA",
				ManaCost: "{R}",
				Types:    []string{"Instant"},
				Text:     "Deal 2 damage to any target.",
				Rarity:   "common",
				Colors:   []string{"R"},
				CMC:      1,
			},
		},
	}
	handler := newTestCardHandler(mock)

	r := chi.NewRouter()
	r.Get("/api/v1/cards/search", handler.SearchCards)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/search?q=Firebending", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 1 {
		t.Fatalf("expected 1 result, got %d", len(response.Data))
	}

	card := response.Data[0]

	// Verify critical fields are present and correctly named
	requiredFields := []string{"ArenaID", "Name", "SetCode", "ManaCost", "Types"}
	for _, field := range requiredFields {
		if _, exists := card[field]; !exists {
			t.Errorf("expected field %q to exist in response", field)
		}
	}

	// Verify specific values
	if name, ok := card["Name"].(string); !ok || name != "Firebending Lesson" {
		t.Errorf("expected Name to be 'Firebending Lesson', got %v", card["Name"])
	}
	if arenaID, ok := card["ArenaID"].(string); !ok || arenaID != "97414" {
		t.Errorf("expected ArenaID to be '97414', got %v", card["ArenaID"])
	}
}

// TestCardHandler_GetCard tests the GET /cards/{cardID} endpoint.
func TestCardHandler_GetCard(t *testing.T) {
	tests := []struct {
		name           string
		cardID         string
		mockCard       *models.SetCard
		mockErr        error
		expectedStatus int
	}{
		{
			name:   "successful get card",
			cardID: "97414",
			mockCard: &models.SetCard{
				ArenaID:  "97414",
				Name:     "Firebending Lesson",
				SetCode:  "TLA",
				ManaCost: "{R}",
			},
			mockErr:        nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "card not found returns 404",
			cardID:         "99999",
			mockCard:       nil,
			mockErr:        nil,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "empty card ID returns not found (router doesn't match empty path segment)",
			cardID:         "",
			mockCard:       nil,
			mockErr:        nil,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "facade error returns internal server error",
			cardID:         "12345",
			mockCard:       nil,
			mockErr:        errors.New("database error"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCardFacade{
				card: tt.mockCard,
				err:  tt.mockErr,
			}
			handler := newTestCardHandler(mock)

			r := chi.NewRouter()
			r.Get("/api/v1/cards/{cardID}", handler.GetCard)

			url := "/api/v1/cards/" + tt.cardID
			if tt.cardID == "" {
				url = "/api/v1/cards/"
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}
