package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// ─── stubs ──────────────────────────────────────────────────────────────────

type cardsAccountLookup struct {
	accountID int64
	found     bool
	err       error
}

func (c *cardsAccountLookup) GetAccountIDByUserID(_ context.Context, _ int64) (int64, bool, error) {
	return c.accountID, c.found, c.err
}

type stubCardsReader struct {
	search    []repository.SetCardRow
	searchErr error

	byArenaID    *repository.SetCardRow
	byArenaIDErr error

	bySet    []repository.SetCardRow
	bySetErr error

	withCollection    []repository.SetCardWithQty
	withCollectionErr error

	collectionQty    map[int]int
	collectionQtyErr error

	allSets    []repository.SetInfoRow
	allSetsErr error

	cardRatings    []repository.CardRatingRow
	cardRatingsErr error

	colorRatings    []repository.ColorRatingRow
	colorRatingsErr error

	staleness    repository.RatingsStalenessRow
	stalenessErr error

	touchErr error

	cfbBySet    []repository.CFBRatingRow
	cfbBySetErr error

	cfbByCard    *repository.CFBRatingRow
	cfbByCardErr error

	cfbCount    int
	cfbCountErr error

	imported    int
	importedErr error

	linked    int
	linkedErr error

	deleted    int
	deletedErr error
}

func (s *stubCardsReader) SearchCards(_ context.Context, _, _ string, _ int) ([]repository.SetCardRow, error) {
	return s.search, s.searchErr
}

func (s *stubCardsReader) CardByArenaID(_ context.Context, _ int) (*repository.SetCardRow, error) {
	return s.byArenaID, s.byArenaIDErr
}

func (s *stubCardsReader) CardsBySetCode(_ context.Context, _ string) ([]repository.SetCardRow, error) {
	return s.bySet, s.bySetErr
}

func (s *stubCardsReader) SearchCardsWithCollection(_ context.Context, _ int64, _ string, _ []string, _ int) ([]repository.SetCardWithQty, error) {
	return s.withCollection, s.withCollectionErr
}

func (s *stubCardsReader) CollectionQuantities(_ context.Context, _ int64, _ []int) (map[int]int, error) {
	return s.collectionQty, s.collectionQtyErr
}

func (s *stubCardsReader) AllSetInfo(_ context.Context) ([]repository.SetInfoRow, error) {
	return s.allSets, s.allSetsErr
}

func (s *stubCardsReader) CardRatings(_ context.Context, _, _ string) ([]repository.CardRatingRow, error) {
	return s.cardRatings, s.cardRatingsErr
}

func (s *stubCardsReader) ColorRatings(_ context.Context, _ string) ([]repository.ColorRatingRow, error) {
	return s.colorRatings, s.colorRatingsErr
}

func (s *stubCardsReader) RatingsStaleness(_ context.Context, _, _ string) (repository.RatingsStalenessRow, error) {
	return s.staleness, s.stalenessErr
}

func (s *stubCardsReader) TouchRatingsCachedAt(_ context.Context, _, _ string) error {
	return s.touchErr
}

func (s *stubCardsReader) CFBRatingsBySet(_ context.Context, _ string) ([]repository.CFBRatingRow, error) {
	return s.cfbBySet, s.cfbBySetErr
}

func (s *stubCardsReader) CFBRatingByCard(_ context.Context, _, _ string) (*repository.CFBRatingRow, error) {
	return s.cfbByCard, s.cfbByCardErr
}

func (s *stubCardsReader) CFBRatingsCount(_ context.Context, _ string) (int, error) {
	return s.cfbCount, s.cfbCountErr
}

func (s *stubCardsReader) ImportCFBRatings(_ context.Context, imports []repository.CFBImport) (int, error) {
	if s.importedErr != nil {
		return 0, s.importedErr
	}
	return len(imports), nil
}

func (s *stubCardsReader) LinkCFBArenaIds(_ context.Context, _ string) (int, error) {
	return s.linked, s.linkedErr
}

func (s *stubCardsReader) DeleteCFBRatings(_ context.Context, _ string) (int, error) {
	return s.deleted, s.deletedErr
}

// ─── helpers ────────────────────────────────────────────────────────────────

func authedCardsRequest(t *testing.T, method, target string, body []byte, userID int64) *http.Request {
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

func decodeCardsEnvelope(t *testing.T, body []byte, into any) {
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

func chiCardsContext(req *http.Request, kvs ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i+1 < len(kvs); i += 2 {
		rctx.URLParams.Add(kvs[i], kvs[i+1])
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// ─── Search / GetByArenaID / Sets ──────────────────────────────────────────

func TestCardsSearch_HappyPath(t *testing.T) {
	reader := &stubCardsReader{search: []repository.SetCardRow{
		{ArenaID: 100, Name: "Lightning Bolt", SetCode: "M21", TypeLine: "Instant"},
	}}
	h := handlers.NewCardsHandler(reader, &cardsAccountLookup{accountID: 7, found: true})
	req := authedCardsRequest(t, http.MethodGet, "/api/v1/cards?q=lightning", nil, 168)
	rr := httptest.NewRecorder()
	h.Search(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeCardsEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["Name"] != "Lightning Bolt" {
		t.Errorf("search: %v", arr)
	}
}

func TestCardsSearch_MissingQuery(t *testing.T) {
	h := handlers.NewCardsHandler(&stubCardsReader{}, &cardsAccountLookup{accountID: 7, found: true})
	req := authedCardsRequest(t, http.MethodGet, "/api/v1/cards", nil, 168)
	rr := httptest.NewRecorder()
	h.Search(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestCardsGetByArenaID_NotFound(t *testing.T) {
	h := handlers.NewCardsHandler(&stubCardsReader{byArenaID: nil}, &cardsAccountLookup{accountID: 7, found: true})
	req := authedCardsRequest(t, http.MethodGet, "/api/v1/cards/99", nil, 168)
	req = chiCardsContext(req, "arenaId", "99")
	rr := httptest.NewRecorder()
	h.GetByArenaID(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestCardsAllSets_HappyPath(t *testing.T) {
	reader := &stubCardsReader{allSets: []repository.SetInfoRow{
		{Code: "DSK", Name: "Duskmourn", IconSvgURI: "x.svg", SetType: "expansion"},
	}}
	h := handlers.NewCardsHandler(reader, &cardsAccountLookup{accountID: 7, found: true})
	req := authedCardsRequest(t, http.MethodGet, "/api/v1/cards/sets", nil, 168)
	rr := httptest.NewRecorder()
	h.AllSets(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var arr []map[string]any
	decodeCardsEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["code"] != "DSK" {
		t.Errorf("sets: %v", arr)
	}
}

// ─── Ratings ───────────────────────────────────────────────────────────────

func TestCardsRatings_HappyPathWithStaleness(t *testing.T) {
	cached := time.Now().UTC().Add(-3 * time.Hour)
	gihwr := 0.585
	reader := &stubCardsReader{
		cardRatings: []repository.CardRatingRow{
			{ArenaID: 100, Name: "Lightning Bolt", GIHWR: &gihwr, CachedAt: cached},
		},
		staleness: repository.RatingsStalenessRow{CachedAt: &cached, IsStale: false, CardCount: 1},
	}
	h := handlers.NewCardsHandler(reader, &cardsAccountLookup{accountID: 7, found: true})
	req := authedCardsRequest(t, http.MethodGet, "/api/v1/cards/ratings/dsk/PremierDraft", nil, 168)
	req = chiCardsContext(req, "setCode", "dsk", "format", "PremierDraft")
	rr := httptest.NewRecorder()
	h.CardRatings(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("X-Cache-Age-Hours") == "" {
		t.Errorf("expected X-Cache-Age-Hours header")
	}
	if rr.Header().Get("X-Cache-Degraded") != "" {
		t.Errorf("not stale, X-Cache-Degraded should be absent")
	}
	var arr []map[string]any
	decodeCardsEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["tier"] != "A" {
		t.Errorf("ratings tier mapping: %v", arr)
	}
}

func TestCardsColorRatings_HappyPath(t *testing.T) {
	wr := 0.55
	games := 1000
	reader := &stubCardsReader{colorRatings: []repository.ColorRatingRow{
		{ColorCombination: "WU", WinRate: &wr, GamesPlayed: &games},
	}}
	h := handlers.NewCardsHandler(reader, &cardsAccountLookup{accountID: 7, found: true})
	req := authedCardsRequest(t, http.MethodGet, "/api/v1/cards/ratings/dsk/colors", nil, 168)
	req = chiCardsContext(req, "setCode", "dsk")
	rr := httptest.NewRecorder()
	h.ColorRatings(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var arr []map[string]any
	decodeCardsEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["color_combination"] != "WU" {
		t.Errorf("colors: %v", arr)
	}
}

func TestCardsRefresh_StubBumpsCachedAt(t *testing.T) {
	reader := &stubCardsReader{}
	h := handlers.NewCardsHandler(reader, &cardsAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"format": "PremierDraft"})
	req := authedCardsRequest(t, http.MethodPost, "/api/v1/cards/ratings/dsk/refresh", body, 168)
	req = chiCardsContext(req, "setCode", "dsk")
	rr := httptest.NewRecorder()
	h.RefreshRatings(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
}

// ─── Account-scoped ────────────────────────────────────────────────────────

func TestCardsCollectionQuantities_HappyPath(t *testing.T) {
	reader := &stubCardsReader{collectionQty: map[int]int{100: 4, 200: 2}}
	h := handlers.NewCardsHandler(reader, &cardsAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"arenaIDs": []int{100, 200}})
	req := authedCardsRequest(t, http.MethodPost, "/api/v1/cards/collection-quantities", body, 168)
	rr := httptest.NewRecorder()
	h.CollectionQuantities(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var resp map[string]int
	decodeCardsEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["100"] != 4 || resp["200"] != 2 {
		t.Errorf("qty map: %v", resp)
	}
}

func TestCardsSearchWithCollection_HappyPath(t *testing.T) {
	reader := &stubCardsReader{withCollection: []repository.SetCardWithQty{
		{SetCardRow: repository.SetCardRow{ArenaID: 100, Name: "Bolt", SetCode: "DSK"}, Quantity: 4},
	}}
	h := handlers.NewCardsHandler(reader, &cardsAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"query": "bolt"})
	req := authedCardsRequest(t, http.MethodPost, "/api/v1/cards/search-with-collection", body, 168)
	rr := httptest.NewRecorder()
	h.SearchWithCollection(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeCardsEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["quantity"].(float64) != 4 {
		t.Errorf("with-collection: %v", arr)
	}
}

// ─── CFB ──────────────────────────────────────────────────────────────────

func TestCardsCFBRatings_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubCardsReader{cfbBySet: []repository.CFBRatingRow{
		{ID: 1, CardName: "Bolt", SetCode: "DSK", LimitedRating: 4.5, ImportedAt: now, UpdatedAt: now},
	}}
	h := handlers.NewCardsHandler(reader, &cardsAccountLookup{accountID: 7, found: true})
	req := authedCardsRequest(t, http.MethodGet, "/api/v1/cards/cfb/dsk", nil, 168)
	req = chiCardsContext(req, "setCode", "dsk")
	rr := httptest.NewRecorder()
	h.CFBRatings(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var arr []map[string]any
	decodeCardsEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["limitedRating"].(float64) != 4.5 {
		t.Errorf("cfb: %v", arr)
	}
}

func TestCardsCFBImport_HappyPath(t *testing.T) {
	reader := &stubCardsReader{}
	h := handlers.NewCardsHandler(reader, &cardsAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"ratings": []map[string]any{
		{"card_name": "Bolt", "set_code": "DSK", "limited_rating": 4.5},
	}})
	req := authedCardsRequest(t, http.MethodPost, "/api/v1/cards/cfb/import", body, 168)
	rr := httptest.NewRecorder()
	h.ImportCFB(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeCardsEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["imported"].(float64) != 1 {
		t.Errorf("imported: %v", resp)
	}
}

func TestCardsCFBImport_RejectsEmpty(t *testing.T) {
	h := handlers.NewCardsHandler(&stubCardsReader{}, &cardsAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"ratings": []any{}})
	req := authedCardsRequest(t, http.MethodPost, "/api/v1/cards/cfb/import", body, 168)
	rr := httptest.NewRecorder()
	h.ImportCFB(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestCardsCFBLinkArenaIds_HappyPath(t *testing.T) {
	reader := &stubCardsReader{linked: 5}
	h := handlers.NewCardsHandler(reader, &cardsAccountLookup{accountID: 7, found: true})
	req := authedCardsRequest(t, http.MethodPost, "/api/v1/cards/cfb/dsk/link-arena-ids", nil, 168)
	req = chiCardsContext(req, "setCode", "dsk")
	rr := httptest.NewRecorder()
	h.LinkCFBArenaIds(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var resp map[string]any
	decodeCardsEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["linked"].(float64) != 5 {
		t.Errorf("linked: %v", resp)
	}
}

func TestCardsCFBDelete_HappyPath(t *testing.T) {
	reader := &stubCardsReader{deleted: 10}
	h := handlers.NewCardsHandler(reader, &cardsAccountLookup{accountID: 7, found: true})
	req := authedCardsRequest(t, http.MethodDelete, "/api/v1/cards/cfb/dsk", nil, 168)
	req = chiCardsContext(req, "setCode", "dsk")
	rr := httptest.NewRecorder()
	h.DeleteCFB(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── Auth ──────────────────────────────────────────────────────────────────

func TestCardsAllSets_Unauthorized(t *testing.T) {
	h := handlers.NewCardsHandler(&stubCardsReader{}, &cardsAccountLookup{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/sets", nil)
	rr := httptest.NewRecorder()
	h.AllSets(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: %d", rr.Code)
	}
}
