// Phase 2 PR #4 — /api/v1/standard handlers.
//
// Replaces the SPA's daemonClient /standard surface. Responses use
// camelCase JSON keys to match the SPA's standard.ts TypeScript
// interfaces (StandardSet, UpcomingRotation, DeckValidationResult, etc.)
// and are wrapped in the {"data": ...} envelope apiClient expects.
//
// Auth: every route is guarded by DaemonAPIKeyAuth. Read-only / global
// data (sets, config, card legality) ignores account scope; deck-aware
// endpoints (validate, affected-decks) scope by accountID.

package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// rotationSoonDays is the window inside which a Standard set is considered
// "rotating soon" for SPA highlighting. 90 days mirrors the existing
// daemon contract.
const rotationSoonDays = 90

// standardReader is the minimal repo surface the handler needs.
type standardReader interface {
	ListStandardSets(ctx context.Context) ([]repository.StandardSetRow, error)
	GetStandardConfig(ctx context.Context) (repository.StandardConfigRow, error)
	CardByArenaID(ctx context.Context, arenaID int) (repository.CardLegalityRow, error)
	DeckByID(ctx context.Context, accountID int64, deckID string) (*repository.StandardDeckRow, error)
	DeckCardsForValidation(ctx context.Context, deckID string) ([]repository.DeckCardForValidation, error)
	ListAccountStandardDecks(ctx context.Context, accountID int64) ([]repository.AccountStandardDeckRow, error)
	SetByCode(ctx context.Context, code string) (*repository.StandardSetRow, error)
	CountStandardCardsAcrossSets(ctx context.Context, rotationDate string) (int, error)
	CountStandardSetsRotatingOn(ctx context.Context, rotationDate string) (int, error)
	SetsRotatingOn(ctx context.Context, rotationDate string) ([]repository.StandardSetRow, error)
}

// StandardHandler serves the cloud-data Phase 2 standard format API.
type StandardHandler struct {
	standard standardReader
	accounts AccountLookup
}

// NewStandardHandler returns a StandardHandler wired with the given repo +
// account lookup.
func NewStandardHandler(s standardReader, accounts AccountLookup) *StandardHandler {
	return &StandardHandler{standard: s, accounts: accounts}
}

// ─── wire shapes ────────────────────────────────────────────────────────────

type standardSetResponse struct {
	Code              string  `json:"code"`
	Name              string  `json:"name"`
	ReleasedAt        string  `json:"releasedAt"`
	RotationDate      *string `json:"rotationDate,omitempty"`
	IsStandardLegal   bool    `json:"isStandardLegal"`
	IconSvgURI        string  `json:"iconSvgUri"`
	CardCount         int     `json:"cardCount"`
	DaysUntilRotation *int    `json:"daysUntilRotation,omitempty"`
	IsRotatingSoon    bool    `json:"isRotatingSoon"`
}

type standardConfigResponse struct {
	ID               int    `json:"id"`
	NextRotationDate string `json:"nextRotationDate"`
	RotationEnabled  bool   `json:"rotationEnabled"`
	UpdatedAt        string `json:"updatedAt"`
}

type cardLegalityResponse struct {
	Standard  string `json:"standard"`
	Historic  string `json:"historic"`
	Explorer  string `json:"explorer"`
	Pioneer   string `json:"pioneer"`
	Modern    string `json:"modern"`
	Alchemy   string `json:"alchemy"`
	Brawl     string `json:"brawl"`
	Commander string `json:"commander"`
}

type rotatingCardResponse struct {
	CardID            int    `json:"cardId"`
	CardName          string `json:"cardName"`
	SetCode           string `json:"setCode"`
	SetName           string `json:"setName"`
	RotationDate      string `json:"rotationDate"`
	DaysUntilRotation int    `json:"daysUntilRotation"`
}

type deckSetInfoResponse struct {
	SetCode    string `json:"setCode"`
	SetName    string `json:"setName"`
	CardCount  int    `json:"cardCount"`
	IconSvgURI string `json:"iconSvgUri"`
	IsRotating bool   `json:"isRotating"`
}

type validationErrorResponse struct {
	CardID   int    `json:"cardId"`
	CardName string `json:"cardName"`
	Reason   string `json:"reason"`
	Details  string `json:"details"`
}

type validationWarningResponse struct {
	CardID   int    `json:"cardId"`
	CardName string `json:"cardName"`
	Type     string `json:"type"`
	Details  string `json:"details"`
}

type deckValidationResponse struct {
	IsLegal       bool                        `json:"isLegal"`
	Errors        []validationErrorResponse   `json:"errors"`
	Warnings      []validationWarningResponse `json:"warnings"`
	RotatingCards []rotatingCardResponse      `json:"rotatingCards"`
	SetBreakdown  []deckSetInfoResponse       `json:"setBreakdown"`
}

type rotationAffectedDeckResponse struct {
	DeckID            string                 `json:"deckId"`
	DeckName          string                 `json:"deckName"`
	Format            string                 `json:"format"`
	RotatingCardCount int                    `json:"rotatingCardCount"`
	TotalCards        int                    `json:"totalCards"`
	PercentAffected   float64                `json:"percentAffected"`
	RotatingCards     []rotatingCardResponse `json:"rotatingCards"`
}

type upcomingRotationResponse struct {
	NextRotationDate  string                `json:"nextRotationDate"`
	DaysUntilRotation int                   `json:"daysUntilRotation"`
	RotatingSets      []standardSetResponse `json:"rotatingSets"`
	RotatingCardCount int                   `json:"rotatingCardCount"`
	AffectedDecks     int                   `json:"affectedDecks"`
}

// ─── handlers ───────────────────────────────────────────────────────────────

// Sets handles GET /api/v1/standard/sets.
func (h *StandardHandler) Sets(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r, "Sets") {
		return
	}
	rows, err := h.standard.ListStandardSets(r.Context())
	if err != nil {
		log.Printf("[StandardHandler.Sets] ListStandardSets: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	now := time.Now().UTC()
	out := make([]standardSetResponse, 0, len(rows))
	for _, s := range rows {
		out = append(out, setRowToResponse(s, true, now))
	}
	writeMatchesJSON(w, out)
}

// Config handles GET /api/v1/standard/config.
func (h *StandardHandler) Config(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r, "Config") {
		return
	}
	c, err := h.standard.GetStandardConfig(r.Context())
	if err != nil {
		log.Printf("[StandardHandler.Config] GetStandardConfig: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, standardConfigResponse{
		ID:               c.ID,
		NextRotationDate: c.NextRotationDate,
		RotationEnabled:  c.RotationEnabled,
		UpdatedAt:        c.UpdatedAt.UTC().Format(time.RFC3339),
	})
}

// Rotation handles GET /api/v1/standard/rotation.
func (h *StandardHandler) Rotation(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Rotation")
	if !ok {
		return
	}
	cfg, err := h.standard.GetStandardConfig(r.Context())
	if err != nil {
		log.Printf("[StandardHandler.Rotation] GetStandardConfig: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	now := time.Now().UTC()
	days := daysUntilRotation(cfg.NextRotationDate, now)
	rotatingSets, err := h.standard.SetsRotatingOn(r.Context(), cfg.NextRotationDate)
	if err != nil {
		log.Printf("[StandardHandler.Rotation] SetsRotatingOn: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	cardCount, err := h.standard.CountStandardCardsAcrossSets(r.Context(), cfg.NextRotationDate)
	if err != nil {
		log.Printf("[StandardHandler.Rotation] CountStandardCardsAcrossSets: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	rotatingSetResponses := make([]standardSetResponse, 0, len(rotatingSets))
	for _, s := range rotatingSets {
		rotatingSetResponses = append(rotatingSetResponses, setRowToResponse(s, true, now))
	}

	resp := upcomingRotationResponse{
		NextRotationDate:  cfg.NextRotationDate,
		DaysUntilRotation: days,
		RotatingSets:      rotatingSetResponses,
		RotatingCardCount: cardCount,
	}
	if found {
		// Affected-deck count is per-account; skip for accountless callers.
		decks, err := h.standard.ListAccountStandardDecks(r.Context(), accountID)
		if err != nil {
			log.Printf("[StandardHandler.Rotation] ListAccountStandardDecks accountID=%d: %v", accountID, err)
			writeJSONError(w, "internal server error", http.StatusInternalServerError)
			return
		}
		// Count decks that contain at least one rotating-set card.
		rotatingSetCodes := make(map[string]struct{}, len(rotatingSets))
		for _, s := range rotatingSets {
			rotatingSetCodes[strings.ToLower(s.Code)] = struct{}{}
		}
		for _, d := range decks {
			cards, err := h.standard.DeckCardsForValidation(r.Context(), d.ID)
			if err != nil {
				log.Printf("[StandardHandler.Rotation] DeckCardsForValidation deckID=%s: %v", d.ID, err)
				continue
			}
			for _, c := range cards {
				if _, hit := rotatingSetCodes[strings.ToLower(c.SetCode)]; hit {
					resp.AffectedDecks++
					break
				}
			}
		}
	}
	writeMatchesJSON(w, resp)
}

// AffectedDecks handles GET /api/v1/standard/rotation/affected-decks.
func (h *StandardHandler) AffectedDecks(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "AffectedDecks")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, []rotationAffectedDeckResponse{})
		return
	}
	cfg, err := h.standard.GetStandardConfig(r.Context())
	if err != nil {
		log.Printf("[StandardHandler.AffectedDecks] GetStandardConfig: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	now := time.Now().UTC()
	decks, err := h.standard.ListAccountStandardDecks(r.Context(), accountID)
	if err != nil {
		log.Printf("[StandardHandler.AffectedDecks] ListAccountStandardDecks accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	out := make([]rotationAffectedDeckResponse, 0, len(decks))
	for _, d := range decks {
		cards, err := h.standard.DeckCardsForValidation(r.Context(), d.ID)
		if err != nil {
			log.Printf("[StandardHandler.AffectedDecks] DeckCardsForValidation deckID=%s: %v", d.ID, err)
			continue
		}
		entry := rotationAffectedDeckResponse{
			DeckID: d.ID, DeckName: d.Name, Format: d.Format,
			RotatingCards: []rotatingCardResponse{},
		}
		for _, c := range cards {
			entry.TotalCards += c.Quantity
			if c.RotationDate != nil && *c.RotationDate == cfg.NextRotationDate {
				entry.RotatingCardCount += c.Quantity
				entry.RotatingCards = append(entry.RotatingCards, rotatingCardResponse{
					CardID: c.CardID, CardName: c.Name,
					SetCode: c.SetCode, SetName: c.SetCode,
					RotationDate:      *c.RotationDate,
					DaysUntilRotation: daysUntilRotation(*c.RotationDate, now),
				})
			}
		}
		if entry.TotalCards > 0 {
			entry.PercentAffected = float64(entry.RotatingCardCount) / float64(entry.TotalCards)
		}
		out = append(out, entry)
	}
	writeMatchesJSON(w, out)
}

// ValidateDeck handles POST /api/v1/standard/validate/{deckId}.
func (h *StandardHandler) ValidateDeck(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "ValidateDeck")
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
	deck, err := h.standard.DeckByID(r.Context(), accountID, deckID)
	if err != nil {
		log.Printf("[StandardHandler.ValidateDeck] DeckByID accountID=%d deckID=%s: %v", accountID, deckID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if deck == nil {
		writeJSONError(w, "deck not found", http.StatusNotFound)
		return
	}

	cfg, err := h.standard.GetStandardConfig(r.Context())
	if err != nil {
		log.Printf("[StandardHandler.ValidateDeck] GetStandardConfig: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	cards, err := h.standard.DeckCardsForValidation(r.Context(), deckID)
	if err != nil {
		log.Printf("[StandardHandler.ValidateDeck] DeckCardsForValidation deckID=%s: %v", deckID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	resp := deckValidationResponse{
		IsLegal:       true,
		Errors:        []validationErrorResponse{},
		Warnings:      []validationWarningResponse{},
		RotatingCards: []rotatingCardResponse{},
		SetBreakdown:  []deckSetInfoResponse{},
	}
	setCardCount := map[string]int{}
	setRotating := map[string]bool{}
	setNames := map[string]string{}
	for _, c := range cards {
		setCardCount[c.SetCode] += c.Quantity
		// Banned / not legal in standard → error.
		if status := standardLegalityFromJSON(c.Legalities); status != "legal" && status != "" {
			resp.IsLegal = false
			resp.Errors = append(resp.Errors, validationErrorResponse{
				CardID: c.CardID, CardName: c.Name, Reason: "not_standard_legal",
				Details: "Card legality in standard: " + status,
			})
		} else if !c.SetIsStandardLegal && c.SetCode != "" {
			resp.IsLegal = false
			resp.Errors = append(resp.Errors, validationErrorResponse{
				CardID: c.CardID, CardName: c.Name, Reason: "set_not_standard_legal",
				Details: "Set " + c.SetCode + " is not Standard-legal",
			})
		}
		if c.RotationDate != nil && *c.RotationDate == cfg.NextRotationDate {
			setRotating[c.SetCode] = true
			resp.RotatingCards = append(resp.RotatingCards, rotatingCardResponse{
				CardID: c.CardID, CardName: c.Name,
				SetCode: c.SetCode, SetName: c.SetCode,
				RotationDate:      *c.RotationDate,
				DaysUntilRotation: daysUntilRotation(*c.RotationDate, now),
			})
		}
	}

	// Resolve set names + icon for the breakdown.
	for code, count := range setCardCount {
		info := deckSetInfoResponse{SetCode: code, CardCount: count, IsRotating: setRotating[code]}
		if name, hit := setNames[code]; hit {
			info.SetName = name
		} else {
			s, err := h.standard.SetByCode(r.Context(), code)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				log.Printf("[StandardHandler.ValidateDeck] SetByCode code=%s: %v", code, err)
			}
			if s != nil {
				info.SetName = s.Name
				info.IconSvgURI = s.IconSvgURI
				setNames[code] = s.Name
			} else {
				info.SetName = code
			}
		}
		resp.SetBreakdown = append(resp.SetBreakdown, info)
	}
	writeMatchesJSON(w, resp)
}

// CardLegality handles GET /api/v1/standard/cards/{arenaId}/legality.
func (h *StandardHandler) CardLegality(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r, "CardLegality") {
		return
	}
	arenaIDStr := strings.TrimSpace(chi.URLParam(r, "arenaId"))
	if arenaIDStr == "" {
		writeJSONError(w, "arenaId is required", http.StatusBadRequest)
		return
	}
	arenaID, err := strconv.Atoi(arenaIDStr)
	if err != nil || arenaID <= 0 {
		writeJSONError(w, "arenaId must be a positive integer", http.StatusBadRequest)
		return
	}
	row, err := h.standard.CardByArenaID(r.Context(), arenaID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, "card not found", http.StatusNotFound)
			return
		}
		log.Printf("[StandardHandler.CardLegality] CardByArenaID arenaID=%d: %v", arenaID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, parseLegalitiesJSON(row.Legalities))
}

// ─── helpers ────────────────────────────────────────────────────────────────

// requireAuth handles the user-id-from-context check for endpoints that do
// not need an account_id (sets / config / card-legality). Returns false
// after writing 401 when the request is unauthenticated.
func (h *StandardHandler) requireAuth(w http.ResponseWriter, r *http.Request, op string) bool {
	if _, ok := bffmiddleware.UserIDFromContext(r.Context()); !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	_ = op // op kept for symmetry / future logging
	return true
}

// resolveAccount mirrors the helper used by other Phase 2 handlers. found=
// false means the user has no accounts row yet — caller decides what an
// empty payload looks like.
func (h *StandardHandler) resolveAccount(w http.ResponseWriter, r *http.Request, op string) (int64, bool, bool) {
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return 0, false, false
	}
	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[StandardHandler.%s] GetAccountIDByUserID userID=%d: %v", op, userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return 0, false, false
	}
	return accountID, found, true
}

// setRowToResponse converts a repo row into the wire shape, computing
// daysUntilRotation + isRotatingSoon from rotation_date.
func setRowToResponse(s repository.StandardSetRow, isStandard bool, now time.Time) standardSetResponse {
	resp := standardSetResponse{
		Code: s.Code, Name: s.Name, ReleasedAt: s.ReleasedAt,
		RotationDate:    s.RotationDate,
		IsStandardLegal: isStandard,
		IconSvgURI:      s.IconSvgURI, CardCount: s.CardCount,
	}
	if s.RotationDate != nil && *s.RotationDate != "" {
		days := daysUntilRotation(*s.RotationDate, now)
		resp.DaysUntilRotation = &days
		resp.IsRotatingSoon = days >= 0 && days <= rotationSoonDays
	}
	return resp
}

// daysUntilRotation parses a YYYY-MM-DD or RFC3339 date and returns the
// integer number of days from now until that date. Returns 0 on parse
// failure (so callers don't have to special-case bad data).
func daysUntilRotation(date string, now time.Time) int {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		t, err = time.Parse(time.RFC3339, date)
		if err != nil {
			return 0
		}
	}
	hours := t.Sub(now).Hours()
	return int(math.Ceil(hours / 24))
}

// parseLegalitiesJSON parses the cards.legalities TEXT column (Scryfall
// JSON shape) into the SPA's CardLegality response. Missing keys default
// to "not_legal" so the SPA renders a complete grid.
func parseLegalitiesJSON(raw string) cardLegalityResponse {
	def := "not_legal"
	resp := cardLegalityResponse{
		Standard: def, Historic: def, Explorer: def, Pioneer: def,
		Modern: def, Alchemy: def, Brawl: def, Commander: def,
	}
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" || raw == "{}" {
		return resp
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return resp
	}
	if v, ok := m["standard"]; ok && v != "" {
		resp.Standard = v
	}
	if v, ok := m["historic"]; ok && v != "" {
		resp.Historic = v
	}
	if v, ok := m["explorer"]; ok && v != "" {
		resp.Explorer = v
	}
	if v, ok := m["pioneer"]; ok && v != "" {
		resp.Pioneer = v
	}
	if v, ok := m["modern"]; ok && v != "" {
		resp.Modern = v
	}
	if v, ok := m["alchemy"]; ok && v != "" {
		resp.Alchemy = v
	}
	if v, ok := m["brawl"]; ok && v != "" {
		resp.Brawl = v
	}
	if v, ok := m["commander"]; ok && v != "" {
		resp.Commander = v
	}
	return resp
}

// standardLegalityFromJSON returns the "standard" key from a legalities
// JSON blob, normalised to lowercase. Returns empty string when the blob
// is missing, malformed, or has no standard entry.
func standardLegalityFromJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" || raw == "{}" {
		return ""
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return ""
	}
	if v, ok := m["standard"]; ok {
		return strings.ToLower(strings.TrimSpace(v))
	}
	return ""
}
