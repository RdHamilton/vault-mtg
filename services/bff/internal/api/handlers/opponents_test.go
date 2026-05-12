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

type oppAccountLookup struct {
	accountID int64
	found     bool
	err       error
}

func (o *oppAccountLookup) GetAccountIDByUserID(_ context.Context, _ int64) (int64, bool, error) {
	return o.accountID, o.found, o.err
}

type stubOpponentsReader struct {
	profile    *repository.OpponentDeckProfileRow
	profileErr error

	cards    []repository.OpponentObservedCardRow
	cardsErr error

	decks       []repository.OpponentDeckProfileRow
	decksTotal  int
	decksErr    error
	decksFilter repository.OpponentDeckFilter

	matchups      []repository.MatchupStatRow
	matchupsTotal int
	matchupsErr   error

	matchup    *repository.MatchupStatRow
	matchupErr error

	expected    []repository.ExpectedCardRow
	expectedErr error

	summary    repository.OpponentHistorySummaryRow
	summaryErr error

	breakdown    []repository.ArchetypeBreakdownRow
	breakdownErr error

	colors    []repository.ColorIdentityBreakdownRow
	colorsErr error
}

func (s *stubOpponentsReader) OpponentProfileForMatch(_ context.Context, _ int64, _ string) (*repository.OpponentDeckProfileRow, error) {
	return s.profile, s.profileErr
}

func (s *stubOpponentsReader) OpponentCardsForMatch(_ context.Context, _ int64, _ string) ([]repository.OpponentObservedCardRow, error) {
	return s.cards, s.cardsErr
}

func (s *stubOpponentsReader) ListOpponentDecks(_ context.Context, _ int64, f repository.OpponentDeckFilter) ([]repository.OpponentDeckProfileRow, int, error) {
	s.decksFilter = f
	return s.decks, s.decksTotal, s.decksErr
}

func (s *stubOpponentsReader) ListMatchups(_ context.Context, _ int64, _ string) ([]repository.MatchupStatRow, int, error) {
	return s.matchups, s.matchupsTotal, s.matchupsErr
}

func (s *stubOpponentsReader) MatchupForArchetypes(_ context.Context, _ int64, _, _, _ string) (*repository.MatchupStatRow, error) {
	return s.matchup, s.matchupErr
}

func (s *stubOpponentsReader) ExpectedCardsForArchetype(_ context.Context, _, _ string) ([]repository.ExpectedCardRow, error) {
	return s.expected, s.expectedErr
}

func (s *stubOpponentsReader) OpponentHistorySummary(_ context.Context, _ int64, _ string) (repository.OpponentHistorySummaryRow, error) {
	return s.summary, s.summaryErr
}

func (s *stubOpponentsReader) ArchetypeBreakdown(_ context.Context, _ int64, _ string) ([]repository.ArchetypeBreakdownRow, error) {
	return s.breakdown, s.breakdownErr
}

func (s *stubOpponentsReader) ColorIdentityBreakdown(_ context.Context, _ int64, _ string) ([]repository.ColorIdentityBreakdownRow, error) {
	return s.colors, s.colorsErr
}

// ─── helpers ────────────────────────────────────────────────────────────────

func authedOppRequest(t *testing.T, method, target string, userID int64) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	return req.WithContext(bffmiddleware.WithUserID(req.Context(), userID))
}

func decodeOppEnvelope(t *testing.T, body []byte, into any) {
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

func chiOppContext(req *http.Request, kvs ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i+1 < len(kvs); i += 2 {
		rctx.URLParams.Add(kvs[i], kvs[i+1])
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// ─── OpponentAnalysis ──────────────────────────────────────────────────────

func TestOpponentsAnalysis_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	archetype := "Mono Red"
	format := "standard"
	reader := &stubOpponentsReader{
		profile: &repository.OpponentDeckProfileRow{
			ID: 1, MatchID: "m1", DetectedArchetype: &archetype,
			ArchetypeConfidence: 0.85, ColorIdentity: "R",
			CardsObserved: 10, EstimatedDeckSize: 60,
			Format: &format, CreatedAt: now, UpdatedAt: now,
		},
		cards: []repository.OpponentObservedCardRow{
			{CardID: 100, TimesSeen: 2},
		},
		expected: []repository.ExpectedCardRow{
			{CardID: 100, CardName: "Goblin Guide", InclusionRate: 0.95, AvgCopies: 4},
			{CardID: 200, CardName: "Lightning Bolt", InclusionRate: 0.9, AvgCopies: 4},
		},
	}
	h := handlers.NewOpponentsHandler(reader, &oppAccountLookup{accountID: 7, found: true})
	req := authedOppRequest(t, http.MethodGet, "/api/v1/matches/m1/opponent-analysis", 168)
	req = chiOppContext(req, "matchId", "m1")
	rr := httptest.NewRecorder()
	h.OpponentAnalysis(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeOppEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["profile"] == nil {
		t.Errorf("expected profile: %v", resp)
	}
	expected, _ := resp["expectedCards"].([]any)
	if len(expected) != 2 {
		t.Errorf("expected expected-cards: %v", resp)
	}
	// First expected card has CardID=100 which was observed → wasSeen=true
	if first, _ := expected[0].(map[string]any); first["wasSeen"] != true {
		t.Errorf("wasSeen flag: %v", first)
	}
}

func TestOpponentsAnalysis_NoAccount(t *testing.T) {
	h := handlers.NewOpponentsHandler(&stubOpponentsReader{}, &oppAccountLookup{found: false})
	req := authedOppRequest(t, http.MethodGet, "/api/v1/matches/m1/opponent-analysis", 168)
	req = chiOppContext(req, "matchId", "m1")
	rr := httptest.NewRecorder()
	h.OpponentAnalysis(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
}

// ─── ListDecks ─────────────────────────────────────────────────────────────

func TestOpponentsListDecks_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubOpponentsReader{
		decks: []repository.OpponentDeckProfileRow{
			{ID: 1, MatchID: "m1", ColorIdentity: "R", CreatedAt: now, UpdatedAt: now},
		},
		decksTotal: 1,
	}
	h := handlers.NewOpponentsHandler(reader, &oppAccountLookup{accountID: 7, found: true})
	req := authedOppRequest(t, http.MethodGet, "/api/v1/opponents/decks?archetype=Mono+Red&min_confidence=0.7", 168)
	rr := httptest.NewRecorder()
	h.ListDecks(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	if reader.decksFilter.Archetype != "Mono Red" || reader.decksFilter.MinConfidence != 0.7 {
		t.Errorf("filter not forwarded: %+v", reader.decksFilter)
	}
}

func TestOpponentsListDecks_BadConfidence(t *testing.T) {
	h := handlers.NewOpponentsHandler(&stubOpponentsReader{}, &oppAccountLookup{accountID: 7, found: true})
	req := authedOppRequest(t, http.MethodGet, "/api/v1/opponents/decks?min_confidence=2", 168)
	rr := httptest.NewRecorder()
	h.ListDecks(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── MatchupStats ──────────────────────────────────────────────────────────

func TestOpponentsMatchupStats_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubOpponentsReader{
		matchups: []repository.MatchupStatRow{
			{
				ID: 1, AccountID: 7, PlayerArchetype: "Mono Red", OpponentArchetype: "Esper Control",
				Format: "standard", TotalMatches: 10, Wins: 6, Losses: 4, CreatedAt: now, UpdatedAt: now,
			},
		},
		matchupsTotal: 1,
	}
	h := handlers.NewOpponentsHandler(reader, &oppAccountLookup{accountID: 7, found: true})
	req := authedOppRequest(t, http.MethodGet, "/api/v1/analytics/matchups", 168)
	rr := httptest.NewRecorder()
	h.MatchupStats(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeOppEnvelope(t, rr.Body.Bytes(), &resp)
	matchups, _ := resp["matchups"].([]any)
	if len(matchups) != 1 {
		t.Fatalf("matchups: %v", resp)
	}
	first, _ := matchups[0].(map[string]any)
	if first["winRate"].(float64) != 0.6 {
		t.Errorf("winRate: %v", first)
	}
}

// ─── OpponentHistory ───────────────────────────────────────────────────────

func TestOpponentsHistory_HappyPath(t *testing.T) {
	reader := &stubOpponentsReader{
		summary: repository.OpponentHistorySummaryRow{
			TotalOpponents: 10, UniqueArchetypes: 3,
			MostCommonArchetype: "Mono Red", MostCommonCount: 5,
		},
		breakdown: []repository.ArchetypeBreakdownRow{
			{Archetype: "Mono Red", Count: 5, Wins: 3},
		},
		colors: []repository.ColorIdentityBreakdownRow{
			{ColorIdentity: "R", Count: 5, Wins: 3},
		},
	}
	h := handlers.NewOpponentsHandler(reader, &oppAccountLookup{accountID: 7, found: true})
	req := authedOppRequest(t, http.MethodGet, "/api/v1/analytics/opponent-history", 168)
	rr := httptest.NewRecorder()
	h.OpponentHistory(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeOppEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["totalOpponents"].(float64) != 10 {
		t.Errorf("totals: %v", resp)
	}
	bd, _ := resp["archetypeBreakdown"].([]any)
	if len(bd) != 1 {
		t.Fatalf("breakdown: %v", resp)
	}
	first, _ := bd[0].(map[string]any)
	if first["percentage"].(float64) != 0.5 || first["winRate"].(float64) != 0.6 {
		t.Errorf("entry math: %v", first)
	}
}

// ─── ExpectedCards ─────────────────────────────────────────────────────────

func TestOpponentsExpectedCards_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	cat := "removal"
	reader := &stubOpponentsReader{
		expected: []repository.ExpectedCardRow{
			{
				ID: 1, ArchetypeName: "Mono Red", Format: "standard",
				CardID: 100, CardName: "Lightning Bolt", InclusionRate: 0.95,
				AvgCopies: 4, IsSignature: true, Category: &cat, CreatedAt: now,
			},
		},
	}
	h := handlers.NewOpponentsHandler(reader, &oppAccountLookup{accountID: 7, found: true})
	req := authedOppRequest(t, http.MethodGet, "/api/v1/archetypes/Mono+Red/expected-cards?format=standard", 168)
	req = chiOppContext(req, "name", "Mono Red")
	rr := httptest.NewRecorder()
	h.ExpectedCardsByArchetype(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeOppEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["archetype"] != "Mono Red" || resp["total"].(float64) != 1 {
		t.Errorf("response: %v", resp)
	}
}

func TestOpponentsExpectedCards_Unauthorized(t *testing.T) {
	h := handlers.NewOpponentsHandler(&stubOpponentsReader{}, &oppAccountLookup{accountID: 7, found: true})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/archetypes/Mono+Red/expected-cards", nil)
	req = chiOppContext(req, "name", "Mono Red")
	rr := httptest.NewRecorder()
	h.ExpectedCardsByArchetype(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: %d", rr.Code)
	}
}
