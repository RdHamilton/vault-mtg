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

// stubAccountLookup is already declared in history_test.go; reuse it via
// matchesAccountLookup to avoid the duplicate declaration error.
type matchesAccountLookup struct {
	accountID int64
	found     bool
	err       error
}

func (s *matchesAccountLookup) GetAccountIDByUserID(_ context.Context, _ int64) (int64, bool, error) {
	return s.accountID, s.found, s.err
}

type stubMatchesReader struct {
	listRows   []repository.MatchRow
	listFilter repository.MatchFilter
	listErr    error

	// Captured cursor args from the most recent ListByAccountIDCursorFiltered call.
	capturedCursorTS *time.Time
	capturedCursorID string

	getRow *repository.MatchRow
	getErr error

	formats   []string
	formatErr error

	games    []repository.GameRow
	gamesErr error

	statsAgg repository.StatsAggregate
	statsErr error
	statsCap repository.MatchFilter

	formatDist    []repository.FormatStatsRow
	formatDistErr error

	perfHour    []repository.HourBucket
	perfHourErr error

	matchup    []repository.MatchupRow
	matchupErr error

	archetypes    []string
	archetypesErr error

	trends    []repository.TrendBucket
	trendsErr error

	rankSnap    *repository.RankSnapshot
	rankSnapErr error

	rankTimeline    []repository.RankTimelineRow
	rankTimelineErr error

	exportRows []repository.ExportRow
	exportErr  error
}

func (s *stubMatchesReader) ListByAccountIDCursorFiltered(_ context.Context, _ int64, f repository.MatchFilter, cursorTS *time.Time, cursorID string, _ int) ([]repository.MatchRow, error) {
	s.listFilter = f
	s.capturedCursorTS = cursorTS
	s.capturedCursorID = cursorID
	return s.listRows, s.listErr
}

func (s *stubMatchesReader) GetByID(_ context.Context, _ int64, _ string) (*repository.MatchRow, error) {
	return s.getRow, s.getErr
}

func (s *stubMatchesReader) DistinctFormats(_ context.Context, _ int64) ([]string, error) {
	return s.formats, s.formatErr
}

func (s *stubMatchesReader) GamesByMatchID(_ context.Context, _ int64, _ string) ([]repository.GameRow, error) {
	return s.games, s.gamesErr
}

func (s *stubMatchesReader) AggregateStats(_ context.Context, _ int64, f repository.MatchFilter) (repository.StatsAggregate, error) {
	s.statsCap = f
	return s.statsAgg, s.statsErr
}

func (s *stubMatchesReader) FormatDistribution(_ context.Context, _ int64, _ repository.MatchFilter) ([]repository.FormatStatsRow, error) {
	return s.formatDist, s.formatDistErr
}

func (s *stubMatchesReader) PerformanceByHour(_ context.Context, _ int64, _ repository.MatchFilter) ([]repository.HourBucket, error) {
	return s.perfHour, s.perfHourErr
}

func (s *stubMatchesReader) MatchupMatrix(_ context.Context, _ int64, _ repository.MatchFilter) ([]repository.MatchupRow, error) {
	return s.matchup, s.matchupErr
}

func (s *stubMatchesReader) DistinctArchetypes(_ context.Context, _ int64) ([]string, error) {
	return s.archetypes, s.archetypesErr
}

func (s *stubMatchesReader) Trends(_ context.Context, _ int64, _ string, _ repository.MatchFilter) ([]repository.TrendBucket, error) {
	return s.trends, s.trendsErr
}

func (s *stubMatchesReader) LatestRankInFormat(_ context.Context, _ int64, _ string) (*repository.RankSnapshot, error) {
	return s.rankSnap, s.rankSnapErr
}

func (s *stubMatchesReader) RankTimelineForFormat(_ context.Context, _ int64, _ string, _, _ time.Time) ([]repository.RankTimelineRow, error) {
	return s.rankTimeline, s.rankTimelineErr
}

func (s *stubMatchesReader) ExportAll(_ context.Context, _ int64) ([]repository.ExportRow, error) {
	return s.exportRows, s.exportErr
}

// ─── helpers ────────────────────────────────────────────────────────────────

// requestWithUserID builds an authenticated request — UserIDFromContext picks
// up the user id the same way DaemonAPIKeyAuth would have set it.
func requestWithUserID(t *testing.T, method, target string, body []byte, userID int64) *http.Request {
	t.Helper()
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, target, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	return r.WithContext(bffmiddleware.WithUserID(r.Context(), userID))
}

// ─── List ───────────────────────────────────────────────────────────────────

func TestMatchesList_HappyPath(t *testing.T) {
	timestamp := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	dur := 480
	deck := "deck-abc"
	reader := &stubMatchesReader{
		listRows: []repository.MatchRow{
			{ID: "m1", Format: "standard_bo1", Result: "win", Timestamp: timestamp, DurationSeconds: &dur, DeckID: &deck, PlayerWins: 2, OpponentWins: 1},
			{ID: "m2", Format: "draft_bo1", Result: "loss", Timestamp: timestamp.Add(-time.Hour), PlayerWins: 0, OpponentWins: 2},
		},
	}
	accts := &matchesAccountLookup{accountID: 7, found: true}
	h := handlers.NewMatchesHandler(reader, accts)

	body, _ := json.Marshal(map[string]any{"format": "standard_bo1", "limit": 50})
	req := requestWithUserID(t, http.MethodPost, "/api/v1/matches", body, 168)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200, body=%s", rr.Code, rr.Body.String())
	}
	// Responses are wrapped in {"data": ...} so the SPA's apiClient
	// (which reads response.json().data) gets the payload after unwrap.
	// Field keys are PascalCase to match the SPA's existing models.Match
	// class (Wails-era types not yet regenerated).
	var env struct {
		Data struct {
			Matches []map[string]any `json:"Matches"`
			HasMore bool             `json:"HasMore"`
			Limit   int              `json:"Limit"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data.Matches) != 2 {
		t.Fatalf("matches count: want 2, got %d, body=%s", len(env.Data.Matches), rr.Body.String())
	}
	if env.Data.HasMore {
		t.Errorf("HasMore: want false for 2 rows with limit=50")
	}
	first := env.Data.Matches[0]
	if _, ok := first["ID"]; !ok {
		t.Errorf("missing 'ID' key in PascalCase response: %v", first)
	}
	if _, ok := first["DurationSeconds"]; !ok {
		t.Errorf("missing 'DurationSeconds' key in PascalCase response: %v", first)
	}
	if first["PlayerWins"].(float64) != 2 {
		t.Errorf("PlayerWins: got %v", first["PlayerWins"])
	}
	// filter was forwarded
	if reader.listFilter.Format != "standard_bo1" {
		t.Errorf("filter.Format: got %q", reader.listFilter.Format)
	}
}

// TestMatchesList_HasMore verifies that when the repo returns limit+1 rows the
// response sets HasMore=true and NextCursorTS/NextCursorID, and trims the list
// to exactly limit items.
func TestMatchesList_HasMore(t *testing.T) {
	ts1 := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	ts2 := ts1.Add(-time.Hour)
	// Stub returns limit+1 rows (limit=2 → 3 rows) to trigger has_more=true.
	reader := &stubMatchesReader{
		listRows: []repository.MatchRow{
			{ID: "m1", Format: "standard_bo1", Result: "win", Timestamp: ts1, PlayerWins: 1, OpponentWins: 0},
			{ID: "m2", Format: "standard_bo1", Result: "loss", Timestamp: ts2, PlayerWins: 0, OpponentWins: 1},
			{ID: "m3", Format: "standard_bo1", Result: "win", Timestamp: ts2.Add(-time.Minute), PlayerWins: 1, OpponentWins: 0}, // probe row
		},
	}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	body, _ := json.Marshal(map[string]any{"limit": 2})
	req := requestWithUserID(t, http.MethodPost, "/api/v1/matches", body, 168)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var env struct {
		Data struct {
			Matches      []map[string]any `json:"Matches"`
			HasMore      bool             `json:"HasMore"`
			NextCursorTS string           `json:"NextCursorTS"`
			NextCursorID string           `json:"NextCursorID"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !env.Data.HasMore {
		t.Error("HasMore: want true")
	}
	if len(env.Data.Matches) != 2 {
		t.Errorf("Matches len: want 2 (probe trimmed), got %d", len(env.Data.Matches))
	}
	if env.Data.NextCursorTS == "" || env.Data.NextCursorID == "" {
		t.Errorf("NextCursor tokens should be non-empty when HasMore=true: ts=%q id=%q",
			env.Data.NextCursorTS, env.Data.NextCursorID)
	}
	// The cursor should point at the last returned row (m2).
	if env.Data.NextCursorID != "m2" {
		t.Errorf("NextCursorID: want m2, got %q", env.Data.NextCursorID)
	}
}

// TestMatchesList_Cursor verifies that cursor params are forwarded to the repo.
func TestMatchesList_Cursor(t *testing.T) {
	cursorTime := time.Date(2026, 5, 10, 8, 0, 0, 0, time.UTC)
	reader := &stubMatchesReader{
		listRows: []repository.MatchRow{
			{ID: "m5", Format: "standard_bo1", Result: "win", Timestamp: cursorTime.Add(-time.Hour), PlayerWins: 1, OpponentWins: 0},
		},
	}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	body, _ := json.Marshal(map[string]any{
		"cursorTS": cursorTime.UTC().Format(time.RFC3339Nano),
		"cursorID": "m4",
		"limit":    10,
	})
	req := requestWithUserID(t, http.MethodPost, "/api/v1/matches", body, 168)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	// Verify the cursor was forwarded to the repo.
	if reader.capturedCursorID != "m4" {
		t.Errorf("cursorID not forwarded: got %q", reader.capturedCursorID)
	}
	if reader.capturedCursorTS == nil || !reader.capturedCursorTS.Equal(cursorTime) {
		t.Errorf("cursorTS not forwarded: got %v", reader.capturedCursorTS)
	}
}

// TestMatchesList_CursorMissingID verifies that supplying only cursorTS (without
// cursorID) is rejected with a 400.
func TestMatchesList_CursorMissingID(t *testing.T) {
	h := handlers.NewMatchesHandler(&stubMatchesReader{}, &matchesAccountLookup{accountID: 7, found: true})

	ts := time.Now().UTC().Format(time.RFC3339Nano)
	body, _ := json.Marshal(map[string]any{"cursorTS": ts}) // cursorID absent
	req := requestWithUserID(t, http.MethodPost, "/api/v1/matches", body, 168)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestMatchesList_Unauthorized(t *testing.T) {
	reader := &stubMatchesReader{}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/matches", bytes.NewReader([]byte(`{}`)))
	rr := httptest.NewRecorder()
	h.List(rr, req) // no user_id on context

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rr.Code)
	}
}

func TestMatchesList_NoAccountReturnsEmptyPage(t *testing.T) {
	reader := &stubMatchesReader{}
	accts := &matchesAccountLookup{found: false}
	h := handlers.NewMatchesHandler(reader, accts)

	req := requestWithUserID(t, http.MethodPost, "/api/v1/matches", []byte(`{}`), 168)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	var env struct {
		Data struct {
			Matches []any `json:"Matches"`
			HasMore bool  `json:"HasMore"`
		} `json:"data"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&env)
	if env.Data.HasMore || len(env.Data.Matches) != 0 {
		t.Errorf("expected empty page, got hasMore=%v matches=%d", env.Data.HasMore, len(env.Data.Matches))
	}
}

func TestMatchesList_RejectsInvalidResult(t *testing.T) {
	reader := &stubMatchesReader{}
	accts := &matchesAccountLookup{accountID: 7, found: true}
	h := handlers.NewMatchesHandler(reader, accts)

	body, _ := json.Marshal(map[string]any{"result": "bogus"})
	req := requestWithUserID(t, http.MethodPost, "/api/v1/matches", body, 168)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

// ─── Get ────────────────────────────────────────────────────────────────────

func TestMatchesGet_HappyPath(t *testing.T) {
	row := repository.MatchRow{ID: "m1", Format: "standard_bo1", Result: "win", Timestamp: time.Now().UTC(), PlayerWins: 2, OpponentWins: 0}
	reader := &stubMatchesReader{getRow: &row}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	req := requestWithUserID(t, http.MethodGet, "/api/v1/matches/m1", nil, 168)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("matchId", "m1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var env struct {
		Data map[string]any `json:"data"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&env)
	if env.Data["ID"] != "m1" {
		t.Errorf("ID: %v", env.Data["ID"])
	}
}

func TestMatchesGet_NotFound(t *testing.T) {
	reader := &stubMatchesReader{getRow: nil}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	req := requestWithUserID(t, http.MethodGet, "/api/v1/matches/missing", nil, 168)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("matchId", "missing")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rr.Code)
	}
}

// ─── Formats ────────────────────────────────────────────────────────────────

func TestMatchesFormats_HappyPath(t *testing.T) {
	reader := &stubMatchesReader{formats: []string{"standard_bo1", "draft_bo1"}}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	req := requestWithUserID(t, http.MethodGet, "/api/v1/matches/formats", nil, 168)
	rr := httptest.NewRecorder()
	h.Formats(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var env struct {
		Data []string `json:"data"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&env)
	if len(env.Data) != 2 {
		t.Errorf("expected 2 formats, got %v", env.Data)
	}
}

// ─── Phase 2 PR #1 expansion: smoke tests for the 13 new endpoints ───────────
//
// One happy-path test per endpoint plus a single shared no-account check.
// The intent is to lock the wire shape and route plumbing — the handler
// helpers (resolveAccount, requireFilter, decodeJSONBody) are exercised
// transitively by these tests.

func decodeMatchesEnvelope(t *testing.T, body []byte, into any) {
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

func TestMatchesGames_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	dur := 240
	reader := &stubMatchesReader{games: []repository.GameRow{
		{ID: 11, MatchID: "m1", GameNumber: 1, Result: "win", DurationSeconds: &dur, CreatedAt: now},
		{ID: 12, MatchID: "m1", GameNumber: 2, Result: "loss", CreatedAt: now},
	}}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	req := requestWithUserID(t, http.MethodGet, "/api/v1/matches/m1/games", nil, 168)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("matchId", "m1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.Games(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var data []map[string]any
	decodeMatchesEnvelope(t, rr.Body.Bytes(), &data)
	if len(data) != 2 {
		t.Fatalf("len: %d", len(data))
	}
	if data[0]["GameNumber"].(float64) != 1 {
		t.Errorf("GameNumber: %v", data[0]["GameNumber"])
	}
}

func TestMatchesStats_HappyPath(t *testing.T) {
	reader := &stubMatchesReader{statsAgg: repository.StatsAggregate{
		TotalMatches: 10, MatchesWon: 6, MatchesLost: 4,
		TotalGames: 25, GamesWon: 14, GamesLost: 11,
	}}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	body, _ := json.Marshal(map[string]any{"format": "standard_bo1"})
	req := requestWithUserID(t, http.MethodPost, "/api/v1/matches/stats", body, 168)
	rr := httptest.NewRecorder()
	h.Stats(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var stats map[string]any
	decodeMatchesEnvelope(t, rr.Body.Bytes(), &stats)
	if stats["TotalMatches"].(float64) != 10 || stats["MatchesWon"].(float64) != 6 {
		t.Errorf("stats: %v", stats)
	}
	if stats["WinRate"].(float64) != 0.6 {
		t.Errorf("WinRate: %v", stats["WinRate"])
	}
	if reader.statsCap.Format != "standard_bo1" {
		t.Errorf("filter not forwarded: %v", reader.statsCap)
	}
}

func TestMatchesStats_NoAccountReturnsZeroes(t *testing.T) {
	reader := &stubMatchesReader{}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{found: false})

	req := requestWithUserID(t, http.MethodPost, "/api/v1/matches/stats", []byte(`{}`), 168)
	rr := httptest.NewRecorder()
	h.Stats(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var stats map[string]any
	decodeMatchesEnvelope(t, rr.Body.Bytes(), &stats)
	if stats["TotalMatches"].(float64) != 0 {
		t.Errorf("expected zero stats, got %v", stats)
	}
}

func TestMatchesTrends_HappyPath(t *testing.T) {
	bucket := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	reader := &stubMatchesReader{trends: []repository.TrendBucket{
		{BucketStart: bucket, Stats: repository.StatsAggregate{TotalMatches: 4, MatchesWon: 3, GamesWon: 8, TotalGames: 12}},
	}}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	body, _ := json.Marshal(map[string]any{
		"startDate":  "2026-01-01",
		"endDate":    "2026-01-31",
		"periodType": "week",
	})
	req := requestWithUserID(t, http.MethodPost, "/api/v1/matches/trends", body, 168)
	rr := httptest.NewRecorder()
	h.Trends(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeMatchesEnvelope(t, rr.Body.Bytes(), &resp)
	trends, ok := resp["Trends"].([]any)
	if !ok || len(trends) != 1 {
		t.Fatalf("trends: %v", resp["Trends"])
	}
}

func TestMatchesTrends_RejectsBadPeriod(t *testing.T) {
	h := handlers.NewMatchesHandler(&stubMatchesReader{}, &matchesAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"startDate": "2026-01-01", "endDate": "2026-01-31", "periodType": "decade"})
	req := requestWithUserID(t, http.MethodPost, "/api/v1/matches/trends", body, 168)
	rr := httptest.NewRecorder()
	h.Trends(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestMatchesArchetypes_HappyPath(t *testing.T) {
	reader := &stubMatchesReader{archetypes: []string{"Mono Red", "Esper Control"}}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	req := requestWithUserID(t, http.MethodGet, "/api/v1/matches/archetypes", nil, 168)
	rr := httptest.NewRecorder()
	h.Archetypes(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var arr []string
	decodeMatchesEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 2 {
		t.Errorf("got %v", arr)
	}
}

func TestMatchesFormatDistribution_HappyPath(t *testing.T) {
	reader := &stubMatchesReader{formatDist: []repository.FormatStatsRow{
		{Format: "standard_bo1", Stats: repository.StatsAggregate{TotalMatches: 5, MatchesWon: 3}},
		{Format: "draft_bo1", Stats: repository.StatsAggregate{TotalMatches: 2, MatchesWon: 1}},
	}}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	req := requestWithUserID(t, http.MethodPost, "/api/v1/matches/format-distribution", []byte(`{}`), 168)
	rr := httptest.NewRecorder()
	h.FormatDistribution(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var m map[string]map[string]any
	decodeMatchesEnvelope(t, rr.Body.Bytes(), &m)
	if len(m) != 2 {
		t.Errorf("len: %d", len(m))
	}
}

func TestMatchesPerformanceByHour_HappyPath(t *testing.T) {
	avg := 300.0
	fast := 120
	slow := 600
	reader := &stubMatchesReader{perfHour: []repository.HourBucket{
		{Hour: 18, MatchCount: 5, AvgMatchDurationSecs: &avg, FastestMatchSecs: &fast, SlowestMatchSecs: &slow},
	}}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	req := requestWithUserID(t, http.MethodPost, "/api/v1/matches/performance-by-hour", []byte(`{}`), 168)
	rr := httptest.NewRecorder()
	h.PerformanceByHour(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var m map[string]any
	decodeMatchesEnvelope(t, rr.Body.Bytes(), &m)
	if got, _ := m["AvgMatchDuration"].(float64); got != 300 {
		t.Errorf("AvgMatchDuration: %v", m["AvgMatchDuration"])
	}
}

func TestMatchesMatchupMatrix_HappyPath(t *testing.T) {
	reader := &stubMatchesReader{matchup: []repository.MatchupRow{
		{OpponentLabel: "Mono Red", Stats: repository.StatsAggregate{TotalMatches: 3, MatchesWon: 1}},
	}}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	req := requestWithUserID(t, http.MethodPost, "/api/v1/matches/matchup-matrix", []byte(`{}`), 168)
	rr := httptest.NewRecorder()
	h.MatchupMatrix(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var m map[string]map[string]any
	decodeMatchesEnvelope(t, rr.Body.Bytes(), &m)
	if _, ok := m["Mono Red"]; !ok {
		t.Errorf("missing Mono Red row: %v", m)
	}
}

func TestMatchesRankProgression_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubMatchesReader{rankSnap: &repository.RankSnapshot{Format: "standard_bo1", RankAfter: "Diamond 3", OccurredAt: now}}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	req := requestWithUserID(t, http.MethodGet, "/api/v1/matches/rank-progression/standard_bo1", nil, 168)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("format", "standard_bo1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.RankProgression(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var m map[string]any
	decodeMatchesEnvelope(t, rr.Body.Bytes(), &m)
	if m["CurrentRank"] != "Diamond 3" {
		t.Errorf("CurrentRank: %v", m["CurrentRank"])
	}
}

func TestMatchesRankProgressionTimeline_HappyPath(t *testing.T) {
	rb := "Gold 1"
	ra := "Platinum 4"
	reader := &stubMatchesReader{rankTimeline: []repository.RankTimelineRow{
		{MatchID: "m1", OccurredAt: time.Now().UTC(), RankBefore: &rb, RankAfter: &ra, Result: "win"},
	}}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	req := requestWithUserID(t, http.MethodGet, "/api/v1/matches/rank-progression-timeline?format=standard_bo1&start_date=2026-01-01&end_date=2026-02-01", nil, 168)
	rr := httptest.NewRecorder()
	h.RankProgressionTimeline(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var m map[string]any
	decodeMatchesEnvelope(t, rr.Body.Bytes(), &m)
	entries, _ := m["entries"].([]any)
	if len(entries) != 1 {
		t.Errorf("entries: %v", m["entries"])
	}
}

func TestMatchesExport_JSONHappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubMatchesReader{exportRows: []repository.ExportRow{
		{ID: "m1", Format: "standard_bo1", Result: "win", Timestamp: now, PlayerWins: 2, OpponentWins: 0, EventName: "Best of 1"},
	}}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	req := requestWithUserID(t, http.MethodGet, "/api/v1/matches/export?format=json", nil, 168)
	rr := httptest.NewRecorder()
	h.Export(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeMatchesEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["id"] != "m1" {
		t.Errorf("export: %v", arr)
	}
}

func TestMatchesExport_CSVHappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubMatchesReader{exportRows: []repository.ExportRow{
		{ID: "m1", Format: "standard_bo1", Result: "win", Timestamp: now, PlayerWins: 2, OpponentWins: 0, EventName: "Best of 1"},
	}}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	req := requestWithUserID(t, http.MethodGet, "/api/v1/matches/export?format=csv", nil, 168)
	rr := httptest.NewRecorder()
	h.Export(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/csv" {
		t.Errorf("Content-Type: %s", ct)
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("m1")) {
		t.Errorf("missing m1 in CSV body: %s", rr.Body.String())
	}
}

func TestMatchesCompare_HappyPath(t *testing.T) {
	reader := &stubMatchesReader{statsAgg: repository.StatsAggregate{TotalMatches: 5, MatchesWon: 3, TotalGames: 12, GamesWon: 7}}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	body, _ := json.Marshal(map[string]any{"groups": []map[string]any{
		{"label": "Last 7d", "filter": map[string]any{"format": "standard_bo1"}},
		{"label": "All time", "filter": map[string]any{}},
	}})
	req := requestWithUserID(t, http.MethodPost, "/api/v1/matches/compare", body, 168)
	rr := httptest.NewRecorder()
	h.Compare(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var m map[string]any
	decodeMatchesEnvelope(t, rr.Body.Bytes(), &m)
	groups, _ := m["Groups"].([]any)
	if len(groups) != 2 {
		t.Errorf("groups: %v", m["Groups"])
	}
}

func TestMatchesCompareFormats_HappyPath(t *testing.T) {
	reader := &stubMatchesReader{statsAgg: repository.StatsAggregate{TotalMatches: 4, MatchesWon: 2}}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	body, _ := json.Marshal(map[string]any{"formats": []string{"standard_bo1", "draft_bo1"}, "baseFilter": map[string]any{}})
	req := requestWithUserID(t, http.MethodPost, "/api/v1/matches/compare/formats", body, 168)
	rr := httptest.NewRecorder()
	h.CompareFormats(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var m map[string]any
	decodeMatchesEnvelope(t, rr.Body.Bytes(), &m)
	if groups, _ := m["Groups"].([]any); len(groups) != 2 {
		t.Errorf("groups len: %d", len(groups))
	}
}

func TestMatchesCompareDecks_HappyPath(t *testing.T) {
	reader := &stubMatchesReader{statsAgg: repository.StatsAggregate{TotalMatches: 1, MatchesWon: 1}}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	body, _ := json.Marshal(map[string]any{"deckIDs": []string{"d1", "d2"}, "baseFilter": map[string]any{}})
	req := requestWithUserID(t, http.MethodPost, "/api/v1/matches/compare/decks", body, 168)
	rr := httptest.NewRecorder()
	h.CompareDecks(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestMatchesCompareTimePeriods_HappyPath(t *testing.T) {
	reader := &stubMatchesReader{statsAgg: repository.StatsAggregate{TotalMatches: 3, MatchesWon: 2}}
	h := handlers.NewMatchesHandler(reader, &matchesAccountLookup{accountID: 7, found: true})

	body, _ := json.Marshal(map[string]any{"periods": []map[string]any{
		{"label": "Jan", "startDate": "2026-01-01", "endDate": "2026-01-31"},
		{"label": "Feb", "startDate": "2026-02-01", "endDate": "2026-02-28"},
	}, "baseFilter": map[string]any{}})
	req := requestWithUserID(t, http.MethodPost, "/api/v1/matches/compare/time-periods", body, 168)
	rr := httptest.NewRecorder()
	h.CompareTimePeriods(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
}
