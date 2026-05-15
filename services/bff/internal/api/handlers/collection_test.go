package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// ─── stubs ──────────────────────────────────────────────────────────────────

// collectionAccountLookup mirrors matchesAccountLookup. Defined separately so
// the two handlers' test suites stay independent.
type collectionAccountLookup struct {
	accountID int64
	found     bool
	err       error
}

func (c *collectionAccountLookup) GetAccountIDByUserID(_ context.Context, _ int64) (int64, bool, error) {
	return c.accountID, c.found, c.err
}

type stubCollectionReader struct {
	listRows   []repository.CollectionItem
	listFilter repository.CollectionFilter
	listErr    error

	counts    repository.CollectionCounts
	countsErr error

	rarity    []repository.RarityCount
	rarityErr error

	setCardCount    int
	setCardCountErr error
	setCardCountArg string // last setCode passed to SetCardCount

	sets      []repository.SetCompletionRow
	setsErr   error
	setRarity []repository.SetRarityRow
	setRarErr error

	values        []repository.CardValueRow
	unpriced      int
	valuesErr     error
	lastUpdated   int64
	lastUpdatedEr error
}

func (s *stubCollectionReader) ListCollection(_ context.Context, _ int64, f repository.CollectionFilter) ([]repository.CollectionItem, error) {
	s.listFilter = f
	return s.listRows, s.listErr
}

func (s *stubCollectionReader) CountCollection(_ context.Context, _ int64) (repository.CollectionCounts, error) {
	return s.counts, s.countsErr
}

func (s *stubCollectionReader) CountByRarity(_ context.Context, _ int64) ([]repository.RarityCount, error) {
	return s.rarity, s.rarityErr
}

func (s *stubCollectionReader) SetCardCount(_ context.Context, setCode string) (int, error) {
	s.setCardCountArg = setCode
	return s.setCardCount, s.setCardCountErr
}

func (s *stubCollectionReader) SetCompletion(_ context.Context, _ int64) ([]repository.SetCompletionRow, error) {
	return s.sets, s.setsErr
}

func (s *stubCollectionReader) SetRarityBreakdown(_ context.Context, _ int64) ([]repository.SetRarityRow, error) {
	return s.setRarity, s.setRarErr
}

func (s *stubCollectionReader) ValueRows(_ context.Context, _ int64) ([]repository.CardValueRow, int, error) {
	return s.values, s.unpriced, s.valuesErr
}

func (s *stubCollectionReader) LastPriceUpdate(_ context.Context, _ int64) (int64, error) {
	return s.lastUpdated, s.lastUpdatedEr
}

// authedCollectionRequest mirrors requestWithUserID from matches_test.go but
// is defined locally so the two suites stay decoupled.
func authedCollectionRequest(t *testing.T, method, target string, body []byte, userID int64) *http.Request {
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

// decodeCollectionEnvelope unwraps a {"data": ...} body into target.
func decodeCollectionEnvelope(t *testing.T, body []byte, into any) {
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

// ─── List ───────────────────────────────────────────────────────────────────

func TestCollectionList_HappyPath(t *testing.T) {
	priceUSD := 1.5
	power := "2"
	tough := "3"
	prices := int64(1_700_000_000)
	reader := &stubCollectionReader{
		listRows: []repository.CollectionItem{
			{
				CardID: 100, ArenaID: 100, Quantity: 4, Name: "Llanowar Elves",
				SetCode: "DOM", SetName: "Dominaria", Rarity: "common",
				ManaCost: "{G}", CMC: 1, TypeLine: "Creature — Elf Druid",
				Colors: `["G"]`, ColorIdentity: `["G"]`,
				ImageURIs: `{"normal":"https://img/normal.png","small":"https://img/small.png"}`,
				Power:     &power, Toughness: &tough,
				PriceUSD: &priceUSD, PricesUpdated: &prices,
			},
		},
		counts: repository.CollectionCounts{UniqueCards: 1, TotalCards: 4},
	}
	h := handlers.NewCollectionHandler(reader, &collectionAccountLookup{accountID: 7, found: true})

	body, _ := json.Marshal(map[string]any{"set_code": "DOM"})
	req := authedCollectionRequest(t, http.MethodPost, "/api/v1/collection", body, 168)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Cards []map[string]any `json:"cards"`
		Total int              `json:"totalCount"`
	}
	decodeCollectionEnvelope(t, rr.Body.Bytes(), &resp)
	if len(resp.Cards) != 1 {
		t.Fatalf("cards: %v", resp.Cards)
	}
	card := resp.Cards[0]
	if card["name"] != "Llanowar Elves" || card["setCode"] != "DOM" {
		t.Errorf("card shape: %v", card)
	}
	if card["imageUri"] != "https://img/normal.png" {
		t.Errorf("imageUri: %v", card["imageUri"])
	}
	colors, _ := card["colors"].([]any)
	if len(colors) != 1 || colors[0] != "G" {
		t.Errorf("colors: %v", card["colors"])
	}
	if reader.listFilter.SetCode != "DOM" {
		t.Errorf("filter not forwarded: %v", reader.listFilter)
	}
}

func TestCollectionList_NoAccountReturnsEmpty(t *testing.T) {
	h := handlers.NewCollectionHandler(&stubCollectionReader{}, &collectionAccountLookup{found: false})

	req := authedCollectionRequest(t, http.MethodPost, "/api/v1/collection", []byte(`{}`), 168)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var resp struct {
		Cards []any `json:"cards"`
	}
	decodeCollectionEnvelope(t, rr.Body.Bytes(), &resp)
	if len(resp.Cards) != 0 {
		t.Errorf("expected empty cards: %v", resp.Cards)
	}
}

func TestCollectionList_Unauthorized(t *testing.T) {
	h := handlers.NewCollectionHandler(&stubCollectionReader{}, &collectionAccountLookup{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/collection", bytes.NewReader([]byte(`{}`)))
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── Stats ──────────────────────────────────────────────────────────────────

func TestCollectionStats_HappyPath(t *testing.T) {
	reader := &stubCollectionReader{
		counts: repository.CollectionCounts{UniqueCards: 50, TotalCards: 200},
		rarity: []repository.RarityCount{
			{Rarity: "common", TotalCards: 120},
			{Rarity: "uncommon", TotalCards: 50},
			{Rarity: "rare", TotalCards: 25},
			{Rarity: "mythic", TotalCards: 5},
		},
	}
	h := handlers.NewCollectionHandler(reader, &collectionAccountLookup{accountID: 7, found: true})
	req := authedCollectionRequest(t, http.MethodGet, "/api/v1/collection/stats", nil, 168)
	rr := httptest.NewRecorder()
	h.Stats(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var stats map[string]any
	decodeCollectionEnvelope(t, rr.Body.Bytes(), &stats)
	if stats["totalUniqueCards"].(float64) != 50 || stats["commonCount"].(float64) != 120 || stats["mythicCount"].(float64) != 5 {
		t.Errorf("stats: %v", stats)
	}
}

// TestCollectionStats_CardsInSet_NoSetCode verifies that, when no set_code
// query param is supplied, SetCardCount is called with an empty string and
// the result is returned as cardsInSet in the response.
func TestCollectionStats_CardsInSet_NoSetCode(t *testing.T) {
	reader := &stubCollectionReader{
		counts:       repository.CollectionCounts{UniqueCards: 10, TotalCards: 40},
		setCardCount: 300,
	}
	h := handlers.NewCollectionHandler(reader, &collectionAccountLookup{accountID: 7, found: true})
	req := authedCollectionRequest(t, http.MethodGet, "/api/v1/collection/stats", nil, 168)
	rr := httptest.NewRecorder()
	h.Stats(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var stats map[string]any
	decodeCollectionEnvelope(t, rr.Body.Bytes(), &stats)
	if stats["cardsInSet"].(float64) != 300 {
		t.Errorf("cardsInSet: got %v, want 300", stats["cardsInSet"])
	}
	if reader.setCardCountArg != "" {
		t.Errorf("SetCardCount arg: got %q, want empty string", reader.setCardCountArg)
	}
}

// TestCollectionStats_CardsInSet_WithSetCode verifies that, when ?set_code=DOM
// is supplied, SetCardCount is called with "DOM" and the result flows through.
func TestCollectionStats_CardsInSet_WithSetCode(t *testing.T) {
	reader := &stubCollectionReader{
		counts:       repository.CollectionCounts{UniqueCards: 10, TotalCards: 40},
		setCardCount: 247,
	}
	h := handlers.NewCollectionHandler(reader, &collectionAccountLookup{accountID: 7, found: true})
	req := authedCollectionRequest(t, http.MethodGet, "/api/v1/collection/stats?set_code=DOM", nil, 168)
	rr := httptest.NewRecorder()
	h.Stats(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var stats map[string]any
	decodeCollectionEnvelope(t, rr.Body.Bytes(), &stats)
	if stats["cardsInSet"].(float64) != 247 {
		t.Errorf("cardsInSet: got %v, want 247", stats["cardsInSet"])
	}
	if reader.setCardCountArg != "DOM" {
		t.Errorf("SetCardCount arg: got %q, want DOM", reader.setCardCountArg)
	}
}

// TestCollectionStats_CardsInSet_DBError verifies that a SetCardCount failure
// surfaces as a 500 rather than silently returning zero.
func TestCollectionStats_CardsInSet_DBError(t *testing.T) {
	reader := &stubCollectionReader{
		counts:          repository.CollectionCounts{UniqueCards: 5, TotalCards: 20},
		setCardCountErr: fmt.Errorf("connection reset"),
	}
	h := handlers.NewCollectionHandler(reader, &collectionAccountLookup{accountID: 7, found: true})
	req := authedCollectionRequest(t, http.MethodGet, "/api/v1/collection/stats", nil, 168)
	rr := httptest.NewRecorder()
	h.Stats(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on SetCardCount error, got %d body=%s", rr.Code, rr.Body.String())
	}
}

// ─── Sets ───────────────────────────────────────────────────────────────────

func TestCollectionSets_HappyPath(t *testing.T) {
	reader := &stubCollectionReader{
		sets: []repository.SetCompletionRow{
			{SetCode: "DOM", SetName: "Dominaria", TotalCards: 100, OwnedCards: 60},
		},
		setRarity: []repository.SetRarityRow{
			{SetCode: "DOM", Rarity: "common", Total: 60, Owned: 40},
			{SetCode: "DOM", Rarity: "rare", Total: 20, Owned: 10},
		},
	}
	h := handlers.NewCollectionHandler(reader, &collectionAccountLookup{accountID: 7, found: true})
	req := authedCollectionRequest(t, http.MethodGet, "/api/v1/collection/sets", nil, 168)
	rr := httptest.NewRecorder()
	h.Sets(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeCollectionEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["SetCode"] != "DOM" || arr[0]["TotalCards"].(float64) != 100 {
		t.Errorf("sets: %v", arr)
	}
	rarity, _ := arr[0]["RarityBreakdown"].(map[string]any)
	if rarity["common"] == nil || rarity["rare"] == nil {
		t.Errorf("rarity breakdown: %v", rarity)
	}
}

// ─── Value ──────────────────────────────────────────────────────────────────

func TestCollectionValue_HappyPath(t *testing.T) {
	reader := &stubCollectionReader{
		values: []repository.CardValueRow{
			{CardID: 100, Name: "Goblin Guide", SetCode: "ZNR", Rarity: "rare", Quantity: 4, PriceUSD: 5, PriceEUR: 4.5},
			{CardID: 200, Name: "Plains", SetCode: "DOM", Rarity: "common", Quantity: 24, PriceUSD: 0.05, PriceEUR: 0.04},
		},
		lastUpdated: 1_700_000_000,
	}
	h := handlers.NewCollectionHandler(reader, &collectionAccountLookup{accountID: 7, found: true})
	req := authedCollectionRequest(t, http.MethodGet, "/api/v1/collection/value", nil, 168)
	rr := httptest.NewRecorder()
	h.Value(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var v map[string]any
	decodeCollectionEnvelope(t, rr.Body.Bytes(), &v)
	if v["totalValueUsd"].(float64) <= 0 {
		t.Errorf("totalValueUsd: %v", v["totalValueUsd"])
	}
	if v["uniqueCardsWithPrice"].(float64) != 2 {
		t.Errorf("uniqueCardsWithPrice: %v", v["uniqueCardsWithPrice"])
	}
	tops, _ := v["topCards"].([]any)
	if len(tops) != 2 {
		t.Errorf("topCards count: %v", tops)
	}
	if tops[0].(map[string]any)["cardId"].(float64) != 100 {
		t.Errorf("topCards[0] should be Goblin Guide ($20 total): %v", tops[0])
	}
}
