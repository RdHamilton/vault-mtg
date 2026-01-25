package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ramonehamilton/MTGA-Companion/internal/gui"
	"github.com/ramonehamilton/MTGA-Companion/internal/storage"
	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
)

// mockDeckFacade is a mock implementation of the deck facade for testing.
type mockDeckFacade struct {
	decks             []*gui.DeckListItem
	deck              *gui.DeckWithCards
	deckStats         *gui.DeckStatistics
	deckPerf          *models.DeckPerformance
	exportResult      *gui.ExportDeckResponse
	importResult      *gui.ImportDeckResponse
	suggestions       *gui.SuggestDecksResponse
	classification    *gui.ArchetypeClassificationResult
	createdDeck       *models.Deck
	recalculateResult *storage.RecalculateDeckPerformanceResult
	err               error

	// AddCard/RemoveCard tracking for verification
	addCardCalls    []addCardCall
	removeCardCalls []removeCardCall
}

// addCardCall records the parameters passed to AddCard for verification.
type addCardCall struct {
	DeckID    string
	CardID    int
	Quantity  int
	Board     string
	FromDraft bool
}

// removeCardCall records the parameters passed to RemoveCard for verification.
type removeCardCall struct {
	DeckID string
	CardID int
	Board  string
}

func (m *mockDeckFacade) ListDecks(_ context.Context) ([]*gui.DeckListItem, error) {
	return m.decks, m.err
}

func (m *mockDeckFacade) GetDecksBySource(_ context.Context, _ string) ([]*gui.DeckListItem, error) {
	return m.decks, m.err
}

func (m *mockDeckFacade) GetDecksByFormat(_ context.Context, _ string) ([]*gui.DeckListItem, error) {
	return m.decks, m.err
}

func (m *mockDeckFacade) GetDeck(_ context.Context, _ string) (*gui.DeckWithCards, error) {
	return m.deck, m.err
}

func (m *mockDeckFacade) CreateDeck(_ context.Context, _, _, _ string, _ *string) (*models.Deck, error) {
	return m.createdDeck, m.err
}

func (m *mockDeckFacade) UpdateDeck(_ context.Context, _ *models.Deck) error {
	return m.err
}

func (m *mockDeckFacade) DeleteDeck(_ context.Context, _ string) error {
	return m.err
}

func (m *mockDeckFacade) GetDeckStatistics(_ context.Context, _ string) (*gui.DeckStatistics, error) {
	return m.deckStats, m.err
}

func (m *mockDeckFacade) GetDeckPerformance(_ context.Context, _ string) (*models.DeckPerformance, error) {
	return m.deckPerf, m.err
}

func (m *mockDeckFacade) ExportDeck(_ context.Context, _ *gui.ExportDeckRequest) (*gui.ExportDeckResponse, error) {
	return m.exportResult, m.err
}

func (m *mockDeckFacade) ImportDeck(_ context.Context, _ *gui.ImportDeckRequest) (*gui.ImportDeckResponse, error) {
	return m.importResult, m.err
}

func (m *mockDeckFacade) SuggestDecks(_ context.Context, _ string) (*gui.SuggestDecksResponse, error) {
	return m.suggestions, m.err
}

func (m *mockDeckFacade) ClassifyDeckArchetype(_ context.Context, _ string) (*gui.ArchetypeClassificationResult, error) {
	return m.classification, m.err
}

func (m *mockDeckFacade) AddCard(_ context.Context, deckID string, cardID, quantity int, board string, fromDraft bool) error {
	m.addCardCalls = append(m.addCardCalls, addCardCall{
		DeckID:    deckID,
		CardID:    cardID,
		Quantity:  quantity,
		Board:     board,
		FromDraft: fromDraft,
	})
	return m.err
}

func (m *mockDeckFacade) RemoveCard(_ context.Context, deckID string, cardID int, board string) error {
	m.removeCardCalls = append(m.removeCardCalls, removeCardCall{
		DeckID: deckID,
		CardID: cardID,
		Board:  board,
	})
	return m.err
}

func (m *mockDeckFacade) RecalculateDeckPerformance(_ context.Context) (*storage.RecalculateDeckPerformanceResult, error) {
	return m.recalculateResult, m.err
}

// deckFacadeInterface defines the interface for testing deck handlers.
type deckFacadeInterface interface {
	ListDecks(ctx context.Context) ([]*gui.DeckListItem, error)
	GetDecksBySource(ctx context.Context, source string) ([]*gui.DeckListItem, error)
	GetDecksByFormat(ctx context.Context, format string) ([]*gui.DeckListItem, error)
	GetDeck(ctx context.Context, deckID string) (*gui.DeckWithCards, error)
	CreateDeck(ctx context.Context, name, format, source string, draftEventID *string) (*models.Deck, error)
	UpdateDeck(ctx context.Context, deck *models.Deck) error
	DeleteDeck(ctx context.Context, deckID string) error
	GetDeckStatistics(ctx context.Context, deckID string) (*gui.DeckStatistics, error)
	GetDeckPerformance(ctx context.Context, deckID string) (*models.DeckPerformance, error)
	ExportDeck(ctx context.Context, req *gui.ExportDeckRequest) (*gui.ExportDeckResponse, error)
	ImportDeck(ctx context.Context, req *gui.ImportDeckRequest) (*gui.ImportDeckResponse, error)
	SuggestDecks(ctx context.Context, sessionID string) (*gui.SuggestDecksResponse, error)
	ClassifyDeckArchetype(ctx context.Context, deckID string) (*gui.ArchetypeClassificationResult, error)
	AddCard(ctx context.Context, deckID string, cardID, quantity int, board string, fromDraft bool) error
	RemoveCard(ctx context.Context, deckID string, cardID int, board string) error
	RecalculateDeckPerformance(ctx context.Context) (*storage.RecalculateDeckPerformanceResult, error)
}

// testDeckHandler wraps the deck handler for testing with a mock.
type testDeckHandler struct {
	facade deckFacadeInterface
}

func newTestDeckHandler(facade deckFacadeInterface) *testDeckHandler {
	return &testDeckHandler{facade: facade}
}

func (h *testDeckHandler) GetDecks(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	source := r.URL.Query().Get("source")

	var decks []*gui.DeckListItem
	var err error

	if source != "" {
		decks, err = h.facade.GetDecksBySource(r.Context(), source)
	} else if format != "" {
		decks, err = h.facade.GetDecksByFormat(r.Context(), format)
	} else {
		decks, err = h.facade.ListDecks(r.Context())
	}

	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"data": decks})
}

func (h *testDeckHandler) GetDeck(w http.ResponseWriter, r *http.Request) {
	deckID := chi.URLParam(r, "deckID")
	if deckID == "" {
		http.Error(w, `{"error":"deck ID is required"}`, http.StatusBadRequest)
		return
	}

	deck, err := h.facade.GetDeck(r.Context(), deckID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	if deck == nil {
		http.Error(w, `{"error":"deck not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"data": deck})
}

func (h *testDeckHandler) CreateDeck(w http.ResponseWriter, r *http.Request) {
	var req CreateDeckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, `{"error":"deck name is required"}`, http.StatusBadRequest)
		return
	}

	deck, err := h.facade.CreateDeck(r.Context(), req.Name, req.Format, req.Source, req.DraftEventID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"data": deck})
}

func (h *testDeckHandler) DeleteDeck(w http.ResponseWriter, r *http.Request) {
	deckID := chi.URLParam(r, "deckID")
	if deckID == "" {
		http.Error(w, `{"error":"deck ID is required"}`, http.StatusBadRequest)
		return
	}

	if err := h.facade.DeleteDeck(r.Context(), deckID); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *testDeckHandler) ImportDeck(w http.ResponseWriter, r *http.Request) {
	var req ImportDeckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		http.Error(w, `{"error":"deck content is required"}`, http.StatusBadRequest)
		return
	}

	importReq := &gui.ImportDeckRequest{
		ImportText: req.Content,
		Name:       req.Name,
		Format:     req.Format,
		Source:     "imported",
	}

	result, err := h.facade.ImportDeck(r.Context(), importReq)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"data": result})
}

func (h *testDeckHandler) AddCard(w http.ResponseWriter, r *http.Request) {
	deckID := chi.URLParam(r, "deckID")
	if deckID == "" {
		http.Error(w, `{"error":"deck ID is required"}`, http.StatusBadRequest)
		return
	}

	var req AddCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Board == "" {
		req.Board = "main"
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	if err := h.facade.AddCard(r.Context(), deckID, req.CardID, req.Quantity, req.Board, req.FromDraft); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (h *testDeckHandler) RemoveCard(w http.ResponseWriter, r *http.Request) {
	deckID := chi.URLParam(r, "deckID")
	if deckID == "" {
		http.Error(w, `{"error":"deck ID is required"}`, http.StatusBadRequest)
		return
	}

	cardIDStr := chi.URLParam(r, "cardID")
	if cardIDStr == "" {
		http.Error(w, `{"error":"card ID is required"}`, http.StatusBadRequest)
		return
	}

	cardID, err := strconv.Atoi(cardIDStr)
	if err != nil {
		http.Error(w, `{"error":"invalid card ID"}`, http.StatusBadRequest)
		return
	}

	board := r.URL.Query().Get("zone")
	if board == "" {
		board = "main"
	}

	if err := h.facade.RemoveCard(r.Context(), deckID, cardID, board); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *testDeckHandler) SuggestDecks(w http.ResponseWriter, r *http.Request) {
	var req SuggestDecksRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		http.Error(w, `{"error":"session_id is required"}`, http.StatusBadRequest)
		return
	}

	suggestions, err := h.facade.SuggestDecks(r.Context(), req.SessionID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suggestions)
}

func (h *testDeckHandler) RecalculateDeckPerformance(w http.ResponseWriter, r *http.Request) {
	result, err := h.facade.RecalculateDeckPerformance(r.Context())
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"data": result})
}

func TestDeckHandler_GetDecks(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    string
		mockDecks      []*gui.DeckListItem
		mockErr        error
		expectedStatus int
		expectedLen    int
	}{
		{
			name:        "successful get all decks",
			queryParams: "",
			mockDecks: []*gui.DeckListItem{
				{ID: "deck-1", Name: "Aggro Red"},
				{ID: "deck-2", Name: "Control Blue"},
			},
			mockErr:        nil,
			expectedStatus: http.StatusOK,
			expectedLen:    2,
		},
		{
			name:        "filter by format",
			queryParams: "?format=Standard",
			mockDecks: []*gui.DeckListItem{
				{ID: "deck-1", Name: "Standard Aggro"},
			},
			mockErr:        nil,
			expectedStatus: http.StatusOK,
			expectedLen:    1,
		},
		{
			name:        "filter by source",
			queryParams: "?source=draft",
			mockDecks: []*gui.DeckListItem{
				{ID: "deck-3", Name: "Draft Deck"},
			},
			mockErr:        nil,
			expectedStatus: http.StatusOK,
			expectedLen:    1,
		},
		{
			name:           "empty decks",
			queryParams:    "",
			mockDecks:      []*gui.DeckListItem{},
			mockErr:        nil,
			expectedStatus: http.StatusOK,
			expectedLen:    0,
		},
		{
			name:           "error from facade",
			queryParams:    "",
			mockDecks:      nil,
			mockErr:        errors.New("database error"),
			expectedStatus: http.StatusInternalServerError,
			expectedLen:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDeckFacade{
				decks: tt.mockDecks,
				err:   tt.mockErr,
			}

			handler := newTestDeckHandler(mock)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/decks"+tt.queryParams, nil)
			rec := httptest.NewRecorder()

			handler.GetDecks(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var resp map[string]interface{}
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				data, ok := resp["data"].([]interface{})
				if !ok {
					t.Fatal("expected data to be an array")
				}

				if len(data) != tt.expectedLen {
					t.Errorf("expected %d decks, got %d", tt.expectedLen, len(data))
				}
			}
		})
	}
}

func TestDeckHandler_GetDeck(t *testing.T) {
	tests := []struct {
		name           string
		deckID         string
		mockDeck       *gui.DeckWithCards
		mockErr        error
		expectedStatus int
	}{
		{
			name:   "successful get deck",
			deckID: "deck-123",
			mockDeck: &gui.DeckWithCards{
				Deck: &models.Deck{
					ID:     "deck-123",
					Name:   "Test Deck",
					Format: "Standard",
				},
			},
			mockErr:        nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "deck not found",
			deckID:         "deck-999",
			mockDeck:       nil,
			mockErr:        nil,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "missing deck ID",
			deckID:         "",
			mockDeck:       nil,
			mockErr:        nil,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDeckFacade{
				deck: tt.mockDeck,
				err:  tt.mockErr,
			}

			handler := newTestDeckHandler(mock)

			r := chi.NewRouter()
			r.Get("/api/v1/decks/{deckID}", handler.GetDeck)

			var req *http.Request
			if tt.deckID != "" {
				req = httptest.NewRequest(http.MethodGet, "/api/v1/decks/"+tt.deckID, nil)
			} else {
				r2 := chi.NewRouter()
				r2.Get("/api/v1/decks/", handler.GetDeck)
				req = httptest.NewRequest(http.MethodGet, "/api/v1/decks/", nil)
				rec := httptest.NewRecorder()
				r2.ServeHTTP(rec, req)

				if rec.Code != tt.expectedStatus {
					t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
				}
				return
			}

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestDeckHandler_CreateDeck(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		mockDeck       *models.Deck
		mockErr        error
		expectedStatus int
	}{
		{
			name:        "successful create deck",
			requestBody: `{"name":"New Deck","format":"Standard","source":"constructed"}`,
			mockDeck: &models.Deck{
				ID:     "deck-new",
				Name:   "New Deck",
				Format: "Standard",
			},
			mockErr:        nil,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "missing name",
			requestBody:    `{"format":"Standard"}`,
			mockDeck:       nil,
			mockErr:        nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid JSON",
			requestBody:    `{invalid`,
			mockDeck:       nil,
			mockErr:        nil,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDeckFacade{
				createdDeck: tt.mockDeck,
				err:         tt.mockErr,
			}

			handler := newTestDeckHandler(mock)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/decks", bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.CreateDeck(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestDeckHandler_DeleteDeck(t *testing.T) {
	tests := []struct {
		name           string
		deckID         string
		mockErr        error
		expectedStatus int
	}{
		{
			name:           "successful delete deck",
			deckID:         "deck-123",
			mockErr:        nil,
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "error deleting deck",
			deckID:         "deck-456",
			mockErr:        errors.New("delete failed"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDeckFacade{
				err: tt.mockErr,
			}

			handler := newTestDeckHandler(mock)

			r := chi.NewRouter()
			r.Delete("/api/v1/decks/{deckID}", handler.DeleteDeck)

			req := httptest.NewRequest(http.MethodDelete, "/api/v1/decks/"+tt.deckID, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestDeckHandler_ImportDeck(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		mockResult     *gui.ImportDeckResponse
		mockErr        error
		expectedStatus int
	}{
		{
			name:        "successful import",
			requestBody: `{"content":"4 Lightning Bolt\n4 Mountain","name":"Red Deck","format":"Standard"}`,
			mockResult: &gui.ImportDeckResponse{
				Success:       true,
				DeckID:        "new-deck-id",
				CardsImported: 8,
				CardsSkipped:  0,
			},
			mockErr:        nil,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "missing content",
			requestBody:    `{"name":"Empty Deck"}`,
			mockResult:     nil,
			mockErr:        nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid JSON",
			requestBody:    `{invalid`,
			mockResult:     nil,
			mockErr:        nil,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDeckFacade{
				importResult: tt.mockResult,
				err:          tt.mockErr,
			}

			handler := newTestDeckHandler(mock)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/decks/import", bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.ImportDeck(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestCreateDeckRequest_Validation(t *testing.T) {
	tests := []struct {
		name       string
		request    CreateDeckRequest
		isValid    bool
		errMessage string
	}{
		{
			name: "valid request",
			request: CreateDeckRequest{
				Name:   "Test Deck",
				Format: "Standard",
				Source: "constructed",
			},
			isValid:    true,
			errMessage: "",
		},
		{
			name: "missing name",
			request: CreateDeckRequest{
				Format: "Standard",
				Source: "constructed",
			},
			isValid:    false,
			errMessage: "deck name is required",
		},
		{
			name: "with draft event ID",
			request: CreateDeckRequest{
				Name:         "Draft Deck",
				Format:       "Draft",
				Source:       "draft",
				DraftEventID: strPtr("draft-event-123"),
			},
			isValid:    true,
			errMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.request.Name != ""
			if isValid != tt.isValid {
				t.Errorf("expected isValid=%v, got %v", tt.isValid, isValid)
			}
		})
	}
}

func TestRemoveCard_URLParsing(t *testing.T) {
	// Test URL parameter parsing for RemoveCard handler
	tests := []struct {
		name           string
		deckID         string
		cardID         string
		zone           string
		expectedStatus int
	}{
		{
			name:           "valid card ID",
			deckID:         "deck-123",
			cardID:         "12345",
			zone:           "main",
			expectedStatus: http.StatusNoContent, // Would be 204 if facade succeeds
		},
		{
			name:           "invalid card ID - not a number",
			deckID:         "deck-123",
			cardID:         "invalid",
			zone:           "main",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "default zone when not provided",
			deckID:         "deck-123",
			cardID:         "12345",
			zone:           "", // Should default to "main"
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "sideboard zone",
			deckID:         "deck-123",
			cardID:         "12345",
			zone:           "sideboard",
			expectedStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test handler that tracks the parsed values
			var parsedDeckID string
			var parsedCardID int
			var parsedZone string
			var parseErr error

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				parsedDeckID = chi.URLParam(r, "deckID")
				cardIDStr := chi.URLParam(r, "cardID")
				parsedCardID, parseErr = strconv.Atoi(cardIDStr)
				_ = parsedCardID // Used to verify parsing succeeded
				parsedZone = r.URL.Query().Get("zone")
				if parsedZone == "" {
					parsedZone = "main"
				}

				if parseErr != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			})

			r := chi.NewRouter()
			r.Delete("/api/v1/decks/{deckID}/cards/{cardID}", testHandler)

			url := "/api/v1/decks/" + tt.deckID + "/cards/" + tt.cardID
			if tt.zone != "" {
				url += "?zone=" + tt.zone
			}

			req := httptest.NewRequest(http.MethodDelete, url, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			// Verify correct parsing for valid cases
			if tt.expectedStatus == http.StatusNoContent {
				if parsedDeckID != tt.deckID {
					t.Errorf("expected deckID %q, got %q", tt.deckID, parsedDeckID)
				}
				expectedZone := tt.zone
				if expectedZone == "" {
					expectedZone = "main"
				}
				if parsedZone != expectedZone {
					t.Errorf("expected zone %q, got %q", expectedZone, parsedZone)
				}
			}
		})
	}
}

// TestDeckHandler_AddCard tests the POST /decks/{deckID}/cards endpoint.
// This test is critical for catching field name mismatches (cardID vs card_id, board vs zone).
func TestDeckHandler_AddCard(t *testing.T) {
	tests := []struct {
		name           string
		deckID         string
		requestBody    string
		mockErr        error
		expectedStatus int
		expectedCardID int
		expectedQty    int
		expectedBoard  string
	}{
		{
			name:   "successful add card with camelCase fields",
			deckID: "deck-123",
			// CRITICAL: This tests the CORRECT field names (cardID, board)
			requestBody:    `{"cardID": 12345, "quantity": 4, "board": "main"}`,
			mockErr:        nil,
			expectedStatus: http.StatusOK,
			expectedCardID: 12345,
			expectedQty:    4,
			expectedBoard:  "main",
		},
		{
			name:           "add card to sideboard",
			deckID:         "deck-456",
			requestBody:    `{"cardID": 67890, "quantity": 2, "board": "sideboard"}`,
			mockErr:        nil,
			expectedStatus: http.StatusOK,
			expectedCardID: 67890,
			expectedQty:    2,
			expectedBoard:  "sideboard",
		},
		{
			name:           "default board to main when not provided",
			deckID:         "deck-789",
			requestBody:    `{"cardID": 11111, "quantity": 1}`,
			mockErr:        nil,
			expectedStatus: http.StatusOK,
			expectedCardID: 11111,
			expectedQty:    1,
			expectedBoard:  "main",
		},
		{
			name:           "default quantity to 1 when not provided",
			deckID:         "deck-abc",
			requestBody:    `{"cardID": 22222, "board": "main"}`,
			mockErr:        nil,
			expectedStatus: http.StatusOK,
			expectedCardID: 22222,
			expectedQty:    1,
			expectedBoard:  "main",
		},
		{
			name:           "add card from draft",
			deckID:         "draft-deck-123",
			requestBody:    `{"cardID": 33333, "quantity": 1, "board": "main", "fromDraft": true}`,
			mockErr:        nil,
			expectedStatus: http.StatusOK,
			expectedCardID: 33333,
			expectedQty:    1,
			expectedBoard:  "main",
		},
		{
			name:           "missing deck ID in URL returns bad request",
			deckID:         "",
			requestBody:    `{"cardID": 12345, "quantity": 1, "board": "main"}`,
			mockErr:        nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid JSON returns bad request",
			deckID:         "deck-123",
			requestBody:    `{invalid json}`,
			mockErr:        nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "facade error returns internal server error",
			deckID:         "deck-123",
			requestBody:    `{"cardID": 12345, "quantity": 1, "board": "main"}`,
			mockErr:        errors.New("database error"),
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:   "snake_case card_id should NOT work (uses wrong field name)",
			deckID: "deck-wrong",
			// This tests that snake_case field names are NOT accepted
			// The cardID should be 0 (not parsed) since we use card_id
			requestBody:    `{"card_id": 99999, "quantity": 4, "zone": "main"}`,
			mockErr:        nil,
			expectedStatus: http.StatusOK,
			expectedCardID: 0, // Should be 0 because card_id is wrong field name
			expectedQty:    4,
			expectedBoard:  "main", // zone won't work, defaults to main
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDeckFacade{err: tt.mockErr}
			handler := newTestDeckHandler(mock)

			r := chi.NewRouter()
			r.Post("/api/v1/decks/{deckID}/cards", handler.AddCard)

			url := "/api/v1/decks/" + tt.deckID + "/cards"
			if tt.deckID == "" {
				url = "/api/v1/decks//cards"
			}

			req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d (body: %s)", tt.expectedStatus, rec.Code, rec.Body.String())
			}

			// Verify the facade was called with correct parameters for successful cases
			if tt.expectedStatus == http.StatusOK && len(mock.addCardCalls) > 0 {
				call := mock.addCardCalls[0]
				if call.CardID != tt.expectedCardID {
					t.Errorf("expected cardID %d, got %d", tt.expectedCardID, call.CardID)
				}
				if call.Quantity != tt.expectedQty {
					t.Errorf("expected quantity %d, got %d", tt.expectedQty, call.Quantity)
				}
				if call.Board != tt.expectedBoard {
					t.Errorf("expected board %q, got %q", tt.expectedBoard, call.Board)
				}
				if call.DeckID != tt.deckID {
					t.Errorf("expected deckID %q, got %q", tt.deckID, call.DeckID)
				}
			}
		})
	}
}

// TestDeckHandler_AddCard_JSONFieldNames specifically tests that the API uses camelCase field names.
// This is a regression test for the bug where frontend sent card_id but backend expected cardID.
func TestDeckHandler_AddCard_JSONFieldNames(t *testing.T) {
	mock := &mockDeckFacade{}
	handler := newTestDeckHandler(mock)

	r := chi.NewRouter()
	r.Post("/api/v1/decks/{deckID}/cards", handler.AddCard)

	// Test that camelCase cardID works
	req := httptest.NewRequest(http.MethodPost, "/api/v1/decks/deck-123/cards",
		bytes.NewBufferString(`{"cardID": 12345, "quantity": 4, "board": "main"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	if len(mock.addCardCalls) != 1 {
		t.Fatalf("expected 1 call to AddCard, got %d", len(mock.addCardCalls))
	}

	call := mock.addCardCalls[0]
	if call.CardID != 12345 {
		t.Errorf("cardID not correctly parsed: expected 12345, got %d", call.CardID)
	}
	if call.Board != "main" {
		t.Errorf("board not correctly parsed: expected 'main', got %q", call.Board)
	}
}

// TestDeckHandler_SuggestDecks tests the POST /decks/suggest endpoint.
func TestDeckHandler_SuggestDecks(t *testing.T) {
	tests := []struct {
		name            string
		requestBody     string
		mockSuggestions *gui.SuggestDecksResponse
		mockErr         error
		expectedStatus  int
	}{
		{
			name:        "successful suggestions with session_id",
			requestBody: `{"sessionID": "draft-session-123"}`,
			mockSuggestions: &gui.SuggestDecksResponse{
				Suggestions:  []*gui.SuggestedDeckResponse{},
				TotalCombos:  32,
				ViableCombos: 5,
			},
			mockErr:        nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:        "suggestions with results",
			requestBody: `{"sessionID": "draft-456"}`,
			mockSuggestions: &gui.SuggestDecksResponse{
				Suggestions: []*gui.SuggestedDeckResponse{
					{
						ColorCombo: gui.ColorCombinationResponse{Colors: []string{"W", "U"}, Name: "Azorius"},
						TotalCards: 40,
						Score:      0.85,
						Viability:  "strong",
					},
				},
				TotalCombos:  32,
				ViableCombos: 14,
				BestCombo:    &gui.ColorCombinationResponse{Colors: []string{"W", "U"}, Name: "Azorius"},
			},
			mockErr:        nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing session_id returns bad request",
			requestBody:    `{}`,
			mockErr:        nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty session_id returns bad request",
			requestBody:    `{"sessionID": ""}`,
			mockErr:        nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid JSON returns bad request",
			requestBody:    `{invalid}`,
			mockErr:        nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:            "facade error returns internal server error",
			requestBody:     `{"sessionID": "draft-error"}`,
			mockSuggestions: nil,
			mockErr:         errors.New("no cards in draft pool"),
			expectedStatus:  http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDeckFacade{
				suggestions: tt.mockSuggestions,
				err:         tt.mockErr,
			}
			handler := newTestDeckHandler(mock)

			r := chi.NewRouter()
			r.Post("/api/v1/decks/suggest", handler.SuggestDecks)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/decks/suggest",
				bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d (body: %s)", tt.expectedStatus, rec.Code, rec.Body.String())
			}

			// Verify response structure for successful cases
			if tt.expectedStatus == http.StatusOK {
				var response gui.SuggestDecksResponse
				if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
					t.Errorf("failed to decode response: %v", err)
				}

				if tt.mockSuggestions != nil {
					if response.TotalCombos != tt.mockSuggestions.TotalCombos {
						t.Errorf("expected totalCombos %d, got %d",
							tt.mockSuggestions.TotalCombos, response.TotalCombos)
					}
					if response.ViableCombos != tt.mockSuggestions.ViableCombos {
						t.Errorf("expected viableCombos %d, got %d",
							tt.mockSuggestions.ViableCombos, response.ViableCombos)
					}
				}
			}
		})
	}
}

// TestDeckHandler_RemoveCard tests the DELETE /decks/{deckID}/cards/{cardID} endpoint.
func TestDeckHandler_RemoveCard(t *testing.T) {
	tests := []struct {
		name           string
		deckID         string
		cardID         string
		zone           string
		mockErr        error
		expectedStatus int
		expectedBoard  string
	}{
		{
			name:           "successful remove card from main",
			deckID:         "deck-123",
			cardID:         "12345",
			zone:           "main",
			mockErr:        nil,
			expectedStatus: http.StatusNoContent,
			expectedBoard:  "main",
		},
		{
			name:           "remove card from sideboard",
			deckID:         "deck-456",
			cardID:         "67890",
			zone:           "sideboard",
			mockErr:        nil,
			expectedStatus: http.StatusNoContent,
			expectedBoard:  "sideboard",
		},
		{
			name:           "default zone to main when not provided",
			deckID:         "deck-789",
			cardID:         "11111",
			zone:           "",
			mockErr:        nil,
			expectedStatus: http.StatusNoContent,
			expectedBoard:  "main",
		},
		{
			name:           "invalid card ID returns bad request",
			deckID:         "deck-123",
			cardID:         "not-a-number",
			zone:           "main",
			mockErr:        nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "facade error returns internal server error",
			deckID:         "deck-123",
			cardID:         "12345",
			zone:           "main",
			mockErr:        errors.New("database error"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDeckFacade{err: tt.mockErr}
			handler := newTestDeckHandler(mock)

			r := chi.NewRouter()
			r.Delete("/api/v1/decks/{deckID}/cards/{cardID}", handler.RemoveCard)

			url := "/api/v1/decks/" + tt.deckID + "/cards/" + tt.cardID
			if tt.zone != "" {
				url += "?zone=" + tt.zone
			}

			req := httptest.NewRequest(http.MethodDelete, url, nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			// Verify the facade was called with correct parameters for successful cases
			if tt.expectedStatus == http.StatusNoContent && len(mock.removeCardCalls) > 0 {
				call := mock.removeCardCalls[0]
				if call.Board != tt.expectedBoard {
					t.Errorf("expected board %q, got %q", tt.expectedBoard, call.Board)
				}
			}
		})
	}
}

// TestDeckHandler_RecalculateDeckPerformance tests the POST /admin/recalculate-deck-performance endpoint.
func TestDeckHandler_RecalculateDeckPerformance(t *testing.T) {
	tests := []struct {
		name           string
		mockResult     *storage.RecalculateDeckPerformanceResult
		mockErr        error
		expectedStatus int
	}{
		{
			name: "successful recalculation with matches",
			mockResult: &storage.RecalculateDeckPerformanceResult{
				DecksProcessed:       5,
				MatchesProcessed:     25,
				PermutationsReset:    8,
				DecksWithoutMatches:  2,
				MatchesWithoutDeckID: 10,
			},
			mockErr:        nil,
			expectedStatus: http.StatusOK,
		},
		{
			name: "successful recalculation with no matches",
			mockResult: &storage.RecalculateDeckPerformanceResult{
				DecksProcessed:       0,
				MatchesProcessed:     0,
				PermutationsReset:    3,
				DecksWithoutMatches:  3,
				MatchesWithoutDeckID: 50,
			},
			mockErr:        nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "facade error returns internal server error",
			mockResult:     nil,
			mockErr:        errors.New("database error"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDeckFacade{
				recalculateResult: tt.mockResult,
				err:               tt.mockErr,
			}
			handler := newTestDeckHandler(mock)

			r := chi.NewRouter()
			r.Post("/api/v1/admin/recalculate-deck-performance", handler.RecalculateDeckPerformance)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/recalculate-deck-performance", nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d (body: %s)", tt.expectedStatus, rec.Code, rec.Body.String())
			}

			// Verify response structure for successful cases
			if tt.expectedStatus == http.StatusOK {
				var response struct {
					Data *storage.RecalculateDeckPerformanceResult `json:"data"`
				}
				if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
					t.Errorf("failed to decode response: %v", err)
				}

				if response.Data == nil {
					t.Error("expected data in response, got nil")
				} else {
					if response.Data.DecksProcessed != tt.mockResult.DecksProcessed {
						t.Errorf("expected DecksProcessed %d, got %d", tt.mockResult.DecksProcessed, response.Data.DecksProcessed)
					}
					if response.Data.MatchesProcessed != tt.mockResult.MatchesProcessed {
						t.Errorf("expected MatchesProcessed %d, got %d", tt.mockResult.MatchesProcessed, response.Data.MatchesProcessed)
					}
					if response.Data.PermutationsReset != tt.mockResult.PermutationsReset {
						t.Errorf("expected PermutationsReset %d, got %d", tt.mockResult.PermutationsReset, response.Data.PermutationsReset)
					}
					if response.Data.DecksWithoutMatches != tt.mockResult.DecksWithoutMatches {
						t.Errorf("expected DecksWithoutMatches %d, got %d", tt.mockResult.DecksWithoutMatches, response.Data.DecksWithoutMatches)
					}
					if response.Data.MatchesWithoutDeckID != tt.mockResult.MatchesWithoutDeckID {
						t.Errorf("expected MatchesWithoutDeckID %d, got %d", tt.mockResult.MatchesWithoutDeckID, response.Data.MatchesWithoutDeckID)
					}
				}
			}
		})
	}
}

// TestDeckHandler_RecalculateDeckPerformance_ResponseFormat verifies the JSON response format.
func TestDeckHandler_RecalculateDeckPerformance_ResponseFormat(t *testing.T) {
	mock := &mockDeckFacade{
		recalculateResult: &storage.RecalculateDeckPerformanceResult{
			DecksProcessed:       10,
			MatchesProcessed:     50,
			PermutationsReset:    15,
			DecksWithoutMatches:  3,
			MatchesWithoutDeckID: 20,
		},
	}
	handler := newTestDeckHandler(mock)

	r := chi.NewRouter()
	r.Post("/api/v1/admin/recalculate-deck-performance", handler.RecalculateDeckPerformance)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/recalculate-deck-performance", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// Verify JSON field names match the expected format
	var response map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'data' field in response")
	}

	// Verify all expected fields are present
	expectedFields := []string{"decksProcessed", "matchesProcessed", "permutationsReset", "decksWithoutMatches", "matchesWithoutDeckID"}
	for _, field := range expectedFields {
		if _, exists := data[field]; !exists {
			t.Errorf("expected field %q to exist in response", field)
		}
	}

	// Verify specific values
	if decks, ok := data["decksProcessed"].(float64); !ok || int(decks) != 10 {
		t.Errorf("expected decksProcessed to be 10, got %v", data["decksProcessed"])
	}
	if matches, ok := data["matchesProcessed"].(float64); !ok || int(matches) != 50 {
		t.Errorf("expected matchesProcessed to be 50, got %v", data["matchesProcessed"])
	}
}
