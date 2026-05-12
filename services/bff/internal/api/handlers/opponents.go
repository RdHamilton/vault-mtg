// Phase 2 PR #6 — opponents + analytics + archetypes-expected handlers.
//
// Replaces the SPA's daemonClient surface for opponents.ts. Routes mount
// across four URL prefixes per the SPA contract:
//   - GET /api/v1/matches/{matchId}/opponent-analysis
//   - GET /api/v1/opponents/decks
//   - GET /api/v1/analytics/matchups
//   - GET /api/v1/analytics/opponent-history
//   - GET /api/v1/archetypes/{name}/expected-cards
//
// All routes are guarded by DaemonAPIKeyAuth + the standard envelope.
// Match-scoped queries enforce account ownership via matches.account_id;
// archetypes-expected is global (catalog data).

package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// opponentsReader is the minimal repo surface the handler needs.
type opponentsReader interface {
	OpponentProfileForMatch(ctx context.Context, accountID int64, matchID string) (*repository.OpponentDeckProfileRow, error)
	OpponentCardsForMatch(ctx context.Context, accountID int64, matchID string) ([]repository.OpponentObservedCardRow, error)
	ListOpponentDecks(ctx context.Context, accountID int64, f repository.OpponentDeckFilter) ([]repository.OpponentDeckProfileRow, int, error)
	ListMatchups(ctx context.Context, accountID int64, format string) ([]repository.MatchupStatRow, int, error)
	MatchupForArchetypes(ctx context.Context, accountID int64, playerArch, opponentArch, format string) (*repository.MatchupStatRow, error)
	ExpectedCardsForArchetype(ctx context.Context, archetype, format string) ([]repository.ExpectedCardRow, error)
	OpponentHistorySummary(ctx context.Context, accountID int64, format string) (repository.OpponentHistorySummaryRow, error)
	ArchetypeBreakdown(ctx context.Context, accountID int64, format string) ([]repository.ArchetypeBreakdownRow, error)
	ColorIdentityBreakdown(ctx context.Context, accountID int64, format string) ([]repository.ColorIdentityBreakdownRow, error)
}

// OpponentsHandler serves the cloud-data Phase 2 opponents API.
type OpponentsHandler struct {
	opponents opponentsReader
	accounts  AccountLookup
}

// NewOpponentsHandler returns a handler wired with the given reader + lookup.
func NewOpponentsHandler(o opponentsReader, accounts AccountLookup) *OpponentsHandler {
	return &OpponentsHandler{opponents: o, accounts: accounts}
}

// ─── wire shapes ────────────────────────────────────────────────────────────

// opponentDeckProfileResponse mirrors the SPA's OpponentDeckProfile.
type opponentDeckProfileResponse struct {
	ID                  int64   `json:"id"`
	MatchID             string  `json:"matchId"`
	DetectedArchetype   *string `json:"detectedArchetype"`
	ArchetypeConfidence float64 `json:"archetypeConfidence"`
	ColorIdentity       string  `json:"colorIdentity"`
	DeckStyle           *string `json:"deckStyle"`
	CardsObserved       int     `json:"cardsObserved"`
	EstimatedDeckSize   int     `json:"estimatedDeckSize"`
	ObservedCardIDs     *string `json:"observedCardIds"`
	InferredCardIDs     *string `json:"inferredCardIds"`
	SignatureCards      *string `json:"signatureCards"`
	Format              *string `json:"format"`
	MetaArchetypeID     *int    `json:"metaArchetypeId"`
	CreatedAt           string  `json:"createdAt"`
	UpdatedAt           string  `json:"updatedAt"`
}

// observedCardResponse mirrors the SPA's ObservedCard.
type observedCardResponse struct {
	CardID        int    `json:"cardId"`
	CardName      string `json:"cardName"`
	Zone          string `json:"zone"`
	TurnFirstSeen int    `json:"turnFirstSeen"`
	TimesSeen     int    `json:"timesSeen"`
	IsSignature   bool   `json:"isSignature"`
	Category      string `json:"category"`
}

// expectedCardResponse mirrors the SPA's ExpectedCard.
type expectedCardResponse struct {
	CardID        int     `json:"cardId"`
	CardName      string  `json:"cardName"`
	InclusionRate float64 `json:"inclusionRate"`
	AvgCopies     float64 `json:"avgCopies"`
	WasSeen       bool    `json:"wasSeen"`
	Category      string  `json:"category"`
	PlayAround    string  `json:"playAround"`
}

// strategicInsightResponse mirrors the SPA's StrategicInsight. Empty slice
// for now — insights generation is a follow-up PR.
type strategicInsightResponse struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Cards       []int  `json:"cards"`
}

// matchupStatisticResponse mirrors the SPA's MatchupStatistic.
type matchupStatisticResponse struct {
	ID                int64   `json:"id"`
	AccountID         int64   `json:"accountId"`
	PlayerArchetype   string  `json:"playerArchetype"`
	OpponentArchetype string  `json:"opponentArchetype"`
	Format            string  `json:"format"`
	TotalMatches      int     `json:"totalMatches"`
	Wins              int     `json:"wins"`
	Losses            int     `json:"losses"`
	WinRate           float64 `json:"winRate"`
	AvgGameDuration   *int    `json:"avgGameDuration"`
	LastMatchAt       *string `json:"lastMatchAt"`
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         string  `json:"updatedAt"`
}

// metaArchetypeMatchResponse mirrors the SPA's MetaArchetypeMatch. Returned
// as nil today — meta-archetype linkage requires the archetype-matching
// algorithm planned for PR #5b's follow-up.
type metaArchetypeMatchResponse struct {
	ArchetypeID   int     `json:"archetypeId"`
	ArchetypeName string  `json:"archetypeName"`
	MetaShare     float64 `json:"metaShare"`
	Tier          int     `json:"tier"`
	Confidence    float64 `json:"confidence"`
	Source        string  `json:"source"`
}

// opponentAnalysisResponse mirrors the SPA's OpponentAnalysis composite.
type opponentAnalysisResponse struct {
	Profile           *opponentDeckProfileResponse `json:"profile"`
	ObservedCards     []observedCardResponse       `json:"observedCards"`
	ExpectedCards     []expectedCardResponse       `json:"expectedCards"`
	StrategicInsights []strategicInsightResponse   `json:"strategicInsights"`
	MatchupStats      *matchupStatisticResponse    `json:"matchupStats"`
	MetaArchetype     *metaArchetypeMatchResponse  `json:"metaArchetype"`
}

// archetypeBreakdownEntryResponse mirrors the SPA's ArchetypeBreakdownEntry.
type archetypeBreakdownEntryResponse struct {
	Archetype  string  `json:"archetype"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
	WinRate    float64 `json:"winRate"`
}

// colorIdentityStatsEntryResponse mirrors ColorIdentityStatsEntry.
type colorIdentityStatsEntryResponse struct {
	ColorIdentity string  `json:"colorIdentity"`
	Count         int     `json:"count"`
	Percentage    float64 `json:"percentage"`
	WinRate       float64 `json:"winRate"`
}

// opponentHistorySummaryResponse mirrors OpponentHistorySummary.
type opponentHistorySummaryResponse struct {
	TotalOpponents      int                               `json:"totalOpponents"`
	UniqueArchetypes    int                               `json:"uniqueArchetypes"`
	MostCommonArchetype string                            `json:"mostCommonArchetype"`
	MostCommonCount     int                               `json:"mostCommonCount"`
	ArchetypeBreakdown  []archetypeBreakdownEntryResponse `json:"archetypeBreakdown"`
	ColorIdentityStats  []colorIdentityStatsEntryResponse `json:"colorIdentityStats"`
}

// archetypeExpectedCardResponse mirrors ArchetypeExpectedCard.
type archetypeExpectedCardResponse struct {
	ID            int64   `json:"id"`
	ArchetypeName string  `json:"archetypeName"`
	Format        string  `json:"format"`
	CardID        int     `json:"cardId"`
	CardName      string  `json:"cardName"`
	InclusionRate float64 `json:"inclusionRate"`
	AvgCopies     float64 `json:"avgCopies"`
	IsSignature   bool    `json:"isSignature"`
	Category      *string `json:"category"`
	CreatedAt     string  `json:"createdAt"`
}

// ─── handlers ───────────────────────────────────────────────────────────────

// OpponentAnalysis handles GET /api/v1/matches/{matchId}/opponent-analysis.
// Composite endpoint that stitches profile + observed cards + expected
// cards + matchup stats into one response. Strategic insights and
// meta-archetype linkage are emitted as empty arrays / null pending the
// supporting infra.
func (h *OpponentsHandler) OpponentAnalysis(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "OpponentAnalysis")
	if !ok {
		return
	}
	matchID := strings.TrimSpace(chi.URLParam(r, "matchId"))
	if matchID == "" {
		writeJSONError(w, "matchId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, opponentAnalysisResponse{
			ObservedCards:     []observedCardResponse{},
			ExpectedCards:     []expectedCardResponse{},
			StrategicInsights: []strategicInsightResponse{},
		})
		return
	}

	profile, err := h.opponents.OpponentProfileForMatch(r.Context(), accountID, matchID)
	if err != nil {
		log.Printf("[OpponentsHandler.OpponentAnalysis] OpponentProfileForMatch accountID=%d matchID=%s: %v", accountID, matchID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	observed, err := h.opponents.OpponentCardsForMatch(r.Context(), accountID, matchID)
	if err != nil {
		log.Printf("[OpponentsHandler.OpponentAnalysis] OpponentCardsForMatch accountID=%d matchID=%s: %v", accountID, matchID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := opponentAnalysisResponse{
		ObservedCards:     observedCardsToResponse(observed),
		ExpectedCards:     []expectedCardResponse{},
		StrategicInsights: []strategicInsightResponse{},
	}
	if profile != nil {
		profileResp := opponentProfileToResponse(*profile)
		resp.Profile = &profileResp
		// Pull expected cards for the detected archetype + format.
		if profile.DetectedArchetype != nil && *profile.DetectedArchetype != "" {
			format := derefOr(profile.Format, "")
			expected, err := h.opponents.ExpectedCardsForArchetype(r.Context(), *profile.DetectedArchetype, format)
			if err != nil {
				log.Printf("[OpponentsHandler.OpponentAnalysis] ExpectedCardsForArchetype: %v", err)
			} else {
				resp.ExpectedCards = expectedCardsToResponse(expected, observed)
			}
		}
	}
	writeMatchesJSON(w, resp)
}

// ListDecks handles GET /api/v1/opponents/decks.
func (h *OpponentsHandler) ListDecks(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "ListDecks")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, map[string]any{"profiles": []opponentDeckProfileResponse{}, "total": 0})
		return
	}
	filter := repository.OpponentDeckFilter{
		Archetype: strings.TrimSpace(r.URL.Query().Get("archetype")),
		Format:    strings.TrimSpace(r.URL.Query().Get("format")),
	}
	if v := strings.TrimSpace(r.URL.Query().Get("min_confidence")); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < 0 || f > 1 {
			writeJSONError(w, "min_confidence must be a number in [0,1]", http.StatusBadRequest)
			return
		}
		filter.MinConfidence = f
	}
	filter.Limit = parseLimitDefault(r, "limit", 50)
	rows, total, err := h.opponents.ListOpponentDecks(r.Context(), accountID, filter)
	if err != nil {
		log.Printf("[OpponentsHandler.ListDecks] ListOpponentDecks accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	profiles := make([]opponentDeckProfileResponse, 0, len(rows))
	for _, p := range rows {
		profiles = append(profiles, opponentProfileToResponse(p))
	}
	writeMatchesJSON(w, map[string]any{"profiles": profiles, "total": total})
}

// MatchupStats handles GET /api/v1/analytics/matchups.
func (h *OpponentsHandler) MatchupStats(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "MatchupStats")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, map[string]any{"matchups": []matchupStatisticResponse{}, "total": 0})
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	rows, total, err := h.opponents.ListMatchups(r.Context(), accountID, format)
	if err != nil {
		log.Printf("[OpponentsHandler.MatchupStats] ListMatchups accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	matchups := make([]matchupStatisticResponse, 0, len(rows))
	for _, m := range rows {
		matchups = append(matchups, matchupRowToResponse(m))
	}
	writeMatchesJSON(w, map[string]any{"matchups": matchups, "total": total})
}

// OpponentHistory handles GET /api/v1/analytics/opponent-history.
func (h *OpponentsHandler) OpponentHistory(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "OpponentHistory")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, opponentHistorySummaryResponse{
			ArchetypeBreakdown: []archetypeBreakdownEntryResponse{},
			ColorIdentityStats: []colorIdentityStatsEntryResponse{},
		})
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	summary, err := h.opponents.OpponentHistorySummary(r.Context(), accountID, format)
	if err != nil {
		log.Printf("[OpponentsHandler.OpponentHistory] OpponentHistorySummary: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	archetypes, err := h.opponents.ArchetypeBreakdown(r.Context(), accountID, format)
	if err != nil {
		log.Printf("[OpponentsHandler.OpponentHistory] ArchetypeBreakdown: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	colors, err := h.opponents.ColorIdentityBreakdown(r.Context(), accountID, format)
	if err != nil {
		log.Printf("[OpponentsHandler.OpponentHistory] ColorIdentityBreakdown: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	resp := opponentHistorySummaryResponse{
		TotalOpponents:      summary.TotalOpponents,
		UniqueArchetypes:    summary.UniqueArchetypes,
		MostCommonArchetype: summary.MostCommonArchetype,
		MostCommonCount:     summary.MostCommonCount,
		ArchetypeBreakdown:  archetypeBreakdownToResponse(archetypes, summary.TotalOpponents),
		ColorIdentityStats:  colorBreakdownToResponse(colors, summary.TotalOpponents),
	}
	writeMatchesJSON(w, resp)
}

// ExpectedCardsByArchetype handles GET /api/v1/archetypes/{name}/expected-cards.
// Catalog endpoint — no account scoping (the data is global).
func (h *OpponentsHandler) ExpectedCardsByArchetype(w http.ResponseWriter, r *http.Request) {
	if _, ok := bffmiddleware.UserIDFromContext(r.Context()); !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	archetype := strings.TrimSpace(chi.URLParam(r, "name"))
	if archetype == "" {
		writeJSONError(w, "archetype name is required", http.StatusBadRequest)
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	rows, err := h.opponents.ExpectedCardsForArchetype(r.Context(), archetype, format)
	if err != nil {
		log.Printf("[OpponentsHandler.ExpectedCardsByArchetype] ExpectedCardsForArchetype archetype=%s format=%s: %v", archetype, format, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	out := make([]archetypeExpectedCardResponse, 0, len(rows))
	for _, e := range rows {
		out = append(out, archetypeExpectedCardResponse{
			ID: e.ID, ArchetypeName: e.ArchetypeName, Format: e.Format,
			CardID: e.CardID, CardName: e.CardName,
			InclusionRate: e.InclusionRate, AvgCopies: e.AvgCopies,
			IsSignature: e.IsSignature, Category: e.Category,
			CreatedAt: e.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	writeMatchesJSON(w, map[string]any{
		"archetype":     archetype,
		"format":        format,
		"expectedCards": out,
		"total":         len(out),
	})
}

// ─── helpers ────────────────────────────────────────────────────────────────

func (h *OpponentsHandler) resolveAccount(w http.ResponseWriter, r *http.Request, op string) (int64, bool, bool) {
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return 0, false, false
	}
	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[OpponentsHandler.%s] GetAccountIDByUserID userID=%d: %v", op, userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return 0, false, false
	}
	return accountID, found, true
}

func opponentProfileToResponse(p repository.OpponentDeckProfileRow) opponentDeckProfileResponse {
	return opponentDeckProfileResponse{
		ID: p.ID, MatchID: p.MatchID,
		DetectedArchetype: p.DetectedArchetype, ArchetypeConfidence: p.ArchetypeConfidence,
		ColorIdentity: p.ColorIdentity, DeckStyle: p.DeckStyle,
		CardsObserved: p.CardsObserved, EstimatedDeckSize: p.EstimatedDeckSize,
		ObservedCardIDs: p.ObservedCardIDs, InferredCardIDs: p.InferredCardIDs,
		SignatureCards: p.SignatureCards, Format: p.Format, MetaArchetypeID: p.MetaArchetypeID,
		CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: p.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func observedCardsToResponse(rows []repository.OpponentObservedCardRow) []observedCardResponse {
	out := make([]observedCardResponse, 0, len(rows))
	for _, c := range rows {
		entry := observedCardResponse{
			CardID: c.CardID, CardName: derefOr(c.CardName, ""),
			Zone: derefOr(c.ZoneObserved, ""), TimesSeen: c.TimesSeen,
		}
		if c.TurnFirstSeen != nil {
			entry.TurnFirstSeen = *c.TurnFirstSeen
		}
		out = append(out, entry)
	}
	return out
}

// expectedCardsToResponse converts archetype expected-card rows into the
// SPA shape, marking wasSeen=true when the card_id is in the observed set.
func expectedCardsToResponse(rows []repository.ExpectedCardRow, observed []repository.OpponentObservedCardRow) []expectedCardResponse {
	seen := map[int]bool{}
	for _, c := range observed {
		seen[c.CardID] = true
	}
	out := make([]expectedCardResponse, 0, len(rows))
	for _, e := range rows {
		out = append(out, expectedCardResponse{
			CardID: e.CardID, CardName: e.CardName,
			InclusionRate: e.InclusionRate, AvgCopies: e.AvgCopies,
			WasSeen: seen[e.CardID], Category: derefOr(e.Category, ""),
			PlayAround: "", // play-around hint generation is a follow-up PR
		})
	}
	return out
}

func matchupRowToResponse(m repository.MatchupStatRow) matchupStatisticResponse {
	winRate := 0.0
	if m.TotalMatches > 0 {
		winRate = float64(m.Wins) / float64(m.TotalMatches)
	}
	resp := matchupStatisticResponse{
		ID: m.ID, AccountID: m.AccountID,
		PlayerArchetype: m.PlayerArchetype, OpponentArchetype: m.OpponentArchetype,
		Format: m.Format, TotalMatches: m.TotalMatches, Wins: m.Wins, Losses: m.Losses,
		WinRate: winRate, AvgGameDuration: m.AvgGameDuration,
		CreatedAt: m.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: m.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if m.LastMatchAt != nil {
		ts := m.LastMatchAt.UTC().Format(time.RFC3339)
		resp.LastMatchAt = &ts
	}
	return resp
}

func archetypeBreakdownToResponse(rows []repository.ArchetypeBreakdownRow, total int) []archetypeBreakdownEntryResponse {
	out := make([]archetypeBreakdownEntryResponse, 0, len(rows))
	for _, b := range rows {
		entry := archetypeBreakdownEntryResponse{Archetype: b.Archetype, Count: b.Count}
		if total > 0 {
			entry.Percentage = float64(b.Count) / float64(total)
		}
		if b.Count > 0 {
			entry.WinRate = float64(b.Wins) / float64(b.Count)
		}
		out = append(out, entry)
	}
	return out
}

func colorBreakdownToResponse(rows []repository.ColorIdentityBreakdownRow, total int) []colorIdentityStatsEntryResponse {
	out := make([]colorIdentityStatsEntryResponse, 0, len(rows))
	for _, c := range rows {
		entry := colorIdentityStatsEntryResponse{ColorIdentity: c.ColorIdentity, Count: c.Count}
		if total > 0 {
			entry.Percentage = float64(c.Count) / float64(total)
		}
		if c.Count > 0 {
			entry.WinRate = float64(c.Wins) / float64(c.Count)
		}
		out = append(out, entry)
	}
	return out
}
