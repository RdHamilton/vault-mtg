package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/posthog/posthog-go"
	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// ─── stubs ──────────────────────────────────────────────────────────────────

type decksAccountLookup struct {
	accountID int64
	found     bool
	err       error
}

func (d *decksAccountLookup) GetAccountIDByUserID(_ context.Context, _ int64) (int64, bool, error) {
	return d.accountID, d.found, d.err
}

type stubDecksReader struct {
	list    []repository.DeckSummaryRow
	listErr error

	deck    *repository.DeckDetailRow
	deckErr error

	byEvent    *repository.DeckDetailRow
	byEventErr error

	created    *repository.DeckDetailRow
	createdErr error

	updated    *repository.DeckDetailRow
	updatedErr error

	deleted    bool
	deletedErr error

	cloned    *repository.DeckDetailRow
	clonedErr error

	cardAdded    bool
	cardAddedErr error

	cardRemoved    bool
	cardRemovedErr error

	allRemoved    bool
	allRemovedErr error

	tagAdded    bool
	tagAddedErr error

	tagRemoved    bool
	tagRemovedErr error

	permutations    []repository.PermutationRow
	permutationsErr error

	permutation    *repository.PermutationRow
	permutationErr error

	currentPerm    *repository.PermutationRow
	currentPermErr error

	permNameUpdated    bool
	permNameUpdatedErr error

	permRestored    bool
	permRestoredErr error

	matchAgg    repository.DeckMatchesAggregate
	matchAggErr error
}

func (s *stubDecksReader) ListDecks(_ context.Context, _ int64, _ repository.DeckListFilter) ([]repository.DeckSummaryRow, error) {
	return s.list, s.listErr
}

func (s *stubDecksReader) GetDeck(_ context.Context, _ int64, _ string) (*repository.DeckDetailRow, error) {
	return s.deck, s.deckErr
}

func (s *stubDecksReader) GetDeckByDraftEvent(_ context.Context, _ int64, _ string) (*repository.DeckDetailRow, error) {
	return s.byEvent, s.byEventErr
}

func (s *stubDecksReader) CreateDeck(_ context.Context, _ repository.CreateDeckInput) (*repository.DeckDetailRow, error) {
	return s.created, s.createdErr
}

func (s *stubDecksReader) UpdateDeck(_ context.Context, _ int64, _ string, _ repository.UpdateDeckInput) (*repository.DeckDetailRow, error) {
	return s.updated, s.updatedErr
}

func (s *stubDecksReader) DeleteDeck(_ context.Context, _ int64, _ string) (bool, error) {
	return s.deleted, s.deletedErr
}

func (s *stubDecksReader) CloneDeck(_ context.Context, _ int64, _, _ string) (*repository.DeckDetailRow, error) {
	return s.cloned, s.clonedErr
}

func (s *stubDecksReader) AddCard(_ context.Context, _ int64, _ string, _ repository.AddCardInput) (bool, error) {
	return s.cardAdded, s.cardAddedErr
}

func (s *stubDecksReader) RemoveCardOne(_ context.Context, _ int64, _ string, _ int, _ string) (bool, error) {
	return s.cardRemoved, s.cardRemovedErr
}

func (s *stubDecksReader) RemoveAllCopies(_ context.Context, _ int64, _ string, _ int, _ string) (bool, error) {
	return s.allRemoved, s.allRemovedErr
}

func (s *stubDecksReader) AddTag(_ context.Context, _ int64, _, _ string) (bool, error) {
	return s.tagAdded, s.tagAddedErr
}

func (s *stubDecksReader) RemoveTag(_ context.Context, _ int64, _, _ string) (bool, error) {
	return s.tagRemoved, s.tagRemovedErr
}

func (s *stubDecksReader) ListPermutations(_ context.Context, _ int64, _ string) ([]repository.PermutationRow, error) {
	return s.permutations, s.permutationsErr
}

func (s *stubDecksReader) GetPermutation(_ context.Context, _ int64, _ string, _ int64) (*repository.PermutationRow, error) {
	return s.permutation, s.permutationErr
}

func (s *stubDecksReader) CurrentPermutation(_ context.Context, _ int64, _ string) (*repository.PermutationRow, error) {
	return s.currentPerm, s.currentPermErr
}

func (s *stubDecksReader) UpdatePermutationName(_ context.Context, _ int64, _ string, _ int64, _ string) (bool, error) {
	return s.permNameUpdated, s.permNameUpdatedErr
}

func (s *stubDecksReader) RestorePermutation(_ context.Context, _ int64, _ string, _ int64) (bool, error) {
	return s.permRestored, s.permRestoredErr
}

func (s *stubDecksReader) DeckMatchesAggregate(_ context.Context, _ int64, _ string) (repository.DeckMatchesAggregate, error) {
	return s.matchAgg, s.matchAggErr
}

// ─── helpers ────────────────────────────────────────────────────────────────

func authedDecksRequest(t *testing.T, method, target string, body []byte, userID int64) *http.Request {
	t.Helper()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, target, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	return req.WithContext(bffmiddleware.WithUserID(req.Context(), userID))
}

func decodeDecksEnvelope(t *testing.T, body []byte, into any) {
	t.Helper()
	wrapper := struct {
		Data json.RawMessage `json:"data"`
	}{}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		t.Fatalf("envelope decode: %v body=%s", err, string(body))
	}
	if err := json.Unmarshal(wrapper.Data, into); err != nil {
		t.Fatalf("payload decode: %v data=%s", err, string(wrapper.Data))
	}
}

func chiDecksContext(req *http.Request, kvs ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i+1 < len(kvs); i += 2 {
		rctx.URLParams.Add(kvs[i], kvs[i+1])
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func sampleDeckDetail() *repository.DeckDetailRow {
	now := time.Now().UTC()
	return &repository.DeckDetailRow{
		DeckSummaryRow: repository.DeckSummaryRow{
			ID: "deck1", Name: "Mono Red", Format: "standard", Source: "constructed",
			MatchesPlayed: 10, MatchesWon: 6, GamesPlayed: 24, GamesWon: 14,
			CreatedAt: now, ModifiedAt: now, CardCount: 60,
		},
		Cards: []repository.DeckCardRow{
			{CardID: 100, Quantity: 4, Board: "main", Name: "Goblin Guide", SetCode: "DSK", CMC: 1, TypeLine: "Creature — Goblin", Colors: `["R"]`},
			{CardID: 200, Quantity: 24, Board: "main", Name: "Mountain", SetCode: "DSK", CMC: 0, TypeLine: "Basic Land — Mountain"},
		},
	}
}

// ─── List / Get / Create / Update / Delete ─────────────────────────────────

func TestDecksList_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubDecksReader{list: []repository.DeckSummaryRow{
		{ID: "d1", Name: "Mono Red", Format: "standard", CreatedAt: now, ModifiedAt: now, MatchesPlayed: 4, MatchesWon: 2},
	}}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	req := authedDecksRequest(t, http.MethodGet, "/api/v1/decks", nil, 168)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeDecksEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["winRate"].(float64) != 0.5 {
		t.Errorf("list: %v", arr)
	}
}

func TestDecksGet_HappyPath(t *testing.T) {
	reader := &stubDecksReader{deck: sampleDeckDetail()}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	req := authedDecksRequest(t, http.MethodGet, "/api/v1/decks/deck1", nil, 168)
	req = chiDecksContext(req, "deckId", "deck1")
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeDecksEnvelope(t, rr.Body.Bytes(), &resp)
	cards, _ := resp["cards"].([]any)
	if resp["id"] != "deck1" || len(cards) != 2 {
		t.Errorf("deck: %v", resp)
	}
}

func TestDecksGet_NotFound(t *testing.T) {
	h := handlers.NewDecksHandler(&stubDecksReader{deck: nil}, &decksAccountLookup{accountID: 7, found: true})
	req := authedDecksRequest(t, http.MethodGet, "/api/v1/decks/missing", nil, 168)
	req = chiDecksContext(req, "deckId", "missing")
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestDecksCreate_HappyPath(t *testing.T) {
	reader := &stubDecksReader{created: sampleDeckDetail()}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"name": "New Deck", "format": "standard", "source": "constructed"})
	req := authedDecksRequest(t, http.MethodPost, "/api/v1/decks", body, 168)
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	// AC1: Create must return 201 Created (not 200 OK).
	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d want %d, body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	var resp map[string]any
	decodeDecksEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["id"] != "deck1" {
		t.Errorf("Create response id: got %v want deck1", resp["id"])
	}
}

func TestDecksCreate_MissingFormat(t *testing.T) {
	reader := &stubDecksReader{created: sampleDeckDetail()}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	// format is deliberately omitted — handler must reject with 400.
	body, _ := json.Marshal(map[string]any{"name": "No Format Deck"})
	req := authedDecksRequest(t, http.MethodPost, "/api/v1/decks", body, 168)
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d want 400 when format is missing", rr.Code)
	}
}

func TestDecksCreate_MissingName(t *testing.T) {
	reader := &stubDecksReader{created: sampleDeckDetail()}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"format": "standard"})
	req := authedDecksRequest(t, http.MethodPost, "/api/v1/decks", body, 168)
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d want 400 when name is missing", rr.Code)
	}
}

func TestDecksCreate_DBError(t *testing.T) {
	reader := &stubDecksReader{createdErr: errors.New("db error")}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"name": "Ray", "format": "standard"})
	req := authedDecksRequest(t, http.MethodPost, "/api/v1/decks", body, 168)
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d want 500 when DB fails", rr.Code)
	}
}

func TestDecksDelete_HappyPath(t *testing.T) {
	reader := &stubDecksReader{deleted: true}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	req := authedDecksRequest(t, http.MethodDelete, "/api/v1/decks/d1", nil, 168)
	req = chiDecksContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.Delete(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── Cards ──────────────────────────────────────────────────────────────────

func TestDecksAddCard_HappyPath(t *testing.T) {
	reader := &stubDecksReader{cardAdded: true}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"cardID": 100, "quantity": 2, "board": "main"})
	req := authedDecksRequest(t, http.MethodPost, "/api/v1/decks/d1/cards", body, 168)
	req = chiDecksContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.AddCard(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestDecksRemoveCard_HappyPath(t *testing.T) {
	reader := &stubDecksReader{cardRemoved: true}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	req := authedDecksRequest(t, http.MethodDelete, "/api/v1/decks/d1/cards/100?zone=main", nil, 168)
	req = chiDecksContext(req, "deckId", "d1", "cardId", "100")
	rr := httptest.NewRecorder()
	h.RemoveCard(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── Tags ───────────────────────────────────────────────────────────────────

func TestDecksAddTag_HappyPath(t *testing.T) {
	reader := &stubDecksReader{tagAdded: true}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"tag": "competitive"})
	req := authedDecksRequest(t, http.MethodPost, "/api/v1/decks/d1/tags", body, 168)
	req = chiDecksContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.AddTag(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestDecksRemoveTag_HappyPath(t *testing.T) {
	reader := &stubDecksReader{tagRemoved: true}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	req := authedDecksRequest(t, http.MethodDelete, "/api/v1/decks/d1/tags/competitive", nil, 168)
	req = chiDecksContext(req, "deckId", "d1", "tag", "competitive")
	rr := httptest.NewRecorder()
	h.RemoveTag(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── Stats / Performance / ValidateDraft ────────────────────────────────────

func TestDecksStats_HappyPath(t *testing.T) {
	reader := &stubDecksReader{deck: sampleDeckDetail()}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	req := authedDecksRequest(t, http.MethodGet, "/api/v1/decks/d1/stats", nil, 168)
	req = chiDecksContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.Stats(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeDecksEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["totalCards"].(float64) != 28 {
		t.Errorf("totalCards: %v", resp)
	}
	if resp["landCount"].(float64) != 24 {
		t.Errorf("landCount: %v", resp)
	}
}

func TestDecksPerformance_HappyPath(t *testing.T) {
	reader := &stubDecksReader{matchAgg: repository.DeckMatchesAggregate{
		TotalMatches: 10, MatchesWon: 6, GamesPlayed: 24, GamesWon: 14,
	}}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	req := authedDecksRequest(t, http.MethodGet, "/api/v1/decks/d1/performance", nil, 168)
	req = chiDecksContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.Performance(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var resp map[string]any
	decodeDecksEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["winRate"].(float64) != 0.6 {
		t.Errorf("winRate: %v", resp)
	}
}

func TestDecksValidateDraft_ValidWith40(t *testing.T) {
	deck := sampleDeckDetail()
	// Bump main count to 40 by inflating the mountains.
	deck.Cards[1].Quantity = 36
	reader := &stubDecksReader{deck: deck}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	req := authedDecksRequest(t, http.MethodGet, "/api/v1/decks/d1/validate-draft", nil, 168)
	req = chiDecksContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.ValidateDraft(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var resp map[string]any
	decodeDecksEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["valid"] != true {
		t.Errorf("valid: %v", resp)
	}
}

// ─── Permutations ──────────────────────────────────────────────────────────

func TestDecksListPermutations_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubDecksReader{
		permutations: []repository.PermutationRow{
			{ID: 1, DeckID: "d1", Cards: `[{"card_id":100,"quantity":4,"board":"main"}]`, VersionNumber: 1, CreatedAt: now},
		},
		currentPerm: &repository.PermutationRow{ID: 1},
	}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	req := authedDecksRequest(t, http.MethodGet, "/api/v1/decks/d1/permutations", nil, 168)
	req = chiDecksContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.ListPermutations(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var arr []map[string]any
	decodeDecksEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["isCurrent"] != true {
		t.Errorf("permutations: %v", arr)
	}
}

func TestDecksRestorePermutation_HappyPath(t *testing.T) {
	reader := &stubDecksReader{permRestored: true}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	req := authedDecksRequest(t, http.MethodPost, "/api/v1/decks/d1/permutations/2/restore", nil, 168)
	req = chiDecksContext(req, "deckId", "d1", "permutationId", "2")
	rr := httptest.NewRecorder()
	h.RestorePermutation(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── Import / Export ───────────────────────────────────────────────────────

func TestDecksImport_HappyPath(t *testing.T) {
	created := &repository.DeckDetailRow{
		DeckSummaryRow: repository.DeckSummaryRow{ID: "d-imp", Name: "Imported", Format: "standard"},
	}
	reader := &stubDecksReader{created: created, deck: sampleDeckDetail(), cardAdded: true}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{
		"name":    "Imported",
		"format":  "standard",
		"content": "Deck\n4 Lightning Bolt (M21) 162\n24 Mountain\n",
	})
	req := authedDecksRequest(t, http.MethodPost, "/api/v1/decks/import", body, 168)
	rr := httptest.NewRecorder()
	h.Import(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestDecksParse_NoSave(t *testing.T) {
	h := handlers.NewDecksHandler(&stubDecksReader{}, &decksAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{
		"content": "Deck\n4 Goblin Guide (DSK) 1\n2 Lightning Bolt\n",
	})
	req := authedDecksRequest(t, http.MethodPost, "/api/v1/decks/parse", body, 168)
	rr := httptest.NewRecorder()
	h.Parse(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeDecksEnvelope(t, rr.Body.Bytes(), &resp)
	cards, _ := resp["cards"].([]any)
	if len(cards) != 2 {
		t.Errorf("parse cards: %v", cards)
	}
}

func TestDecksExport_HappyPath(t *testing.T) {
	reader := &stubDecksReader{deck: sampleDeckDetail()}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"format": "arena"})
	req := authedDecksRequest(t, http.MethodPost, "/api/v1/decks/d1/export", body, 168)
	req = chiDecksContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.Export(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeDecksEnvelope(t, rr.Body.Bytes(), &resp)
	content, _ := resp["content"].(string)
	if content == "" || !contains(content, "Goblin Guide") {
		t.Errorf("export content: %v", content)
	}
}

// ─── STUBs sanity ──────────────────────────────────────────────────────────

func TestDecksArchetypes_StubReturnsList(t *testing.T) {
	h := handlers.NewDecksHandler(&stubDecksReader{}, &decksAccountLookup{accountID: 7, found: true})
	req := authedDecksRequest(t, http.MethodGet, "/api/v1/decks/archetypes", nil, 168)
	rr := httptest.NewRecorder()
	h.Archetypes(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var arr []map[string]any
	decodeDecksEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 8 {
		t.Errorf("archetype count: %d", len(arr))
	}
}

func TestDecksGenerate_StubReturnsShape(t *testing.T) {
	h := handlers.NewDecksHandler(&stubDecksReader{}, &decksAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"seed_card_id": 100, "archetype": "aggro"})
	req := authedDecksRequest(t, http.MethodPost, "/api/v1/decks/generate", body, 168)
	rr := httptest.NewRecorder()
	h.Generate(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
}

// ─── Error paths (issue #1973) ─────────────────────────────────────────────

// TestDecksExport_NotFound verifies that Export returns 404 when the deck
// does not exist for the authenticated account (Bug 2 — route now registered).
func TestDecksExport_NotFound(t *testing.T) {
	reader := &stubDecksReader{deck: nil}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	req := authedDecksRequest(t, http.MethodPost, "/api/v1/decks/missing/export", nil, 168)
	req = chiDecksContext(req, "deckId", "missing")
	rr := httptest.NewRecorder()
	h.Export(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d body=%s", rr.Code, rr.Body.String())
	}
}

// TestDecksExport_DBError verifies that Export returns 500 when GetDeck fails.
func TestDecksExport_DBError(t *testing.T) {
	reader := &stubDecksReader{deckErr: errors.New("db unavailable")}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	req := authedDecksRequest(t, http.MethodPost, "/api/v1/decks/d1/export", nil, 168)
	req = chiDecksContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.Export(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d body=%s", rr.Code, rr.Body.String())
	}
}

// TestDecksGet_DBError verifies that Get returns 500 when GetDeck errors
// (Bug 3 — from_draft_pick type mismatch on legacy INTEGER schema).
func TestDecksGet_DBError(t *testing.T) {
	reader := &stubDecksReader{deckErr: errors.New("db unavailable")}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	req := authedDecksRequest(t, http.MethodGet, "/api/v1/decks/d1", nil, 168)
	req = chiDecksContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d body=%s", rr.Code, rr.Body.String())
	}
}

// ─── Auth ──────────────────────────────────────────────────────────────────

func TestDecksList_Unauthorized(t *testing.T) {
	h := handlers.NewDecksHandler(&stubDecksReader{}, &decksAccountLookup{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/decks", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: %d", rr.Code)
	}
}

// trivial substring helper to avoid importing strings here.
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// isHex16 returns true when s is exactly 16 lowercase hex characters.
// This is the shape produced by hashAccountID (SHA-256 hex[:16]).
func isHex16(s string) bool {
	if len(s) != 16 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// assertHashedID asserts that:
//  1. capture.DistinctId is a 16-char hex string (not a raw integer).
//  2. capture.Properties["account_id_hash"] matches DistinctId.
//  3. capture.Properties["account_id"] is NOT set (PII guard).
func assertHashedID(t *testing.T, capture posthog.Capture) {
	t.Helper()
	if !isHex16(capture.DistinctId) {
		t.Errorf("DistinctId=%q: want 16-char hex string (got raw PII or wrong format)", capture.DistinctId)
	}
	hash, ok := capture.Properties["account_id_hash"]
	if !ok {
		t.Error("PostHog capture missing account_id_hash property")
		return
	}
	hashStr, _ := hash.(string)
	if !isHex16(hashStr) {
		t.Errorf("account_id_hash=%q: want 16-char hex string", hashStr)
	}
	if hashStr != capture.DistinctId {
		t.Errorf("account_id_hash=%q != DistinctId=%q: must match", hashStr, capture.DistinctId)
	}
	if _, present := capture.Properties["account_id"]; present {
		t.Error("PostHog capture must NOT contain raw account_id property (PII violation)")
	}
}

// ─── PostHog instrumentation tests (ticket #1982) ──────────────────────────
//
// Each test wires a mockPostHogClient and asserts that the correct event is
// emitted on the happy path and that NO event is emitted on error paths.
// Sentry.CaptureException is a safe no-op when Sentry is uninitialised, so
// error-path tests exercise that branch without requiring a live Sentry DSN.

// TestDecksGet_EmitsPostHogEvent verifies that a successful Get emits a
// "get_deck" PostHog event with the deck_id property set.
func TestDecksGet_EmitsPostHogEvent(t *testing.T) {
	ph := &mockPostHogClient{}
	reader := &stubDecksReader{deck: sampleDeckDetail()}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true}).
		WithPostHogClient(ph)
	req := authedDecksRequest(t, http.MethodGet, "/api/v1/decks/deck1", nil, 168)
	req = chiDecksContext(req, "deckId", "deck1")
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if len(ph.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(ph.calls))
	}
	capture, ok := ph.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}
	if capture.Event != "get_deck" {
		t.Errorf("event=%q, want %q", capture.Event, "get_deck")
	}
	if capture.Properties["deck_id"] != "deck1" {
		t.Errorf("deck_id=%v, want %q", capture.Properties["deck_id"], "deck1")
	}
	assertHashedID(t, capture)
}

// TestDecksGet_DBError_NoPostHogEvent verifies that no PostHog event is emitted
// when GetDeck returns an error (error path must not capture success metrics).
func TestDecksGet_DBError_NoPostHogEvent(t *testing.T) {
	ph := &mockPostHogClient{}
	reader := &stubDecksReader{deckErr: errors.New("db down")}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true}).
		WithPostHogClient(ph)
	req := authedDecksRequest(t, http.MethodGet, "/api/v1/decks/deck1", nil, 168)
	req = chiDecksContext(req, "deckId", "deck1")
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
	if len(ph.calls) != 0 {
		t.Errorf("expected no PostHog calls on error path, got %d", len(ph.calls))
	}
}

// TestDecksUpdate_EmitsPostHogEvent verifies that a successful Update emits an
// "update_deck" PostHog event.
func TestDecksUpdate_EmitsPostHogEvent(t *testing.T) {
	ph := &mockPostHogClient{}
	reader := &stubDecksReader{updated: sampleDeckDetail()}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true}).
		WithPostHogClient(ph)
	body, _ := json.Marshal(map[string]any{"name": "Updated Name"})
	req := authedDecksRequest(t, http.MethodPut, "/api/v1/decks/deck1", body, 168)
	req = chiDecksContext(req, "deckId", "deck1")
	rr := httptest.NewRecorder()
	h.Update(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if len(ph.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(ph.calls))
	}
	capture, ok := ph.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}
	if capture.Event != "update_deck" {
		t.Errorf("event=%q, want %q", capture.Event, "update_deck")
	}
	assertHashedID(t, capture)
}

// TestDecksUpdate_DBError_NoPostHogEvent verifies that no PostHog event is
// emitted when UpdateDeck returns an error.
func TestDecksUpdate_DBError_NoPostHogEvent(t *testing.T) {
	ph := &mockPostHogClient{}
	reader := &stubDecksReader{updatedErr: errors.New("db down")}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true}).
		WithPostHogClient(ph)
	body, _ := json.Marshal(map[string]any{"name": "Updated Name"})
	req := authedDecksRequest(t, http.MethodPut, "/api/v1/decks/deck1", body, 168)
	req = chiDecksContext(req, "deckId", "deck1")
	rr := httptest.NewRecorder()
	h.Update(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
	if len(ph.calls) != 0 {
		t.Errorf("expected no PostHog calls on error path, got %d", len(ph.calls))
	}
}

// TestDecksDelete_EmitsPostHogEvent verifies that a successful Delete emits a
// "delete_deck" PostHog event.
func TestDecksDelete_EmitsPostHogEvent(t *testing.T) {
	ph := &mockPostHogClient{}
	reader := &stubDecksReader{deleted: true}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true}).
		WithPostHogClient(ph)
	req := authedDecksRequest(t, http.MethodDelete, "/api/v1/decks/d1", nil, 168)
	req = chiDecksContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.Delete(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", rr.Code, rr.Body.String())
	}
	if len(ph.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(ph.calls))
	}
	capture, ok := ph.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}
	if capture.Event != "delete_deck" {
		t.Errorf("event=%q, want %q", capture.Event, "delete_deck")
	}
	if capture.Properties["deck_id"] != "d1" {
		t.Errorf("deck_id=%v, want %q", capture.Properties["deck_id"], "d1")
	}
	assertHashedID(t, capture)
}

// TestDecksDelete_DBError_NoPostHogEvent verifies that no PostHog event is
// emitted when DeleteDeck returns an error.
func TestDecksDelete_DBError_NoPostHogEvent(t *testing.T) {
	ph := &mockPostHogClient{}
	reader := &stubDecksReader{deletedErr: errors.New("db down")}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true}).
		WithPostHogClient(ph)
	req := authedDecksRequest(t, http.MethodDelete, "/api/v1/decks/d1", nil, 168)
	req = chiDecksContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.Delete(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
	if len(ph.calls) != 0 {
		t.Errorf("expected no PostHog calls on error path, got %d", len(ph.calls))
	}
}

// TestDecksClone_EmitsPostHogEvent verifies that a successful Clone emits a
// "clone_deck" PostHog event with source_deck_id and new_deck_id properties.
func TestDecksClone_EmitsPostHogEvent(t *testing.T) {
	ph := &mockPostHogClient{}
	cloned := sampleDeckDetail()
	cloned.ID = "deck-cloned"
	reader := &stubDecksReader{cloned: cloned}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true}).
		WithPostHogClient(ph)
	body, _ := json.Marshal(map[string]any{"name": "Clone of Mono Red"})
	req := authedDecksRequest(t, http.MethodPost, "/api/v1/decks/deck1/clone", body, 168)
	req = chiDecksContext(req, "deckId", "deck1")
	rr := httptest.NewRecorder()
	h.Clone(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if len(ph.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(ph.calls))
	}
	capture, ok := ph.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}
	if capture.Event != "clone_deck" {
		t.Errorf("event=%q, want %q", capture.Event, "clone_deck")
	}
	if capture.Properties["source_deck_id"] != "deck1" {
		t.Errorf("source_deck_id=%v, want %q", capture.Properties["source_deck_id"], "deck1")
	}
	if capture.Properties["new_deck_id"] != "deck-cloned" {
		t.Errorf("new_deck_id=%v, want %q", capture.Properties["new_deck_id"], "deck-cloned")
	}
	assertHashedID(t, capture)
}

// TestDecksClone_DBError_NoPostHogEvent verifies that no PostHog event is
// emitted when CloneDeck returns an error.
func TestDecksClone_DBError_NoPostHogEvent(t *testing.T) {
	ph := &mockPostHogClient{}
	reader := &stubDecksReader{clonedErr: errors.New("db down")}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true}).
		WithPostHogClient(ph)
	body, _ := json.Marshal(map[string]any{"name": "Clone of Mono Red"})
	req := authedDecksRequest(t, http.MethodPost, "/api/v1/decks/deck1/clone", body, 168)
	req = chiDecksContext(req, "deckId", "deck1")
	rr := httptest.NewRecorder()
	h.Clone(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
	if len(ph.calls) != 0 {
		t.Errorf("expected no PostHog calls on error path, got %d", len(ph.calls))
	}
}

// TestDecksExport_EmitsPostHogEvent verifies that a successful Export emits an
// "export_deck" PostHog event with deck_id, deck_name, and format properties.
func TestDecksExport_EmitsPostHogEvent(t *testing.T) {
	ph := &mockPostHogClient{}
	reader := &stubDecksReader{deck: sampleDeckDetail()}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true}).
		WithPostHogClient(ph)
	body, _ := json.Marshal(map[string]any{"format": "arena"})
	req := authedDecksRequest(t, http.MethodPost, "/api/v1/decks/deck1/export", body, 168)
	req = chiDecksContext(req, "deckId", "deck1")
	rr := httptest.NewRecorder()
	h.Export(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if len(ph.calls) != 1 {
		t.Fatalf("expected 1 PostHog call, got %d", len(ph.calls))
	}
	capture, ok := ph.calls[0].(posthog.Capture)
	if !ok {
		t.Fatal("PostHog message is not a posthog.Capture")
	}
	if capture.Event != "export_deck" {
		t.Errorf("event=%q, want %q", capture.Event, "export_deck")
	}
	if capture.Properties["deck_id"] != "deck1" {
		t.Errorf("deck_id=%v, want %q", capture.Properties["deck_id"], "deck1")
	}
	if capture.Properties["deck_name"] != "Mono Red" {
		t.Errorf("deck_name=%v, want %q", capture.Properties["deck_name"], "Mono Red")
	}
	if capture.Properties["format"] != "arena" {
		t.Errorf("format=%v, want %q", capture.Properties["format"], "arena")
	}
	assertHashedID(t, capture)
}

// TestDecksExport_DBError_NoPostHogEvent verifies that no PostHog event is
// emitted when GetDeck returns an error during export.
func TestDecksExport_DBError_NoPostHogEvent(t *testing.T) {
	ph := &mockPostHogClient{}
	reader := &stubDecksReader{deckErr: errors.New("db down")}
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true}).
		WithPostHogClient(ph)
	req := authedDecksRequest(t, http.MethodPost, "/api/v1/decks/deck1/export", nil, 168)
	req = chiDecksContext(req, "deckId", "deck1")
	rr := httptest.NewRecorder()
	h.Export(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
	if len(ph.calls) != 0 {
		t.Errorf("expected no PostHog calls on error path, got %d", len(ph.calls))
	}
}

// TestDecksHandler_NoopPostHog_DefaultBehavior verifies that the handler works
// correctly when no PostHogClient is explicitly wired (uses the noop default).
// This is a regression guard: NewDecksHandler must not panic or fail without
// an explicit WithPostHogClient call.
func TestDecksHandler_NoopPostHog_DefaultBehavior(t *testing.T) {
	reader := &stubDecksReader{deck: sampleDeckDetail()}
	// No WithPostHogClient — uses internal noop default.
	h := handlers.NewDecksHandler(reader, &decksAccountLookup{accountID: 7, found: true})
	req := authedDecksRequest(t, http.MethodGet, "/api/v1/decks/deck1", nil, 168)
	req = chiDecksContext(req, "deckId", "deck1")
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	// Must succeed with default noop client — no panic, correct status.
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
}
