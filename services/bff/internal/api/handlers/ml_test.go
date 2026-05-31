package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/handlers"
	bffmiddleware "github.com/RdHamilton/vault-mtg/services/bff/internal/api/middleware"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
	"github.com/go-chi/chi/v5"
)

// ─── stubs ──────────────────────────────────────────────────────────────────

type mlAccountLookup struct {
	accountID int64
	found     bool
	err       error
}

func (m *mlAccountLookup) GetAccountIDByUserID(_ context.Context, _ int64) (int64, bool, error) {
	return m.accountID, m.found, m.err
}

type stubMLSuggestionsReader struct {
	suggestions    []repository.SuggestionRow
	suggestionsErr error
	dismissed      bool
	dismissedErr   error
}

func (s *stubMLSuggestionsReader) ListSuggestions(_ context.Context, _ int64, _ string, _ bool) ([]repository.SuggestionRow, error) {
	return s.suggestions, s.suggestionsErr
}

func (s *stubMLSuggestionsReader) DismissSuggestion(_ context.Context, _ int64, _ int64) (bool, error) {
	return s.dismissed, s.dismissedErr
}

type stubMLReader struct {
	applied                bool
	appliedErr             error
	synergyReport          *repository.SynergyReportRow
	synergyReportErr       error
	cardSynergies          []repository.CardCombinationStatsRow
	cardSynergiesErr       error
	cardSynergiesLimit     int
	combinationStats       *repository.CardCombinationStatsRow
	combinationStatsErr    error
	processHistoryResult   *repository.ProcessHistoryResult
	processHistoryErr      error
	processHistoryDays     int
	processHistoryCap      int
	playPatterns           *repository.UserPlayPatternsRow
	playPatternsErr        error
	upsertPatterns         *repository.UserPlayPatternsRow
	upsertPatternsErr      error
	clearErr               error
}

func (s *stubMLReader) ApplySuggestion(_ context.Context, _, _ int64) (bool, error) {
	return s.applied, s.appliedErr
}

func (s *stubMLReader) SynergyReport(_ context.Context, _ int64, _ string) (*repository.SynergyReportRow, error) {
	return s.synergyReport, s.synergyReportErr
}

func (s *stubMLReader) CardSynergies(_ context.Context, _ int, _ string, limit int) ([]repository.CardCombinationStatsRow, error) {
	s.cardSynergiesLimit = limit
	return s.cardSynergies, s.cardSynergiesErr
}

func (s *stubMLReader) CombinationStats(_ context.Context, _, _ int, _ string) (*repository.CardCombinationStatsRow, error) {
	return s.combinationStats, s.combinationStatsErr
}

func (s *stubMLReader) ComputeAndWritePairStats(_ context.Context, _ int64, _ string, days int, cap int) (*repository.ProcessHistoryResult, error) {
	s.processHistoryDays = days
	s.processHistoryCap = cap
	if s.processHistoryResult == nil && s.processHistoryErr == nil {
		return &repository.ProcessHistoryResult{}, nil
	}
	return s.processHistoryResult, s.processHistoryErr
}

func (s *stubMLReader) PlayPatterns(_ context.Context, _ string) (*repository.UserPlayPatternsRow, error) {
	return s.playPatterns, s.playPatternsErr
}

func (s *stubMLReader) UpsertPlayPatternsStub(_ context.Context, _ string) (*repository.UserPlayPatternsRow, error) {
	return s.upsertPatterns, s.upsertPatternsErr
}

func (s *stubMLReader) ClearLearnedDataForAccount(_ context.Context, _ int64, _ string) error {
	return s.clearErr
}

// ─── helpers ────────────────────────────────────────────────────────────────

func authedMLRequest(t *testing.T, method, target string, userID int64) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	return req.WithContext(bffmiddleware.WithUserID(req.Context(), userID))
}

func decodeMLEnvelope(t *testing.T, body []byte, into any) {
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

func chiMLContext(req *http.Request, kvs ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i+1 < len(kvs); i += 2 {
		rctx.URLParams.Add(kvs[i], kvs[i+1])
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func newMLHandler(s *stubMLSuggestionsReader, m *stubMLReader) *handlers.MLHandler {
	return handlers.NewMLHandler(s, m, &mlAccountLookup{accountID: 7, found: true})
}

// ─── list / generate / dismiss / apply ──────────────────────────────────────

func TestMLList_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	conf := 0.82
	desc := "Improves curve"
	cardName := "Lightning Strike"
	cid := 1234
	reader := &stubMLSuggestionsReader{suggestions: []repository.SuggestionRow{{
		ID: 1, DeckID: "d1", SuggestionType: "add", CardID: &cid, CardName: &cardName,
		Confidence: conf, ExpectedWinRateChange: 2.5, Title: "Add 2 lightning strikes",
		Description: &desc, IsDismissed: false, WasApplied: false, CreatedAt: now,
	}}}
	h := newMLHandler(reader, &stubMLReader{})
	req := authedMLRequest(t, http.MethodGet, "/api/v1/decks/d1/ml-suggestions", 168)
	req = chiMLContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.ListMLSuggestions(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeMLEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["title"] != "Add 2 lightning strikes" {
		t.Fatalf("payload: %v", arr)
	}
	if arr[0]["confidence"].(float64) != conf {
		t.Errorf("confidence not propagated: %v", arr[0]["confidence"])
	}
	if arr[0]["cardName"] != cardName {
		t.Errorf("cardName not propagated: %v", arr[0]["cardName"])
	}
}

func TestMLList_RequiresDeckID(t *testing.T) {
	h := newMLHandler(&stubMLSuggestionsReader{}, &stubMLReader{})
	req := authedMLRequest(t, http.MethodGet, "/api/v1/decks//ml-suggestions", 168)
	req = chiMLContext(req, "deckId", "")
	rr := httptest.NewRecorder()
	h.ListMLSuggestions(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestMLList_AccountNotFoundReturnsEmpty(t *testing.T) {
	h := handlers.NewMLHandler(&stubMLSuggestionsReader{}, &stubMLReader{}, &mlAccountLookup{found: false})
	req := authedMLRequest(t, http.MethodGet, "/api/v1/decks/d1/ml-suggestions", 168)
	req = chiMLContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.ListMLSuggestions(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeMLEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 0 {
		t.Errorf("expected empty array, got %v", arr)
	}
}

func TestMLGenerate_StubReturnsExistingList(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubMLSuggestionsReader{suggestions: []repository.SuggestionRow{{
		ID: 9, DeckID: "d1", SuggestionType: "swap", Confidence: 0.5,
		Title: "Swap A for B", CreatedAt: now,
	}}}
	h := newMLHandler(reader, &stubMLReader{})
	req := authedMLRequest(t, http.MethodPost, "/api/v1/decks/d1/ml-suggestions/generate", 168)
	req = chiMLContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.GenerateMLSuggestions(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeMLEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 {
		t.Fatalf("expected 1 result, got %v", arr)
	}
	suggestion, ok := arr[0]["suggestion"].(map[string]any)
	if !ok {
		t.Fatalf("missing suggestion key: %v", arr[0])
	}
	if suggestion["id"].(float64) != 9 {
		t.Errorf("id not propagated: %v", suggestion["id"])
	}
}

func TestMLDismiss_HappyPath(t *testing.T) {
	h := newMLHandler(&stubMLSuggestionsReader{dismissed: true}, &stubMLReader{})
	req := authedMLRequest(t, http.MethodPut, "/api/v1/ml-suggestions/3/dismiss", 168)
	req = chiMLContext(req, "suggestionId", "3")
	rr := httptest.NewRecorder()
	h.DismissMLSuggestion(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("status: %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestMLDismiss_NotFound(t *testing.T) {
	h := newMLHandler(&stubMLSuggestionsReader{dismissed: false}, &stubMLReader{})
	req := authedMLRequest(t, http.MethodPut, "/api/v1/ml-suggestions/3/dismiss", 168)
	req = chiMLContext(req, "suggestionId", "3")
	rr := httptest.NewRecorder()
	h.DismissMLSuggestion(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestMLDismiss_RejectsBadID(t *testing.T) {
	h := newMLHandler(&stubMLSuggestionsReader{}, &stubMLReader{})
	req := authedMLRequest(t, http.MethodPut, "/api/v1/ml-suggestions/abc/dismiss", 168)
	req = chiMLContext(req, "suggestionId", "abc")
	rr := httptest.NewRecorder()
	h.DismissMLSuggestion(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestMLApply_HappyPath(t *testing.T) {
	h := newMLHandler(&stubMLSuggestionsReader{}, &stubMLReader{applied: true})
	req := authedMLRequest(t, http.MethodPut, "/api/v1/ml-suggestions/5/apply", 168)
	req = chiMLContext(req, "suggestionId", "5")
	rr := httptest.NewRecorder()
	h.ApplyMLSuggestion(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("status: %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestMLApply_NotFound(t *testing.T) {
	h := newMLHandler(&stubMLSuggestionsReader{}, &stubMLReader{applied: false})
	req := authedMLRequest(t, http.MethodPut, "/api/v1/ml-suggestions/5/apply", 168)
	req = chiMLContext(req, "suggestionId", "5")
	rr := httptest.NewRecorder()
	h.ApplyMLSuggestion(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestMLApply_RepositoryError(t *testing.T) {
	h := newMLHandler(&stubMLSuggestionsReader{}, &stubMLReader{appliedErr: errors.New("boom")})
	req := authedMLRequest(t, http.MethodPut, "/api/v1/ml-suggestions/5/apply", 168)
	req = chiMLContext(req, "suggestionId", "5")
	rr := httptest.NewRecorder()
	h.ApplyMLSuggestion(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── synergy report / card synergies / combinations ─────────────────────────

func TestMLSynergyReport_HappyPath(t *testing.T) {
	name1 := "Sheoldred"
	name2 := "Atraxa"
	h := newMLHandler(&stubMLSuggestionsReader{}, &stubMLReader{
		synergyReport: &repository.SynergyReportRow{
			DeckID: "d1", CardCount: 30, TotalPairs: 1, AvgSynergyScore: 0.6,
			Synergies: []repository.SynergyReportPair{{
				Card1ID: 100, Card1Name: &name1, Card2ID: 200, Card2Name: &name2,
				SynergyScore: 0.6, GamesTogether: 10, WinRate: 0.7,
			}},
		},
	})
	req := authedMLRequest(t, http.MethodGet, "/api/v1/decks/d1/synergy-report", 168)
	req = chiMLContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.SynergyReport(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeMLEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["cardCount"].(float64) != 30 || resp["totalPairs"].(float64) != 1 {
		t.Errorf("counts: %v", resp)
	}
	pairs, _ := resp["synergies"].([]any)
	if len(pairs) != 1 {
		t.Fatalf("synergies: %v", pairs)
	}
}

func TestMLSynergyReport_NotFound(t *testing.T) {
	h := newMLHandler(&stubMLSuggestionsReader{}, &stubMLReader{synergyReport: nil})
	req := authedMLRequest(t, http.MethodGet, "/api/v1/decks/d1/synergy-report", 168)
	req = chiMLContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.SynergyReport(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestMLCardSynergies_HappyPathClampsLimit(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubMLReader{cardSynergies: []repository.CardCombinationStatsRow{{
		ID: 1, CardID1: 10, CardID2: 20, Format: "Standard",
		GamesTogether: 5, WinsTogether: 3, SynergyScore: 0.4, ConfidenceScore: 0.5,
		CreatedAt: now, UpdatedAt: now,
	}}}
	h := newMLHandler(&stubMLSuggestionsReader{}, reader)
	req := authedMLRequest(t, http.MethodGet, "/api/v1/cards/10/synergies?format=Standard&limit=5", 168)
	req = chiMLContext(req, "cardId", "10")
	rr := httptest.NewRecorder()
	h.CardSynergies(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	if reader.cardSynergiesLimit != 5 {
		t.Errorf("limit not propagated: %d", reader.cardSynergiesLimit)
	}
}

func TestMLCardSynergies_DefaultsLimitAndFormat(t *testing.T) {
	reader := &stubMLReader{}
	h := newMLHandler(&stubMLSuggestionsReader{}, reader)
	req := authedMLRequest(t, http.MethodGet, "/api/v1/cards/10/synergies", 168)
	req = chiMLContext(req, "cardId", "10")
	rr := httptest.NewRecorder()
	h.CardSynergies(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	if reader.cardSynergiesLimit != 10 {
		t.Errorf("default limit not 10: %d", reader.cardSynergiesLimit)
	}
}

func TestMLCardSynergies_RejectsBadID(t *testing.T) {
	h := newMLHandler(&stubMLSuggestionsReader{}, &stubMLReader{})
	req := authedMLRequest(t, http.MethodGet, "/api/v1/cards/abc/synergies", 168)
	req = chiMLContext(req, "cardId", "abc")
	rr := httptest.NewRecorder()
	h.CardSynergies(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestMLCombinationStats_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	h := newMLHandler(&stubMLSuggestionsReader{}, &stubMLReader{
		combinationStats: &repository.CardCombinationStatsRow{
			ID: 7, CardID1: 10, CardID2: 20, Format: "Standard",
			SynergyScore: 0.5, CreatedAt: now, UpdatedAt: now,
		},
	})
	req := authedMLRequest(t, http.MethodGet, "/api/v1/ml/combinations?card1=10&card2=20&format=Standard", 168)
	rr := httptest.NewRecorder()
	h.CombinationStats(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeMLEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["cardId1"].(float64) != 10 || resp["cardId2"].(float64) != 20 {
		t.Errorf("ids: %v", resp)
	}
}

func TestMLCombinationStats_NotFound(t *testing.T) {
	h := newMLHandler(&stubMLSuggestionsReader{}, &stubMLReader{combinationStats: nil})
	req := authedMLRequest(t, http.MethodGet, "/api/v1/ml/combinations?card1=10&card2=20", 168)
	rr := httptest.NewRecorder()
	h.CombinationStats(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestMLCombinationStats_RejectsBadParams(t *testing.T) {
	h := newMLHandler(&stubMLSuggestionsReader{}, &stubMLReader{})
	req := authedMLRequest(t, http.MethodGet, "/api/v1/ml/combinations?card1=abc&card2=20", 168)
	rr := httptest.NewRecorder()
	h.CombinationStats(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("bad card1: status %d", rr.Code)
	}

	req2 := authedMLRequest(t, http.MethodGet, "/api/v1/ml/combinations?card1=10&card2=10", 168)
	rr2 := httptest.NewRecorder()
	h.CombinationStats(rr2, req2)
	if rr2.Code != http.StatusBadRequest {
		t.Errorf("equal cards: status %d", rr2.Code)
	}
}

// ─── ML management ───────────────────────────────────────────────────────────

func TestMLProcessHistory_HappyPath(t *testing.T) {
	stub := &stubMLReader{
		processHistoryResult: &repository.ProcessHistoryResult{
			PairsWritten:     12,
			MatchesProcessed: 50,
			Truncated:        false,
		},
	}
	h := newMLHandler(&stubMLSuggestionsReader{}, stub)
	req := authedMLRequest(t, http.MethodPost, "/api/v1/ml/process-history?days=30", 168)
	rr := httptest.NewRecorder()
	h.ProcessMatchHistory(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeMLEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("status: got %v, want ok", resp["status"])
	}
	if resp["pairs_written"].(float64) != 12 {
		t.Errorf("pairs_written: %v", resp["pairs_written"])
	}
	if resp["matches_processed"].(float64) != 50 {
		t.Errorf("matches_processed: %v", resp["matches_processed"])
	}
	if resp["truncated"].(bool) != false {
		t.Errorf("truncated: %v", resp["truncated"])
	}
	// days param must be propagated to the repository.
	if stub.processHistoryDays != 30 {
		t.Errorf("days propagated: got %d, want 30", stub.processHistoryDays)
	}
}

func TestMLProcessHistory_TruncatedPath(t *testing.T) {
	stub := &stubMLReader{
		processHistoryResult: &repository.ProcessHistoryResult{
			PairsWritten:     500,
			MatchesProcessed: 1000,
			Truncated:        true,
		},
	}
	h := newMLHandler(&stubMLSuggestionsReader{}, stub)
	req := authedMLRequest(t, http.MethodPost, "/api/v1/ml/process-history", 168)
	rr := httptest.NewRecorder()
	h.ProcessMatchHistory(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeMLEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["truncated"].(bool) != true {
		t.Errorf("truncated: expected true, got %v", resp["truncated"])
	}
	if resp["matches_processed"].(float64) != 1000 {
		t.Errorf("matches_processed: %v", resp["matches_processed"])
	}
	// Default days (30) should be used when not provided.
	if stub.processHistoryDays != 30 {
		t.Errorf("default days: got %d, want 30", stub.processHistoryDays)
	}
}

func TestMLProcessHistory_RepositoryError(t *testing.T) {
	stub := &stubMLReader{processHistoryErr: errors.New("db error")}
	h := newMLHandler(&stubMLSuggestionsReader{}, stub)
	req := authedMLRequest(t, http.MethodPost, "/api/v1/ml/process-history", 168)
	rr := httptest.NewRecorder()
	h.ProcessMatchHistory(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestMLProcessHistory_DefaultDays(t *testing.T) {
	stub := &stubMLReader{}
	h := newMLHandler(&stubMLSuggestionsReader{}, stub)
	req := authedMLRequest(t, http.MethodPost, "/api/v1/ml/process-history", 168)
	rr := httptest.NewRecorder()
	h.ProcessMatchHistory(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	if stub.processHistoryDays != 30 {
		t.Errorf("default days: got %d, want 30", stub.processHistoryDays)
	}
}

func TestMLPlayPatterns_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	h := newMLHandler(&stubMLSuggestionsReader{}, &stubMLReader{
		playPatterns: &repository.UserPlayPatternsRow{
			ID: 1, AccountIDText: "7", AggroAffinity: 0.4, ControlAffinity: 0.6,
			TotalMatches: 50, TotalDecks: 3, CreatedAt: now, UpdatedAt: now,
		},
	})
	req := authedMLRequest(t, http.MethodGet, "/api/v1/ml/play-patterns", 168)
	rr := httptest.NewRecorder()
	h.GetUserPlayPatterns(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeMLEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["accountId"] != "7" || resp["totalMatches"].(float64) != 50 {
		t.Errorf("payload: %v", resp)
	}
}

func TestMLPlayPatterns_DefaultsWhenAbsent(t *testing.T) {
	h := newMLHandler(&stubMLSuggestionsReader{}, &stubMLReader{playPatterns: nil})
	req := authedMLRequest(t, http.MethodGet, "/api/v1/ml/play-patterns", 168)
	rr := httptest.NewRecorder()
	h.GetUserPlayPatterns(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeMLEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["accountId"] != "7" || resp["aggroAffinity"].(float64) != 0 {
		t.Errorf("payload: %v", resp)
	}
}

func TestMLUpdatePlayPatterns_StubOK(t *testing.T) {
	h := newMLHandler(&stubMLSuggestionsReader{}, &stubMLReader{
		upsertPatterns: &repository.UserPlayPatternsRow{AccountIDText: "7"},
	})
	req := authedMLRequest(t, http.MethodPost, "/api/v1/ml/play-patterns/update", 168)
	rr := httptest.NewRecorder()
	h.UpdateUserPlayPatterns(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeMLEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("status: %v", resp)
	}
}

func TestMLClearLearnedData_HappyPath(t *testing.T) {
	h := newMLHandler(&stubMLSuggestionsReader{}, &stubMLReader{})
	req := authedMLRequest(t, http.MethodDelete, "/api/v1/ml/learned-data", 168)
	rr := httptest.NewRecorder()
	h.ClearLearnedData(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeMLEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("status: %v", resp)
	}
}

func TestMLClearLearnedData_RepositoryError(t *testing.T) {
	h := newMLHandler(&stubMLSuggestionsReader{}, &stubMLReader{clearErr: errors.New("boom")})
	req := authedMLRequest(t, http.MethodDelete, "/api/v1/ml/learned-data", 168)
	rr := httptest.NewRecorder()
	h.ClearLearnedData(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestMLUnauthorized(t *testing.T) {
	h := newMLHandler(&stubMLSuggestionsReader{}, &stubMLReader{})
	// No userID in context — should be 401
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ml/play-patterns", nil)
	rr := httptest.NewRecorder()
	h.GetUserPlayPatterns(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: %d", rr.Code)
	}
}
