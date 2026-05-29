package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/listing"
	mw "github.com/RdHamilton/vault-mtg/services/bff/internal/api/middleware"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
)

// ─── Repository interfaces ────────────────────────────────────────────────────

// DeckPerformanceReader fetches win/loss/draw per deck for an account.
type DeckPerformanceReader interface {
	GetDeckPerformance(ctx context.Context, accountID int64) ([]repository.DeckPerformanceRow, error)
}

// WinRateTrendReader fetches win-rate over time for an account.
type WinRateTrendReader interface {
	GetWinRateTrend(ctx context.Context, accountID int64, granularity string) ([]repository.WinRateBucket, error)
}

// FormatDistributionReader fetches game count per format for an account.
type FormatDistributionReader interface {
	GetFormatDistribution(ctx context.Context, accountID int64) ([]repository.FormatDistributionRow, error)
}

// DraftAnalyticsReader fetches per-draft analytics with cursor pagination.
type DraftAnalyticsReader interface {
	ListDraftAnalytics(
		ctx context.Context,
		accountID int64,
		setCode string,
		afterStartTime *time.Time,
		afterID string,
		limit int,
	) ([]repository.DraftAnalyticsRow, error)
}

// RankProgressionReader fetches rank-change events with cursor pagination.
type RankProgressionReader interface {
	ListRankProgression(
		ctx context.Context,
		accountID int64,
		format string,
		cursorTS *time.Time,
		cursorID string,
		limit int,
	) ([]repository.RankProgressionRow, error)
}

// ResultBreakdownReader fetches aggregate win/loss grouped by format.
type ResultBreakdownReader interface {
	GetResultBreakdown(ctx context.Context, accountID int64, format string) ([]repository.ResultBreakdownRow, error)
}

// ─── Response shapes ─────────────────────────────────────────────────────────

// deckPerformanceResponse is the JSON shape for a single deck's performance.
type deckPerformanceResponse struct {
	DeckID     string `json:"deck_id"`
	DeckName   string `json:"deck_name"`
	Format     string `json:"format"`
	Wins       int    `json:"wins"`
	Losses     int    `json:"losses"`
	Draws      int    `json:"draws"`
	TotalGames int    `json:"total_games"`
}

// winRateBucketResponse is the JSON shape for a single win-rate bucket.
type winRateBucketResponse struct {
	BucketStart time.Time `json:"bucket_start"`
	Wins        int       `json:"wins"`
	Losses      int       `json:"losses"`
	Draws       int       `json:"draws"`
	TotalGames  int       `json:"total_games"`
	WinRate     float64   `json:"win_rate"`
}

// formatDistributionResponse is the JSON shape for a single format distribution row.
type formatDistributionResponse struct {
	Format    string `json:"format"`
	GameCount int    `json:"game_count"`
}

// draftAnalyticsResponse is the JSON shape for a single draft in the analytics list.
type draftAnalyticsResponse struct {
	SessionID   string    `json:"session_id"`
	SetCode     string    `json:"set_code"`
	Format      string    `json:"format"`
	StartedAt   time.Time `json:"started_at"`
	Wins        int       `json:"wins"`
	Losses      int       `json:"losses"`
	TotalPicks  int       `json:"total_picks"`
	AvgGIHWR    *float64  `json:"avg_gihwr"`
	AvgPickRank *float64  `json:"avg_pick_rank"`
}

// rankProgressionResponse is the JSON shape for a single rank-change event.
type rankProgressionResponse struct {
	MatchID    string    `json:"match_id"`
	OccurredAt time.Time `json:"occurred_at"`
	Format     string    `json:"format"`
	RankBefore *string   `json:"rank_before"`
	RankAfter  *string   `json:"rank_after"`
	Result     string    `json:"result"`
}

// resultBreakdownResponse is the JSON shape for a single format breakdown row.
type resultBreakdownResponse struct {
	Format string `json:"format"`
	Wins   int    `json:"wins"`
	Losses int    `json:"losses"`
	Draws  int    `json:"draws"`
}

// ─── Sort allowlists ──────────────────────────────────────────────────────────

var draftAnalyticsSortAllowlist = map[string]struct{}{
	"started_at": {},
}

var rankProgressionSortAllowlist = map[string]struct{}{
	"occurred_at": {},
}

// ─── StatsHandler ────────────────────────────────────────────────────────────

// StatsHandler handles the stats endpoints:
//   - GET /api/v1/stats/deck-performance
//   - GET /api/v1/stats/win-rate-trend
//   - GET /api/v1/stats/format-distribution
//   - GET /api/v1/stats/draft-analytics  (issue #1514)
//   - GET /api/v1/stats/rank-progression (issue #1514)
//   - GET /api/v1/stats/result-breakdown (issue #1514)
type StatsHandler struct {
	accounts        AccountLookup
	deckPerformance DeckPerformanceReader
	winRateTrend    WinRateTrendReader
	formatDist      FormatDistributionReader
	draftAnalytics  DraftAnalyticsReader
	rankProgression RankProgressionReader
	resultBreakdown ResultBreakdownReader
}

// NewStatsHandler returns a StatsHandler wired with the provided dependencies.
func NewStatsHandler(
	accounts AccountLookup,
	deckPerformance DeckPerformanceReader,
	winRateTrend WinRateTrendReader,
	formatDist FormatDistributionReader,
) *StatsHandler {
	return &StatsHandler{
		accounts:        accounts,
		deckPerformance: deckPerformance,
		winRateTrend:    winRateTrend,
		formatDist:      formatDist,
	}
}

// WithDraftAnalytics wires the draft analytics reader.
func (h *StatsHandler) WithDraftAnalytics(r DraftAnalyticsReader) *StatsHandler {
	h.draftAnalytics = r
	return h
}

// WithRankProgression wires the rank progression reader.
func (h *StatsHandler) WithRankProgression(r RankProgressionReader) *StatsHandler {
	h.rankProgression = r
	return h
}

// WithResultBreakdown wires the result breakdown reader.
func (h *StatsHandler) WithResultBreakdown(r ResultBreakdownReader) *StatsHandler {
	h.resultBreakdown = r
	return h
}

// ─── GET /api/v1/stats/deck-performance ──────────────────────────────────────

// GetDeckPerformance handles GET /api/v1/stats/deck-performance.
// Returns win/loss/draw counts for every deck the authenticated user has
// played, ordered by total_games DESC.
func (h *StatsHandler) GetDeckPerformance(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[StatsHandler.GetDeckPerformance] GetAccountIDByUserID userID=%d: %v", userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if !found {
		writeStatsJSON(w, map[string]interface{}{"data": []deckPerformanceResponse{}})
		return
	}

	rows, err := h.deckPerformance.GetDeckPerformance(r.Context(), accountID)
	if err != nil {
		log.Printf("[StatsHandler.GetDeckPerformance] GetDeckPerformance accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	data := make([]deckPerformanceResponse, 0, len(rows))
	for _, row := range rows {
		data = append(data, deckPerformanceResponse{
			DeckID:     row.DeckID,
			DeckName:   row.DeckName,
			Format:     row.Format,
			Wins:       row.Wins,
			Losses:     row.Losses,
			Draws:      row.Draws,
			TotalGames: row.TotalGames,
		})
	}

	writeStatsJSON(w, map[string]interface{}{"data": data})
}

// ─── GET /api/v1/stats/win-rate-trend ────────────────────────────────────────

// GetWinRateTrend handles GET /api/v1/stats/win-rate-trend.
// Query param: granularity — "daily" (default) or "weekly".
// Returns win-rate buckets for the last 90 days.
func (h *StatsHandler) GetWinRateTrend(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "daily"
	}

	if granularity != "daily" && granularity != "weekly" {
		writeJSONError(w, "invalid granularity: must be 'daily' or 'weekly'", http.StatusBadRequest)
		return
	}

	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[StatsHandler.GetWinRateTrend] GetAccountIDByUserID userID=%d: %v", userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if !found {
		writeStatsJSON(w, map[string]interface{}{"data": []winRateBucketResponse{}})
		return
	}

	buckets, err := h.winRateTrend.GetWinRateTrend(r.Context(), accountID, granularity)
	if err != nil {
		log.Printf("[StatsHandler.GetWinRateTrend] GetWinRateTrend accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	data := make([]winRateBucketResponse, 0, len(buckets))
	for _, b := range buckets {
		data = append(data, winRateBucketResponse{
			BucketStart: b.BucketStart,
			Wins:        b.Wins,
			Losses:      b.Losses,
			Draws:       b.Draws,
			TotalGames:  b.TotalGames,
			WinRate:     b.WinRate,
		})
	}

	writeStatsJSON(w, map[string]interface{}{"data": data})
}

// ─── GET /api/v1/stats/format-distribution ───────────────────────────────────

// GetFormatDistribution handles GET /api/v1/stats/format-distribution.
// Returns game count per format, ordered by game_count DESC.
func (h *StatsHandler) GetFormatDistribution(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[StatsHandler.GetFormatDistribution] GetAccountIDByUserID userID=%d: %v", userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if !found {
		writeStatsJSON(w, map[string]interface{}{"data": []formatDistributionResponse{}})
		return
	}

	rows, err := h.formatDist.GetFormatDistribution(r.Context(), accountID)
	if err != nil {
		log.Printf("[StatsHandler.GetFormatDistribution] GetFormatDistribution accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	data := make([]formatDistributionResponse, 0, len(rows))
	for _, row := range rows {
		data = append(data, formatDistributionResponse{
			Format:    row.Format,
			GameCount: row.GameCount,
		})
	}

	writeStatsJSON(w, map[string]interface{}{"data": data})
}

// ─── GET /api/v1/stats/draft-analytics ───────────────────────────────────────

// GetDraftAnalytics handles GET /api/v1/stats/draft-analytics.
// Returns per-draft pick-efficiency and record data for the authenticated user,
// cursor-paginated ordered by started_at DESC.
// Query params: cursor, limit (default 50, max 200), set_code.
func (h *StatsHandler) GetDraftAnalytics(w http.ResponseWriter, r *http.Request) {
	if h.draftAnalytics == nil {
		writeJSONError(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}

	userID, ok := mw.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	params, err := listing.ParseListParams(r, draftAnalyticsSortAllowlist, "started_at")
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	setCode := r.URL.Query().Get("set_code")
	if setCode != "" && !isValidSetCode(setCode) {
		writeJSONError(w, "invalid set_code: must be 3-5 uppercase letters", http.StatusBadRequest)
		return
	}

	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[StatsHandler.GetDraftAnalytics] GetAccountIDByUserID userID=%d: %v", userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if !found {
		writeListV2JSON(w, listing.ListEnvelope[draftAnalyticsResponse]{
			Data: []draftAnalyticsResponse{},
			Page: listing.Page{HasMore: false, Limit: params.Limit},
		})
		return
	}

	var cursorTS *time.Time
	var cursorID string

	if params.Cursor != nil {
		cursorTS = params.Cursor.OccurredAt
		cursorID = params.Cursor.ID
	}

	rows, err := h.draftAnalytics.ListDraftAnalytics(r.Context(), accountID, setCode, cursorTS, cursorID, params.Limit)
	if err != nil {
		log.Printf("[StatsHandler.GetDraftAnalytics] ListDraftAnalytics accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	env := listing.BuildEnvelope(rows, params.Limit, func(d repository.DraftAnalyticsRow) listing.Cursor {
		return listing.Cursor{OccurredAt: &d.StartTime, ID: d.SessionID}
	})

	data := make([]draftAnalyticsResponse, 0, len(env.Data))
	for _, d := range env.Data {
		data = append(data, draftAnalyticsResponse{
			SessionID:   d.SessionID,
			SetCode:     d.SetCode,
			Format:      draftTypeToFormat(d.DraftType),
			StartedAt:   d.StartTime,
			Wins:        d.Wins,
			Losses:      d.Losses,
			TotalPicks:  d.TotalPicks,
			AvgGIHWR:    d.AvgGIHWR,
			AvgPickRank: d.AvgPickRank,
		})
	}

	writeListV2JSON(w, listing.ListEnvelope[draftAnalyticsResponse]{Data: data, Page: env.Page})
}

// ─── GET /api/v1/stats/rank-progression ──────────────────────────────────────

// GetRankProgression handles GET /api/v1/stats/rank-progression.
// Returns rank changes over time for the authenticated user, cursor-paginated
// ordered by occurred_at DESC.  Only matches carrying rank_before or rank_after
// are included.
// Query params: cursor, limit (default 50, max 200), format.
func (h *StatsHandler) GetRankProgression(w http.ResponseWriter, r *http.Request) {
	if h.rankProgression == nil {
		writeJSONError(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}

	userID, ok := mw.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	params, err := listing.ParseListParams(r, rankProgressionSortAllowlist, "occurred_at")
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	format := r.URL.Query().Get("format")
	if format != "" {
		if !IsKnownFormat(format) {
			writeJSONError(w, "unknown format: "+format, http.StatusBadRequest)
			return
		}
	}

	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[StatsHandler.GetRankProgression] GetAccountIDByUserID userID=%d: %v", userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if !found {
		writeListV2JSON(w, listing.ListEnvelope[rankProgressionResponse]{
			Data: []rankProgressionResponse{},
			Page: listing.Page{HasMore: false, Limit: params.Limit},
		})
		return
	}

	var cursorTS *time.Time
	var cursorID string

	if params.Cursor != nil {
		cursorTS = params.Cursor.OccurredAt
		cursorID = params.Cursor.ID
	}

	rows, err := h.rankProgression.ListRankProgression(r.Context(), accountID, format, cursorTS, cursorID, params.Limit)
	if err != nil {
		log.Printf("[StatsHandler.GetRankProgression] ListRankProgression accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	env := listing.BuildEnvelope(rows, params.Limit, func(rp repository.RankProgressionRow) listing.Cursor {
		return listing.Cursor{OccurredAt: &rp.OccurredAt, ID: rp.MatchID}
	})

	data := make([]rankProgressionResponse, 0, len(env.Data))
	for _, rp := range env.Data {
		data = append(data, rankProgressionResponse{
			MatchID:    rp.MatchID,
			OccurredAt: rp.OccurredAt,
			Format:     rp.Format,
			RankBefore: rp.RankBefore,
			RankAfter:  rp.RankAfter,
			Result:     rp.Result,
		})
	}

	writeListV2JSON(w, listing.ListEnvelope[rankProgressionResponse]{Data: data, Page: env.Page})
}

// ─── GET /api/v1/stats/result-breakdown ──────────────────────────────────────

// GetResultBreakdown handles GET /api/v1/stats/result-breakdown.
// Returns aggregate wins/losses grouped by format for the authenticated user.
// Query param: format — optional; when set filters to that format only.
func (h *StatsHandler) GetResultBreakdown(w http.ResponseWriter, r *http.Request) {
	if h.resultBreakdown == nil {
		writeJSONError(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}

	userID, ok := mw.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	format := r.URL.Query().Get("format")
	if format != "" {
		if !IsKnownFormat(format) {
			writeJSONError(w, "unknown format: "+format, http.StatusBadRequest)
			return
		}
	}

	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[StatsHandler.GetResultBreakdown] GetAccountIDByUserID userID=%d: %v", userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if !found {
		writeStatsJSON(w, map[string]interface{}{"data": []resultBreakdownResponse{}})
		return
	}

	rows, err := h.resultBreakdown.GetResultBreakdown(r.Context(), accountID, format)
	if err != nil {
		log.Printf("[StatsHandler.GetResultBreakdown] GetResultBreakdown accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	data := make([]resultBreakdownResponse, 0, len(rows))
	for _, row := range rows {
		data = append(data, resultBreakdownResponse{
			Format: row.Format,
			Wins:   row.Wins,
			Losses: row.Losses,
			Draws:  row.Draws,
		})
	}

	writeStatsJSON(w, map[string]interface{}{"data": data})
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func writeStatsJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[writeStatsJSON] encode: %v", err)
	}
}
