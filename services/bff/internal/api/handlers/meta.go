// Phase 2 PR #5b — /api/v1/meta handlers.
//
// Replaces the SPA's daemonClient /meta surface. Three endpoints
// (archetypes, tier, archetype-cards) are real reads against the
// mtgzone_* tables; four (deck-analysis, identify-archetype, insights,
// refresh) are shape-correct stubs that return an empty response in the
// expected shape so the SPA UI does not crash.
//
// The stub endpoints will be filled in once the supporting infra lands:
//   - identify-archetype + deck-analysis need an archetype-matching
//     algorithm (planned: cosine-similarity over signature_cards from
//     mtgzone_archetypes).
//   - format-insights needs a format-speed / color-power scoring pipeline.
//   - refresh-meta needs a scrape worker that calls MTGGoldfish/MTGTop8.
// Each stub is documented inline with the name of the future PR / ticket.
//
// Auth: every route is guarded by DaemonAPIKeyAuth. Wire shapes match the
// camelCase/snake_case mix the SPA's TS classes already deserialise (see
// meta.ts and frontend/src/types/models.ts).

package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// metaReader is the minimal repo surface the handler needs.
type metaReader interface {
	ListArchetypesByFormat(ctx context.Context, format string, tier int) ([]repository.ArchetypeRow, error)
	LatestArchetypeUpdate(ctx context.Context, format string) (time.Time, bool, error)
	ArchetypeByName(ctx context.Context, format, name string) (*repository.ArchetypeRow, error)
	ArchetypeCardsByID(ctx context.Context, archetypeID int64) ([]repository.ArchetypeCardRow, error)
}

// MetaHandler serves the cloud-data Phase 2 meta API.
type MetaHandler struct {
	meta metaReader
}

// NewMetaHandler returns a handler wired with the given repo.
func NewMetaHandler(m metaReader) *MetaHandler {
	return &MetaHandler{meta: m}
}

// ─── wire shapes ────────────────────────────────────────────────────────────

// metaArchetypeInfoResponse mirrors models.ArchetypeInfo (camelCase).
// Fields that the mtgzone_* schema doesn't currently track default to 0 /
// "stable" — backfilled when the scrape pipeline grows out.
type metaArchetypeInfoResponse struct {
	Name            string   `json:"name"`
	Colors          []string `json:"colors"`
	MetaShare       float64  `json:"metaShare"`
	TournamentTop8s int      `json:"tournamentTop8s"`
	TournamentWins  int      `json:"tournamentWins"`
	Tier            int      `json:"tier"`
	ConfidenceScore float64  `json:"confidenceScore"`
	TrendDirection  string   `json:"trendDirection"`
}

// topCardResponse mirrors gui.TopCard. Only the fields meta endpoints emit
// are listed; the SPA's TS class allows additional fields and ignores any
// it does not declare.
type topCardResponse struct {
	Name string `json:"name"`
}

// archetypeCardsResponse mirrors gui.ArchetypeCards (snake_case).
type archetypeCardsResponse struct {
	Colors       string            `json:"colors"`
	TopCards     []topCardResponse `json:"top_cards"`
	TopCreatures []topCardResponse `json:"top_creatures"`
	TopRemoval   []topCardResponse `json:"top_removal"`
	TopCommons   []topCardResponse `json:"top_commons"`
}

// formatInsightsResponse mirrors insights.FormatInsights (snake_case).
// Empty arrays / objects so the SPA renders a clean empty state until the
// scoring pipeline lands.
type formatInsightsResponse struct {
	SetCode       string            `json:"set_code"`
	DraftFormat   string            `json:"draft_format"`
	ColorRankings []any             `json:"color_rankings"`
	TopBombs      []topCardResponse `json:"top_bombs"`
	TopRemoval    []topCardResponse `json:"top_removal"`
	TopCreatures  []topCardResponse `json:"top_creatures"`
	TopCommons    []topCardResponse `json:"top_commons"`
	FormatSpeed   map[string]any    `json:"format_speed"`
	ColorAnalysis map[string]any    `json:"color_analysis,omitempty"`
}

// metaDashboardResponse mirrors gui.MetaDashboardResponse (camelCase).
type metaDashboardResponse struct {
	Format          string                      `json:"format"`
	Archetypes      []metaArchetypeInfoResponse `json:"archetypes"`
	Tournaments     []any                       `json:"tournaments,omitempty"`
	TotalArchetypes int                         `json:"totalArchetypes"`
	LastUpdated     time.Time                   `json:"lastUpdated"`
	Sources         []string                    `json:"sources"`
	Error           string                      `json:"error,omitempty"`
}

// deckAnalysisResponse mirrors the SPA's DeckAnalysisResult interface.
type deckAnalysisResponse struct {
	Archetype  string   `json:"archetype"`
	Confidence float64  `json:"confidence"`
	Strengths  []string `json:"strengths"`
	Weaknesses []string `json:"weaknesses"`
}

// identifyArchetypeRequest mirrors the SPA's identifyArchetype POST body.
type identifyArchetypeRequest struct {
	CardIDs []int  `json:"cardIds"`
	Format  string `json:"format"`
}

// identifyArchetypeResponse mirrors the SPA's expected return shape.
type identifyArchetypeResponse struct {
	Archetype  string  `json:"archetype"`
	Confidence float64 `json:"confidence"`
}

// ─── handlers ───────────────────────────────────────────────────────────────

// Archetypes handles GET /api/v1/meta/archetypes?format=X.
func (h *MetaHandler) Archetypes(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	if format == "" {
		writeJSONError(w, "format is required", http.StatusBadRequest)
		return
	}
	rows, err := h.meta.ListArchetypesByFormat(r.Context(), format, 0)
	if err != nil {
		log.Printf("[MetaHandler.Archetypes] ListArchetypesByFormat format=%s: %v", format, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, archetypeRowsToResponse(rows))
}

// Tier handles GET /api/v1/meta/tier?format=X&tier=N.
func (h *MetaHandler) Tier(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	if format == "" {
		writeJSONError(w, "format is required", http.StatusBadRequest)
		return
	}
	tierStr := strings.TrimSpace(r.URL.Query().Get("tier"))
	if tierStr == "" {
		writeJSONError(w, "tier is required", http.StatusBadRequest)
		return
	}
	tier, err := strconv.Atoi(tierStr)
	if err != nil || tier <= 0 {
		writeJSONError(w, "tier must be a positive integer", http.StatusBadRequest)
		return
	}
	rows, err := h.meta.ListArchetypesByFormat(r.Context(), format, tier)
	if err != nil {
		log.Printf("[MetaHandler.Tier] ListArchetypesByFormat format=%s tier=%d: %v", format, tier, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, archetypeRowsToResponse(rows))
}

// ArchetypeCards handles GET /api/v1/meta/archetypes/cards?format&archetype.
func (h *MetaHandler) ArchetypeCards(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	archetype := strings.TrimSpace(r.URL.Query().Get("archetype"))
	if format == "" || archetype == "" {
		writeJSONError(w, "format and archetype are required", http.StatusBadRequest)
		return
	}
	row, err := h.meta.ArchetypeByName(r.Context(), format, archetype)
	if err != nil {
		log.Printf("[MetaHandler.ArchetypeCards] ArchetypeByName format=%s archetype=%s: %v", format, archetype, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if row == nil {
		writeMatchesJSON(w, archetypeCardsResponse{
			TopCards: []topCardResponse{}, TopCreatures: []topCardResponse{},
			TopRemoval: []topCardResponse{}, TopCommons: []topCardResponse{},
		})
		return
	}
	cards, err := h.meta.ArchetypeCardsByID(r.Context(), row.ID)
	if err != nil {
		log.Printf("[MetaHandler.ArchetypeCards] ArchetypeCardsByID id=%d: %v", row.ID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, archetypeCardsToResponse(cards))
}

// DeckAnalysis handles GET /api/v1/meta/deck-analysis?deckId=X.
//
// STUB: returns a zero-confidence placeholder. Real deck-vs-archetype
// scoring is planned for a follow-up PR (issue TBD) once the
// signature-card overlap algorithm is in place.
func (h *MetaHandler) DeckAnalysis(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	deckID := strings.TrimSpace(r.URL.Query().Get("deckId"))
	if deckID == "" {
		writeJSONError(w, "deckId is required", http.StatusBadRequest)
		return
	}
	writeMatchesJSON(w, deckAnalysisResponse{
		Archetype:  "Unknown",
		Confidence: 0,
		Strengths:  []string{},
		Weaknesses: []string{},
	})
}

// IdentifyArchetype handles POST /api/v1/meta/identify-archetype.
//
// STUB: returns a zero-confidence placeholder. The real implementation
// requires the same archetype-matching algorithm as DeckAnalysis.
func (h *MetaHandler) IdentifyArchetype(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	var req identifyArchetypeRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
	}
	if strings.TrimSpace(req.Format) == "" {
		writeJSONError(w, "format is required", http.StatusBadRequest)
		return
	}
	writeMatchesJSON(w, identifyArchetypeResponse{Archetype: "Unknown", Confidence: 0})
}

// FormatInsights handles GET /api/v1/meta/insights?format&setCode.
//
// STUB: returns a shape-correct empty response. The real implementation
// requires a format-speed / color-power scoring pipeline that does not
// exist yet.
func (h *MetaHandler) FormatInsights(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	setCode := strings.TrimSpace(r.URL.Query().Get("setCode"))
	if format == "" || setCode == "" {
		writeJSONError(w, "format and setCode are required", http.StatusBadRequest)
		return
	}
	writeMatchesJSON(w, formatInsightsResponse{
		SetCode: setCode, DraftFormat: format,
		ColorRankings: []any{}, TopBombs: []topCardResponse{},
		TopRemoval: []topCardResponse{}, TopCreatures: []topCardResponse{},
		TopCommons:  []topCardResponse{},
		FormatSpeed: map[string]any{},
	})
}

// Refresh handles POST /api/v1/meta/refresh?format.
//
// STUB until the scrape worker lands: returns the dashboard wrapper
// populated with whatever archetypes are currently in the DB. The SPA's
// only consumer treats the response as "fetched dashboard" and ignores
// the lack of a fresh scrape.
func (h *MetaHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	if format == "" {
		writeJSONError(w, "format is required", http.StatusBadRequest)
		return
	}
	rows, err := h.meta.ListArchetypesByFormat(r.Context(), format, 0)
	if err != nil {
		log.Printf("[MetaHandler.Refresh] ListArchetypesByFormat format=%s: %v", format, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	last, ok, err := h.meta.LatestArchetypeUpdate(r.Context(), format)
	if err != nil {
		log.Printf("[MetaHandler.Refresh] LatestArchetypeUpdate format=%s: %v", format, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !ok {
		last = time.Now().UTC()
	}
	writeMatchesJSON(w, metaDashboardResponse{
		Format:          format,
		Archetypes:      archetypeRowsToResponse(rows),
		TotalArchetypes: len(rows),
		LastUpdated:     last,
		Sources:         []string{"mtgzone_archetypes (cached)"},
	})
}

// ─── helpers ────────────────────────────────────────────────────────────────

// requireAuth handles the user-id-from-context check for endpoints that do
// not need an account_id (all meta endpoints are global). Returns false
// after writing 401 when the request is unauthenticated.
func (h *MetaHandler) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := bffmiddleware.UserIDFromContext(r.Context()); !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

// archetypeRowsToResponse converts repo rows into the SPA's ArchetypeInfo
// shape. Tier text → integer is best-effort: if the schema's tier value is
// "S" / "A" / etc. the parser returns 0 and the SPA treats it as
// "untiered".
func archetypeRowsToResponse(rows []repository.ArchetypeRow) []metaArchetypeInfoResponse {
	out := make([]metaArchetypeInfoResponse, 0, len(rows))
	for _, a := range rows {
		out = append(out, metaArchetypeInfoResponse{
			Name:           a.Name,
			Colors:         []string{}, // mtgzone_archetypes doesn't carry colors yet
			Tier:           parseTierToInt(a.Tier),
			TrendDirection: "stable",
		})
	}
	return out
}

// parseTierToInt accepts a *string from the schema and returns the int
// value when parseable (e.g., "1" → 1), otherwise 0.
func parseTierToInt(p *string) int {
	if p == nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(*p))
	if err != nil {
		return 0
	}
	return n
}

// archetypeCardsToResponse buckets the per-archetype cards into the SPA's
// top_cards / top_creatures / top_removal / top_commons slots. Uses the
// `role` column for routing; cards with an unknown role land in top_cards.
func archetypeCardsToResponse(rows []repository.ArchetypeCardRow) archetypeCardsResponse {
	resp := archetypeCardsResponse{
		TopCards: []topCardResponse{}, TopCreatures: []topCardResponse{},
		TopRemoval: []topCardResponse{}, TopCommons: []topCardResponse{},
	}
	for _, c := range rows {
		entry := topCardResponse{Name: c.CardName}
		switch strings.ToLower(strings.TrimSpace(c.Role)) {
		case "creature", "creatures":
			resp.TopCreatures = append(resp.TopCreatures, entry)
		case "removal":
			resp.TopRemoval = append(resp.TopRemoval, entry)
		case "common", "commons":
			resp.TopCommons = append(resp.TopCommons, entry)
		default:
			resp.TopCards = append(resp.TopCards, entry)
		}
	}
	return resp
}
