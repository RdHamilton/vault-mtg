package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	"github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// ─── Stub implementations ────────────────────────────────────────────────────

type stubDeckPerformanceReader struct {
	rows []repository.DeckPerformanceRow
	err  error
}

func (s *stubDeckPerformanceReader) GetDeckPerformance(_ context.Context, _ int64) ([]repository.DeckPerformanceRow, error) {
	return s.rows, s.err
}

type stubWinRateTrendReader struct {
	buckets []repository.WinRateBucket
	err     error
}

func (s *stubWinRateTrendReader) GetWinRateTrend(_ context.Context, _ int64, _ string) ([]repository.WinRateBucket, error) {
	return s.buckets, s.err
}

type stubFormatDistributionReader struct {
	rows []repository.FormatDistributionRow
	err  error
}

func (s *stubFormatDistributionReader) GetFormatDistribution(_ context.Context, _ int64) ([]repository.FormatDistributionRow, error) {
	return s.rows, s.err
}

// ─── Helper ──────────────────────────────────────────────────────────────────

func newStatsHandler(
	accounts *stubAccountLookup,
	dp *stubDeckPerformanceReader,
	wrt *stubWinRateTrendReader,
	fd *stubFormatDistributionReader,
) *handlers.StatsHandler {
	return handlers.NewStatsHandler(accounts, dp, wrt, fd)
}

// authedStatsHandler injects userID into context then calls fn.
func authedStatsHandler(fn http.HandlerFunc, userID int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.WithUserID(r.Context(), userID)
		fn(w, r.WithContext(ctx))
	})
}

func decodeStatsData(t *testing.T, body []byte) []interface{} {
	t.Helper()

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatalf("expected 'data' array, got %T", resp["data"])
	}

	return data
}

// ─── GetDeckPerformance ───────────────────────────────────────────────────────

func TestStatsHandler_GetDeckPerformance_Unauthorized(t *testing.T) {
	h := newStatsHandler(
		&stubAccountLookup{},
		&stubDeckPerformanceReader{},
		&stubWinRateTrendReader{},
		&stubFormatDistributionReader{},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/deck-performance", nil)
	rr := httptest.NewRecorder()
	h.GetDeckPerformance(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rr.Code)
	}
}

func TestStatsHandler_GetDeckPerformance_NoAccount(t *testing.T) {
	h := newStatsHandler(
		&stubAccountLookup{found: false},
		&stubDeckPerformanceReader{},
		&stubWinRateTrendReader{},
		&stubFormatDistributionReader{},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/deck-performance", nil)
	rr := httptest.NewRecorder()
	authedStatsHandler(h.GetDeckPerformance, 1).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}

	data := decodeStatsData(t, rr.Body.Bytes())
	if len(data) != 0 {
		t.Errorf("expected empty data array, got %d items", len(data))
	}
}

func TestStatsHandler_GetDeckPerformance_HappyPath(t *testing.T) {
	rows := []repository.DeckPerformanceRow{
		{DeckID: "deck-1", DeckName: "Stompy", Format: "Standard", Wins: 10, Losses: 3, Draws: 0, TotalGames: 13},
		{DeckID: "deck-2", DeckName: "Control", Format: "Standard", Wins: 5, Losses: 5, Draws: 1, TotalGames: 11},
	}

	h := newStatsHandler(
		&stubAccountLookup{found: true, accountID: 42},
		&stubDeckPerformanceReader{rows: rows},
		&stubWinRateTrendReader{},
		&stubFormatDistributionReader{},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/deck-performance", nil)
	rr := httptest.NewRecorder()
	authedStatsHandler(h.GetDeckPerformance, 1).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}

	data := decodeStatsData(t, rr.Body.Bytes())
	if len(data) != 2 {
		t.Fatalf("want 2 items, got %d", len(data))
	}

	first := data[0].(map[string]interface{})
	if first["deck_id"] != "deck-1" {
		t.Errorf("deck_id: want deck-1, got %v", first["deck_id"])
	}

	if first["deck_name"] != "Stompy" {
		t.Errorf("deck_name: want Stompy, got %v", first["deck_name"])
	}

	if first["wins"].(float64) != 10 {
		t.Errorf("wins: want 10, got %v", first["wins"])
	}

	if first["total_games"].(float64) != 13 {
		t.Errorf("total_games: want 13, got %v", first["total_games"])
	}
}

func TestStatsHandler_GetDeckPerformance_RepoError(t *testing.T) {
	h := newStatsHandler(
		&stubAccountLookup{found: true, accountID: 42},
		&stubDeckPerformanceReader{err: errors.New("db error")},
		&stubWinRateTrendReader{},
		&stubFormatDistributionReader{},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/deck-performance", nil)
	rr := httptest.NewRecorder()
	authedStatsHandler(h.GetDeckPerformance, 1).ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rr.Code)
	}
}

// ─── GetWinRateTrend ──────────────────────────────────────────────────────────

func TestStatsHandler_GetWinRateTrend_Unauthorized(t *testing.T) {
	h := newStatsHandler(
		&stubAccountLookup{},
		&stubDeckPerformanceReader{},
		&stubWinRateTrendReader{},
		&stubFormatDistributionReader{},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/win-rate-trend", nil)
	rr := httptest.NewRecorder()
	h.GetWinRateTrend(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rr.Code)
	}
}

func TestStatsHandler_GetWinRateTrend_InvalidGranularity(t *testing.T) {
	h := newStatsHandler(
		&stubAccountLookup{found: true, accountID: 42},
		&stubDeckPerformanceReader{},
		&stubWinRateTrendReader{},
		&stubFormatDistributionReader{},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/win-rate-trend?granularity=yearly", nil)
	rr := httptest.NewRecorder()
	authedStatsHandler(h.GetWinRateTrend, 1).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestStatsHandler_GetWinRateTrend_DefaultGranularityDaily(t *testing.T) {
	bucket := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	buckets := []repository.WinRateBucket{
		{BucketStart: bucket, Wins: 3, Losses: 1, Draws: 0, TotalGames: 4, WinRate: 0.75},
	}

	h := newStatsHandler(
		&stubAccountLookup{found: true, accountID: 42},
		&stubDeckPerformanceReader{},
		&stubWinRateTrendReader{buckets: buckets},
		&stubFormatDistributionReader{},
	)

	// No granularity param — should default to daily.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/win-rate-trend", nil)
	rr := httptest.NewRecorder()
	authedStatsHandler(h.GetWinRateTrend, 1).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}

	data := decodeStatsData(t, rr.Body.Bytes())
	if len(data) != 1 {
		t.Fatalf("want 1 bucket, got %d", len(data))
	}

	b := data[0].(map[string]interface{})
	if b["win_rate"].(float64) != 0.75 {
		t.Errorf("win_rate: want 0.75, got %v", b["win_rate"])
	}
}

func TestStatsHandler_GetWinRateTrend_WeeklyGranularity(t *testing.T) {
	h := newStatsHandler(
		&stubAccountLookup{found: true, accountID: 42},
		&stubDeckPerformanceReader{},
		&stubWinRateTrendReader{buckets: []repository.WinRateBucket{}},
		&stubFormatDistributionReader{},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/win-rate-trend?granularity=weekly", nil)
	rr := httptest.NewRecorder()
	authedStatsHandler(h.GetWinRateTrend, 1).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}
}

func TestStatsHandler_GetWinRateTrend_NoAccount(t *testing.T) {
	h := newStatsHandler(
		&stubAccountLookup{found: false},
		&stubDeckPerformanceReader{},
		&stubWinRateTrendReader{},
		&stubFormatDistributionReader{},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/win-rate-trend", nil)
	rr := httptest.NewRecorder()
	authedStatsHandler(h.GetWinRateTrend, 1).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}

	data := decodeStatsData(t, rr.Body.Bytes())
	if len(data) != 0 {
		t.Errorf("expected empty data, got %d items", len(data))
	}
}

func TestStatsHandler_GetWinRateTrend_RepoError(t *testing.T) {
	h := newStatsHandler(
		&stubAccountLookup{found: true, accountID: 42},
		&stubDeckPerformanceReader{},
		&stubWinRateTrendReader{err: errors.New("db error")},
		&stubFormatDistributionReader{},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/win-rate-trend", nil)
	rr := httptest.NewRecorder()
	authedStatsHandler(h.GetWinRateTrend, 1).ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rr.Code)
	}
}

// ─── GetFormatDistribution ────────────────────────────────────────────────────

func TestStatsHandler_GetFormatDistribution_Unauthorized(t *testing.T) {
	h := newStatsHandler(
		&stubAccountLookup{},
		&stubDeckPerformanceReader{},
		&stubWinRateTrendReader{},
		&stubFormatDistributionReader{},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/format-distribution", nil)
	rr := httptest.NewRecorder()
	h.GetFormatDistribution(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rr.Code)
	}
}

func TestStatsHandler_GetFormatDistribution_HappyPath(t *testing.T) {
	rows := []repository.FormatDistributionRow{
		{Format: "Standard", GameCount: 50},
		{Format: "Historic", GameCount: 20},
	}

	h := newStatsHandler(
		&stubAccountLookup{found: true, accountID: 42},
		&stubDeckPerformanceReader{},
		&stubWinRateTrendReader{},
		&stubFormatDistributionReader{rows: rows},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/format-distribution", nil)
	rr := httptest.NewRecorder()
	authedStatsHandler(h.GetFormatDistribution, 1).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}

	data := decodeStatsData(t, rr.Body.Bytes())
	if len(data) != 2 {
		t.Fatalf("want 2 rows, got %d", len(data))
	}

	first := data[0].(map[string]interface{})
	if first["format"] != "Standard" {
		t.Errorf("format: want Standard, got %v", first["format"])
	}

	if first["game_count"].(float64) != 50 {
		t.Errorf("game_count: want 50, got %v", first["game_count"])
	}
}

func TestStatsHandler_GetFormatDistribution_NoAccount(t *testing.T) {
	h := newStatsHandler(
		&stubAccountLookup{found: false},
		&stubDeckPerformanceReader{},
		&stubWinRateTrendReader{},
		&stubFormatDistributionReader{},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/format-distribution", nil)
	rr := httptest.NewRecorder()
	authedStatsHandler(h.GetFormatDistribution, 1).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}

	data := decodeStatsData(t, rr.Body.Bytes())
	if len(data) != 0 {
		t.Errorf("expected empty data, got %d items", len(data))
	}
}

func TestStatsHandler_GetFormatDistribution_RepoError(t *testing.T) {
	h := newStatsHandler(
		&stubAccountLookup{found: true, accountID: 42},
		&stubDeckPerformanceReader{},
		&stubWinRateTrendReader{},
		&stubFormatDistributionReader{err: errors.New("db error")},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/format-distribution", nil)
	rr := httptest.NewRecorder()
	authedStatsHandler(h.GetFormatDistribution, 1).ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rr.Code)
	}
}
