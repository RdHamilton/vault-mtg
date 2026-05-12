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
	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// ─── stubs ──────────────────────────────────────────────────────────────────

type draftsAccountLookup struct {
	accountID        int64
	found            bool
	err              error
	calledWithUserID int64 // captured for assertion
}

func (d *draftsAccountLookup) GetAccountIDByUserID(_ context.Context, userID int64) (int64, bool, error) {
	d.calledWithUserID = userID
	return d.accountID, d.found, d.err
}

// stubDraftsReader records every accountID + key positional argument the
// handler forwards so tests can assert account scoping was preserved.
type stubDraftsReader struct {
	sessions    []repository.DraftSessionDetailRow
	sessionsErr error

	session    *repository.DraftSessionDetailRow
	sessionErr error

	sets    []string
	setsErr error

	picks    []repository.DraftPickRow
	picksErr error

	stats    repository.DraftStatsAggregate
	statsErr error

	comparisons    []repository.CommunityComparisonRow
	comparisonsErr error

	comparison    *repository.CommunityComparisonRow
	comparisonErr error

	trends    []repository.TemporalTrendRow
	trendsErr error

	learning    []repository.TemporalTrendRow
	learningErr error

	feedback    repository.RecommendationFeedbackStatsRow
	feedbackErr error

	// captured args + per-method invocation flags. *Called booleans are
	// the source of truth for "did the handler actually reach the repo?"
	// — the captured-ID fields are only meaningful when the corresponding
	// flag is true (zero is a valid accountID/userID and can't double as
	// a sentinel).
	listCalled        bool
	listAccountID     int64
	listFilter        repository.DraftFilter
	sessionCalled     bool
	sessionAccountID  int64
	sessionLookupID   string
	setsCalled        bool
	setsAccountID     int64
	picksCalled       bool
	picksAccountID    int64
	picksSessionID    string
	statsCalled       bool
	statsAccountID    int64
	statsFilter       repository.DraftFilter
	comparisonCalled  bool
	comparisonSetCode string
	comparisonFormat  string
	trendsCalled      bool
	trendsPeriod      string
	trendsSetCode     string
	trendsNumPeriods  int
	learningCalled    bool
	learningSetCode   string
	feedbackCalled    bool
	feedbackAccountID int64
}

func (s *stubDraftsReader) ListSessions(_ context.Context, accountID int64, f repository.DraftFilter) ([]repository.DraftSessionDetailRow, error) {
	s.listCalled = true
	s.listAccountID = accountID
	s.listFilter = f
	return s.sessions, s.sessionsErr
}

func (s *stubDraftsReader) GetSession(_ context.Context, accountID int64, sessionID string) (*repository.DraftSessionDetailRow, error) {
	s.sessionCalled = true
	s.sessionAccountID = accountID
	s.sessionLookupID = sessionID
	return s.session, s.sessionErr
}

func (s *stubDraftsReader) DistinctSets(_ context.Context, accountID int64) ([]string, error) {
	s.setsCalled = true
	s.setsAccountID = accountID
	return s.sets, s.setsErr
}

func (s *stubDraftsReader) PicksForSession(_ context.Context, accountID int64, sessionID string) ([]repository.DraftPickRow, error) {
	s.picksCalled = true
	s.picksAccountID = accountID
	s.picksSessionID = sessionID
	return s.picks, s.picksErr
}

func (s *stubDraftsReader) AggregateStats(_ context.Context, accountID int64, f repository.DraftFilter) (repository.DraftStatsAggregate, error) {
	s.statsCalled = true
	s.statsAccountID = accountID
	s.statsFilter = f
	return s.stats, s.statsErr
}

func (s *stubDraftsReader) CommunityComparisons(_ context.Context) ([]repository.CommunityComparisonRow, error) {
	return s.comparisons, s.comparisonsErr
}

func (s *stubDraftsReader) CommunityComparisonForSet(_ context.Context, setCode, format string) (*repository.CommunityComparisonRow, error) {
	s.comparisonCalled = true
	s.comparisonSetCode = setCode
	s.comparisonFormat = format
	return s.comparison, s.comparisonErr
}

func (s *stubDraftsReader) TemporalTrends(_ context.Context, periodType, setCode string, numPeriods int) ([]repository.TemporalTrendRow, error) {
	s.trendsCalled = true
	s.trendsPeriod = periodType
	s.trendsSetCode = setCode
	s.trendsNumPeriods = numPeriods
	return s.trends, s.trendsErr
}

func (s *stubDraftsReader) LearningCurve(_ context.Context, setCode string) ([]repository.TemporalTrendRow, error) {
	s.learningCalled = true
	s.learningSetCode = setCode
	return s.learning, s.learningErr
}

func (s *stubDraftsReader) RecommendationFeedbackStats(_ context.Context, accountID int64) (repository.RecommendationFeedbackStatsRow, error) {
	s.feedbackCalled = true
	s.feedbackAccountID = accountID
	return s.feedback, s.feedbackErr
}

// ─── helpers ────────────────────────────────────────────────────────────────

func authedDraftsRequest(t *testing.T, method, target string, body []byte, userID int64) *http.Request {
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

func decodeDraftsEnvelope(t *testing.T, body []byte, into any) {
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

func chiDraftsContext(req *http.Request, kvs ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i+1 < len(kvs); i += 2 {
		rctx.URLParams.Add(kvs[i], kvs[i+1])
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// ─── List / Get / Picks / Stats / Formats / Recent ─────────────────────────

func TestDraftsList_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubDraftsReader{sessions: []repository.DraftSessionDetailRow{
		{
			ID: "s1", EventName: "PremierDraft", SetCode: "DSK", DraftType: "PremierDraft",
			StartTime: now, Status: "completed", TotalPicks: 45, CreatedAt: now, UpdatedAt: now,
		},
	}}
	accts := &draftsAccountLookup{accountID: 7, found: true}
	h := handlers.NewDraftsHandler(reader, accts)
	req := authedDraftsRequest(t, http.MethodPost, "/api/v1/drafts", []byte(`{}`), 168)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeDraftsEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["ID"] != "s1" {
		t.Errorf("list: %v", arr)
	}
	// Account scoping: handler should resolve userID 168 via the lookup
	// then pass the resolved accountID 7 to the repo.
	if accts.calledWithUserID != 168 {
		t.Errorf("GetAccountIDByUserID called with %d, want 168", accts.calledWithUserID)
	}
	if reader.listAccountID != 7 {
		t.Errorf("ListSessions accountID = %d, want 7", reader.listAccountID)
	}
}

func TestDraftsGet_NotFound(t *testing.T) {
	h := handlers.NewDraftsHandler(&stubDraftsReader{session: nil}, &draftsAccountLookup{accountID: 7, found: true})
	req := authedDraftsRequest(t, http.MethodGet, "/api/v1/drafts/missing", nil, 168)
	req = chiDraftsContext(req, "sessionId", "missing")
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestDraftsPicks_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubDraftsReader{picks: []repository.DraftPickRow{
		{ID: 1, SessionID: "s1", PackNumber: 1, PickNumber: 1, CardID: "100", Timestamp: now},
	}}
	h := handlers.NewDraftsHandler(reader, &draftsAccountLookup{accountID: 7, found: true})
	req := authedDraftsRequest(t, http.MethodGet, "/api/v1/drafts/s1/picks", nil, 168)
	req = chiDraftsContext(req, "sessionId", "s1")
	rr := httptest.NewRecorder()
	h.Picks(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var arr []map[string]any
	decodeDraftsEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["CardID"] != "100" {
		t.Errorf("picks: %v", arr)
	}
}

func TestDraftsStats_HappyPath(t *testing.T) {
	avgScore := 75.5
	reader := &stubDraftsReader{stats: repository.DraftStatsAggregate{
		TotalDrafts: 10, CompletedDrafts: 8, AvgOverallScore: &avgScore,
		GradeDistribution: map[string]int{"A": 4, "B": 4},
	}}
	h := handlers.NewDraftsHandler(reader, &draftsAccountLookup{accountID: 7, found: true})
	req := authedDraftsRequest(t, http.MethodPost, "/api/v1/drafts/stats", []byte(`{}`), 168)
	rr := httptest.NewRecorder()
	h.Stats(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeDraftsEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["totalDrafts"].(float64) != 10 || resp["avgOverallScore"].(float64) != 75.5 {
		t.Errorf("stats: %v", resp)
	}
}

func TestDraftsFormats_HappyPath(t *testing.T) {
	reader := &stubDraftsReader{sets: []string{"DSK", "ZNR"}}
	h := handlers.NewDraftsHandler(reader, &draftsAccountLookup{accountID: 7, found: true})
	req := authedDraftsRequest(t, http.MethodGet, "/api/v1/drafts/formats", nil, 168)
	rr := httptest.NewRecorder()
	h.Formats(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
}

// ─── 17Lands export ─────────────────────────────────────────────────────────

func TestDraftsExport17Lands_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubDraftsReader{
		session: &repository.DraftSessionDetailRow{
			ID: "s1", EventName: "PremierDraft", SetCode: "DSK",
			DraftType: "PremierDraft", StartTime: now, Status: "completed",
			CreatedAt: now, UpdatedAt: now,
		},
		picks: []repository.DraftPickRow{
			{ID: 1, SessionID: "s1", PackNumber: 1, PickNumber: 1, CardID: "100", Timestamp: now},
		},
	}
	h := handlers.NewDraftsHandler(reader, &draftsAccountLookup{accountID: 7, found: true})
	req := authedDraftsRequest(t, http.MethodGet, "/api/v1/drafts/s1/export/17lands", nil, 168)
	req = chiDraftsContext(req, "sessionId", "s1")
	rr := httptest.NewRecorder()
	h.Export17Lands(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeDraftsEnvelope(t, rr.Body.Bytes(), &resp)
	export, _ := resp["export"].(map[string]any)
	if export["set_code"] != "DSK" || export["draft_id"] != "s1" {
		t.Errorf("export: %v", export)
	}
	picks, _ := export["picks"].([]any)
	if len(picks) != 1 {
		t.Errorf("picks count: %v", picks)
	}
}

// ─── Community + Trends + Learning ─────────────────────────────────────────

func TestDraftsCommunityComparison_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubDraftsReader{
		comparison: &repository.CommunityComparisonRow{
			SetCode: "DSK", DraftFormat: "PremierDraft",
			UserWinRate: 0.55, CommunityAvgWinRate: 0.50,
			SampleSize: 1000, CalculatedAt: now,
		},
	}
	h := handlers.NewDraftsHandler(reader, &draftsAccountLookup{accountID: 7, found: true})
	req := authedDraftsRequest(t, http.MethodGet, "/api/v1/drafts/community-comparison/dsk", nil, 168)
	req = chiDraftsContext(req, "setCode", "dsk")
	rr := httptest.NewRecorder()
	h.CommunityComparisonByGet(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var resp map[string]any
	decodeDraftsEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["user_win_rate"].(float64) != 0.55 {
		t.Errorf("user_win_rate: %v", resp)
	}
}

func TestDraftsTrends_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubDraftsReader{trends: []repository.TemporalTrendRow{
		{
			PeriodType: "week", PeriodStart: now.AddDate(0, 0, -7), PeriodEnd: now,
			SetCode: "DSK", DraftsCount: 5, MatchesPlayed: 25, MatchesWon: 14, CalculatedAt: now,
		},
	}}
	h := handlers.NewDraftsHandler(reader, &draftsAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"period_type": "week", "num_periods": 4, "set_code": "DSK"})
	req := authedDraftsRequest(t, http.MethodPost, "/api/v1/drafts/trends", body, 168)
	rr := httptest.NewRecorder()
	h.Trends(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var resp map[string]any
	decodeDraftsEnvelope(t, rr.Body.Bytes(), &resp)
	trends, _ := resp["trends"].([]any)
	if len(trends) != 1 {
		t.Errorf("trends: %v", trends)
	}
}

func TestDraftsLearningCurve_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubDraftsReader{learning: []repository.TemporalTrendRow{
		{
			PeriodStart: now.AddDate(0, 0, -7), PeriodEnd: now,
			DraftsCount: 5, MatchesPlayed: 25, MatchesWon: 14, CalculatedAt: now,
		},
	}}
	h := handlers.NewDraftsHandler(reader, &draftsAccountLookup{accountID: 7, found: true})
	req := authedDraftsRequest(t, http.MethodGet, "/api/v1/drafts/learning-curve/dsk", nil, 168)
	req = chiDraftsContext(req, "setCode", "dsk")
	rr := httptest.NewRecorder()
	h.LearningCurve(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var resp map[string]any
	decodeDraftsEnvelope(t, rr.Body.Bytes(), &resp)
	periods, _ := resp["periods"].([]any)
	if len(periods) != 1 {
		t.Errorf("periods: %v", periods)
	}
}

// ─── Feedback ──────────────────────────────────────────────────────────────

func TestDraftsFeedbackStats_HappyPath(t *testing.T) {
	wr := 0.6
	reader := &stubDraftsReader{feedback: repository.RecommendationFeedbackStatsRow{
		TotalRecommendations: 100, Accepted: 70, Rejected: 20, WinRateImpact: &wr,
	}}
	h := handlers.NewDraftsHandler(reader, &draftsAccountLookup{accountID: 7, found: true})
	req := authedDraftsRequest(t, http.MethodGet, "/api/v1/feedback/stats", nil, 168)
	rr := httptest.NewRecorder()
	h.FeedbackStats(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var resp map[string]any
	decodeDraftsEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["totalRecommendations"].(float64) != 100 || resp["winRateImpact"].(float64) != 0.6 {
		t.Errorf("feedback: %v", resp)
	}
}

func TestDraftsFeedback_StubsAreNoOp(t *testing.T) {
	h := handlers.NewDraftsHandler(&stubDraftsReader{}, &draftsAccountLookup{accountID: 7, found: true})
	for _, fn := range []http.HandlerFunc{h.FeedbackRecommendation, h.FeedbackAction, h.FeedbackOutcome} {
		req := authedDraftsRequest(t, http.MethodPost, "/api/v1/feedback/x", []byte(`{}`), 168)
		rr := httptest.NewRecorder()
		fn(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", rr.Code)
		}
	}
}

// ─── Auth ──────────────────────────────────────────────────────────────────

func TestDraftsAuth_Unauthorized(t *testing.T) {
	h := handlers.NewDraftsHandler(&stubDraftsReader{}, &draftsAccountLookup{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/drafts", bytes.NewReader([]byte(`{}`)))
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── Negative cases ────────────────────────────────────────────────────────

func TestDraftsList_DatabaseError(t *testing.T) {
	reader := &stubDraftsReader{sessionsErr: errStubDB}
	h := handlers.NewDraftsHandler(reader, &draftsAccountLookup{accountID: 7, found: true})
	req := authedDraftsRequest(t, http.MethodPost, "/api/v1/drafts", []byte(`{}`), 168)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status: %d, want 500", rr.Code)
	}
}

func TestDraftsList_AccountNotFoundReturnsEmpty(t *testing.T) {
	// found=false short-circuits with a 200 + empty list (consistent with
	// every other Phase 2 handler — see matches/collection/quests).
	reader := &stubDraftsReader{}
	h := handlers.NewDraftsHandler(reader, &draftsAccountLookup{found: false})
	req := authedDraftsRequest(t, http.MethodPost, "/api/v1/drafts", []byte(`{}`), 168)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var arr []map[string]any
	decodeDraftsEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 0 {
		t.Errorf("expected empty list, got %v", arr)
	}
	// Repo should NOT be called when the account is missing.
	if reader.listCalled {
		t.Errorf("ListSessions invoked despite missing account (accountID=%d)", reader.listAccountID)
	}
}

func TestDraftsList_BadJSONReturns400(t *testing.T) {
	reader := &stubDraftsReader{}
	h := handlers.NewDraftsHandler(reader, &draftsAccountLookup{accountID: 7, found: true})
	req := authedDraftsRequest(t, http.MethodPost, "/api/v1/drafts", []byte(`{"format":}`), 168)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d, want 400", rr.Code)
	}
	if reader.listCalled {
		t.Errorf("ListSessions invoked despite bad JSON body")
	}
}

func TestDraftsList_BadStartDateReturns400(t *testing.T) {
	reader := &stubDraftsReader{}
	h := handlers.NewDraftsHandler(reader, &draftsAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"start_date": "not-a-date"})
	req := authedDraftsRequest(t, http.MethodPost, "/api/v1/drafts", body, 168)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d, want 400", rr.Code)
	}
	if reader.listCalled {
		t.Errorf("ListSessions invoked despite bad start_date")
	}
}

func TestDraftsStats_PassesStatusFilter(t *testing.T) {
	// Regression: Stats handler used to omit Status from the request body
	// even though AggregateStats now respects it.
	reader := &stubDraftsReader{stats: repository.DraftStatsAggregate{}}
	h := handlers.NewDraftsHandler(reader, &draftsAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{
		"format": "PremierDraft", "set_code": "DSK", "status": "completed",
	})
	req := authedDraftsRequest(t, http.MethodPost, "/api/v1/drafts/stats", body, 168)
	rr := httptest.NewRecorder()
	h.Stats(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	if !reader.statsCalled {
		t.Fatalf("AggregateStats not called")
	}
	if reader.statsFilter.Status != "completed" {
		t.Errorf("Status filter not forwarded: got %q", reader.statsFilter.Status)
	}
	if reader.statsFilter.Format != "PremierDraft" || reader.statsFilter.SetCode != "DSK" {
		t.Errorf("filter shape: %+v", reader.statsFilter)
	}
}

// ─── Trends period normalization (regression) ──────────────────────────────

func TestDraftsTrends_NormalizesWeeklyToWeek(t *testing.T) {
	// SPA's TemporalTrendsRequest defines period_type as "weekly"|"monthly";
	// the repo's TemporalTrends accepts "week"|"month". Verify the handler
	// folds the SPA payload down to the SQL value.
	reader := &stubDraftsReader{trends: []repository.TemporalTrendRow{}}
	h := handlers.NewDraftsHandler(reader, &draftsAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"period_type": "WEEKLY", "num_periods": 4, "set_code": "DSK"})
	req := authedDraftsRequest(t, http.MethodPost, "/api/v1/drafts/trends", body, 168)
	rr := httptest.NewRecorder()
	h.Trends(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	if reader.trendsPeriod != "week" {
		t.Errorf("repo received period=%q, want %q", reader.trendsPeriod, "week")
	}
}

func TestDraftsTrends_NormalizesMonthlyToMonth(t *testing.T) {
	reader := &stubDraftsReader{trends: []repository.TemporalTrendRow{}}
	h := handlers.NewDraftsHandler(reader, &draftsAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"period_type": "monthly", "num_periods": 12})
	req := authedDraftsRequest(t, http.MethodPost, "/api/v1/drafts/trends", body, 168)
	rr := httptest.NewRecorder()
	h.Trends(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	if reader.trendsPeriod != "month" {
		t.Errorf("repo received period=%q, want %q", reader.trendsPeriod, "month")
	}
}

// ─── 17Lands export numeric IDs (regression) ───────────────────────────────

func TestDraftsExport17Lands_NumericPickAndPack(t *testing.T) {
	// SeventeenLandsPickData on the SPA types pick:number and pack:number[].
	// The schema stores card_id + alternatives_json as TEXT — make sure the
	// handler converts.
	now := time.Now().UTC()
	alt := `["12345","12346","12347"]`
	reader := &stubDraftsReader{
		session: &repository.DraftSessionDetailRow{
			ID: "s1", EventName: "PremierDraft", SetCode: "DSK",
			DraftType: "PremierDraft", StartTime: now, Status: "completed",
			CreatedAt: now, UpdatedAt: now,
		},
		picks: []repository.DraftPickRow{
			{
				ID: 1, SessionID: "s1", PackNumber: 1, PickNumber: 1,
				CardID: "12345", Timestamp: now, AlternativesJSON: &alt,
			},
		},
	}
	h := handlers.NewDraftsHandler(reader, &draftsAccountLookup{accountID: 7, found: true})
	req := authedDraftsRequest(t, http.MethodGet, "/api/v1/drafts/s1/export/17lands", nil, 168)
	req = chiDraftsContext(req, "sessionId", "s1")
	rr := httptest.NewRecorder()
	h.Export17Lands(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeDraftsEnvelope(t, rr.Body.Bytes(), &resp)
	export, _ := resp["export"].(map[string]any)
	picks, _ := export["picks"].([]any)
	if len(picks) != 1 {
		t.Fatalf("expected 1 pick, got %v", picks)
	}
	first := picks[0].(map[string]any)
	if first["pick"].(float64) != 12345 {
		t.Errorf("pick = %v, want numeric 12345", first["pick"])
	}
	pack, _ := first["pack"].([]any)
	if len(pack) != 3 || pack[0].(float64) != 12345 || pack[1].(float64) != 12346 {
		t.Errorf("pack = %v, want [12345,12346,12347] as numbers", pack)
	}
}

// errStubDB is a sentinel error used by negative-case tests to drive the
// repo stubs into their failure paths.
var errStubDB = errors.New("stub: database unavailable")
