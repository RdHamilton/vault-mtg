package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ramonehamilton/MTGA-Companion/internal/api/response"
	"github.com/ramonehamilton/MTGA-Companion/internal/gui"
	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
)

// QuestHandler handles quest-related API requests.
type QuestHandler struct {
	facade *gui.MatchFacade
}

// NewQuestHandler creates a new QuestHandler.
func NewQuestHandler(facade *gui.MatchFacade) *QuestHandler {
	return &QuestHandler{facade: facade}
}

// ActiveQuestsResponse is the response shape for the active quests endpoint.
type ActiveQuestsResponse struct {
	Quests       []*models.Quest `json:"quests"`
	HasQuestData bool            `json:"has_quest_data"`
	LastUpdated  *time.Time      `json:"last_updated,omitempty"`
}

// GetActiveQuests returns active quests with metadata about quest data availability.
func (h *QuestHandler) GetActiveQuests(w http.ResponseWriter, r *http.Request) {
	quests, err := h.facade.GetActiveQuests(r.Context())
	if err != nil {
		response.InternalError(w, err)
		return
	}

	// Return empty array instead of nil
	if quests == nil {
		quests = []*models.Quest{}
	}

	hasQuestData := h.facade.HasAnyQuestData(r.Context())

	// Determine last updated time from the most recent quest's last_seen_at
	var lastUpdated *time.Time
	for _, q := range quests {
		if q.LastSeenAt != nil {
			if lastUpdated == nil || q.LastSeenAt.After(*lastUpdated) {
				lastUpdated = q.LastSeenAt
			}
		}
	}

	response.Success(w, ActiveQuestsResponse{
		Quests:       quests,
		HasQuestData: hasQuestData,
		LastUpdated:  lastUpdated,
	})
}

// GetQuestHistory returns quest history.
func (h *QuestHandler) GetQuestHistory(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")
	limitStr := r.URL.Query().Get("limit")

	limit := 50 // default
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	// Default dates if not provided
	if startDate == "" {
		startDate = time.Now().Add(-90 * 24 * time.Hour).Format("2006-01-02") // 90 days ago
	}
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}

	quests, err := h.facade.GetQuestHistory(r.Context(), startDate, endDate, limit)
	if err != nil {
		response.InternalError(w, err)
		return
	}

	// Return empty array instead of nil
	if quests == nil {
		quests = []*models.Quest{}
	}

	response.Success(w, quests)
}

// GetDailyWins returns daily wins progress, calculated from actual match data.
func (h *QuestHandler) GetDailyWins(w http.ResponseWriter, r *http.Request) {
	dailyWins, err := h.facade.GetDailyWins(r.Context())
	if err != nil {
		response.InternalError(w, err)
		return
	}

	response.Success(w, map[string]interface{}{
		"dailyWins": dailyWins,
		"goal":      15,
	})
}

// GetWeeklyWins returns weekly wins progress, calculated from actual match data.
func (h *QuestHandler) GetWeeklyWins(w http.ResponseWriter, r *http.Request) {
	weeklyWins, err := h.facade.GetWeeklyWins(r.Context())
	if err != nil {
		response.InternalError(w, err)
		return
	}

	response.Success(w, map[string]interface{}{
		"weeklyWins": weeklyWins,
		"goal":       15,
	})
}
