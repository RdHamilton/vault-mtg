// Phase 2 PR #2 — /api/v1/collection handlers.
//
// Replaces the SPA's daemonClient /collection surface. All responses are
// wrapped in the {"data": ...} envelope the SPA's apiClient expects, with
// JSON keys chosen to match the existing TS class constructors:
//   - CollectionCard / CollectionStats / CollectionResponse / CollectionValue
//     use camelCase (Wails-era TS classes already deserialise camelCase
//     keys for these shapes).
//   - SetCompletion / RarityCompletion use PascalCase (Wails-era classes
//     deserialise PascalCase). The casing inconsistency is documented in
//     models.ts and slated for cleanup once the SPA type tree is
//     regenerated post-Phase 2.
//
// Auth: every route is guarded by DaemonAPIKeyAuth (same as
// /api/v1/matches/*). Collection rows are scoped to the authenticated
// user's account.

package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"

	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// collectionReader is the minimal repo surface the handler depends on. The
// concrete repository.CollectionRepository satisfies it; tests stub it
// directly.
type collectionReader interface {
	ListCollection(ctx context.Context, accountID int64, f repository.CollectionFilter) ([]repository.CollectionItem, error)
	CountCollection(ctx context.Context, accountID int64) (repository.CollectionCounts, error)
	CountByRarity(ctx context.Context, accountID int64) ([]repository.RarityCount, error)
	SetCardCount(ctx context.Context, setCode string) (int, error)
	SetCompletion(ctx context.Context, accountID int64) ([]repository.SetCompletionRow, error)
	SetRarityBreakdown(ctx context.Context, accountID int64) ([]repository.SetRarityRow, error)
	ValueRows(ctx context.Context, accountID int64) ([]repository.CardValueRow, int, error)
	LastPriceUpdate(ctx context.Context, accountID int64) (int64, error)
}

// CollectionHandler serves the cloud-data Phase 2 collection API.
type CollectionHandler struct {
	collection collectionReader
	accounts   AccountLookup
}

// NewCollectionHandler returns a handler wired with the provided reader and
// account lookup.
func NewCollectionHandler(c collectionReader, accounts AccountLookup) *CollectionHandler {
	return &CollectionHandler{collection: c, accounts: accounts}
}

// ─── wire shapes ────────────────────────────────────────────────────────────

// collectionFilterRequest mirrors the SPA's CollectionFilter (snake_case to
// match the existing daemon contract).
type collectionFilterRequest struct {
	SetCode     string   `json:"set_code"`
	Rarity      string   `json:"rarity"`
	Colors      []string `json:"colors"`
	OwnedOnly   bool     `json:"owned_only"`
	MissingOnly bool     `json:"missing_only"`
}

// collectionCardResponse is one item in the list response — matches the
// SPA's gui.CollectionCard (camelCase keys).
type collectionCardResponse struct {
	CardID          int      `json:"cardId"`
	ArenaID         int      `json:"arenaId"`
	Quantity        int      `json:"quantity"`
	Name            string   `json:"name"`
	SetCode         string   `json:"setCode"`
	SetName         string   `json:"setName"`
	Rarity          string   `json:"rarity"`
	ManaCost        string   `json:"manaCost"`
	CMC             float64  `json:"cmc"`
	TypeLine        string   `json:"typeLine"`
	Colors          []string `json:"colors"`
	ColorIdentity   []string `json:"colorIdentity"`
	ImageURI        string   `json:"imageUri"`
	Power           *string  `json:"power,omitempty"`
	Toughness       *string  `json:"toughness,omitempty"`
	PriceUSD        *float64 `json:"priceUsd,omitempty"`
	PriceUSDFoil    *float64 `json:"priceUsdFoil,omitempty"`
	PriceEUR        *float64 `json:"priceEur,omitempty"`
	PricesUpdatedAt *int64   `json:"pricesUpdatedAt,omitempty"`
}

// collectionResponse mirrors the SPA's CollectionResponse.
type collectionResponse struct {
	Cards                 []collectionCardResponse `json:"cards"`
	TotalCount            int                      `json:"totalCount"`
	FilterCount           int                      `json:"filterCount"`
	UnknownCardsRemaining int                      `json:"unknownCardsRemaining"`
	UnknownCardsFetched   int                      `json:"unknownCardsFetched"`
}

// collectionStatsResponse mirrors gui.CollectionStats.
type collectionStatsResponse struct {
	TotalUniqueCards int `json:"totalUniqueCards"`
	TotalCards       int `json:"totalCards"`
	CommonCount      int `json:"commonCount"`
	UncommonCount    int `json:"uncommonCount"`
	RareCount        int `json:"rareCount"`
	MythicCount      int `json:"mythicCount"`
	// CardsInSet is the total number of unique cards available in the selected
	// set from set_cards.  When no setCode query param is provided, it reflects
	// the total across all sets.  This is the authoritative source for the
	// "CARDS IN SET" header stat on the Collection page (issue #2017).
	CardsInSet int `json:"cardsInSet"`
}

// rarityCompletionResponse mirrors models.RarityCompletion. PascalCase per
// the existing TS class constructor.
type rarityCompletionResponse struct {
	Rarity     string  `json:"Rarity"`
	Total      int     `json:"Total"`
	Owned      int     `json:"Owned"`
	Percentage float64 `json:"Percentage"`
}

// setCompletionResponse mirrors models.SetCompletion. PascalCase per the
// existing TS class constructor.
type setCompletionResponse struct {
	SetCode         string                              `json:"SetCode"`
	SetName         string                              `json:"SetName"`
	TotalCards      int                                 `json:"TotalCards"`
	OwnedCards      int                                 `json:"OwnedCards"`
	Percentage      float64                             `json:"Percentage"`
	RarityBreakdown map[string]rarityCompletionResponse `json:"RarityBreakdown"`
}

// cardValueResponse / collectionValueResponse mirror the CardValue and
// CollectionValue interfaces declared in collection.ts (camelCase).
type cardValueResponse struct {
	CardID   int     `json:"cardId"`
	Name     string  `json:"name"`
	SetCode  string  `json:"setCode"`
	Rarity   string  `json:"rarity"`
	Quantity int     `json:"quantity"`
	PriceUSD float64 `json:"priceUsd"`
	TotalUSD float64 `json:"totalUsd"`
}

type collectionValueResponse struct {
	TotalValueUSD        float64             `json:"totalValueUsd"`
	TotalValueEUR        float64             `json:"totalValueEur"`
	UniqueCardsWithPrice int                 `json:"uniqueCardsWithPrice"`
	CardCount            int                 `json:"cardCount"`
	ValueByRarity        map[string]float64  `json:"valueByRarity"`
	TopCards             []cardValueResponse `json:"topCards"`
	LastUpdated          *int64              `json:"lastUpdated,omitempty"`
}

// ─── handlers ───────────────────────────────────────────────────────────────

// List handles POST /api/v1/collection. Returns the user's collection,
// filtered per the request body, with metadata counts.
func (h *CollectionHandler) List(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "List")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, collectionResponse{Cards: []collectionCardResponse{}})
		return
	}

	var req collectionFilterRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	filter := repository.CollectionFilter{
		SetCode:     strings.TrimSpace(req.SetCode),
		Rarity:      strings.TrimSpace(req.Rarity),
		Colors:      dedupeNonEmpty(req.Colors),
		OwnedOnly:   req.OwnedOnly,
		MissingOnly: req.MissingOnly,
	}

	rows, err := h.collection.ListCollection(r.Context(), accountID, filter)
	if err != nil {
		log.Printf("[CollectionHandler.List] ListCollection accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	totals, err := h.collection.CountCollection(r.Context(), accountID)
	if err != nil {
		log.Printf("[CollectionHandler.List] CountCollection accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	cards := make([]collectionCardResponse, 0, len(rows))
	for _, c := range rows {
		cards = append(cards, collectionItemToCard(c))
	}
	writeMatchesJSON(w, collectionResponse{
		Cards:       cards,
		TotalCount:  totals.UniqueCards,
		FilterCount: len(cards),
	})
}

// Stats handles GET /api/v1/collection/stats. Aggregated owned-card counts
// and per-rarity breakdown. Accepts an optional ?set_code= query parameter;
// when present, cardsInSet reflects that set's catalogue size; otherwise it
// reflects the total across all sets.
func (h *CollectionHandler) Stats(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Stats")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, collectionStatsResponse{})
		return
	}
	setCode := strings.TrimSpace(r.URL.Query().Get("set_code"))

	totals, err := h.collection.CountCollection(r.Context(), accountID)
	if err != nil {
		log.Printf("[CollectionHandler.Stats] CountCollection accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	rarities, err := h.collection.CountByRarity(r.Context(), accountID)
	if err != nil {
		log.Printf("[CollectionHandler.Stats] CountByRarity accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	cardsInSet, err := h.collection.SetCardCount(r.Context(), setCode)
	if err != nil {
		log.Printf("[CollectionHandler.Stats] SetCardCount setCode=%q: %v", setCode, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	resp := collectionStatsResponse{
		TotalUniqueCards: totals.UniqueCards,
		TotalCards:       totals.TotalCards,
		CardsInSet:       cardsInSet,
	}
	for _, r := range rarities {
		switch strings.ToLower(r.Rarity) {
		case "common":
			resp.CommonCount = r.TotalCards
		case "uncommon":
			resp.UncommonCount = r.TotalCards
		case "rare":
			resp.RareCount = r.TotalCards
		case "mythic", "mythic rare":
			resp.MythicCount = r.TotalCards
		}
	}
	writeMatchesJSON(w, resp)
}

// Sets handles GET /api/v1/collection/sets. Returns one entry per set the
// account owns at least one card from, with totals + per-rarity breakdown.
func (h *CollectionHandler) Sets(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Sets")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, []setCompletionResponse{})
		return
	}
	rows, err := h.collection.SetCompletion(r.Context(), accountID)
	if err != nil {
		log.Printf("[CollectionHandler.Sets] SetCompletion accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	rarityRows, err := h.collection.SetRarityBreakdown(r.Context(), accountID)
	if err != nil {
		log.Printf("[CollectionHandler.Sets] SetRarityBreakdown accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Index rarity rows by set so the assembly below is O(N).
	rarityBySet := map[string][]repository.SetRarityRow{}
	for _, rr := range rarityRows {
		rarityBySet[rr.SetCode] = append(rarityBySet[rr.SetCode], rr)
	}

	out := make([]setCompletionResponse, 0, len(rows))
	for _, s := range rows {
		percent := 0.0
		if s.TotalCards > 0 {
			percent = float64(s.OwnedCards) / float64(s.TotalCards)
		}
		breakdown := map[string]rarityCompletionResponse{}
		for _, rr := range rarityBySet[s.SetCode] {
			label := rr.Rarity
			if label == "" {
				label = "unknown"
			}
			rarityPct := 0.0
			if rr.Total > 0 {
				rarityPct = float64(rr.Owned) / float64(rr.Total)
			}
			breakdown[label] = rarityCompletionResponse{
				Rarity: label, Total: rr.Total, Owned: rr.Owned, Percentage: rarityPct,
			}
		}
		out = append(out, setCompletionResponse{
			SetCode: s.SetCode, SetName: s.SetName,
			TotalCards: s.TotalCards, OwnedCards: s.OwnedCards,
			Percentage: percent, RarityBreakdown: breakdown,
		})
	}
	writeMatchesJSON(w, out)
}

// Value handles GET /api/v1/collection/value. Computes total USD/EUR value,
// per-rarity totals, and the top 25 cards by value.
func (h *CollectionHandler) Value(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Value")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, collectionValueResponse{ValueByRarity: map[string]float64{}, TopCards: []cardValueResponse{}})
		return
	}
	rows, _, err := h.collection.ValueRows(r.Context(), accountID)
	if err != nil {
		log.Printf("[CollectionHandler.Value] ValueRows accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	lastUpdated, err := h.collection.LastPriceUpdate(r.Context(), accountID)
	if err != nil {
		log.Printf("[CollectionHandler.Value] LastPriceUpdate accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := collectionValueResponse{
		ValueByRarity: map[string]float64{},
		TopCards:      []cardValueResponse{},
	}
	cardValues := make([]cardValueResponse, 0, len(rows))
	for _, v := range rows {
		totalUSD := v.PriceUSD * float64(v.Quantity)
		totalEUR := v.PriceEUR * float64(v.Quantity)
		resp.TotalValueUSD += totalUSD
		resp.TotalValueEUR += totalEUR
		resp.CardCount += v.Quantity
		resp.UniqueCardsWithPrice++
		rarityKey := v.Rarity
		if rarityKey == "" {
			rarityKey = "unknown"
		}
		resp.ValueByRarity[rarityKey] += totalUSD
		cardValues = append(cardValues, cardValueResponse{
			CardID: v.CardID, Name: v.Name, SetCode: v.SetCode, Rarity: v.Rarity,
			Quantity: v.Quantity, PriceUSD: v.PriceUSD, TotalUSD: totalUSD,
		})
	}
	sort.Slice(cardValues, func(i, j int) bool { return cardValues[i].TotalUSD > cardValues[j].TotalUSD })
	const topN = 25
	if len(cardValues) > topN {
		cardValues = cardValues[:topN]
	}
	resp.TopCards = cardValues
	if lastUpdated > 0 {
		resp.LastUpdated = &lastUpdated
	}
	writeMatchesJSON(w, resp)
}

// ─── helpers ────────────────────────────────────────────────────────────────

// resolveAccount mirrors MatchesHandler.resolveAccount — same contract, same
// failure modes. Duplicated to keep the two handlers independently
// refactorable; if a third handler ends up needing the same preamble we can
// promote it to a shared helper.
func (h *CollectionHandler) resolveAccount(w http.ResponseWriter, r *http.Request, op string) (int64, bool, bool) {
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return 0, false, false
	}
	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[CollectionHandler.%s] GetAccountIDByUserID userID=%d: %v", op, userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return 0, false, false
	}
	return accountID, found, true
}

// collectionItemToCard maps a repository row into the wire shape, parsing
// the JSON-text columns (colors, color_identity, image_uris) into their
// expected response types. Failed parses degrade to empty values rather
// than 500'ing the request — a single corrupt cards row should not break
// the whole collection list.
func collectionItemToCard(c repository.CollectionItem) collectionCardResponse {
	out := collectionCardResponse{
		CardID: c.CardID, ArenaID: c.ArenaID, Quantity: c.Quantity,
		Name: c.Name, SetCode: c.SetCode, SetName: c.SetName,
		Rarity: c.Rarity, ManaCost: c.ManaCost, CMC: c.CMC, TypeLine: c.TypeLine,
		Colors: parseStringArray(c.Colors), ColorIdentity: parseStringArray(c.ColorIdentity),
		ImageURI: extractImageURI(c.ImageURIs),
		Power:    c.Power, Toughness: c.Toughness,
		PriceUSD: c.PriceUSD, PriceUSDFoil: c.PriceUSDFoil, PriceEUR: c.PriceEUR,
	}
	if c.PricesUpdated != nil && *c.PricesUpdated > 0 {
		ts := *c.PricesUpdated
		out.PricesUpdatedAt = &ts
	}
	return out
}

// parseStringArray decodes a JSON array of strings stored as TEXT. Returns
// an empty slice on any decode failure.
func parseStringArray(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []string{}
	}
	return out
}

// extractImageURI pulls the "normal" Scryfall image URI out of the
// image_uris JSON object. Falls back to "large" then "small" then empty.
func extractImageURI(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" || raw == "{}" {
		return ""
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return ""
	}
	for _, k := range []string{"normal", "large", "small", "png"} {
		if v, ok := m[k]; ok && v != "" {
			return v
		}
	}
	return ""
}
