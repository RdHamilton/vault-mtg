// Phase 2 PR #3 — /api/v1/quests handlers.
//
// Replaces the SPA's daemonClient /quests surface. All responses are
// wrapped in the {"data": ...} envelope the SPA's apiClient expects.
// JSON keys use snake_case to match the existing models.Quest /
// models.QuestStats TS class constructors (Wails-era types).
//
// Auth: every route is guarded by DaemonAPIKeyAuth. Quest rows are scoped
// to the authenticated user's account.

package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	bffmiddleware "github.com/RdHamilton/vault-mtg/services/bff/internal/api/middleware"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
)

// MTGA wins-per-day / wins-per-week goals. Both currently 15 — the rewards
// table tops out at 15 daily wins (gold + XP). Hardcoded here so the handler
// can return a canonical value; the SPA does not let the user override.
const (
	dailyWinsGoal  = 15
	weeklyWinsGoal = 15
)

// questsReader is the minimal repo surface the handler needs.
type questsReader interface {
	ListActiveByAccountID(ctx context.Context, accountID int64) ([]repository.QuestRow, error)
	ListHistoryByAccountID(ctx context.Context, accountID int64, start, end *time.Time, limit int) ([]repository.QuestRow, error)
	CountWinsSince(ctx context.Context, accountID int64, since time.Time) (int, error)
	QuestStats(ctx context.Context, accountID int64, start, end time.Time) (repository.QuestStatsAggregate, error)
	LastQuestSeenAt(ctx context.Context, accountID int64) (time.Time, bool, error)
}

// QuestsHandler serves the cloud-data Phase 2 quests API.
type QuestsHandler struct {
	quests   questsReader
	accounts AccountLookup
}

// NewQuestsHandler returns a QuestsHandler wired with the given repo + lookup.
func NewQuestsHandler(q questsReader, accounts AccountLookup) *QuestsHandler {
	return &QuestsHandler{quests: q, accounts: accounts}
}

// ─── wire shapes ────────────────────────────────────────────────────────────

// questResponse mirrors models.Quest (snake_case keys). Pointer fields are
// nullable in the schema; we mark them omitempty so a clean JSON response
// drops them when absent.
type questResponse struct {
	ID               int64      `json:"id"`
	QuestID          string     `json:"quest_id"`
	QuestType        string     `json:"quest_type"`
	Goal             int        `json:"goal"`
	StartingProgress int        `json:"starting_progress"`
	EndingProgress   int        `json:"ending_progress"`
	Completed        bool       `json:"completed"`
	CanSwap          bool       `json:"can_swap"`
	Rewards          string     `json:"rewards"`
	FirstSeenAt      time.Time  `json:"first_seen_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	LastSeenAt       *time.Time `json:"last_seen_at,omitempty"`
	Rerolled         bool       `json:"rerolled"`
	CreatedAt        time.Time  `json:"created_at"`
	SessionID        string     `json:"session_id,omitempty"`
	CompletionSource string     `json:"completion_source,omitempty"`
}

// activeQuestsResponse mirrors the SPA's ActiveQuestsResponse interface.
type activeQuestsResponse struct {
	Quests       []questResponse `json:"quests"`
	HasQuestData bool            `json:"has_quest_data"`
	LastUpdated  string          `json:"last_updated,omitempty"`
}

// dailyWinsResponse / weeklyWinsResponse: the SPA wrappers unwrap into
// {wins, goal} but the daemon contract used dailyWins/weeklyWins as the
// counter key. Preserve that here so the SPA wrapper still works without
// changes.
type dailyWinsResponse struct {
	DailyWins int `json:"dailyWins"`
	Goal      int `json:"goal"`
}

type weeklyWinsResponse struct {
	WeeklyWins int `json:"weeklyWins"`
	Goal       int `json:"goal"`
}

// questStatsResponse mirrors models.QuestStats (snake_case).
type questStatsResponse struct {
	TotalQuests         int     `json:"total_quests"`
	CompletedQuests     int     `json:"completed_quests"`
	ActiveQuests        int     `json:"active_quests"`
	CompletionRate      float64 `json:"completion_rate"`
	TotalGoldEarned     int     `json:"total_gold_earned"`
	AverageCompletionMS int64   `json:"average_completion_ms"`
	RerollCount         int     `json:"reroll_count"`
}

// ─── handlers ───────────────────────────────────────────────────────────────

// Active handles GET /api/v1/quests/active.
func (h *QuestsHandler) Active(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Active")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, activeQuestsResponse{Quests: []questResponse{}})
		return
	}
	rows, err := h.quests.ListActiveByAccountID(r.Context(), accountID)
	if err != nil {
		log.Printf("[QuestsHandler.Active] ListActiveByAccountID accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	last, hasData, err := h.quests.LastQuestSeenAt(r.Context(), accountID)
	if err != nil {
		log.Printf("[QuestsHandler.Active] LastQuestSeenAt accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	resp := activeQuestsResponse{
		Quests:       questsToResponse(rows),
		HasQuestData: hasData,
	}
	if hasData {
		resp.LastUpdated = last.Format(time.RFC3339)
	}
	writeMatchesJSON(w, resp)
}

// History handles GET /api/v1/quests/history.
// Query params: startDate, endDate, limit.
func (h *QuestsHandler) History(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "History")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, []questResponse{})
		return
	}
	start, end, err := parseQuestWindow(r)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	limit := parseLimitDefault(r, "limit", 100)
	rows, err := h.quests.ListHistoryByAccountID(r.Context(), accountID, start, end, limit)
	if err != nil {
		log.Printf("[QuestsHandler.History] ListHistoryByAccountID accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, questsToResponse(rows))
}

// DailyWins handles GET /api/v1/quests/wins/daily — wins in the last 24h.
func (h *QuestsHandler) DailyWins(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "DailyWins")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, dailyWinsResponse{Goal: dailyWinsGoal})
		return
	}
	since := time.Now().UTC().Add(-24 * time.Hour)
	wins, err := h.quests.CountWinsSince(r.Context(), accountID, since)
	if err != nil {
		log.Printf("[QuestsHandler.DailyWins] CountWinsSince accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, dailyWinsResponse{DailyWins: wins, Goal: dailyWinsGoal})
}

// WeeklyWins handles GET /api/v1/quests/wins/weekly — wins in the last 7 days.
func (h *QuestsHandler) WeeklyWins(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "WeeklyWins")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, weeklyWinsResponse{Goal: weeklyWinsGoal})
		return
	}
	since := time.Now().UTC().AddDate(0, 0, -7)
	wins, err := h.quests.CountWinsSince(r.Context(), accountID, since)
	if err != nil {
		log.Printf("[QuestsHandler.WeeklyWins] CountWinsSince accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, weeklyWinsResponse{WeeklyWins: wins, Goal: weeklyWinsGoal})
}

// Stats handles GET /api/v1/quests/stats?startDate=...&endDate=...
func (h *QuestsHandler) Stats(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Stats")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, questStatsResponse{})
		return
	}
	start, end, err := parseQuestStatsWindow(r)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	agg, err := h.quests.QuestStats(r.Context(), accountID, start, end)
	if err != nil {
		log.Printf("[QuestsHandler.Stats] QuestStats accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	resp := questStatsResponse{
		TotalQuests:         agg.TotalQuests,
		CompletedQuests:     agg.CompletedQuests,
		ActiveQuests:        agg.ActiveQuests,
		TotalGoldEarned:     agg.TotalGoldEarned,
		AverageCompletionMS: agg.AverageCompletionMS,
		RerollCount:         agg.RerollCount,
	}
	if agg.TotalQuests > 0 {
		resp.CompletionRate = float64(agg.CompletedQuests) / float64(agg.TotalQuests)
	}
	writeMatchesJSON(w, resp)
}

// ─── helpers ────────────────────────────────────────────────────────────────

// resolveAccount mirrors the helper used by MatchesHandler / CollectionHandler.
func (h *QuestsHandler) resolveAccount(w http.ResponseWriter, r *http.Request, op string) (int64, bool, bool) {
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return 0, false, false
	}
	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[QuestsHandler.%s] GetAccountIDByUserID userID=%d: %v", op, userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return 0, false, false
	}
	return accountID, found, true
}

// questsToResponse converts repo rows into wire shape. Centralised so the
// handler funcs stay terse.
func questsToResponse(rows []repository.QuestRow) []questResponse {
	out := make([]questResponse, 0, len(rows))
	for _, q := range rows {
		out = append(out, questResponse{
			ID:               q.ID,
			QuestID:          q.QuestID,
			QuestType:        derefOr(q.QuestType, ""),
			Goal:             q.Goal,
			StartingProgress: q.StartingProgress,
			EndingProgress:   q.EndingProgress,
			Completed:        q.Completed,
			CanSwap:          q.CanSwap,
			Rewards:          derefOr(q.Rewards, ""),
			FirstSeenAt:      q.FirstSeenAt,
			CompletedAt:      q.CompletedAt,
			LastSeenAt:       q.LastSeenAt,
			Rerolled:         q.Rerolled,
			CreatedAt:        q.CreatedAt,
			SessionID:        derefOr(q.SessionID, ""),
			CompletionSource: derefOr(q.CompletionSource, ""),
		})
	}
	return out
}

// parseQuestWindow reads optional ?startDate / ?endDate query params for
// /quests/history. Returns nil pointers when the corresponding param is
// missing — a nil bound means "unbounded on that side".
func parseQuestWindow(r *http.Request) (*time.Time, *time.Time, error) {
	var start, end *time.Time
	if s := r.URL.Query().Get("startDate"); s != "" {
		t, err := parseFilterDate(s)
		if err != nil {
			return nil, nil, err
		}
		start = &t
	}
	if s := r.URL.Query().Get("endDate"); s != "" {
		t, err := parseFilterDate(s)
		if err != nil {
			return nil, nil, err
		}
		end = &t
	}
	return start, end, nil
}

// parseQuestStatsWindow reads required ?startDate / ?endDate params for the
// stats endpoint. Defaults to a 30-day rolling window if unset (the SPA
// always sends both today, but defending the endpoint costs nothing).
func parseQuestStatsWindow(r *http.Request) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	var start, end time.Time
	if s := r.URL.Query().Get("endDate"); s != "" {
		t, err := parseFilterDate(s)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		end = t
	} else {
		end = now
	}
	if s := r.URL.Query().Get("startDate"); s != "" {
		t, err := parseFilterDate(s)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		start = t
	} else {
		start = end.AddDate(0, 0, -30)
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, &fieldError{"endDate must be after startDate"}
	}
	return start, end, nil
}
