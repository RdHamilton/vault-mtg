// Phase 2 PR #8 — /api/v1/cards/* handlers.
//
// Replaces the SPA's daemonClient surface for cards.ts. 16 endpoints
// across multiple sub-prefixes:
//   - GET    /cards (search), /cards/{arenaId}, /cards/sets,
//            /cards/sets/{setCode}/cards
//   - GET    /cards/ratings/{setCode}/{format}[/staleness]
//            /cards/ratings/{setCode}/colors
//   - POST   /cards/ratings/{setCode}/refresh                (STUB)
//   - POST   /cards/collection-quantities, /cards/search-with-collection
//   - GET    /cards/cfb/{setCode}[/count|/card/{name}]
//   - POST   /cards/cfb/import, /cards/cfb/{setCode}/link-arena-ids
//   - DELETE /cards/cfb/{setCode}
//
// All routes are guarded by DaemonAPIKeyAuth + the standard envelope.
// Most reads are global (catalog data, no account scope);
// /cards/collection-quantities and /cards/search-with-collection are the
// two account-scoped endpoints — they join card_inventory.
//
// `refreshSetRatings` is a STUB: the SPA's "Refresh from 17Lands" button
// triggers a cached_at bump only — the actual scrape pipeline lives in
// services/sync (Lambda). Documented inline.

package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	bffmiddleware "github.com/RdHamilton/vault-mtg/services/bff/internal/api/middleware"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
	"github.com/go-chi/chi/v5"
)

// cardsReader is the minimal repo surface the handler needs.
type cardsReader interface {
	SearchCards(ctx context.Context, query, setCode string, limit int) ([]repository.SetCardRow, error)
	CardByArenaID(ctx context.Context, arenaID int) (*repository.SetCardRow, error)
	CardsBySetCode(ctx context.Context, setCode string) ([]repository.SetCardRow, error)
	SearchCardsWithCollection(ctx context.Context, accountID int64, query string, sets []string, limit int) ([]repository.SetCardWithQty, error)
	CollectionQuantities(ctx context.Context, accountID int64, arenaIDs []int) (map[int]int, error)
	AllSetInfo(ctx context.Context) ([]repository.SetInfoRow, error)
	CardRatings(ctx context.Context, setCode, format string) ([]repository.CardRatingRow, error)
	ColorRatings(ctx context.Context, setCode string) ([]repository.ColorRatingRow, error)
	RatingsStaleness(ctx context.Context, setCode, format string) (repository.RatingsStalenessRow, error)
	TouchRatingsCachedAt(ctx context.Context, setCode, format string) error
	CFBRatingsBySet(ctx context.Context, setCode string) ([]repository.CFBRatingRow, error)
	CFBRatingByCard(ctx context.Context, setCode, cardName string) (*repository.CFBRatingRow, error)
	CFBRatingsCount(ctx context.Context, setCode string) (int, error)
	ImportCFBRatings(ctx context.Context, imports []repository.CFBImport) (int, error)
	LinkCFBArenaIds(ctx context.Context, setCode string) (int, error)
	DeleteCFBRatings(ctx context.Context, setCode string) (int, error)
}

// CardsHandler serves the cloud-data Phase 2 cards API.
type CardsHandler struct {
	cards    cardsReader
	accounts AccountLookup
}

// NewCardsHandler returns a CardsHandler wired with the given repo + lookup.
func NewCardsHandler(c cardsReader, accounts AccountLookup) *CardsHandler {
	return &CardsHandler{cards: c, accounts: accounts}
}

// ─── wire shapes ────────────────────────────────────────────────────────────

// setCardResponse mirrors models.SetCard. PascalCase to match the SPA's
// existing models.SetCard TS class.
type setCardResponse struct {
	ID            int      `json:"ID"`
	SetCode       string   `json:"SetCode"`
	ArenaID       string   `json:"ArenaID"`
	ScryfallID    string   `json:"ScryfallID"`
	Name          string   `json:"Name"`
	ManaCost      string   `json:"ManaCost"`
	CMC           float64  `json:"CMC"`
	Types         []string `json:"Types"`
	Colors        []string `json:"Colors"`
	Rarity        string   `json:"Rarity"`
	Text          string   `json:"Text"`
	Power         string   `json:"Power"`
	Toughness     string   `json:"Toughness"`
	ImageURL      string   `json:"ImageURL"`
	ImageURLSmall string   `json:"ImageURLSmall"`
	ImageURLArt   string   `json:"ImageURLArt"`
	FetchedAt     string   `json:"FetchedAt"`
}

// setCardWithQtyResponse extends setCardResponse with a quantity field
// for /cards/search-with-collection.
type setCardWithQtyResponse struct {
	setCardResponse
	Quantity int `json:"quantity"`
}

// setInfoResponse mirrors gui.SetInfo.
type setInfoResponse struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	IconSvgURI string `json:"iconSvgUri"`
	SetType    string `json:"setType"`
	ReleasedAt string `json:"releasedAt"`
	CardCount  int    `json:"cardCount"`
}

// cardRatingResponse mirrors gui.CardRatingWithTier (snake_case + literal
// "# foo" count keys, per the existing SPA TS class).
type cardRatingResponse struct {
	Name                          string   `json:"name"`
	Color                         string   `json:"color"`
	Rarity                        string   `json:"rarity"`
	MTGAID                        *int     `json:"mtga_id,omitempty"`
	EverDrawnWinRate              float64  `json:"ever_drawn_win_rate"`
	OpeningHandWinRate            float64  `json:"opening_hand_win_rate"`
	EverDrawnGameWinRate          float64  `json:"ever_drawn_game_win_rate"`
	DrawnWinRate                  float64  `json:"drawn_win_rate"`
	InHandWinRate                 float64  `json:"in_hand_win_rate"`
	EverDrawnImprovementWinRate   float64  `json:"ever_drawn_improvement_win_rate"`
	OpeningHandImprovementWinRate float64  `json:"opening_hand_improvement_win_rate"`
	DrawnImprovementWinRate       float64  `json:"drawn_improvement_win_rate"`
	InHandImprovementWinRate      float64  `json:"in_hand_improvement_win_rate"`
	AvgSeen                       float64  `json:"avg_seen"`
	AvgPick                       float64  `json:"avg_pick"`
	PickRate                      float64  `json:"pick_rate,omitempty"`
	NumEverDrawn                  int      `json:"# ever_drawn"`
	NumOpeningHand                int      `json:"# opening_hand"`
	NumGames                      int      `json:"# games"`
	NumDrawn                      int      `json:"# drawn"`
	NumInHandDrawn                int      `json:"# in_hand_drawn"`
	NumGamesPlayed                int      `json:"# games_played,omitempty"`
	NumDecks                      int      `json:"# decks,omitempty"`
	Tier                          string   `json:"tier"`
	Colors                        []string `json:"colors"`
	URL                           string   `json:"url,omitempty"`
}

// colorRatingResponse mirrors seventeenlands.ColorRating (snake_case).
type colorRatingResponse struct {
	ColorCombination string  `json:"color_combination"`
	WinRate          float64 `json:"win_rate"`
	GamesPlayed      int     `json:"games_played"`
}

// ratingsStalenessResponse mirrors the SPA's RatingsStaleness interface.
type ratingsStalenessResponse struct {
	CachedAt  string `json:"cachedAt"`
	IsStale   bool   `json:"isStale"`
	CardCount int    `json:"cardCount"`
}

// cfbRatingResponse mirrors the SPA's CFBRating interface.
type cfbRatingResponse struct {
	ID                int64   `json:"id"`
	CardName          string  `json:"cardName"`
	SetCode           string  `json:"setCode"`
	ArenaID           *int    `json:"arenaId,omitempty"`
	LimitedRating     float64 `json:"limitedRating"`
	LimitedScore      float64 `json:"limitedScore"`
	ConstructedRating *string `json:"constructedRating,omitempty"`
	ConstructedScore  float64 `json:"constructedScore,omitempty"`
	ArchetypeFit      *string `json:"archetypeFit,omitempty"`
	Commentary        *string `json:"commentary,omitempty"`
	SourceURL         *string `json:"sourceUrl,omitempty"`
	Author            *string `json:"author,omitempty"`
	ImportedAt        string  `json:"importedAt"`
	UpdatedAt         string  `json:"updatedAt"`
}

// collectionQuantitiesRequest mirrors the SPA's POST body for
// /cards/collection-quantities.
type collectionQuantitiesRequest struct {
	ArenaIDs []int `json:"arenaIDs"`
}

// searchWithCollectionRequest mirrors the SPA's POST body for
// /cards/search-with-collection.
type searchWithCollectionRequest struct {
	Query    string   `json:"query"`
	SetCodes []string `json:"setCodes,omitempty"`
	Limit    int      `json:"limit,omitempty"`
}

// cfbImportRow mirrors one entry in the SPA's importCFBRatings POST body.
type cfbImportRow struct {
	CardName          string  `json:"card_name"`
	SetCode           string  `json:"set_code"`
	LimitedRating     float64 `json:"limited_rating"`
	ConstructedRating *string `json:"constructed_rating,omitempty"`
	ArchetypeFit      *string `json:"archetype_fit,omitempty"`
	Commentary        *string `json:"commentary,omitempty"`
	SourceURL         *string `json:"source_url,omitempty"`
	Author            *string `json:"author,omitempty"`
}

// cfbImportRequest is the POST body for /cards/cfb/import.
type cfbImportRequest struct {
	Ratings []cfbImportRow `json:"ratings"`
}

// refreshRequest is the optional POST body for /cards/ratings/{set}/refresh.
type refreshRequest struct {
	Format string `json:"format"`
}

// ─── handlers ───────────────────────────────────────────────────────────────

// Search handles GET /api/v1/cards?q=...&set=...&limit=N.
func (h *CardsHandler) Search(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSONError(w, "q is required", http.StatusBadRequest)
		return
	}
	setCode := strings.TrimSpace(r.URL.Query().Get("set"))
	limit := parseLimitDefault(r, "limit", 50)
	rows, err := h.cards.SearchCards(r.Context(), q, setCode, limit)
	if err != nil {
		log.Printf("[CardsHandler.Search] q=%q: %v", q, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, setCardRowsToResponse(rows))
}

// GetByArenaID handles GET /api/v1/cards/{arenaId}.
func (h *CardsHandler) GetByArenaID(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	arenaID, err := strconv.Atoi(strings.TrimSpace(chi.URLParam(r, "arenaId")))
	if err != nil || arenaID <= 0 {
		writeJSONError(w, "arenaId must be a positive integer", http.StatusBadRequest)
		return
	}
	row, err := h.cards.CardByArenaID(r.Context(), arenaID)
	if err != nil {
		log.Printf("[CardsHandler.GetByArenaID] arenaID=%d: %v", arenaID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if row == nil {
		writeJSONError(w, "card not found", http.StatusNotFound)
		return
	}
	writeMatchesJSON(w, setCardRowToResponse(*row))
}

// AllSets handles GET /api/v1/cards/sets.
func (h *CardsHandler) AllSets(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	rows, err := h.cards.AllSetInfo(r.Context())
	if err != nil {
		log.Printf("[CardsHandler.AllSets] AllSetInfo: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	out := make([]setInfoResponse, 0, len(rows))
	for _, s := range rows {
		out = append(out, setInfoResponse{
			Code: s.Code, Name: s.Name, IconSvgURI: s.IconSvgURI,
			SetType: s.SetType, ReleasedAt: s.ReleasedAt, CardCount: s.CardCount,
		})
	}
	writeMatchesJSON(w, out)
}

// SetCards handles GET /api/v1/cards/sets/{setCode}/cards.
func (h *CardsHandler) SetCards(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	setCode := strings.TrimSpace(chi.URLParam(r, "setCode"))
	if setCode == "" {
		writeJSONError(w, "setCode is required", http.StatusBadRequest)
		return
	}
	rows, err := h.cards.CardsBySetCode(r.Context(), setCode)
	if err != nil {
		log.Printf("[CardsHandler.SetCards] setCode=%s: %v", setCode, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, setCardRowsToResponse(rows))
}

// CardRatings handles GET /api/v1/cards/ratings/{setCode}/{format}.
// Sets X-Cache-Degraded + X-Cache-Age-Hours headers from staleness.
func (h *CardsHandler) CardRatings(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	setCode := strings.TrimSpace(chi.URLParam(r, "setCode"))
	format := strings.TrimSpace(chi.URLParam(r, "format"))
	if setCode == "" || format == "" {
		writeJSONError(w, "setCode and format are required", http.StatusBadRequest)
		return
	}
	rows, err := h.cards.CardRatings(r.Context(), setCode, format)
	if err != nil {
		log.Printf("[CardsHandler.CardRatings] setCode=%s format=%s: %v", setCode, format, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	staleness, err := h.cards.RatingsStaleness(r.Context(), setCode, format)
	if err == nil && staleness.CachedAt != nil {
		ageHours := time.Since(*staleness.CachedAt).Hours()
		w.Header().Set("X-Cache-Age-Hours", strconv.FormatFloat(ageHours, 'f', 2, 64))
		if staleness.IsStale {
			w.Header().Set("X-Cache-Degraded", "true")
		}
	}
	writeMatchesJSON(w, cardRatingRowsToResponse(rows))
}

// ColorRatings handles GET /api/v1/cards/ratings/{setCode}/colors.
func (h *CardsHandler) ColorRatings(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	setCode := strings.TrimSpace(chi.URLParam(r, "setCode"))
	if setCode == "" {
		writeJSONError(w, "setCode is required", http.StatusBadRequest)
		return
	}
	rows, err := h.cards.ColorRatings(r.Context(), setCode)
	if err != nil {
		log.Printf("[CardsHandler.ColorRatings] setCode=%s: %v", setCode, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	out := make([]colorRatingResponse, 0, len(rows))
	for _, c := range rows {
		entry := colorRatingResponse{ColorCombination: c.ColorCombination}
		if c.WinRate != nil {
			entry.WinRate = *c.WinRate
		}
		if c.GamesPlayed != nil {
			entry.GamesPlayed = *c.GamesPlayed
		}
		out = append(out, entry)
	}
	writeMatchesJSON(w, out)
}

// RatingsStaleness handles GET /api/v1/cards/ratings/{setCode}/{format}/staleness.
func (h *CardsHandler) RatingsStaleness(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	setCode := strings.TrimSpace(chi.URLParam(r, "setCode"))
	format := strings.TrimSpace(chi.URLParam(r, "format"))
	if setCode == "" || format == "" {
		writeJSONError(w, "setCode and format are required", http.StatusBadRequest)
		return
	}
	s, err := h.cards.RatingsStaleness(r.Context(), setCode, format)
	if err != nil {
		log.Printf("[CardsHandler.RatingsStaleness] setCode=%s format=%s: %v", setCode, format, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	resp := ratingsStalenessResponse{IsStale: s.IsStale, CardCount: s.CardCount}
	if s.CachedAt != nil {
		resp.CachedAt = s.CachedAt.UTC().Format(time.RFC3339)
	}
	writeMatchesJSON(w, resp)
}

// RefreshRatings handles POST /api/v1/cards/ratings/{setCode}/refresh.
//
// STUB: bumps cached_at on existing draft_card_ratings rows for the (set,
// format) so the staleness check resets. The actual scrape pipeline
// (download from 17Lands) lives in services/sync (Lambda) and is not
// invoked from the BFF in this PR.
func (h *CardsHandler) RefreshRatings(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	setCode := strings.TrimSpace(chi.URLParam(r, "setCode"))
	if setCode == "" {
		writeJSONError(w, "setCode is required", http.StatusBadRequest)
		return
	}
	var req refreshRequest
	_ = decodeJSONBody(r, &req)
	format := strings.TrimSpace(req.Format)
	if format == "" {
		format = "PremierDraft"
	}
	if err := h.cards.TouchRatingsCachedAt(r.Context(), setCode, format); err != nil {
		log.Printf("[CardsHandler.RefreshRatings] setCode=%s format=%s: %v", setCode, format, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, map[string]any{"status": "refreshed", "set_code": setCode, "format": format})
}

// CollectionQuantities handles POST /api/v1/cards/collection-quantities.
func (h *CardsHandler) CollectionQuantities(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "CollectionQuantities")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, map[string]int{})
		return
	}
	var req collectionQuantitiesRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	out, err := h.cards.CollectionQuantities(r.Context(), accountID, req.ArenaIDs)
	if err != nil {
		log.Printf("[CardsHandler.CollectionQuantities] accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	// JSON map keys must be strings; the SPA's expected
	// Record<number, number> happily decodes string-keyed numbers.
	stringKeyed := make(map[string]int, len(out))
	for k, v := range out {
		stringKeyed[strconv.Itoa(k)] = v
	}
	writeMatchesJSON(w, stringKeyed)
}

// SearchWithCollection handles POST /api/v1/cards/search-with-collection.
func (h *CardsHandler) SearchWithCollection(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "SearchWithCollection")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, []setCardWithQtyResponse{})
		return
	}
	var req searchWithCollectionRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		writeJSONError(w, "query is required", http.StatusBadRequest)
		return
	}
	rows, err := h.cards.SearchCardsWithCollection(r.Context(), accountID, req.Query, req.SetCodes, req.Limit)
	if err != nil {
		log.Printf("[CardsHandler.SearchWithCollection] accountID=%d query=%q: %v", accountID, req.Query, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	out := make([]setCardWithQtyResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, setCardWithQtyResponse{
			setCardResponse: setCardRowToResponse(row.SetCardRow),
			Quantity:        row.Quantity,
		})
	}
	writeMatchesJSON(w, out)
}

// CFBRatings handles GET /api/v1/cards/cfb/{setCode}.
func (h *CardsHandler) CFBRatings(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	setCode := strings.TrimSpace(chi.URLParam(r, "setCode"))
	if setCode == "" {
		writeJSONError(w, "setCode is required", http.StatusBadRequest)
		return
	}
	rows, err := h.cards.CFBRatingsBySet(r.Context(), setCode)
	if err != nil {
		log.Printf("[CardsHandler.CFBRatings] setCode=%s: %v", setCode, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, cfbRatingRowsToResponse(rows))
}

// CFBRatingsCount handles GET /api/v1/cards/cfb/{setCode}/count.
func (h *CardsHandler) CFBRatingsCount(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	setCode := strings.TrimSpace(chi.URLParam(r, "setCode"))
	if setCode == "" {
		writeJSONError(w, "setCode is required", http.StatusBadRequest)
		return
	}
	count, err := h.cards.CFBRatingsCount(r.Context(), setCode)
	if err != nil {
		log.Printf("[CardsHandler.CFBRatingsCount] setCode=%s: %v", setCode, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, map[string]any{"set_code": setCode, "count": count})
}

// CFBRatingByCard handles GET /api/v1/cards/cfb/{setCode}/card/{cardName}.
func (h *CardsHandler) CFBRatingByCard(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	setCode := strings.TrimSpace(chi.URLParam(r, "setCode"))
	cardName := strings.TrimSpace(chi.URLParam(r, "cardName"))
	if setCode == "" || cardName == "" {
		writeJSONError(w, "setCode and cardName are required", http.StatusBadRequest)
		return
	}
	row, err := h.cards.CFBRatingByCard(r.Context(), setCode, cardName)
	if err != nil {
		log.Printf("[CardsHandler.CFBRatingByCard] setCode=%s cardName=%s: %v", setCode, cardName, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if row == nil {
		writeJSONError(w, "rating not found", http.StatusNotFound)
		return
	}
	writeMatchesJSON(w, cfbRatingRowToResponse(*row))
}

// ImportCFB handles POST /api/v1/cards/cfb/import.
func (h *CardsHandler) ImportCFB(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	var req cfbImportRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Ratings) == 0 {
		writeJSONError(w, "ratings must not be empty", http.StatusBadRequest)
		return
	}
	imports := make([]repository.CFBImport, 0, len(req.Ratings))
	for _, r := range req.Ratings {
		if strings.TrimSpace(r.CardName) == "" || strings.TrimSpace(r.SetCode) == "" {
			writeJSONError(w, "card_name and set_code are required on every row", http.StatusBadRequest)
			return
		}
		imports = append(imports, repository.CFBImport{
			CardName: r.CardName, SetCode: r.SetCode,
			LimitedRating:     r.LimitedRating,
			ConstructedRating: r.ConstructedRating,
			ArchetypeFit:      r.ArchetypeFit,
			Commentary:        r.Commentary,
			SourceURL:         r.SourceURL,
			Author:            r.Author,
		})
	}
	count, err := h.cards.ImportCFBRatings(r.Context(), imports)
	if err != nil {
		log.Printf("[CardsHandler.ImportCFB] count=%d: %v", count, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, map[string]any{
		"status": "ok", "imported": count,
		"message": strconv.Itoa(count) + " ratings upserted",
	})
}

// LinkCFBArenaIds handles POST /api/v1/cards/cfb/{setCode}/link-arena-ids.
func (h *CardsHandler) LinkCFBArenaIds(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	setCode := strings.TrimSpace(chi.URLParam(r, "setCode"))
	if setCode == "" {
		writeJSONError(w, "setCode is required", http.StatusBadRequest)
		return
	}
	linked, err := h.cards.LinkCFBArenaIds(r.Context(), setCode)
	if err != nil {
		log.Printf("[CardsHandler.LinkCFBArenaIds] setCode=%s: %v", setCode, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, map[string]any{
		"status": "ok", "set_code": setCode, "linked": linked,
		"message": strconv.Itoa(linked) + " ratings linked to arena ids",
	})
}

// DeleteCFB handles DELETE /api/v1/cards/cfb/{setCode}.
func (h *CardsHandler) DeleteCFB(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	setCode := strings.TrimSpace(chi.URLParam(r, "setCode"))
	if setCode == "" {
		writeJSONError(w, "setCode is required", http.StatusBadRequest)
		return
	}
	if _, err := h.cards.DeleteCFBRatings(r.Context(), setCode); err != nil {
		log.Printf("[CardsHandler.DeleteCFB] setCode=%s: %v", setCode, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── helpers ────────────────────────────────────────────────────────────────

func (h *CardsHandler) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := bffmiddleware.UserIDFromContext(r.Context()); !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func (h *CardsHandler) resolveAccount(w http.ResponseWriter, r *http.Request, op string) (int64, bool, bool) {
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return 0, false, false
	}
	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[CardsHandler.%s] GetAccountIDByUserID userID=%d: %v", op, userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return 0, false, false
	}
	return accountID, found, true
}

func setCardRowsToResponse(rows []repository.SetCardRow) []setCardResponse {
	out := make([]setCardResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, setCardRowToResponse(row))
	}
	return out
}

func setCardRowToResponse(row repository.SetCardRow) setCardResponse {
	resp := setCardResponse{
		ID: row.CardID, SetCode: row.SetCode, ArenaID: strconv.Itoa(row.ArenaID),
		Name: row.Name, ManaCost: row.ManaCost, CMC: row.CMC,
		Types:  typesFromTypeLine(row.TypeLine),
		Colors: parseStringArray(row.Colors),
		Rarity: row.Rarity, Text: "",
		Power:         derefOr(row.Power, ""),
		Toughness:     derefOr(row.Toughness, ""),
		ImageURL:      row.ImageURL,
		ImageURLSmall: row.ImageURLSmall,
		ImageURLArt:   row.ImageURLArt,
	}
	return resp
}

// typesFromTypeLine splits a Scryfall type_line ("Creature — Elf Druid")
// into the prefix words before the em-dash.
func typesFromTypeLine(t string) []string {
	if t == "" {
		return []string{}
	}
	parts := strings.Split(t, "—")
	left := strings.TrimSpace(parts[0])
	if left == "" {
		return []string{}
	}
	return strings.Fields(left)
}

func cardRatingRowsToResponse(rows []repository.CardRatingRow) []cardRatingResponse {
	out := make([]cardRatingResponse, 0, len(rows))
	for _, rr := range rows {
		entry := cardRatingResponse{
			Name:   rr.Name,
			Color:  derefOr(rr.Color, ""),
			Rarity: derefOr(rr.Rarity, ""),
			Tier:   tierFromGIHWR(deref(rr.GIHWR)),
			Colors: colorsFromString(derefOr(rr.Color, "")),
		}
		mtgaID := rr.ArenaID
		entry.MTGAID = &mtgaID
		entry.EverDrawnWinRate = deref(rr.GIHWR)
		entry.OpeningHandWinRate = deref(rr.OHWR)
		entry.AvgSeen = deref(rr.ALSA)
		entry.AvgPick = deref(rr.ATA)
		if rr.GIHCount != nil {
			entry.NumGames = *rr.GIHCount
			entry.NumEverDrawn = *rr.GIHCount
		}
		entry.URL = derefOr(rr.URL, "")
		out = append(out, entry)
	}
	return out
}

// tierFromGIHWR returns the SPA's tier letter ("S","A","B",...) for a
// games-in-hand win rate. Mirrors the bucketing used by 17Lands.
func tierFromGIHWR(winRate float64) string {
	switch {
	case winRate >= 0.595:
		return "S"
	case winRate >= 0.575:
		return "A"
	case winRate >= 0.555:
		return "B"
	case winRate >= 0.535:
		return "C"
	case winRate >= 0.515:
		return "D"
	case winRate > 0:
		return "F"
	default:
		return ""
	}
}

func deref(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// colorsFromString splits a Scryfall color text like "WU" or "RG" into
// individual letter slices.
func colorsFromString(s string) []string {
	out := make([]string, 0, len(s))
	for _, c := range strings.ToUpper(strings.TrimSpace(s)) {
		out = append(out, string(c))
	}
	return out
}

func cfbRatingRowsToResponse(rows []repository.CFBRatingRow) []cfbRatingResponse {
	out := make([]cfbRatingResponse, 0, len(rows))
	for _, c := range rows {
		out = append(out, cfbRatingRowToResponse(c))
	}
	return out
}

func cfbRatingRowToResponse(c repository.CFBRatingRow) cfbRatingResponse {
	return cfbRatingResponse{
		ID: c.ID, CardName: c.CardName, SetCode: c.SetCode, ArenaID: c.ArenaID,
		LimitedRating: c.LimitedRating, LimitedScore: c.LimitedScore,
		ConstructedRating: c.ConstructedRating, ConstructedScore: c.ConstructedScore,
		ArchetypeFit: c.ArchetypeFit, Commentary: c.Commentary,
		SourceURL: c.SourceURL, Author: c.Author,
		ImportedAt: c.ImportedAt.UTC().Format(time.RFC3339),
		UpdatedAt:  c.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
