package handlers_test

import (
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

type standardAccountLookup struct {
	accountID int64
	found     bool
	err       error
}

func (s *standardAccountLookup) GetAccountIDByUserID(_ context.Context, _ int64) (int64, bool, error) {
	return s.accountID, s.found, s.err
}

type stubStandardReader struct {
	sets    []repository.StandardSetRow
	setsErr error

	config    repository.StandardConfigRow
	configErr error

	cardRow repository.CardLegalityRow
	cardErr error

	deck    *repository.StandardDeckRow
	deckErr error

	deckCards    []repository.DeckCardForValidation
	deckCardsErr error

	accountDecks    []repository.AccountStandardDeckRow
	accountDecksErr error

	setByCode    *repository.StandardSetRow
	setByCodeErr error

	cardCount       int
	cardCountErr    error
	rotatingSetCnt  int
	rotatingSetErr  error
	rotatingSets    []repository.StandardSetRow
	rotatingSetsErr error
}

func (s *stubStandardReader) ListStandardSets(_ context.Context) ([]repository.StandardSetRow, error) {
	return s.sets, s.setsErr
}

func (s *stubStandardReader) GetStandardConfig(_ context.Context) (repository.StandardConfigRow, error) {
	return s.config, s.configErr
}

func (s *stubStandardReader) CardByArenaID(_ context.Context, _ int) (repository.CardLegalityRow, error) {
	return s.cardRow, s.cardErr
}

func (s *stubStandardReader) DeckByID(_ context.Context, _ int64, _ string) (*repository.StandardDeckRow, error) {
	return s.deck, s.deckErr
}

func (s *stubStandardReader) DeckCardsForValidation(_ context.Context, _ string) ([]repository.DeckCardForValidation, error) {
	return s.deckCards, s.deckCardsErr
}

func (s *stubStandardReader) ListAccountStandardDecks(_ context.Context, _ int64) ([]repository.AccountStandardDeckRow, error) {
	return s.accountDecks, s.accountDecksErr
}

func (s *stubStandardReader) SetByCode(_ context.Context, _ string) (*repository.StandardSetRow, error) {
	return s.setByCode, s.setByCodeErr
}

func (s *stubStandardReader) CountStandardCardsAcrossSets(_ context.Context, _ string) (int, error) {
	return s.cardCount, s.cardCountErr
}

func (s *stubStandardReader) CountStandardSetsRotatingOn(_ context.Context, _ string) (int, error) {
	return s.rotatingSetCnt, s.rotatingSetErr
}

func (s *stubStandardReader) SetsRotatingOn(_ context.Context, _ string) ([]repository.StandardSetRow, error) {
	return s.rotatingSets, s.rotatingSetsErr
}

// ─── helpers ────────────────────────────────────────────────────────────────

func authedStandardRequest(t *testing.T, method, target string, userID int64) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	return req.WithContext(bffmiddleware.WithUserID(req.Context(), userID))
}

func decodeStandardEnvelope(t *testing.T, body []byte, into any) {
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

func chiURLContext(req *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// ─── Sets ──────────────────────────────────────────────────────────────────

func TestStandardSets_HappyPath(t *testing.T) {
	rotation := time.Now().UTC().AddDate(0, 0, 30).Format("2006-01-02")
	reader := &stubStandardReader{sets: []repository.StandardSetRow{
		{Code: "DSK", Name: "Duskmourn", ReleasedAt: "2024-09-27", RotationDate: &rotation, IconSvgURI: "x.svg", CardCount: 286},
	}}
	h := handlers.NewStandardHandler(reader, &standardAccountLookup{accountID: 7, found: true})
	req := authedStandardRequest(t, http.MethodGet, "/api/v1/standard/sets", 168)
	rr := httptest.NewRecorder()
	h.Sets(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeStandardEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["code"] != "DSK" {
		t.Errorf("sets: %v", arr)
	}
	if !arr[0]["isRotatingSoon"].(bool) {
		t.Errorf("expected isRotatingSoon=true for date 30 days out: %v", arr[0])
	}
}

func TestStandardSets_Unauthorized(t *testing.T) {
	h := handlers.NewStandardHandler(&stubStandardReader{}, &standardAccountLookup{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/standard/sets", nil)
	rr := httptest.NewRecorder()
	h.Sets(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── Config ────────────────────────────────────────────────────────────────

func TestStandardConfig_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubStandardReader{config: repository.StandardConfigRow{
		ID: 1, NextRotationDate: "2026-09-01", RotationEnabled: true, UpdatedAt: now,
	}}
	h := handlers.NewStandardHandler(reader, &standardAccountLookup{accountID: 7, found: true})
	req := authedStandardRequest(t, http.MethodGet, "/api/v1/standard/config", 168)
	rr := httptest.NewRecorder()
	h.Config(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var resp map[string]any
	decodeStandardEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["nextRotationDate"] != "2026-09-01" || !resp["rotationEnabled"].(bool) {
		t.Errorf("config: %v", resp)
	}
}

// ─── Rotation ──────────────────────────────────────────────────────────────

func TestStandardRotation_HappyPath(t *testing.T) {
	rotation := "2026-09-01"
	reader := &stubStandardReader{
		config: repository.StandardConfigRow{NextRotationDate: rotation, RotationEnabled: true},
		rotatingSets: []repository.StandardSetRow{
			{Code: "DOM", Name: "Dominaria", RotationDate: &rotation, CardCount: 250},
		},
		cardCount:    250,
		accountDecks: []repository.AccountStandardDeckRow{},
	}
	h := handlers.NewStandardHandler(reader, &standardAccountLookup{accountID: 7, found: true})
	req := authedStandardRequest(t, http.MethodGet, "/api/v1/standard/rotation", 168)
	rr := httptest.NewRecorder()
	h.Rotation(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeStandardEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["nextRotationDate"] != rotation || resp["rotatingCardCount"].(float64) != 250 {
		t.Errorf("rotation: %v", resp)
	}
}

// ─── AffectedDecks ─────────────────────────────────────────────────────────

func TestStandardAffectedDecks_HappyPath(t *testing.T) {
	rotation := "2026-09-01"
	reader := &stubStandardReader{
		config: repository.StandardConfigRow{NextRotationDate: rotation},
		accountDecks: []repository.AccountStandardDeckRow{
			{ID: "d1", Name: "Mono Red", Format: "standard"},
		},
		deckCards: []repository.DeckCardForValidation{
			{CardID: 1, Quantity: 4, Board: "main", Name: "Goblin Guide", SetCode: "DOM", RotationDate: &rotation},
			{CardID: 2, Quantity: 4, Board: "main", Name: "Lightning Strike", SetCode: "ZNR"},
		},
	}
	h := handlers.NewStandardHandler(reader, &standardAccountLookup{accountID: 7, found: true})
	req := authedStandardRequest(t, http.MethodGet, "/api/v1/standard/rotation/affected-decks", 168)
	rr := httptest.NewRecorder()
	h.AffectedDecks(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeStandardEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["rotatingCardCount"].(float64) != 4 || arr[0]["totalCards"].(float64) != 8 {
		t.Errorf("affected: %v", arr)
	}
	if arr[0]["percentAffected"].(float64) != 0.5 {
		t.Errorf("percentAffected: %v", arr[0])
	}
}

// ─── ValidateDeck ──────────────────────────────────────────────────────────

func TestStandardValidateDeck_LegalDeck(t *testing.T) {
	reader := &stubStandardReader{
		config: repository.StandardConfigRow{NextRotationDate: "2026-09-01"},
		deck:   &repository.StandardDeckRow{ID: "d1", Name: "Mono Red", Format: "standard"},
		deckCards: []repository.DeckCardForValidation{
			{CardID: 1, Quantity: 4, Board: "main", Name: "Goblin Guide", SetCode: "DSK", SetIsStandardLegal: true, Legalities: `{"standard":"legal"}`},
		},
	}
	h := handlers.NewStandardHandler(reader, &standardAccountLookup{accountID: 7, found: true})
	req := authedStandardRequest(t, http.MethodPost, "/api/v1/standard/validate/d1", 168)
	req = chiURLContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.ValidateDeck(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeStandardEnvelope(t, rr.Body.Bytes(), &resp)
	if !resp["isLegal"].(bool) {
		t.Errorf("expected isLegal=true: %v", resp)
	}
}

func TestStandardValidateDeck_DeckNotFound(t *testing.T) {
	reader := &stubStandardReader{deck: nil}
	h := handlers.NewStandardHandler(reader, &standardAccountLookup{accountID: 7, found: true})
	req := authedStandardRequest(t, http.MethodPost, "/api/v1/standard/validate/missing", 168)
	req = chiURLContext(req, "deckId", "missing")
	rr := httptest.NewRecorder()
	h.ValidateDeck(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: %d (expected 404)", rr.Code)
	}
}

func TestStandardValidateDeck_FlagsBannedCard(t *testing.T) {
	reader := &stubStandardReader{
		config: repository.StandardConfigRow{NextRotationDate: "2026-09-01"},
		deck:   &repository.StandardDeckRow{ID: "d1", Name: "Test", Format: "standard"},
		deckCards: []repository.DeckCardForValidation{
			{CardID: 99, Quantity: 4, Board: "main", Name: "Banned Card", SetCode: "DSK", SetIsStandardLegal: true, Legalities: `{"standard":"banned"}`},
		},
	}
	h := handlers.NewStandardHandler(reader, &standardAccountLookup{accountID: 7, found: true})
	req := authedStandardRequest(t, http.MethodPost, "/api/v1/standard/validate/d1", 168)
	req = chiURLContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.ValidateDeck(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeStandardEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["isLegal"].(bool) {
		t.Errorf("expected isLegal=false: %v", resp)
	}
	errs, _ := resp["errors"].([]any)
	if len(errs) == 0 {
		t.Errorf("expected at least one error: %v", resp)
	}
}

// ─── CardLegality ──────────────────────────────────────────────────────────

func TestStandardCardLegality_HappyPath(t *testing.T) {
	reader := &stubStandardReader{cardRow: repository.CardLegalityRow{
		ArenaID: 100, Name: "Llanowar Elves", SetCode: "DOM",
		Legalities: `{"standard":"legal","historic":"legal","modern":"not_legal"}`,
	}}
	h := handlers.NewStandardHandler(reader, &standardAccountLookup{accountID: 7, found: true})
	req := authedStandardRequest(t, http.MethodGet, "/api/v1/standard/cards/100/legality", 168)
	req = chiURLContext(req, "arenaId", "100")
	rr := httptest.NewRecorder()
	h.CardLegality(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeStandardEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["standard"] != "legal" || resp["modern"] != "not_legal" {
		t.Errorf("legality: %v", resp)
	}
}

func TestStandardCardLegality_BadArenaID(t *testing.T) {
	h := handlers.NewStandardHandler(&stubStandardReader{}, &standardAccountLookup{accountID: 7, found: true})
	req := authedStandardRequest(t, http.MethodGet, "/api/v1/standard/cards/notanumber/legality", 168)
	req = chiURLContext(req, "arenaId", "notanumber")
	rr := httptest.NewRecorder()
	h.CardLegality(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}
