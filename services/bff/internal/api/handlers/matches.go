// Phase 2 PR #1 — /api/v1/matches handlers.
//
// Replaces the legacy daemonClient /matches surface with proper cloud-data
// endpoints under /api/v1/matches/*. All responses use camelCase JSON keys
// per the Phase 2 architecture lock-in
// (docs/product/milestones/v0.3.1/daemon-local-api-phase2-audit.md).
//
// Auth: every route is guarded by DaemonAPIKeyAuth (Bearer = daemon api_key
// from the OS keychain), which resolves to the int64 users.id on context.
// Match rows are scoped to that user's accounts.

package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// matchesListReader is the minimal repo interface the handler needs. Defined
// here so tests can stub it without pulling in the SQL repo. Phase 2 PR #1
// expansion added Games / AggregateStats / Trends / FormatDistribution /
// PerformanceByHour / MatchupMatrix / DistinctArchetypes / RankProgression /
// RankProgressionTimeline / ExportAll alongside the original three methods.
//
// #2031: ListByAccountIDFiltered (offset) replaced by
// ListByAccountIDCursorFiltered (keyset) — no OFFSET, no SELECT COUNT(*).
type matchesListReader interface {
	ListByAccountIDCursorFiltered(ctx context.Context, accountID int64, filter repository.MatchFilter, cursorTS *time.Time, cursorID string, limit int) ([]repository.MatchRow, error)
	GetByID(ctx context.Context, accountID int64, matchID string) (*repository.MatchRow, error)
	DistinctFormats(ctx context.Context, accountID int64) ([]string, error)
	GamesByMatchID(ctx context.Context, accountID int64, matchID string) ([]repository.GameRow, error)
	AggregateStats(ctx context.Context, accountID int64, f repository.MatchFilter) (repository.StatsAggregate, error)
	FormatDistribution(ctx context.Context, accountID int64, f repository.MatchFilter) ([]repository.FormatStatsRow, error)
	PerformanceByHour(ctx context.Context, accountID int64, f repository.MatchFilter) ([]repository.HourBucket, error)
	MatchupMatrix(ctx context.Context, accountID int64, f repository.MatchFilter) ([]repository.MatchupRow, error)
	DistinctArchetypes(ctx context.Context, accountID int64) ([]string, error)
	Trends(ctx context.Context, accountID int64, period string, f repository.MatchFilter) ([]repository.TrendBucket, error)
	LatestRankInFormat(ctx context.Context, accountID int64, format string) (*repository.RankSnapshot, error)
	RankTimelineForFormat(ctx context.Context, accountID int64, format string, startDate, endDate time.Time) ([]repository.RankTimelineRow, error)
	ExportAll(ctx context.Context, accountID int64) ([]repository.ExportRow, error)
}

// MatchesHandler serves the cloud-data Phase 2 matches API. It depends on a
// matches list reader (for filtering + pagination + lookups) and an account
// lookup that resolves users.id → accounts.id.
type MatchesHandler struct {
	matches  matchesListReader
	accounts AccountLookup
}

// NewMatchesHandler returns a handler wired with the provided reader + account lookup.
func NewMatchesHandler(matches matchesListReader, accounts AccountLookup) *MatchesHandler {
	return &MatchesHandler{matches: matches, accounts: accounts}
}

// matchListItem is a single match in the list response. PascalCase keys to
// match the existing models.Match TS class (Wails-era) so SPA components can
// keep reading match.Format / match.PlayerWins / etc. unchanged. The whole
// SPA type tree will be regenerated as part of a later cleanup PR; until
// then, the wire format honors what consumers actually parse.
type matchListItem struct {
	ID              string    `json:"ID"`
	Format          string    `json:"Format"`
	Result          string    `json:"Result"`
	Timestamp       time.Time `json:"Timestamp"`
	DurationSeconds *int      `json:"DurationSeconds,omitempty"`
	DeckID          *string   `json:"DeckID,omitempty"`
	RankBefore      *string   `json:"RankBefore,omitempty"`
	RankAfter       *string   `json:"RankAfter,omitempty"`
	PlayerWins      int       `json:"PlayerWins"`
	OpponentWins    int       `json:"OpponentWins"`
}

// matchListResponse wraps a page of matches using keyset (cursor) pagination.
// HasMore signals whether a next page exists (limit+1 probe pattern).
// NextCursorTS and NextCursorID are opaque tokens the caller passes back on
// the next request; when HasMore is false they are empty strings.
//
// The legacy Total/Page/Limit fields are omitted — new callers must use
// HasMore. The SPA's existing getMatches() only reads Matches so removing
// Total/Page/Limit is wire-compatible with the current frontend.
type matchListResponse struct {
	Matches      []matchListItem `json:"Matches"`
	HasMore      bool            `json:"HasMore"`
	NextCursorTS string          `json:"NextCursorTS,omitempty"`
	NextCursorID string          `json:"NextCursorID,omitempty"`
	Limit        int             `json:"Limit"`
}

// matchesListFilterRequest is the JSON body the SPA's getMatches() posts. All
// fields are optional; the handler treats missing fields as "no filter on that
// dimension". Mirrors StatsFilterRequest in frontend/src/services/api/matches.ts.
//
// CursorTS and CursorID replace the old Page field. On the first request both
// are empty; subsequent requests echo back the NextCursorTS/NextCursorID from
// the previous response.
type matchesListFilterRequest struct {
	StartDate string   `json:"startDate,omitempty"`
	EndDate   string   `json:"endDate,omitempty"`
	Format    string   `json:"format,omitempty"`
	Formats   []string `json:"formats,omitempty"`
	DeckID    string   `json:"deckId,omitempty"`
	Result    string   `json:"result,omitempty"`
	// CursorTS and CursorID are keyset pagination tokens (RFC3339 timestamp +
	// match ID). Both must be supplied together; omitting both means first page.
	CursorTS string `json:"cursorTS,omitempty"`
	CursorID string `json:"cursorID,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	// Page is accepted but ignored — use CursorTS/CursorID instead.
	// Retained for a brief transitional period so existing SPA builds do not
	// receive a decode error on the ignored field.
	Page int `json:"page,omitempty"`
}

// List handles POST /api/v1/matches. Returns a keyset-paginated, filtered list
// of matches for the authenticated user. Uses POST (not GET) to match the
// SPA's existing call shape — bodies are easier than serialising filter[] params.
//
// Pagination: pass CursorTS + CursorID from the previous response's
// NextCursorTS/NextCursorID. Both must be present together. Omitting both
// returns the first page. The response includes HasMore to signal whether
// another page exists.
func (h *MatchesHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req matchesListFilterRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	filter, err := buildMatchFilter(req, 1, limit)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Clear the offset-era Page/Limit so analytics methods (AggregateStats etc.)
	// that reuse MatchFilter are not confused.
	filter.Page = 0
	filter.Limit = 0

	// Parse the keyset cursor when supplied. Both fields must be present.
	var cursorTS *time.Time
	cursorID := ""
	if req.CursorTS != "" || req.CursorID != "" {
		if req.CursorTS == "" || req.CursorID == "" {
			writeJSONError(w, "cursorTS and cursorID must both be provided together", http.StatusBadRequest)
			return
		}
		t, parseErr := time.Parse(time.RFC3339Nano, req.CursorTS)
		if parseErr != nil {
			writeJSONError(w, "invalid cursorTS: must be RFC3339 timestamp", http.StatusBadRequest)
			return
		}
		cursorTS = &t
		cursorID = req.CursorID
	}

	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[MatchesHandler.List] GetAccountIDByUserID userID=%d: %v", userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !found {
		// No account row yet — return an empty page rather than 404.
		writeMatchesJSON(w, matchListResponse{Matches: []matchListItem{}, HasMore: false, Limit: limit})
		return
	}

	// Fetch limit+1 rows; the extra row is the has_more probe.
	rows, err := h.matches.ListByAccountIDCursorFiltered(r.Context(), accountID, filter, cursorTS, cursorID, limit)
	if err != nil {
		log.Printf("[MatchesHandler.List] ListByAccountIDCursorFiltered accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit] // trim the probe row
	}

	items := make([]matchListItem, 0, len(rows))
	for _, m := range rows {
		items = append(items, matchRowToListItem(m))
	}

	resp := matchListResponse{
		Matches: items,
		HasMore: hasMore,
		Limit:   limit,
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		resp.NextCursorTS = last.Timestamp.UTC().Format(time.RFC3339Nano)
		resp.NextCursorID = last.ID
	}

	writeMatchesJSON(w, resp)
}

// Get handles GET /api/v1/matches/{matchId}. Returns a single match scoped to
// the authenticated user. 404 when the match exists but belongs to another user
// (we don't leak that distinction; 404 covers both "not found" and "not yours").
func (h *MatchesHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	matchID := strings.TrimSpace(chi.URLParam(r, "matchId"))
	if matchID == "" {
		writeJSONError(w, "matchId is required", http.StatusBadRequest)
		return
	}

	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[MatchesHandler.Get] GetAccountIDByUserID userID=%d: %v", userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !found {
		writeJSONError(w, "match not found", http.StatusNotFound)
		return
	}

	row, err := h.matches.GetByID(r.Context(), accountID, matchID)
	if err != nil {
		log.Printf("[MatchesHandler.Get] GetByID accountID=%d matchID=%s: %v", accountID, matchID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if row == nil {
		writeJSONError(w, "match not found", http.StatusNotFound)
		return
	}
	writeMatchesJSON(w, matchRowToListItem(*row))
}

// Formats handles GET /api/v1/matches/formats. Returns the distinct formats
// the user has match data for. Used by the SPA's format-filter dropdown.
func (h *MatchesHandler) Formats(w http.ResponseWriter, r *http.Request) {
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[MatchesHandler.Formats] GetAccountIDByUserID userID=%d: %v", userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !found {
		writeMatchesJSON(w, []string{})
		return
	}
	formats, err := h.matches.DistinctFormats(r.Context(), accountID)
	if err != nil {
		log.Printf("[MatchesHandler.Formats] DistinctFormats accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if formats == nil {
		formats = []string{}
	}
	writeMatchesJSON(w, formats)
}

// buildMatchFilter validates the incoming request and shapes it into the repo
// filter struct. Validation errors get propagated to the handler as 400s.
// The page and limit args are retained for call-sites that still use the
// offset-based ListByAccountID path (history handler); cursor-based callers
// should clear filter.Page and filter.Limit after calling this function.
func buildMatchFilter(req matchesListFilterRequest, page, limit int) (repository.MatchFilter, error) {
	f := repository.MatchFilter{
		Page:    page,
		Limit:   limit,
		Format:  strings.TrimSpace(req.Format),
		Formats: dedupeNonEmpty(req.Formats),
		DeckID:  strings.TrimSpace(req.DeckID),
		Result:  strings.TrimSpace(req.Result),
	}
	if req.StartDate != "" {
		t, err := parseFilterDate(req.StartDate)
		if err != nil {
			return f, err
		}
		f.StartDate = &t
	}
	if req.EndDate != "" {
		t, err := parseFilterDate(req.EndDate)
		if err != nil {
			return f, err
		}
		f.EndDate = &t
	}
	if f.Result != "" {
		switch strings.ToLower(f.Result) {
		case "win", "loss", "draw":
		default:
			return f, &fieldError{"result must be win|loss|draw"}
		}
	}
	return f, nil
}

// parseFilterDate accepts either an RFC3339 timestamp or a YYYY-MM-DD date.
// The SPA's matches.ts formatDateParam helper emits YYYY-MM-DD; older callers
// may send full ISO strings.
func parseFilterDate(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, &fieldError{"invalid date format (want RFC3339 or YYYY-MM-DD): " + s}
}

// dedupeNonEmpty returns a copy of in with empty strings dropped and
// duplicates removed (case-insensitive). Order is preserved.
func dedupeNonEmpty(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		k := strings.ToLower(v)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, v)
	}
	return out
}

// matchRowToListItem converts a repository row into the API DTO. Keeps the
// field mapping in one place so callers (List, Get) stay terse.
func matchRowToListItem(m repository.MatchRow) matchListItem {
	return matchListItem{
		ID:              m.ID,
		Format:          m.Format,
		Result:          m.Result,
		Timestamp:       m.Timestamp,
		DurationSeconds: m.DurationSeconds,
		DeckID:          m.DeckID,
		RankBefore:      m.RankBefore,
		RankAfter:       m.RankAfter,
		PlayerWins:      m.PlayerWins,
		OpponentWins:    m.OpponentWins,
	}
}

// fieldError is a small typed error used for 400-class request validation.
type fieldError struct{ msg string }

func (e *fieldError) Error() string { return e.msg }

// writeMatchesJSON serialises payload as a {"data": ...} envelope so the
// SPA's apiClient (which reads response.json().data) gets a usable result.
// Centralised so we don't repeat the Content-Type / envelope dance.
func writeMatchesJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": payload})
}

// parseLimitDefault returns the integer value of the named query param, or
// fallback when the param is missing or invalid. Used by handlers that take
// pagination via query string rather than body.
func parseLimitDefault(r *http.Request, name string, fallback int) int {
	v := r.URL.Query().Get(name)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// ─── Phase 2 PR #1 expansion: analytics, comparison, games, export ───────────
//
// Wire shape for the new endpoints follows the existing matches.ts contract
// (PascalCase keys via the response struct json tags, wrapped by
// writeMatchesJSON in a {"data": ...} envelope). The SPA's matches.ts adapter
// layer maps these into models.Match / models.Statistics / etc.

// statsResponse mirrors models.Statistics on the SPA. Win-rate is computed
// server-side so the SPA can render it directly.
type statsResponse struct {
	TotalMatches int     `json:"TotalMatches"`
	MatchesWon   int     `json:"MatchesWon"`
	MatchesLost  int     `json:"MatchesLost"`
	TotalGames   int     `json:"TotalGames"`
	GamesWon     int     `json:"GamesWon"`
	GamesLost    int     `json:"GamesLost"`
	WinRate      float64 `json:"WinRate"`
	GameWinRate  float64 `json:"GameWinRate"`
}

func toStatsResponse(s repository.StatsAggregate) statsResponse {
	out := statsResponse{
		TotalMatches: s.TotalMatches,
		MatchesWon:   s.MatchesWon,
		MatchesLost:  s.MatchesLost,
		TotalGames:   s.TotalGames,
		GamesWon:     s.GamesWon,
		GamesLost:    s.GamesLost,
	}
	if s.TotalMatches > 0 {
		out.WinRate = float64(s.MatchesWon) / float64(s.TotalMatches)
	}
	if s.TotalGames > 0 {
		out.GameWinRate = float64(s.GamesWon) / float64(s.TotalGames)
	}
	return out
}

// gameResponse mirrors models.Game on the SPA. Field tags use PascalCase to
// match the existing TS class deserialisation.
type gameResponse struct {
	ID              int64     `json:"ID"`
	MatchID         string    `json:"MatchID"`
	GameNumber      int       `json:"GameNumber"`
	Result          string    `json:"Result"`
	DurationSeconds *int      `json:"DurationSeconds,omitempty"`
	ResultReason    *string   `json:"ResultReason,omitempty"`
	CreatedAt       time.Time `json:"CreatedAt"`
}

// performanceMetricsResponse mirrors models.PerformanceMetrics. Only the
// duration aggregates are meaningful here; per-game timings are omitted
// because the games table does not always carry duration_seconds.
type performanceMetricsResponse struct {
	AvgMatchDuration *float64 `json:"AvgMatchDuration,omitempty"`
	AvgGameDuration  *float64 `json:"AvgGameDuration,omitempty"`
	FastestMatch     *int     `json:"FastestMatch,omitempty"`
	SlowestMatch     *int     `json:"SlowestMatch,omitempty"`
	FastestGame      *int     `json:"FastestGame,omitempty"`
	SlowestGame      *int     `json:"SlowestGame,omitempty"`
}

// trendPeriodResponse mirrors models.TrendPeriod. The label is derived
// server-side using the bucket's date+period.
type trendPeriodResponse struct {
	StartDate time.Time `json:"StartDate"`
	EndDate   time.Time `json:"EndDate"`
	Label     string    `json:"Label"`
}

// trendDataResponse mirrors models.TrendData.
type trendDataResponse struct {
	Period      trendPeriodResponse `json:"Period"`
	Stats       statsResponse       `json:"Stats"`
	WinRate     float64             `json:"WinRate"`
	GameWinRate float64             `json:"GameWinRate"`
}

// trendAnalysisResponse wraps a series of trend buckets. Matches the loose
// `unknown` typing on the SPA — extra fields can be added without breaking
// callers.
type trendAnalysisResponse struct {
	StartDate  time.Time           `json:"StartDate"`
	EndDate    time.Time           `json:"EndDate"`
	PeriodType string              `json:"PeriodType"`
	Trends     []trendDataResponse `json:"Trends"`
}

// rankProgressionResponse mirrors models.RankProgression on the SPA. The
// next-rank / steps fields are computed server-side from the latest
// rank_after value. When we don't have rank progression data for a format
// the handler returns 200 with zero-valued fields so the SPA can render an
// empty state without error handling.
type matchesRankProgressionResponse struct {
	CurrentRank      string    `json:"CurrentRank"`
	NextRank         string    `json:"NextRank"`
	CurrentStep      int       `json:"CurrentStep"`
	StepsToNext      int       `json:"StepsToNext"`
	IsAtFloor        bool      `json:"IsAtFloor"`
	EstimatedMatches *int      `json:"EstimatedMatches,omitempty"`
	WinRateUsed      *float64  `json:"WinRateUsed,omitempty"`
	Format           string    `json:"Format"`
	LastUpdated      time.Time `json:"LastUpdated"`
}

// rankTimelineEntryResponse is a single point in the rank-progression
// timeline. Snake-case field tags match the storage.RankTimelineEntry
// shape on the SPA.
type rankTimelineEntryResponse struct {
	OccurredAt time.Time `json:"occurred_at"`
	Rank       string    `json:"rank"`
	Result     string    `json:"result"`
	MatchID    string    `json:"match_id"`
}

// rankTimelineResponse mirrors storage.RankTimeline (snake_case namespace).
type rankTimelineResponse struct {
	Format         string                      `json:"format"`
	StartDate      time.Time                   `json:"start_date"`
	EndDate        time.Time                   `json:"end_date"`
	Entries        []rankTimelineEntryResponse `json:"entries"`
	TotalChanges   int                         `json:"total_changes"`
	Milestones     int                         `json:"milestones"`
	StartRank      string                      `json:"start_rank"`
	EndRank        string                      `json:"end_rank"`
	HighestRank    string                      `json:"highest_rank"`
	LowestRank     string                      `json:"lowest_rank"`
	SeasonsCovered []int                       `json:"seasons_covered"`
}

// comparisonGroupResponse mirrors ComparisonGroup on the SPA.
type comparisonGroupResponse struct {
	Label      string                   `json:"Label"`
	Filter     matchesListFilterRequest `json:"Filter"`
	Statistics statsResponse            `json:"Statistics"`
	MatchCount int                      `json:"MatchCount"`
}

// comparisonResultResponse mirrors ComparisonResult on the SPA. Best/worst
// are pointers so we can omit them when the comparison has < 2 groups.
type comparisonResultResponse struct {
	Groups         []comparisonGroupResponse `json:"Groups"`
	BestGroup      *comparisonGroupResponse  `json:"BestGroup"`
	WorstGroup     *comparisonGroupResponse  `json:"WorstGroup"`
	WinRateDiff    float64                   `json:"WinRateDiff"`
	TotalMatches   int                       `json:"TotalMatches"`
	ComparisonDate string                    `json:"ComparisonDate"`
}

// matchesFormatDistributionResponse is the {format: Statistics} map the SPA
// expects. Named with the matches prefix to avoid colliding with the existing
// formatDistributionResponse used by /api/v1/stats/format-distribution
// (different shape, different feature area).
type matchesFormatDistributionResponse map[string]statsResponse

// matchupMatrixResponse is the {opponentLabel: Statistics} map the SPA expects.
type matchupMatrixResponse map[string]statsResponse

// trendAnalysisRequest is the JSON body for /matches/trends. Mirrors the
// SPA's TrendAnalysisRequest. periodType is required (day|week|month).
type trendAnalysisRequest struct {
	StartDate  string   `json:"startDate"`
	EndDate    string   `json:"endDate"`
	PeriodType string   `json:"periodType"`
	Formats    []string `json:"formats,omitempty"`
}

// timePeriodRequest is one labeled time window inside a CompareTimePeriods
// request body.
type timePeriodRequest struct {
	Label     string `json:"label"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// compareMatchesRequest is the JSON body for /matches/compare. Each group
// gets its own filter; the handler runs one stats query per group.
type compareMatchesRequest struct {
	Groups []struct {
		Label  string                   `json:"label"`
		Filter matchesListFilterRequest `json:"filter"`
	} `json:"groups"`
}

type compareFormatsRequest struct {
	Formats    []string                 `json:"formats"`
	BaseFilter matchesListFilterRequest `json:"baseFilter"`
}

type compareDecksRequest struct {
	DeckIDs    []string                 `json:"deckIDs"`
	BaseFilter matchesListFilterRequest `json:"baseFilter"`
}

type compareTimePeriodsRequest struct {
	Periods    []timePeriodRequest      `json:"periods"`
	BaseFilter matchesListFilterRequest `json:"baseFilter"`
}

// ─── Games ───────────────────────────────────────────────────────────────────

// Games handles GET /api/v1/matches/{matchId}/games. Returns the games
// belonging to the match, scoped to the authenticated user.
func (h *MatchesHandler) Games(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Games")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, []gameResponse{})
		return
	}
	matchID := strings.TrimSpace(chi.URLParam(r, "matchId"))
	if matchID == "" {
		writeJSONError(w, "matchId is required", http.StatusBadRequest)
		return
	}
	rows, err := h.matches.GamesByMatchID(r.Context(), accountID, matchID)
	if err != nil {
		log.Printf("[MatchesHandler.Games] GamesByMatchID accountID=%d matchID=%s: %v", accountID, matchID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	out := make([]gameResponse, 0, len(rows))
	for _, g := range rows {
		out = append(out, gameResponse{
			ID:              g.ID,
			MatchID:         g.MatchID,
			GameNumber:      g.GameNumber,
			Result:          g.Result,
			DurationSeconds: g.DurationSeconds,
			ResultReason:    g.ResultReason,
			CreatedAt:       g.CreatedAt,
		})
	}
	writeMatchesJSON(w, out)
}

// ─── Stats / Trends / Distribution / Performance / Matchup / Archetypes ─────

// Stats handles POST /api/v1/matches/stats. Returns aggregated win/loss
// counts and rates for the filter.
func (h *MatchesHandler) Stats(w http.ResponseWriter, r *http.Request) {
	accountID, filter, found, ok := h.requireFilter(w, r, "Stats")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, statsResponse{})
		return
	}
	agg, err := h.matches.AggregateStats(r.Context(), accountID, filter)
	if err != nil {
		log.Printf("[MatchesHandler.Stats] AggregateStats accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, toStatsResponse(agg))
}

// Trends handles POST /api/v1/matches/trends. Returns time-bucketed stats.
func (h *MatchesHandler) Trends(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Trends")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, trendAnalysisResponse{Trends: []trendDataResponse{}})
		return
	}
	var req trendAnalysisRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	period := strings.ToLower(strings.TrimSpace(req.PeriodType))
	if period == "" {
		period = "week"
	}
	if period != "day" && period != "week" && period != "month" {
		writeJSONError(w, "periodType must be day|week|month", http.StatusBadRequest)
		return
	}
	startDate, endDate, err := parseTrendWindow(req.StartDate, req.EndDate)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	filter := repository.MatchFilter{
		StartDate: &startDate,
		EndDate:   &endDate,
		Formats:   dedupeNonEmpty(req.Formats),
	}
	buckets, err := h.matches.Trends(r.Context(), accountID, period, filter)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidTrendPeriod) {
			writeJSONError(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("[MatchesHandler.Trends] Trends accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	resp := trendAnalysisResponse{
		StartDate:  startDate,
		EndDate:    endDate,
		PeriodType: period,
		Trends:     make([]trendDataResponse, 0, len(buckets)),
	}
	for _, b := range buckets {
		stats := toStatsResponse(b.Stats)
		resp.Trends = append(resp.Trends, trendDataResponse{
			Period: trendPeriodResponse{
				StartDate: b.BucketStart,
				EndDate:   bucketEnd(b.BucketStart, period),
				Label:     bucketLabel(b.BucketStart, period),
			},
			Stats:       stats,
			WinRate:     stats.WinRate,
			GameWinRate: stats.GameWinRate,
		})
	}
	writeMatchesJSON(w, resp)
}

// Archetypes handles GET /api/v1/matches/archetypes. Returns the distinct
// opponent_name values the user has played against (used as the SPA's
// archetype filter dropdown source).
func (h *MatchesHandler) Archetypes(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Archetypes")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, []string{})
		return
	}
	out, err := h.matches.DistinctArchetypes(r.Context(), accountID)
	if err != nil {
		log.Printf("[MatchesHandler.Archetypes] DistinctArchetypes accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if out == nil {
		out = []string{}
	}
	writeMatchesJSON(w, out)
}

// FormatDistribution handles POST /api/v1/matches/format-distribution.
func (h *MatchesHandler) FormatDistribution(w http.ResponseWriter, r *http.Request) {
	accountID, filter, found, ok := h.requireFilter(w, r, "FormatDistribution")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, matchesFormatDistributionResponse{})
		return
	}
	rows, err := h.matches.FormatDistribution(r.Context(), accountID, filter)
	if err != nil {
		log.Printf("[MatchesHandler.FormatDistribution] FormatDistribution accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	out := make(matchesFormatDistributionResponse, len(rows))
	for _, fr := range rows {
		out[fr.Format] = toStatsResponse(fr.Stats)
	}
	writeMatchesJSON(w, out)
}

// PerformanceByHour handles POST /api/v1/matches/performance-by-hour. The SPA
// surfaces a single PerformanceMetrics shape; we synthesise it from the per-
// hour aggregate (overall avg / fastest / slowest across the filtered
// window).
func (h *MatchesHandler) PerformanceByHour(w http.ResponseWriter, r *http.Request) {
	accountID, filter, found, ok := h.requireFilter(w, r, "PerformanceByHour")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, performanceMetricsResponse{})
		return
	}
	buckets, err := h.matches.PerformanceByHour(r.Context(), accountID, filter)
	if err != nil {
		log.Printf("[MatchesHandler.PerformanceByHour] PerformanceByHour accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	resp := performanceMetricsResponse{}
	if len(buckets) > 0 {
		var sumAvg float64
		var sumCount int
		var fastest, slowest *int
		for _, b := range buckets {
			if b.AvgMatchDurationSecs != nil {
				sumAvg += *b.AvgMatchDurationSecs * float64(b.MatchCount)
				sumCount += b.MatchCount
			}
			if b.FastestMatchSecs != nil && (fastest == nil || *b.FastestMatchSecs < *fastest) {
				fastest = b.FastestMatchSecs
			}
			if b.SlowestMatchSecs != nil && (slowest == nil || *b.SlowestMatchSecs > *slowest) {
				slowest = b.SlowestMatchSecs
			}
		}
		if sumCount > 0 {
			avg := sumAvg / float64(sumCount)
			resp.AvgMatchDuration = &avg
		}
		resp.FastestMatch = fastest
		resp.SlowestMatch = slowest
	}
	writeMatchesJSON(w, resp)
}

// MatchupMatrix handles POST /api/v1/matches/matchup-matrix.
func (h *MatchesHandler) MatchupMatrix(w http.ResponseWriter, r *http.Request) {
	accountID, filter, found, ok := h.requireFilter(w, r, "MatchupMatrix")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, matchupMatrixResponse{})
		return
	}
	rows, err := h.matches.MatchupMatrix(r.Context(), accountID, filter)
	if err != nil {
		log.Printf("[MatchesHandler.MatchupMatrix] MatchupMatrix accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	out := make(matchupMatrixResponse, len(rows))
	for _, m := range rows {
		out[m.OpponentLabel] = toStatsResponse(m.Stats)
	}
	writeMatchesJSON(w, out)
}

// ─── Rank progression ────────────────────────────────────────────────────────

// RankProgression handles GET /api/v1/matches/rank-progression/{format}.
// Returns a current-rank summary derived from the most recent ranked match
// in that format for the user. When no ranked match exists, returns 200
// with zero-valued fields so the SPA renders an empty state.
func (h *MatchesHandler) RankProgression(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "RankProgression")
	if !ok {
		return
	}
	format := strings.TrimSpace(chi.URLParam(r, "format"))
	if format == "" {
		writeJSONError(w, "format is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, matchesRankProgressionResponse{Format: format})
		return
	}
	snap, err := h.matches.LatestRankInFormat(r.Context(), accountID, format)
	if err != nil {
		log.Printf("[MatchesHandler.RankProgression] LatestRankInFormat accountID=%d format=%s: %v", accountID, format, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	resp := matchesRankProgressionResponse{Format: format}
	if snap != nil {
		resp.CurrentRank = snap.RankAfter
		resp.LastUpdated = snap.OccurredAt
	}
	writeMatchesJSON(w, resp)
}

// RankProgressionTimeline handles GET /api/v1/matches/rank-progression-timeline.
// Query: format, start_date, end_date, period.
func (h *MatchesHandler) RankProgressionTimeline(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "RankProgressionTimeline")
	if !ok {
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	if format == "" {
		writeJSONError(w, "format is required", http.StatusBadRequest)
		return
	}
	startStr := r.URL.Query().Get("start_date")
	endStr := r.URL.Query().Get("end_date")
	startDate, endDate, err := parseTrendWindow(startStr, endStr)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, rankTimelineResponse{
			Format: format, StartDate: startDate, EndDate: endDate,
			Entries: []rankTimelineEntryResponse{},
		})
		return
	}
	rows, err := h.matches.RankTimelineForFormat(r.Context(), accountID, format, startDate, endDate)
	if err != nil {
		log.Printf("[MatchesHandler.RankProgressionTimeline] RankTimelineForFormat accountID=%d format=%s: %v", accountID, format, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	resp := rankTimelineResponse{
		Format:    format,
		StartDate: startDate,
		EndDate:   endDate,
		Entries:   make([]rankTimelineEntryResponse, 0, len(rows)),
	}
	if len(rows) > 0 {
		resp.StartRank = derefOr(rows[0].RankBefore, derefOr(rows[0].RankAfter, ""))
		resp.EndRank = derefOr(rows[len(rows)-1].RankAfter, derefOr(rows[len(rows)-1].RankBefore, ""))
	}
	for _, rt := range rows {
		entry := rankTimelineEntryResponse{
			OccurredAt: rt.OccurredAt,
			Rank:       derefOr(rt.RankAfter, derefOr(rt.RankBefore, "")),
			Result:     rt.Result,
			MatchID:    rt.MatchID,
		}
		if rt.RankBefore != nil && rt.RankAfter != nil && *rt.RankBefore != *rt.RankAfter {
			resp.TotalChanges++
		}
		if entry.Rank != "" && (resp.HighestRank == "" || entry.Rank > resp.HighestRank) {
			resp.HighestRank = entry.Rank
		}
		if entry.Rank != "" && (resp.LowestRank == "" || entry.Rank < resp.LowestRank) {
			resp.LowestRank = entry.Rank
		}
		resp.Entries = append(resp.Entries, entry)
	}
	writeMatchesJSON(w, resp)
}

// ─── Export ──────────────────────────────────────────────────────────────────

// Export handles GET /api/v1/matches/export?format=json|csv. Returns the
// user's full match history in the requested format. CSV exports stream a
// flat row-per-match shape; JSON wraps the same rows in the standard
// envelope.
func (h *MatchesHandler) Export(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Export")
	if !ok {
		return
	}
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "csv" {
		writeJSONError(w, "format must be json|csv", http.StatusBadRequest)
		return
	}
	if !found {
		// No account → empty CSV header / empty JSON array.
		if format == "csv" {
			w.Header().Set("Content-Type", "text/csv")
			writer := csv.NewWriter(w)
			_ = writer.Write([]string{
				"id", "format", "result", "result_reason", "timestamp",
				"duration_seconds", "deck_id", "rank_before", "rank_after",
				"opponent_name", "opponent_id", "player_wins", "opponent_wins", "event_name",
			})
			writer.Flush()
			return
		}
		writeMatchesJSON(w, []map[string]any{})
		return
	}
	rows, err := h.matches.ExportAll(r.Context(), accountID)
	if err != nil {
		log.Printf("[MatchesHandler.Export] ExportAll accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	timestamp := time.Now().UTC().Format("20060102-150405")
	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="matches-`+timestamp+`.csv"`)
		writer := csv.NewWriter(w)
		_ = writer.Write([]string{
			"id", "format", "result", "result_reason", "timestamp",
			"duration_seconds", "deck_id", "rank_before", "rank_after",
			"opponent_name", "opponent_id", "player_wins", "opponent_wins", "event_name",
		})
		for _, ex := range rows {
			_ = writer.Write([]string{
				ex.ID, ex.Format, ex.Result, derefOr(ex.ResultReason, ""),
				ex.Timestamp.Format(time.RFC3339), intPtrToString(ex.DurationSeconds),
				derefOr(ex.DeckID, ""), derefOr(ex.RankBefore, ""), derefOr(ex.RankAfter, ""),
				derefOr(ex.OpponentName, ""), derefOr(ex.OpponentID, ""),
				strconv.Itoa(ex.PlayerWins), strconv.Itoa(ex.OpponentWins), ex.EventName,
			})
		}
		writer.Flush()
		return
	}
	// JSON path
	w.Header().Set("Content-Disposition", `attachment; filename="matches-`+timestamp+`.json"`)
	out := make([]map[string]any, 0, len(rows))
	for _, ex := range rows {
		out = append(out, map[string]any{
			"id":              ex.ID,
			"format":          ex.Format,
			"result":          ex.Result,
			"resultReason":    derefOr(ex.ResultReason, ""),
			"timestamp":       ex.Timestamp,
			"durationSeconds": ex.DurationSeconds,
			"deckId":          derefOr(ex.DeckID, ""),
			"rankBefore":      derefOr(ex.RankBefore, ""),
			"rankAfter":       derefOr(ex.RankAfter, ""),
			"opponentName":    derefOr(ex.OpponentName, ""),
			"opponentId":      derefOr(ex.OpponentID, ""),
			"playerWins":      ex.PlayerWins,
			"opponentWins":    ex.OpponentWins,
			"eventName":       ex.EventName,
		})
	}
	writeMatchesJSON(w, out)
}

// ─── Compare ─────────────────────────────────────────────────────────────────

// Compare handles POST /api/v1/matches/compare. Runs one stats query per
// group, then derives best/worst/diff client-friendly fields.
func (h *MatchesHandler) Compare(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Compare")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, comparisonResultResponse{Groups: []comparisonGroupResponse{}, ComparisonDate: time.Now().UTC().Format(time.RFC3339)})
		return
	}
	var req compareMatchesRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Groups) == 0 {
		writeJSONError(w, "groups must not be empty", http.StatusBadRequest)
		return
	}
	groups := make([]comparisonGroupResponse, 0, len(req.Groups))
	for _, g := range req.Groups {
		group, err := h.runComparisonGroup(r.Context(), accountID, g.Label, g.Filter)
		if err != nil {
			writeJSONError(w, err.Error(), httpStatusForFilterErr(err))
			return
		}
		groups = append(groups, group)
	}
	writeMatchesJSON(w, buildComparisonResult(groups))
}

// CompareFormats handles POST /api/v1/matches/compare/formats. Each format
// becomes one group; baseFilter applies to all of them.
func (h *MatchesHandler) CompareFormats(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "CompareFormats")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, comparisonResultResponse{Groups: []comparisonGroupResponse{}, ComparisonDate: time.Now().UTC().Format(time.RFC3339)})
		return
	}
	var req compareFormatsRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Formats) == 0 {
		writeJSONError(w, "formats must not be empty", http.StatusBadRequest)
		return
	}
	groups := make([]comparisonGroupResponse, 0, len(req.Formats))
	for _, format := range req.Formats {
		filter := req.BaseFilter
		filter.Format = format
		group, err := h.runComparisonGroup(r.Context(), accountID, format, filter)
		if err != nil {
			writeJSONError(w, err.Error(), httpStatusForFilterErr(err))
			return
		}
		groups = append(groups, group)
	}
	writeMatchesJSON(w, buildComparisonResult(groups))
}

// CompareDecks handles POST /api/v1/matches/compare/decks. One group per deckID.
func (h *MatchesHandler) CompareDecks(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "CompareDecks")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, comparisonResultResponse{Groups: []comparisonGroupResponse{}, ComparisonDate: time.Now().UTC().Format(time.RFC3339)})
		return
	}
	var req compareDecksRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.DeckIDs) == 0 {
		writeJSONError(w, "deckIDs must not be empty", http.StatusBadRequest)
		return
	}
	groups := make([]comparisonGroupResponse, 0, len(req.DeckIDs))
	for _, deckID := range req.DeckIDs {
		filter := req.BaseFilter
		filter.DeckID = deckID
		group, err := h.runComparisonGroup(r.Context(), accountID, deckID, filter)
		if err != nil {
			writeJSONError(w, err.Error(), httpStatusForFilterErr(err))
			return
		}
		groups = append(groups, group)
	}
	writeMatchesJSON(w, buildComparisonResult(groups))
}

// CompareTimePeriods handles POST /api/v1/matches/compare/time-periods.
func (h *MatchesHandler) CompareTimePeriods(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "CompareTimePeriods")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, comparisonResultResponse{Groups: []comparisonGroupResponse{}, ComparisonDate: time.Now().UTC().Format(time.RFC3339)})
		return
	}
	var req compareTimePeriodsRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Periods) == 0 {
		writeJSONError(w, "periods must not be empty", http.StatusBadRequest)
		return
	}
	groups := make([]comparisonGroupResponse, 0, len(req.Periods))
	for _, p := range req.Periods {
		filter := req.BaseFilter
		filter.StartDate = p.StartDate
		filter.EndDate = p.EndDate
		group, err := h.runComparisonGroup(r.Context(), accountID, p.Label, filter)
		if err != nil {
			writeJSONError(w, err.Error(), httpStatusForFilterErr(err))
			return
		}
		groups = append(groups, group)
	}
	writeMatchesJSON(w, buildComparisonResult(groups))
}

// ─── shared handler helpers ─────────────────────────────────────────────────

// resolveAccount reads the user id from context, looks up the account id, and
// writes a 401 / 500 error when either step fails. Returns (accountID, found,
// ok). When ok is false the response has already been written. When ok is
// true and found is false the caller must write its own empty-shaped
// response (an empty Statistics, an empty map, etc.) rather than reusing a
// generic empty payload, since each endpoint has its own expected shape.
func (h *MatchesHandler) resolveAccount(w http.ResponseWriter, r *http.Request, op string) (int64, bool, bool) {
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return 0, false, false
	}
	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[MatchesHandler.%s] GetAccountIDByUserID userID=%d: %v", op, userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return 0, false, false
	}
	return accountID, found, true
}

// requireFilter is the common preamble for POST endpoints that take a
// matchesListFilterRequest body. Returns the resolved account id, the
// shaped MatchFilter, and a found flag indicating whether the user already
// has an accounts row. When ok is false the response was written; when
// found is false the caller is expected to emit its own empty-shaped
// response and return.
func (h *MatchesHandler) requireFilter(w http.ResponseWriter, r *http.Request, op string) (int64, repository.MatchFilter, bool, bool) {
	accountID, found, ok := h.resolveAccount(w, r, op)
	if !ok {
		return 0, repository.MatchFilter{}, false, false
	}
	var req matchesListFilterRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return 0, repository.MatchFilter{}, false, false
	}
	filter, err := buildMatchFilter(req, 1, 0)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return 0, repository.MatchFilter{}, false, false
	}
	// requireFilter callers don't paginate — clear the page/limit fields the
	// list builder set.
	filter.Page = 0
	filter.Limit = 0
	return accountID, filter, found, true
}

// runComparisonGroup runs one stats query for a comparison group and shapes
// the response.
func (h *MatchesHandler) runComparisonGroup(ctx context.Context, accountID int64, label string, req matchesListFilterRequest) (comparisonGroupResponse, error) {
	filter, err := buildMatchFilter(req, 1, 0)
	if err != nil {
		return comparisonGroupResponse{}, err
	}
	filter.Page = 0
	filter.Limit = 0
	agg, err := h.matches.AggregateStats(ctx, accountID, filter)
	if err != nil {
		log.Printf("[MatchesHandler.runComparisonGroup] AggregateStats accountID=%d label=%s: %v", accountID, label, err)
		return comparisonGroupResponse{}, fmt.Errorf("internal server error")
	}
	stats := toStatsResponse(agg)
	return comparisonGroupResponse{
		Label:      label,
		Filter:     req,
		Statistics: stats,
		MatchCount: agg.TotalMatches,
	}, nil
}

// buildComparisonResult derives best/worst/diff fields from a finished set
// of group results.
func buildComparisonResult(groups []comparisonGroupResponse) comparisonResultResponse {
	resp := comparisonResultResponse{
		Groups:         groups,
		ComparisonDate: time.Now().UTC().Format(time.RFC3339),
	}
	if len(groups) == 0 {
		return resp
	}
	best := groups[0]
	worst := groups[0]
	for _, g := range groups {
		resp.TotalMatches += g.MatchCount
		if g.Statistics.WinRate > best.Statistics.WinRate {
			best = g
		}
		if g.Statistics.WinRate < worst.Statistics.WinRate {
			worst = g
		}
	}
	resp.BestGroup = &best
	resp.WorstGroup = &worst
	resp.WinRateDiff = best.Statistics.WinRate - worst.Statistics.WinRate
	return resp
}

// httpStatusForFilterErr maps the typed validation error to 400, otherwise 500.
func httpStatusForFilterErr(err error) int {
	var fe *fieldError
	if errors.As(err, &fe) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

// decodeJSONBody decodes the request body into v. Treats an empty body as a
// zero-value v (so handlers don't need to special-case the no-filter case).
func decodeJSONBody(r *http.Request, v any) error {
	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return errors.New("invalid request body: " + err.Error())
	}
	return nil
}

// parseTrendWindow validates the start/end dates passed via query string or
// JSON body. Defaults to a 30-day rolling window when both are empty so the
// SPA can call /matches/trends without picking dates upfront.
func parseTrendWindow(startStr, endStr string) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	startStr = strings.TrimSpace(startStr)
	endStr = strings.TrimSpace(endStr)
	var start, end time.Time
	var err error
	if endStr == "" {
		end = now
	} else {
		end, err = parseFilterDate(endStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if startStr == "" {
		start = end.AddDate(0, 0, -30)
	} else {
		start, err = parseFilterDate(startStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, &fieldError{"end_date must be after start_date"}
	}
	return start, end, nil
}

// bucketEnd returns the last instant covered by the bucket starting at start
// for the given period. Used for labeling trend buckets.
func bucketEnd(start time.Time, period string) time.Time {
	switch period {
	case "day":
		return start.AddDate(0, 0, 1).Add(-time.Second)
	case "month":
		return start.AddDate(0, 1, 0).Add(-time.Second)
	default: // week
		return start.AddDate(0, 0, 7).Add(-time.Second)
	}
}

// bucketLabel returns a human-readable label for a trend bucket.
func bucketLabel(start time.Time, period string) string {
	switch period {
	case "day":
		return start.Format("2006-01-02")
	case "month":
		return start.Format("2006-01")
	default: // week
		_, week := start.ISOWeek()
		return start.Format("2006") + "-W" + strconv.Itoa(week)
	}
}

// derefOr returns *p when p != nil, otherwise fallback. Useful for shaping
// optional string columns into JSON without sprinkling nil checks.
func derefOr(p *string, fallback string) string {
	if p == nil {
		return fallback
	}
	return *p
}

// intPtrToString renders an *int as the string form of its value, or empty
// when the pointer is nil. Used by CSV export.
func intPtrToString(p *int) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(*p)
}
