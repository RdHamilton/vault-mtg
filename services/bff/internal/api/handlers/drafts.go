// Phase 2 PR #10 — /api/v1/drafts/* handlers.
//
// Replaces the SPA's daemonClient surface for drafts.ts. ~38 endpoints
// covering draft CRUD reads, picks, deck metrics, stats, 17Lands export,
// community comparison, temporal trends, learning curve, and the
// /decks/* + /feedback/* strays the SPA's drafts.ts wraps. Many
// recommendation/grading endpoints are documented STUBs pending the ML
// pipeline (same reasoning as PR #5b/#9).
//
// All routes are guarded by DaemonAPIKeyAuth + the standard envelope.
// Per-session reads are scoped via draft_sessions.account_id; community
// + temporal data is global (catalog).

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// draftsReader is the minimal repo surface the handler needs.
type draftsReader interface {
	ListSessions(ctx context.Context, accountID int64, f repository.DraftFilter) ([]repository.DraftSessionDetailRow, error)
	GetSession(ctx context.Context, accountID int64, sessionID string) (*repository.DraftSessionDetailRow, error)
	DistinctSets(ctx context.Context, accountID int64) ([]string, error)
	PicksForSession(ctx context.Context, accountID int64, sessionID string) ([]repository.DraftPickRow, error)
	AggregateStats(ctx context.Context, accountID int64, f repository.DraftFilter) (repository.DraftStatsAggregate, error)
	CommunityComparisons(ctx context.Context) ([]repository.CommunityComparisonRow, error)
	CommunityComparisonForSet(ctx context.Context, setCode, format string) (*repository.CommunityComparisonRow, error)
	TemporalTrends(ctx context.Context, periodType, setCode string, numPeriods int) ([]repository.TemporalTrendRow, error)
	LearningCurve(ctx context.Context, setCode string) ([]repository.TemporalTrendRow, error)
	RecommendationFeedbackStats(ctx context.Context, accountID int64) (repository.RecommendationFeedbackStatsRow, error)
}

// DraftsHandler serves the cloud-data Phase 2 drafts API.
type DraftsHandler struct {
	drafts   draftsReader
	accounts AccountLookup
}

// NewDraftsHandler returns a DraftsHandler wired with the given repo + lookup.
func NewDraftsHandler(d draftsReader, accounts AccountLookup) *DraftsHandler {
	return &DraftsHandler{drafts: d, accounts: accounts}
}

// ─── wire shapes ────────────────────────────────────────────────────────────

// draftSessionResponse mirrors models.DraftSession (PascalCase per the
// existing TS class).
type draftSessionResponse struct {
	ID                   string     `json:"ID"`
	EventName            string     `json:"EventName"`
	SetCode              string     `json:"SetCode"`
	DraftType            string     `json:"DraftType"`
	StartTime            time.Time  `json:"StartTime"`
	EndTime              *time.Time `json:"EndTime,omitempty"`
	Status               string     `json:"Status"`
	TotalPicks           int        `json:"TotalPicks"`
	OverallGrade         *string    `json:"OverallGrade,omitempty"`
	OverallScore         *int       `json:"OverallScore,omitempty"`
	PickQualityScore     *float64   `json:"PickQualityScore,omitempty"`
	ColorDisciplineScore *float64   `json:"ColorDisciplineScore,omitempty"`
	DeckCompositionScore *float64   `json:"DeckCompositionScore,omitempty"`
	StrategicScore       *float64   `json:"StrategicScore,omitempty"`
	PredictedWinRate     *float64   `json:"PredictedWinRate,omitempty"`
	PredictedWinRateMin  *float64   `json:"PredictedWinRateMin,omitempty"`
	PredictedWinRateMax  *float64   `json:"PredictedWinRateMax,omitempty"`
	PredictionFactors    *string    `json:"PredictionFactors,omitempty"`
	PredictedAt          *time.Time `json:"PredictedAt,omitempty"`
	CreatedAt            time.Time  `json:"CreatedAt"`
	UpdatedAt            time.Time  `json:"UpdatedAt"`
}

// draftPickResponse mirrors models.DraftPickSession.
type draftPickResponse struct {
	ID               int64     `json:"ID"`
	SessionID        string    `json:"SessionID"`
	PackNumber       int       `json:"PackNumber"`
	PickNumber       int       `json:"PickNumber"`
	CardID           string    `json:"CardID"`
	Timestamp        time.Time `json:"Timestamp"`
	PickQualityGrade *string   `json:"PickQualityGrade,omitempty"`
	PickQualityRank  *int      `json:"PickQualityRank,omitempty"`
	PackBestGIHWR    *float64  `json:"PackBestGIHWR,omitempty"`
	PickedCardGIHWR  *float64  `json:"PickedCardGIHWR,omitempty"`
	AlternativesJSON *string   `json:"AlternativesJSON,omitempty"`
}

// draftStatsResponse mirrors metrics.DraftStats. Loose shape — extra
// fields are tolerated by the SPA.
type draftStatsResponse struct {
	TotalDrafts       int            `json:"totalDrafts"`
	CompletedDrafts   int            `json:"completedDrafts"`
	AvgOverallScore   float64        `json:"avgOverallScore"`
	AvgPickQuality    float64        `json:"avgPickQuality"`
	AvgPredictedWR    float64        `json:"avgPredictedWinRate"`
	GradeDistribution map[string]int `json:"gradeDistribution"`
}

// communityComparisonResponse mirrors analytics.CommunityComparisonResponse
// (snake_case per the SPA's TS class).
type communityComparisonResponse struct {
	SetCode             string  `json:"set_code"`
	DraftFormat         string  `json:"draft_format"`
	UserWinRate         float64 `json:"user_win_rate"`
	CommunityAvgWinRate float64 `json:"community_avg_win_rate"`
	PercentileRank      float64 `json:"percentile_rank"`
	SampleSize          int     `json:"sample_size"`
	CalculatedAt        string  `json:"calculated_at"`
	ArchetypeComparison []any   `json:"archetype_comparison"`
}

// trendEntryResponse mirrors analytics.TrendEntry (snake_case).
type trendEntryResponse struct {
	PeriodType    string  `json:"period_type"`
	PeriodStart   string  `json:"period_start"`
	PeriodEnd     string  `json:"period_end"`
	SetCode       string  `json:"set_code"`
	DraftsCount   int     `json:"drafts_count"`
	MatchesPlayed int     `json:"matches_played"`
	MatchesWon    int     `json:"matches_won"`
	WinRate       float64 `json:"win_rate"`
	AvgDraftGrade float64 `json:"avg_draft_grade"`
}

// trendAnalysisResponseDrafts mirrors analytics.TrendAnalysisResponse
// (snake_case). Renamed to avoid collision with the matches-trend type.
type trendAnalysisResponseDrafts struct {
	PeriodType  string               `json:"period_type"`
	NumPeriods  int                  `json:"num_periods"`
	SetCode     string               `json:"set_code"`
	Trends      []trendEntryResponse `json:"trends"`
	Summary     map[string]any       `json:"summary"`
	GeneratedAt string               `json:"generated_at"`
}

// learningCurveResponse mirrors analytics.LearningCurveResponse.
type learningCurveResponse struct {
	SetCode     string                `json:"set_code"`
	Periods     []learningPeriodEntry `json:"periods"`
	GeneratedAt string                `json:"generated_at"`
}

// learningPeriodEntry mirrors analytics.LearningPeriodEntry.
type learningPeriodEntry struct {
	PeriodStart   string  `json:"period_start"`
	PeriodEnd     string  `json:"period_end"`
	DraftsCount   int     `json:"drafts_count"`
	MatchesPlayed int     `json:"matches_played"`
	MatchesWon    int     `json:"matches_won"`
	WinRate       float64 `json:"win_rate"`
	AvgGrade      float64 `json:"avg_grade"`
}

// exportableDraftResponse is the SPA's DraftSession-shaped row used by
// /drafts/exportable. Same wire shape as draftSessionResponse.

// ─── handlers — sessions ───────────────────────────────────────────────────

// List handles POST /api/v1/drafts (filter body).
func (h *DraftsHandler) List(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "List")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, []draftSessionResponse{})
		return
	}
	var body struct {
		Format    string `json:"format"`
		SetCode   string `json:"set_code"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
		Status    string `json:"status"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	filter := repository.DraftFilter{
		Format: body.Format, SetCode: body.SetCode, Status: body.Status,
	}
	if body.StartDate != "" {
		t, err := parseFilterDate(body.StartDate)
		if err != nil {
			writeJSONError(w, "start_date: "+err.Error(), http.StatusBadRequest)
			return
		}
		filter.StartDate = &t
	}
	if body.EndDate != "" {
		t, err := parseFilterDate(body.EndDate)
		if err != nil {
			writeJSONError(w, "end_date: "+err.Error(), http.StatusBadRequest)
			return
		}
		filter.EndDate = &t
	}
	rows, err := h.drafts.ListSessions(r.Context(), accountID, filter)
	if err != nil {
		log.Printf("[DraftsHandler.List] accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, draftSessionsToResponse(rows))
}

// Get handles GET /api/v1/drafts/{sessionId}.
func (h *DraftsHandler) Get(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Get")
	if !ok {
		return
	}
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionId"))
	if sessionID == "" {
		writeJSONError(w, "sessionId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "draft not found", http.StatusNotFound)
		return
	}
	s, err := h.drafts.GetSession(r.Context(), accountID, sessionID)
	if err != nil {
		log.Printf("[DraftsHandler.Get] accountID=%d sessionID=%s: %v", accountID, sessionID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if s == nil {
		writeJSONError(w, "draft not found", http.StatusNotFound)
		return
	}
	writeMatchesJSON(w, draftSessionRowToResponse(*s))
}

// Picks handles GET /api/v1/drafts/{sessionId}/picks.
func (h *DraftsHandler) Picks(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Picks")
	if !ok {
		return
	}
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionId"))
	if sessionID == "" {
		writeJSONError(w, "sessionId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, []draftPickResponse{})
		return
	}
	rows, err := h.drafts.PicksForSession(r.Context(), accountID, sessionID)
	if err != nil {
		log.Printf("[DraftsHandler.Picks] accountID=%d sessionID=%s: %v", accountID, sessionID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, draftPickRowsToResponse(rows))
}

// Pool handles GET /api/v1/drafts/{sessionId}/pool. Returns the picked
// cards as a SetCard[]. Currently returns empty — the SPA's draft pool
// view also reads from the deck after the draft completes; populating
// this endpoint richly requires a join we're deferring to a follow-up.
func (h *DraftsHandler) Pool(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	writeMatchesJSON(w, []any{})
}

// Curve handles GET /api/v1/drafts/{sessionId}/curve. STUB until we
// build a draft-pool aggregator (the SPA shows curve charts based on
// the picked cards, requires a join we haven't wired). Returns empty
// curve buckets.
func (h *DraftsHandler) Curve(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	writeMatchesJSON(w, map[int]int{})
}

// Colors handles GET /api/v1/drafts/{sessionId}/colors. STUB (same
// rationale as Curve). Returns empty per-color counts.
func (h *DraftsHandler) Colors(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	writeMatchesJSON(w, map[string]int{})
}

// Stats handles POST /api/v1/drafts/stats.
func (h *DraftsHandler) Stats(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Stats")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, draftStatsResponse{GradeDistribution: map[string]int{}})
		return
	}
	var body struct {
		Format    string `json:"format"`
		SetCode   string `json:"set_code"`
		Status    string `json:"status"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	filter := repository.DraftFilter{
		Format: body.Format, SetCode: body.SetCode, Status: body.Status,
	}
	if body.StartDate != "" {
		t, err := parseFilterDate(body.StartDate)
		if err != nil {
			writeJSONError(w, "start_date: "+err.Error(), http.StatusBadRequest)
			return
		}
		filter.StartDate = &t
	}
	if body.EndDate != "" {
		t, err := parseFilterDate(body.EndDate)
		if err != nil {
			writeJSONError(w, "end_date: "+err.Error(), http.StatusBadRequest)
			return
		}
		filter.EndDate = &t
	}
	agg, err := h.drafts.AggregateStats(r.Context(), accountID, filter)
	if err != nil {
		log.Printf("[DraftsHandler.Stats] accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, draftStatsResponse{
		TotalDrafts: agg.TotalDrafts, CompletedDrafts: agg.CompletedDrafts,
		AvgOverallScore:   deref(agg.AvgOverallScore),
		AvgPickQuality:    deref(agg.AvgPickQuality),
		AvgPredictedWR:    deref(agg.AvgPredictedWR),
		GradeDistribution: agg.GradeDistribution,
	})
}

// Formats handles GET /api/v1/drafts/formats.
func (h *DraftsHandler) Formats(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Formats")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, []string{})
		return
	}
	out, err := h.drafts.DistinctSets(r.Context(), accountID)
	if err != nil {
		log.Printf("[DraftsHandler.Formats] accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if out == nil {
		out = []string{}
	}
	writeMatchesJSON(w, out)
}

// Recent handles GET /api/v1/drafts/recent[?limit=N].
func (h *DraftsHandler) Recent(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Recent")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, []draftSessionResponse{})
		return
	}
	limit := parseLimitDefault(r, "limit", 10)
	rows, err := h.drafts.ListSessions(r.Context(), accountID, repository.DraftFilter{Limit: limit})
	if err != nil {
		log.Printf("[DraftsHandler.Recent] accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, draftSessionsToResponse(rows))
}

// DeckMetrics handles GET /api/v1/drafts/{sessionId}/deck-metrics.
// STUB until the deck-pool aggregator lands; returns zero metrics.
func (h *DraftsHandler) DeckMetrics(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	writeMatchesJSON(w, map[string]any{
		"totalCards": 0, "averageCMC": 0.0,
		"manaCurve":   map[int]int{},
		"colorCounts": map[string]int{},
	})
}

// Exportable handles GET /api/v1/drafts/exportable[?limit=N]. Returns
// completed sessions newest-first.
func (h *DraftsHandler) Exportable(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Exportable")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, []draftSessionResponse{})
		return
	}
	limit := parseLimitDefault(r, "limit", 50)
	rows, err := h.drafts.ListSessions(r.Context(), accountID, repository.DraftFilter{
		Status: "completed", Limit: limit,
	})
	if err != nil {
		log.Printf("[DraftsHandler.Exportable] accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, draftSessionsToResponse(rows))
}

// Export17Lands handles GET /api/v1/drafts/{sessionId}/export/17lands.
// Renders the picks into the 17lands JSON format (draft_id, event_type,
// set_code, picks[]). Real impl backed by draft_picks.
func (h *DraftsHandler) Export17Lands(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Export17Lands")
	if !ok {
		return
	}
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionId"))
	if sessionID == "" {
		writeJSONError(w, "sessionId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "draft not found", http.StatusNotFound)
		return
	}
	session, err := h.drafts.GetSession(r.Context(), accountID, sessionID)
	if err != nil {
		log.Printf("[DraftsHandler.Export17Lands] GetSession: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if session == nil {
		writeJSONError(w, "draft not found", http.StatusNotFound)
		return
	}
	picks, err := h.drafts.PicksForSession(r.Context(), accountID, sessionID)
	if err != nil {
		log.Printf("[DraftsHandler.Export17Lands] PicksForSession: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	export := map[string]any{
		"draft_id":   session.ID,
		"event_type": session.DraftType,
		"set_code":   session.SetCode,
		"draft_time": session.StartTime.UTC().Format(time.RFC3339),
		"picks":      buildSeventeenLandsPicks(picks),
		"metadata": map[string]any{
			"exported_at":   time.Now().UTC().Format(time.RFC3339),
			"exported_from": "MTGA-Companion",
		},
	}
	writeMatchesJSON(w, map[string]any{
		"session_id": session.ID,
		"file_name":  "draft_" + session.SetCode + "_" + session.StartTime.UTC().Format("2006-01-02_15-04-05") + ".json",
		"export":     export,
	})
}

// ─── handlers — community + trends + learning ──────────────────────────────

// CommunityComparisonByGet handles GET /api/v1/drafts/community-comparison/{setCode}.
func (h *DraftsHandler) CommunityComparisonByGet(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	setCode := strings.TrimSpace(chi.URLParam(r, "setCode"))
	if setCode == "" {
		writeJSONError(w, "setCode is required", http.StatusBadRequest)
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	row, err := h.drafts.CommunityComparisonForSet(r.Context(), setCode, format)
	if err != nil {
		log.Printf("[DraftsHandler.CommunityComparisonByGet] setCode=%s: %v", setCode, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if row == nil {
		writeMatchesJSON(w, communityComparisonResponse{SetCode: setCode, ArchetypeComparison: []any{}})
		return
	}
	writeMatchesJSON(w, communityComparisonRowToResponse(*row))
}

// CommunityComparisonByPost handles POST /api/v1/drafts/community-comparison.
func (h *DraftsHandler) CommunityComparisonByPost(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	var body struct {
		SetCode     string `json:"set_code"`
		DraftFormat string `json:"draft_format"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.SetCode == "" {
		writeJSONError(w, "set_code is required", http.StatusBadRequest)
		return
	}
	row, err := h.drafts.CommunityComparisonForSet(r.Context(), body.SetCode, body.DraftFormat)
	if err != nil {
		log.Printf("[DraftsHandler.CommunityComparisonByPost] setCode=%s: %v", body.SetCode, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if row == nil {
		writeMatchesJSON(w, communityComparisonResponse{SetCode: body.SetCode, DraftFormat: body.DraftFormat, ArchetypeComparison: []any{}})
		return
	}
	writeMatchesJSON(w, communityComparisonRowToResponse(*row))
}

// AllCommunityComparisons handles GET /api/v1/drafts/community-comparison.
func (h *DraftsHandler) AllCommunityComparisons(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	rows, err := h.drafts.CommunityComparisons(r.Context())
	if err != nil {
		log.Printf("[DraftsHandler.AllCommunityComparisons] %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	out := make([]communityComparisonResponse, 0, len(rows))
	for _, c := range rows {
		out = append(out, communityComparisonRowToResponse(c))
	}
	writeMatchesJSON(w, out)
}

// Trends handles POST /api/v1/drafts/trends.
func (h *DraftsHandler) Trends(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	var body struct {
		PeriodType string `json:"period_type"`
		NumPeriods int    `json:"num_periods"`
		SetCode    string `json:"set_code"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	period := normalizePeriodType(body.PeriodType)
	if period == "" {
		writeJSONError(w, "period_type must be week|month", http.StatusBadRequest)
		return
	}
	rows, err := h.drafts.TemporalTrends(r.Context(), period, body.SetCode, body.NumPeriods)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidDraftPeriodType) {
			writeJSONError(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("[DraftsHandler.Trends] period=%s set=%s: %v", period, body.SetCode, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	resp := trendAnalysisResponseDrafts{
		PeriodType: period, NumPeriods: body.NumPeriods, SetCode: body.SetCode,
		Trends:      trendRowsToResponse(rows),
		Summary:     map[string]any{"total_periods": len(rows)},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	writeMatchesJSON(w, resp)
}

// LearningCurve handles GET /api/v1/drafts/learning-curve/{setCode}.
func (h *DraftsHandler) LearningCurve(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	setCode := strings.TrimSpace(chi.URLParam(r, "setCode"))
	if setCode == "" {
		writeJSONError(w, "setCode is required", http.StatusBadRequest)
		return
	}
	rows, err := h.drafts.LearningCurve(r.Context(), setCode)
	if err != nil {
		log.Printf("[DraftsHandler.LearningCurve] setCode=%s: %v", setCode, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	periods := make([]learningPeriodEntry, 0, len(rows))
	for _, t := range rows {
		entry := learningPeriodEntry{
			PeriodStart: t.PeriodStart.UTC().Format("2006-01-02"),
			PeriodEnd:   t.PeriodEnd.UTC().Format("2006-01-02"),
			DraftsCount: t.DraftsCount, MatchesPlayed: t.MatchesPlayed, MatchesWon: t.MatchesWon,
		}
		if t.MatchesPlayed > 0 {
			entry.WinRate = float64(t.MatchesWon) / float64(t.MatchesPlayed)
		}
		if t.AvgDraftGrade != nil {
			entry.AvgGrade = *t.AvgDraftGrade
		}
		periods = append(periods, entry)
	}
	writeMatchesJSON(w, learningCurveResponse{
		SetCode: setCode, Periods: periods,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

// ─── handlers — feedback (drafts.ts strays into /feedback/*) ──────────────

// FeedbackStats handles GET /api/v1/feedback/stats. Real read from
// recommendation_feedback.
func (h *DraftsHandler) FeedbackStats(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "FeedbackStats")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, map[string]any{
			"totalRecommendations": 0, "accepted": 0, "rejected": 0, "winRateImpact": 0.0,
		})
		return
	}
	stats, err := h.drafts.RecommendationFeedbackStats(r.Context(), accountID)
	if err != nil {
		log.Printf("[DraftsHandler.FeedbackStats] accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	winRateImpact := 0.0
	if stats.WinRateImpact != nil {
		winRateImpact = *stats.WinRateImpact
	}
	writeMatchesJSON(w, map[string]any{
		"totalRecommendations": stats.TotalRecommendations,
		"accepted":             stats.Accepted,
		"rejected":             stats.Rejected,
		"winRateImpact":        winRateImpact,
	})
}

// FeedbackRecommendation / FeedbackAction / FeedbackOutcome are accept-
// only STUBs: they 204 to confirm the SPA's record-recommendation flow
// doesn't 404. Persistence to recommendation_feedback is a follow-up
// PR.
func (h *DraftsHandler) FeedbackRecommendation(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DraftsHandler) FeedbackAction(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DraftsHandler) FeedbackOutcome(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── handlers — STUBs (ML / grading pipeline) ──────────────────────────────

// Insights handles POST /api/v1/drafts/insights. STUB returns empty
// FormatInsights shape (snake_case per insights.FormatInsights).
func (h *DraftsHandler) Insights(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	var body struct {
		Format  string `json:"format"`
		SetCode string `json:"set_code"`
	}
	_ = decodeJSONBody(r, &body)
	writeMatchesJSON(w, map[string]any{
		"set_code": body.SetCode, "draft_format": body.Format,
		"color_rankings": []any{}, "top_bombs": []any{},
		"top_removal": []any{}, "top_creatures": []any{}, "top_commons": []any{},
		"format_speed": map[string]any{},
	})
}

// CalculatePrediction handles POST /api/v1/drafts/{sessionId}/calculate-prediction. STUB.
func (h *DraftsHandler) CalculatePrediction(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	writeMatchesJSON(w, map[string]any{
		"predicted_win_rate": 0.0, "predicted_win_rate_min": 0.0, "predicted_win_rate_max": 0.0,
		"factors": []any{}, "calculated_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// CalculateGrade handles POST /api/v1/drafts/{sessionId}/calculate-grade. STUB.
func (h *DraftsHandler) CalculateGrade(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	writeMatchesJSON(w, draftGradeStub())
}

// DraftGrade handles GET /api/v1/drafts/{sessionId}/analysis. STUB
// returning the SPA's DraftGrade shape.
func (h *DraftsHandler) DraftGrade(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	writeMatchesJSON(w, draftGradeStub())
}

// AnalyzeSessionPickQuality handles POST /api/v1/drafts/{sessionId}/analyze-picks. STUB.
func (h *DraftsHandler) AnalyzeSessionPickQuality(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RecalculateSetGrades handles POST /api/v1/drafts/recalculate-set-grades. STUB.
func (h *DraftsHandler) RecalculateSetGrades(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	var body struct {
		SetCode string `json:"set_code"`
	}
	_ = decodeJSONBody(r, &body)
	writeMatchesJSON(w, map[string]any{
		"status": "ok", "set": body.SetCode, "count": 0,
		"message": "stub: grade recalculation pipeline not yet wired",
	})
}

// ─── /decks/* strays from drafts.ts ────────────────────────────────────────
//
// The SPA's drafts.ts wraps three /decks/* endpoints (recommendations,
// explain-recommendation, classify-draft-pool). They live here rather
// than in DecksHandler because the data flow is draft-driven.

// DecksRecommendations handles POST /api/v1/decks/recommendations. STUB.
func (h *DraftsHandler) DecksRecommendations(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	writeMatchesJSON(w, map[string]any{
		"recommendations": []any{}, "session_id": "", "generated_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// DecksExplainRecommendation handles POST /api/v1/decks/explain-recommendation. STUB.
func (h *DraftsHandler) DecksExplainRecommendation(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	writeMatchesJSON(w, map[string]any{
		"explanation": "Recommendation explanations require the ML pipeline (pending PR).",
		"factors":     []any{},
	})
}

// DecksClassifyDraftPool handles POST /api/v1/decks/classify-draft-pool. STUB.
func (h *DraftsHandler) DecksClassifyDraftPool(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	writeMatchesJSON(w, map[string]any{
		"archetype": "Unknown", "confidence": 0.0,
		"strengths":  []string{},
		"weaknesses": []string{},
	})
}

// ─── helpers ────────────────────────────────────────────────────────────────

func (h *DraftsHandler) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := bffmiddleware.UserIDFromContext(r.Context()); !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func (h *DraftsHandler) resolveAccount(w http.ResponseWriter, r *http.Request, op string) (int64, bool, bool) {
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return 0, false, false
	}
	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[DraftsHandler.%s] GetAccountIDByUserID userID=%d: %v", op, userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return 0, false, false
	}
	return accountID, found, true
}

func draftSessionsToResponse(rows []repository.DraftSessionDetailRow) []draftSessionResponse {
	out := make([]draftSessionResponse, 0, len(rows))
	for _, s := range rows {
		out = append(out, draftSessionRowToResponse(s))
	}
	return out
}

func draftSessionRowToResponse(s repository.DraftSessionDetailRow) draftSessionResponse {
	return draftSessionResponse{
		ID: s.ID, EventName: s.EventName, SetCode: s.SetCode, DraftType: s.DraftType,
		StartTime: s.StartTime, EndTime: s.EndTime, Status: s.Status,
		TotalPicks: s.TotalPicks, OverallGrade: s.OverallGrade, OverallScore: s.OverallScore,
		PickQualityScore: s.PickQualityScore, ColorDisciplineScore: s.ColorDisciplineScore,
		DeckCompositionScore: s.DeckCompositionScore, StrategicScore: s.StrategicScore,
		PredictedWinRate: s.PredictedWinRate, PredictedWinRateMin: s.PredictedWinRateMin,
		PredictedWinRateMax: s.PredictedWinRateMax, PredictionFactors: s.PredictionFactors,
		PredictedAt: s.PredictedAt,
		CreatedAt:   s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

func draftPickRowsToResponse(rows []repository.DraftPickRow) []draftPickResponse {
	out := make([]draftPickResponse, 0, len(rows))
	for _, p := range rows {
		out = append(out, draftPickResponse{
			ID: p.ID, SessionID: p.SessionID, PackNumber: p.PackNumber, PickNumber: p.PickNumber,
			CardID: p.CardID, Timestamp: p.Timestamp,
			PickQualityGrade: p.PickQualityGrade, PickQualityRank: p.PickQualityRank,
			PackBestGIHWR: p.PackBestGIHWR, PickedCardGIHWR: p.PickedCardGIHWR,
			AlternativesJSON: p.AlternativesJSON,
		})
	}
	return out
}

func communityComparisonRowToResponse(c repository.CommunityComparisonRow) communityComparisonResponse {
	resp := communityComparisonResponse{
		SetCode: c.SetCode, DraftFormat: c.DraftFormat,
		UserWinRate: c.UserWinRate, CommunityAvgWinRate: c.CommunityAvgWinRate,
		SampleSize:          c.SampleSize,
		CalculatedAt:        c.CalculatedAt.UTC().Format(time.RFC3339),
		ArchetypeComparison: []any{},
	}
	if c.PercentileRank != nil {
		resp.PercentileRank = *c.PercentileRank
	}
	return resp
}

func trendRowsToResponse(rows []repository.TemporalTrendRow) []trendEntryResponse {
	out := make([]trendEntryResponse, 0, len(rows))
	for _, t := range rows {
		entry := trendEntryResponse{
			PeriodType:  t.PeriodType,
			PeriodStart: t.PeriodStart.UTC().Format("2006-01-02"),
			PeriodEnd:   t.PeriodEnd.UTC().Format("2006-01-02"),
			SetCode:     t.SetCode,
			DraftsCount: t.DraftsCount, MatchesPlayed: t.MatchesPlayed, MatchesWon: t.MatchesWon,
		}
		if t.MatchesPlayed > 0 {
			entry.WinRate = float64(t.MatchesWon) / float64(t.MatchesPlayed)
		}
		if t.AvgDraftGrade != nil {
			entry.AvgDraftGrade = *t.AvgDraftGrade
		}
		out = append(out, entry)
	}
	return out
}

// buildSeventeenLandsPicks shapes draft_picks rows into the 17lands
// pick array. The SPA's SeventeenLandsPickData contract requires `pick`
// and `pack[]` as numbers (arena card ids), so we parse the schema's
// TEXT-typed card_id + alternatives_json into ints. Bad rows fall back
// to a single-element pack containing the picked id (or empty when
// even the picked id won't parse).
func buildSeventeenLandsPicks(picks []repository.DraftPickRow) []map[string]any {
	out := make([]map[string]any, 0, len(picks))
	for _, p := range picks {
		pick, _ := strconv.Atoi(strings.TrimSpace(p.CardID))
		entry := map[string]any{
			"pack_number": p.PackNumber,
			"pick_number": p.PickNumber,
			"pick":        pick,
			"pick_time":   p.Timestamp.UTC().Format(time.RFC3339),
		}
		var pack []int
		if p.AlternativesJSON != nil && strings.TrimSpace(*p.AlternativesJSON) != "" {
			pack = parseIntArray(*p.AlternativesJSON)
		}
		if len(pack) == 0 {
			if pick > 0 {
				pack = []int{pick}
			} else {
				pack = []int{}
			}
		}
		entry["pack"] = pack
		out = append(out, entry)
	}
	return out
}

// parseIntArray decodes a JSON array stored as TEXT into a slice of ints.
// Accepts both ["123","456"] (the daemon's existing alternatives_json
// format) and [123,456] for forward compatibility. Returns nil on parse
// failure so the caller can fall back to the picked card id.
func parseIntArray(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	// Try numeric array first.
	var nums []int
	if err := json.Unmarshal([]byte(raw), &nums); err == nil {
		return nums
	}
	// Fall back to string array, parsing each element.
	var strs []string
	if err := json.Unmarshal([]byte(raw), &strs); err != nil {
		return nil
	}
	out := make([]int, 0, len(strs))
	for _, s := range strs {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// normalizePeriodType folds the SPA's "week"/"month" payload (and legacy
// "weekly"/"monthly" variants) down to the SQL names accepted by
// repository.TemporalTrends ("week" or "month"). Returns an empty string
// for unrecognised values so the caller can return a 400 to the client.
func normalizePeriodType(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	// Accept both canonical ("week", "month") and legacy long-form values.
	switch v {
	case "week", "weekly":
		return "week"
	case "month", "monthly":
		return "month"
	default:
		return ""
	}
}

// draftGradeStub returns a zero-confidence DraftGrade placeholder.
func draftGradeStub() map[string]any {
	return map[string]any{
		"overallGrade": "Unknown", "overallScore": 0,
		"pickQualityScore": 0.0, "colorDisciplineScore": 0.0,
		"deckCompositionScore": 0.0, "strategicScore": 0.0,
		"calculatedAt": time.Now().UTC().Format(time.RFC3339),
	}
}
