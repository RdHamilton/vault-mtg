package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ramonehamilton/mtga-bff/internal/config"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// DraftRatingsGetter is the minimal read interface required by DraftRatingsHandler.
type DraftRatingsGetter interface {
	GetRatings(ctx context.Context, setCode, draftFormat string) (*repository.DraftRatingsResult, error)
}

// DraftRatingsHandler serves GET /api/v1/draft-ratings/{setCode}/{format}.
type DraftRatingsHandler struct {
	repo DraftRatingsGetter
	cfg  *config.Config
}

// NewDraftRatingsHandler constructs a DraftRatingsHandler.
func NewDraftRatingsHandler(repo DraftRatingsGetter, cfg *config.Config) *DraftRatingsHandler {
	return &DraftRatingsHandler{repo: repo, cfg: cfg}
}

// draftRatingsResponse is the JSON envelope returned to callers.
type draftRatingsResponse struct {
	SetCode      string            `json:"set_code"`
	DraftFormat  string            `json:"draft_format"`
	CachedAt     time.Time         `json:"cached_at"`
	CardRatings  []cardRatingJSON  `json:"card_ratings"`
	ColorRatings []colorRatingJSON `json:"color_ratings"`
}

type cardRatingJSON struct {
	ArenaID  int      `json:"arena_id"`
	Name     string   `json:"name"`
	Color    string   `json:"color,omitempty"`
	Rarity   string   `json:"rarity,omitempty"`
	GIHWR    *float64 `json:"gihwr,omitempty"`
	OHWR     *float64 `json:"ohwr,omitempty"`
	ALSA     *float64 `json:"alsa,omitempty"`
	ATA      *float64 `json:"ata,omitempty"`
	GIHCount *int     `json:"gih_count,omitempty"`
}

type colorRatingJSON struct {
	ColorCombination string   `json:"color_combination"`
	WinRate          *float64 `json:"win_rate,omitempty"`
	GamesPlayed      *int     `json:"games_played,omitempty"`
}

// GetDraftRatings handles GET /api/v1/draft-ratings/{setCode}/{format}.
//
// Response contract (per ADR-004):
//   - 200 with body when rows exist (fresh or stale).
//   - X-Cache-Degraded: true and X-Cache-Age-Hours: <N> when stale and bypass
//     is not enabled.
//   - 404 when no rows exist for the requested set/format.
//   - Never returns 5xx due to stale data alone.
func (h *DraftRatingsHandler) GetDraftRatings(w http.ResponseWriter, r *http.Request) {
	setCode := chi.URLParam(r, "setCode")
	format := chi.URLParam(r, "format")

	if setCode == "" || format == "" {
		http.Error(w, "setCode and format are required", http.StatusBadRequest)
		return
	}

	result, err := h.repo.GetRatings(r.Context(), setCode, format)
	if err != nil {
		log.Printf("[DraftRatingsHandler] GetRatings error set=%s format=%s: %v", setCode, format, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if result == nil {
		http.Error(w, "no ratings found for the requested set/format", http.StatusNotFound)
		return
	}

	// Staleness check — bypassed when the escape-hatch flag is set.
	if !h.cfg.DraftRatingsBypassFreshnessCheck {
		ageHours := time.Since(result.CachedAt).Hours()
		if ageHours > float64(h.cfg.DraftRatingsStalenessThresholdHours) {
			rounded := int(math.Round(ageHours))
			w.Header().Set("X-Cache-Degraded", "true")
			w.Header().Set("X-Cache-Age-Hours", fmt.Sprintf("%d", rounded))
			log.Printf("[DraftRatingsHandler] degraded mode set=%s format=%s age_hours=%d threshold=%d",
				setCode, format, rounded, h.cfg.DraftRatingsStalenessThresholdHours)
		}
	}

	// Build response envelope.
	resp := draftRatingsResponse{
		SetCode:     result.SetCode,
		DraftFormat: result.DraftFormat,
		CachedAt:    result.CachedAt,
	}

	for _, c := range result.CardRatings {
		resp.CardRatings = append(resp.CardRatings, cardRatingJSON{
			ArenaID:  c.ArenaID,
			Name:     c.Name,
			Color:    c.Color,
			Rarity:   c.Rarity,
			GIHWR:    c.GIHWR,
			OHWR:     c.OHWR,
			ALSA:     c.ALSA,
			ATA:      c.ATA,
			GIHCount: c.GIHCount,
		})
	}

	for _, cr := range result.ColorRatings {
		resp.ColorRatings = append(resp.ColorRatings, colorRatingJSON{
			ColorCombination: cr.ColorCombination,
			WinRate:          cr.WinRate,
			GamesPlayed:      cr.GamesPlayed,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("[DraftRatingsHandler] encode response: %v", err)
	}
}
