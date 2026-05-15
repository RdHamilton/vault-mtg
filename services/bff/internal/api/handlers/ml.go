// Phase 2 PR #11 — ml-suggestions + synergy + play-patterns handler.
//
// Replaces the SPA's daemonClient surface for mlSuggestions.ts. Mounts:
//
//   GET    /api/v1/decks/{deckId}/ml-suggestions[?active=]   list (alias)
//   POST   /api/v1/decks/{deckId}/ml-suggestions/generate    STUB (alias)
//   PUT    /api/v1/ml-suggestions/{id}/dismiss               dismiss (alias)
//   PUT    /api/v1/ml-suggestions/{id}/apply                 apply
//   GET    /api/v1/decks/{deckId}/synergy-report             synergy report
//   GET    /api/v1/cards/{cardId}/synergies?format=&limit=   per-card synergies
//   GET    /api/v1/ml/combinations?card1=&card2=&format=     exact pair lookup
//   POST   /api/v1/ml/process-history?format=&days=          STUB
//   GET    /api/v1/ml/play-patterns[?account_id=]            play patterns read
//   POST   /api/v1/ml/play-patterns/update[?account_id=]     STUB upsert
//   DELETE /api/v1/ml/learned-data                           account-scoped wipe
//
// All routes guarded by DaemonAPIKeyAuth. Account ownership is enforced in
// the repository layer (joins on decks.account_id). Generate-suggestions
// and process-history are documented STUBs — the real ML pipeline lives
// outside this PR.
//
// Three of the eleven endpoints (list / generate / dismiss) are aliases for
// NotesRepository methods that PR #7 already wired under
// /api/v1/decks/{id}/suggestions and /api/v1/suggestions/{id}/dismiss.
// We mount them under the /ml-suggestions/* prefix that the SPA module
// expects without duplicating the read/dismiss code.

package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// mlSuggestionsReader is the subset of NotesRepository that MLHandler reuses
// for the list / dismiss aliases.
type mlSuggestionsReader interface {
	ListSuggestions(ctx context.Context, accountID int64, deckID string, activeOnly bool) ([]repository.SuggestionRow, error)
	DismissSuggestion(ctx context.Context, accountID, suggestionID int64) (bool, error)
}

// mlReader is the new MLRepository surface: synergy + play-patterns + apply
// + clear.
type mlReader interface {
	ApplySuggestion(ctx context.Context, accountID, suggestionID int64) (bool, error)
	SynergyReport(ctx context.Context, accountID int64, deckID string) (*repository.SynergyReportRow, error)
	CardSynergies(ctx context.Context, cardID int, format string, limit int) ([]repository.CardCombinationStatsRow, error)
	CombinationStats(ctx context.Context, card1, card2 int, format string) (*repository.CardCombinationStatsRow, error)
	PlayPatterns(ctx context.Context, accountIDText string) (*repository.UserPlayPatternsRow, error)
	UpsertPlayPatternsStub(ctx context.Context, accountIDText string) (*repository.UserPlayPatternsRow, error)
	ClearLearnedDataForAccount(ctx context.Context, accountID int64, accountIDText string) error
}

// MLHandler serves the Phase 2 ml-suggestions + synergy + play-patterns surface.
type MLHandler struct {
	suggestions mlSuggestionsReader
	ml          mlReader
	accounts    AccountLookup
}

// NewMLHandler wires the handler with its read-side dependencies.
func NewMLHandler(s mlSuggestionsReader, m mlReader, accounts AccountLookup) *MLHandler {
	return &MLHandler{suggestions: s, ml: m, accounts: accounts}
}

// ─── wire shapes ──────────────────────────────────────────────────────────────

// mlSuggestionResponse mirrors the SPA's MLSuggestion type. Richer than
// notes.go's suggestionResponse: includes raw confidence + cardId/swap
// columns + apply timestamps so the panel can render the full card.
type mlSuggestionResponse struct {
	ID                    int64    `json:"id"`
	DeckID                string   `json:"deckId"`
	SuggestionType        string   `json:"suggestionType"`
	CardID                *int     `json:"cardId,omitempty"`
	CardName              *string  `json:"cardName,omitempty"`
	SwapForCardID         *int     `json:"swapForCardId,omitempty"`
	SwapForCardName       *string  `json:"swapForCardName,omitempty"`
	Confidence            float64  `json:"confidence"`
	ExpectedWinRateChange float64  `json:"expectedWinRateChange"`
	Title                 string   `json:"title"`
	Description           *string  `json:"description,omitempty"`
	Reasoning             *string  `json:"reasoning,omitempty"`
	Evidence              *string  `json:"evidence,omitempty"`
	IsDismissed           bool     `json:"isDismissed"`
	WasApplied            bool     `json:"wasApplied"`
	OutcomeWinRateChange  *float64 `json:"outcomeWinRateChange,omitempty"`
	CreatedAt             string   `json:"createdAt"`
	AppliedAt             *string  `json:"appliedAt,omitempty"`
	OutcomeRecordedAt     *string  `json:"outcomeRecordedAt,omitempty"`
}

// mlSuggestionResultResponse mirrors the SPA's MLSuggestionResult.
// generateMLSuggestions returns this richer shape (stub: empty
// synergyData/reasons until the ML pipeline lands).
type mlSuggestionResultResponse struct {
	Suggestion  mlSuggestionResponse `json:"suggestion"`
	SynergyData []cardSynergyInfo    `json:"synergyData"`
	Reasons     []mlSuggestionReason `json:"reasons"`
}

type cardSynergyInfo struct {
	CardID          int     `json:"cardId"`
	CardName        string  `json:"cardName"`
	SynergyScore    float64 `json:"synergyScore"`
	WinRateTogether float64 `json:"winRateTogether"`
	GamesTogether   int     `json:"gamesTogether"`
}

type mlSuggestionReason struct {
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Impact      float64 `json:"impact"`
	Confidence  float64 `json:"confidence"`
}

type cardCombinationStatsResponse struct {
	ID              int64   `json:"id"`
	CardID1         int     `json:"cardId1"`
	CardID2         int     `json:"cardId2"`
	DeckID          *string `json:"deckId,omitempty"`
	Format          string  `json:"format"`
	GamesTogether   int     `json:"gamesTogether"`
	GamesCard1Only  int     `json:"gamesCard1Only"`
	GamesCard2Only  int     `json:"gamesCard2Only"`
	WinsTogether    int     `json:"winsTogether"`
	WinsCard1Only   int     `json:"winsCard1Only"`
	WinsCard2Only   int     `json:"winsCard2Only"`
	SynergyScore    float64 `json:"synergyScore"`
	ConfidenceScore float64 `json:"confidenceScore"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

type cardPairSynergy struct {
	Card1ID       int     `json:"card1Id"`
	Card1Name     *string `json:"card1Name,omitempty"`
	Card2ID       int     `json:"card2Id"`
	Card2Name     *string `json:"card2Name,omitempty"`
	SynergyScore  float64 `json:"synergyScore"`
	GamesTogether int     `json:"gamesTogether"`
	WinRate       float64 `json:"winRate"`
}

type synergyReportResponse struct {
	DeckID          string            `json:"deckId"`
	CardCount       int               `json:"cardCount"`
	TotalPairs      int               `json:"totalPairs"`
	AvgSynergyScore float64           `json:"avgSynergyScore"`
	Synergies       []cardPairSynergy `json:"synergies"`
}

type userPlayPatternsResponse struct {
	ID                 int64   `json:"id"`
	AccountID          string  `json:"accountId"`
	PreferredArchetype *string `json:"preferredArchetype,omitempty"`
	AggroAffinity      float64 `json:"aggroAffinity"`
	MidrangeAffinity   float64 `json:"midrangeAffinity"`
	ControlAffinity    float64 `json:"controlAffinity"`
	ComboAffinity      float64 `json:"comboAffinity"`
	ColorPreferences   *string `json:"colorPreferences,omitempty"`
	AvgGameLength      float64 `json:"avgGameLength"`
	AggressionScore    float64 `json:"aggressionScore"`
	InteractionScore   float64 `json:"interactionScore"`
	TotalMatches       int     `json:"totalMatches"`
	TotalDecks         int     `json:"totalDecks"`
	CreatedAt          string  `json:"createdAt"`
	UpdatedAt          string  `json:"updatedAt"`
}

type mlStatusResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ─── ml_suggestions list / generate / dismiss / apply ────────────────────────

// ListMLSuggestions handles GET /api/v1/decks/{deckId}/ml-suggestions[?active=].
// Alias for the notes-side ListSuggestions read; emits the richer MLSuggestion shape.
func (h *MLHandler) ListMLSuggestions(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "ListMLSuggestions")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	if deckID == "" {
		writeJSONError(w, "deckId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, []mlSuggestionResponse{})
		return
	}
	activeOnly := true
	if v := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("active"))); v == "false" {
		activeOnly = false
	}
	rows, err := h.suggestions.ListSuggestions(r.Context(), accountID, deckID, activeOnly)
	if err != nil {
		log.Printf("[MLHandler.ListMLSuggestions] accountID=%d deckID=%s: %v", accountID, deckID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, suggestionRowsToMLResponse(rows))
}

// GenerateMLSuggestions handles POST /api/v1/decks/{deckId}/ml-suggestions/generate.
//
// STUB: returns the existing ml_suggestions list wrapped in MLSuggestionResult
// objects with empty synergyData/reasons. The real generation pipeline
// (ML model invocation) lives outside this PR.
func (h *MLHandler) GenerateMLSuggestions(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "GenerateMLSuggestions")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	if deckID == "" {
		writeJSONError(w, "deckId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, []mlSuggestionResultResponse{})
		return
	}
	rows, err := h.suggestions.ListSuggestions(r.Context(), accountID, deckID, true)
	if err != nil {
		log.Printf("[MLHandler.GenerateMLSuggestions] accountID=%d deckID=%s: %v", accountID, deckID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	out := make([]mlSuggestionResultResponse, 0, len(rows))
	for _, s := range rows {
		out = append(out, mlSuggestionResultResponse{
			Suggestion:  suggestionRowToMLResponse(s),
			SynergyData: []cardSynergyInfo{},
			Reasons:     parseReasonsBlob(s.Reasoning),
		})
	}
	writeMatchesJSON(w, out)
}

// DismissMLSuggestion handles PUT /api/v1/ml-suggestions/{suggestionId}/dismiss.
func (h *MLHandler) DismissMLSuggestion(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "DismissMLSuggestion")
	if !ok {
		return
	}
	suggestionID, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "suggestionId")), 10, 64)
	if err != nil || suggestionID <= 0 {
		writeJSONError(w, "suggestionId must be a positive integer", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "suggestion not found", http.StatusNotFound)
		return
	}
	dismissed, err := h.suggestions.DismissSuggestion(r.Context(), accountID, suggestionID)
	if err != nil {
		log.Printf("[MLHandler.DismissMLSuggestion] accountID=%d suggestionID=%d: %v", accountID, suggestionID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !dismissed {
		writeJSONError(w, "suggestion not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ApplyMLSuggestion handles PUT /api/v1/ml-suggestions/{suggestionId}/apply.
// Marks the suggestion as applied (was_applied=TRUE, applied_at=NOW()).
func (h *MLHandler) ApplyMLSuggestion(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "ApplyMLSuggestion")
	if !ok {
		return
	}
	suggestionID, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "suggestionId")), 10, 64)
	if err != nil || suggestionID <= 0 {
		writeJSONError(w, "suggestionId must be a positive integer", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "suggestion not found", http.StatusNotFound)
		return
	}
	applied, err := h.ml.ApplySuggestion(r.Context(), accountID, suggestionID)
	if err != nil {
		log.Printf("[MLHandler.ApplyMLSuggestion] accountID=%d suggestionID=%d: %v", accountID, suggestionID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !applied {
		writeJSONError(w, "suggestion not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── synergy / combinations ──────────────────────────────────────────────────

// SynergyReport handles GET /api/v1/decks/{deckId}/synergy-report.
func (h *MLHandler) SynergyReport(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "SynergyReport")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	if deckID == "" {
		writeJSONError(w, "deckId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "deck not found", http.StatusNotFound)
		return
	}
	row, err := h.ml.SynergyReport(r.Context(), accountID, deckID)
	if err != nil {
		log.Printf("[MLHandler.SynergyReport] accountID=%d deckID=%s: %v", accountID, deckID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if row == nil {
		writeJSONError(w, "deck not found", http.StatusNotFound)
		return
	}
	writeMatchesJSON(w, synergyReportRowToResponse(*row))
}

// CardSynergies handles GET /api/v1/cards/{cardId}/synergies?format=&limit=.
func (h *MLHandler) CardSynergies(w http.ResponseWriter, r *http.Request) {
	cardID, err := strconv.Atoi(strings.TrimSpace(chi.URLParam(r, "cardId")))
	if err != nil || cardID <= 0 {
		writeJSONError(w, "cardId must be a positive integer", http.StatusBadRequest)
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	if format == "" {
		format = "Standard"
	}
	limit := 10
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	rows, err := h.ml.CardSynergies(r.Context(), cardID, format, limit)
	if err != nil {
		log.Printf("[MLHandler.CardSynergies] cardID=%d format=%s: %v", cardID, format, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	out := make([]cardCombinationStatsResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, combinationStatsRowToResponse(row))
	}
	writeMatchesJSON(w, out)
}

// CombinationStats handles GET /api/v1/ml/combinations?card1=&card2=&format=.
func (h *MLHandler) CombinationStats(w http.ResponseWriter, r *http.Request) {
	card1, err1 := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("card1")))
	card2, err2 := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("card2")))
	if err1 != nil || err2 != nil || card1 <= 0 || card2 <= 0 {
		writeJSONError(w, "card1 and card2 must be positive integers", http.StatusBadRequest)
		return
	}
	if card1 == card2 {
		writeJSONError(w, "card1 and card2 must be different", http.StatusBadRequest)
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	if format == "" {
		format = "Standard"
	}
	row, err := h.ml.CombinationStats(r.Context(), card1, card2, format)
	if err != nil {
		log.Printf("[MLHandler.CombinationStats] card1=%d card2=%d format=%s: %v", card1, card2, format, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if row == nil {
		writeJSONError(w, "combination not found", http.StatusNotFound)
		return
	}
	writeMatchesJSON(w, combinationStatsRowToResponse(*row))
}

// ─── ML management ───────────────────────────────────────────────────────────

// ProcessMatchHistory handles POST /api/v1/ml/process-history?format=&days=.
//
// STUB: the real pipeline ingests match history into card_combination_stats
// via the analytics worker. Returns a structured ok payload so the SPA
// progress UI can render.
func (h *MLHandler) ProcessMatchHistory(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.resolveAccount(w, r, "ProcessMatchHistory"); !ok {
		return
	}
	writeMatchesJSON(w, mlStatusResponse{
		Status:  "queued",
		Message: "Match history processing is queued. The analytics pipeline will populate synergy data on its next run.",
	})
}

// GetUserPlayPatterns handles GET /api/v1/ml/play-patterns[?account_id=].
// The account_id query param is ignored — patterns always resolve to the
// authenticated user — to prevent cross-account reads.
func (h *MLHandler) GetUserPlayPatterns(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "GetUserPlayPatterns")
	if !ok {
		return
	}
	if !found {
		writeJSONError(w, "play patterns not found", http.StatusNotFound)
		return
	}
	row, err := h.ml.PlayPatterns(r.Context(), repository.AccountIDToText(accountID))
	if err != nil {
		log.Printf("[MLHandler.GetUserPlayPatterns] accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if row == nil {
		// No row yet — return a defaulted shape (account_id stringified) so
		// the SPA can render zeros without a 404 spinner.
		writeMatchesJSON(w, defaultPlayPatternsResponse(accountID))
		return
	}
	writeMatchesJSON(w, playPatternsRowToResponse(*row))
}

// UpdateUserPlayPatterns handles POST /api/v1/ml/play-patterns/update[?account_id=].
//
// STUB: ensures a row exists and bumps updated_at. The real recompute
// (aggro/midrange/control/combo affinity from matches.archetype + game
// telemetry) runs in the analytics worker.
func (h *MLHandler) UpdateUserPlayPatterns(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "UpdateUserPlayPatterns")
	if !ok {
		return
	}
	if !found {
		writeJSONError(w, "account not found", http.StatusNotFound)
		return
	}
	if _, err := h.ml.UpsertPlayPatternsStub(r.Context(), repository.AccountIDToText(accountID)); err != nil {
		log.Printf("[MLHandler.UpdateUserPlayPatterns] accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, mlStatusResponse{
		Status:  "ok",
		Message: "Play patterns refreshed. Detailed recompute runs on the analytics pipeline.",
	})
}

// ClearLearnedData handles DELETE /api/v1/ml/learned-data.
// Account-scoped: wipes ml_suggestions for the user's decks +
// user_play_patterns row for this account. Global card_combination_stats
// learnings (cross-user) are not touched.
func (h *MLHandler) ClearLearnedData(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "ClearLearnedData")
	if !ok {
		return
	}
	if !found {
		writeJSONError(w, "account not found", http.StatusNotFound)
		return
	}
	if err := h.ml.ClearLearnedDataForAccount(r.Context(), accountID, repository.AccountIDToText(accountID)); err != nil {
		log.Printf("[MLHandler.ClearLearnedData] accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, mlStatusResponse{
		Status:  "ok",
		Message: "Cleared account-scoped learned data (suggestions + play patterns). Global synergy learnings retained.",
	})
}

// ─── helpers ────────────────────────────────────────────────────────────────

func (h *MLHandler) resolveAccount(w http.ResponseWriter, r *http.Request, op string) (int64, bool, bool) {
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return 0, false, false
	}
	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[MLHandler.%s] GetAccountIDByUserID userID=%d: %v", op, userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return 0, false, false
	}
	return accountID, found, true
}

func suggestionRowsToMLResponse(rows []repository.SuggestionRow) []mlSuggestionResponse {
	out := make([]mlSuggestionResponse, 0, len(rows))
	for _, s := range rows {
		out = append(out, suggestionRowToMLResponse(s))
	}
	return out
}

func suggestionRowToMLResponse(s repository.SuggestionRow) mlSuggestionResponse {
	resp := mlSuggestionResponse{
		ID:                    s.ID,
		DeckID:                s.DeckID,
		SuggestionType:        s.SuggestionType,
		CardID:                s.CardID,
		CardName:              s.CardName,
		SwapForCardID:         s.SwapForCardID,
		SwapForCardName:       s.SwapForCardName,
		Confidence:            s.Confidence,
		ExpectedWinRateChange: s.ExpectedWinRateChange,
		Title:                 s.Title,
		Description:           s.Description,
		Reasoning:             s.Reasoning,
		Evidence:              s.Evidence,
		IsDismissed:           s.IsDismissed,
		WasApplied:            s.WasApplied,
		OutcomeWinRateChange:  s.OutcomeWinRateChange,
		CreatedAt:             s.CreatedAt.UTC().Format(time.RFC3339),
	}
	if s.AppliedAt != nil {
		v := s.AppliedAt.UTC().Format(time.RFC3339)
		resp.AppliedAt = &v
	}
	if s.OutcomeRecordedAt != nil {
		v := s.OutcomeRecordedAt.UTC().Format(time.RFC3339)
		resp.OutcomeRecordedAt = &v
	}
	return resp
}

func parseReasonsBlob(blob *string) []mlSuggestionReason {
	if blob == nil || strings.TrimSpace(*blob) == "" {
		return []mlSuggestionReason{}
	}
	var reasons []mlSuggestionReason
	if err := json.Unmarshal([]byte(*blob), &reasons); err != nil {
		return []mlSuggestionReason{}
	}
	return reasons
}

func combinationStatsRowToResponse(c repository.CardCombinationStatsRow) cardCombinationStatsResponse {
	return cardCombinationStatsResponse{
		ID:              c.ID,
		CardID1:         c.CardID1,
		CardID2:         c.CardID2,
		DeckID:          c.DeckID,
		Format:          c.Format,
		GamesTogether:   c.GamesTogether,
		GamesCard1Only:  c.GamesCard1Only,
		GamesCard2Only:  c.GamesCard2Only,
		WinsTogether:    c.WinsTogether,
		WinsCard1Only:   c.WinsCard1Only,
		WinsCard2Only:   c.WinsCard2Only,
		SynergyScore:    c.SynergyScore,
		ConfidenceScore: c.ConfidenceScore,
		CreatedAt:       c.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       c.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func synergyReportRowToResponse(s repository.SynergyReportRow) synergyReportResponse {
	pairs := make([]cardPairSynergy, 0, len(s.Synergies))
	for _, p := range s.Synergies {
		pairs = append(pairs, cardPairSynergy{
			Card1ID: p.Card1ID, Card1Name: p.Card1Name,
			Card2ID: p.Card2ID, Card2Name: p.Card2Name,
			SynergyScore:  p.SynergyScore,
			GamesTogether: p.GamesTogether,
			WinRate:       p.WinRate,
		})
	}
	return synergyReportResponse{
		DeckID:          s.DeckID,
		CardCount:       s.CardCount,
		TotalPairs:      s.TotalPairs,
		AvgSynergyScore: s.AvgSynergyScore,
		Synergies:       pairs,
	}
}

func playPatternsRowToResponse(u repository.UserPlayPatternsRow) userPlayPatternsResponse {
	return userPlayPatternsResponse{
		ID:                 u.ID,
		AccountID:          u.AccountIDText,
		PreferredArchetype: u.PreferredArchetype,
		AggroAffinity:      u.AggroAffinity,
		MidrangeAffinity:   u.MidrangeAffinity,
		ControlAffinity:    u.ControlAffinity,
		ComboAffinity:      u.ComboAffinity,
		ColorPreferences:   u.ColorPreferences,
		AvgGameLength:      u.AvgGameLength,
		AggressionScore:    u.AggressionScore,
		InteractionScore:   u.InteractionScore,
		TotalMatches:       u.TotalMatches,
		TotalDecks:         u.TotalDecks,
		CreatedAt:          u.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:          u.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func defaultPlayPatternsResponse(accountID int64) userPlayPatternsResponse {
	now := time.Now().UTC().Format(time.RFC3339)
	return userPlayPatternsResponse{
		AccountID: repository.AccountIDToText(accountID),
		CreatedAt: now,
		UpdatedAt: now,
	}
}
