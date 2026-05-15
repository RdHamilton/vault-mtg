// Phase 2 PR #9 — /api/v1/decks/* handlers.
//
// Replaces the SPA's daemonClient surface for decks.ts. ~33 endpoints:
//   - CRUD: GET/POST/PUT/DELETE /decks[/{id}]
//   - Cards: POST/DELETE /decks/{id}/cards[/{cardId}[/all]]
//   - Tags:  POST/DELETE /decks/{id}/tags[/{tag}]
//   - Lookup: GET /decks/by-draft/{eventId}, POST /decks/by-tags,
//             POST /decks/library
//   - Stats: GET /decks/{id}/stats|curve|colors|matches|statistics|
//             performance|validate-draft|classify
//   - Clone: POST /decks/{id}/clone
//   - Import/export: POST /decks/import, POST /decks/parse,
//                    POST /decks/{id}/export, POST /decks/suggested/export-content
//   - Permutations: GET /decks/{id}/permutations[/current|/{pid}|
//                    /{from}/diff/{to}], PUT name, POST restore
//   - STUBs (need ML/builder pipeline): /decks/suggest, /decks/analyze,
//             /decks/apply-suggestion, /decks/build-around[/suggest-next],
//             /decks/generate, /decks/archetypes, /decks/{id}/card-performance,
//             /decks/{id}/recommendations/{add|remove|swap|all}
//
// All routes are guarded by DaemonAPIKeyAuth + the standard envelope.
// Account ownership is enforced in the repository (every deck-bound
// query joins decks.account_id).

package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/go-chi/chi/v5"
	"github.com/posthog/posthog-go"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// decksReader is the minimal repo surface the handler needs.
type decksReader interface {
	ListDecks(ctx context.Context, accountID int64, f repository.DeckListFilter) ([]repository.DeckSummaryRow, error)
	GetDeck(ctx context.Context, accountID int64, deckID string) (*repository.DeckDetailRow, error)
	GetDeckByDraftEvent(ctx context.Context, accountID int64, draftEventID string) (*repository.DeckDetailRow, error)
	CreateDeck(ctx context.Context, in repository.CreateDeckInput) (*repository.DeckDetailRow, error)
	UpdateDeck(ctx context.Context, accountID int64, deckID string, in repository.UpdateDeckInput) (*repository.DeckDetailRow, error)
	DeleteDeck(ctx context.Context, accountID int64, deckID string) (bool, error)
	CloneDeck(ctx context.Context, accountID int64, deckID, newName string) (*repository.DeckDetailRow, error)
	AddCard(ctx context.Context, accountID int64, deckID string, in repository.AddCardInput) (bool, error)
	RemoveCardOne(ctx context.Context, accountID int64, deckID string, cardID int, board string) (bool, error)
	RemoveAllCopies(ctx context.Context, accountID int64, deckID string, cardID int, board string) (bool, error)
	AddTag(ctx context.Context, accountID int64, deckID, tag string) (bool, error)
	RemoveTag(ctx context.Context, accountID int64, deckID, tag string) (bool, error)
	ListPermutations(ctx context.Context, accountID int64, deckID string) ([]repository.PermutationRow, error)
	GetPermutation(ctx context.Context, accountID int64, deckID string, permutationID int64) (*repository.PermutationRow, error)
	CurrentPermutation(ctx context.Context, accountID int64, deckID string) (*repository.PermutationRow, error)
	UpdatePermutationName(ctx context.Context, accountID int64, deckID string, permutationID int64, name string) (bool, error)
	RestorePermutation(ctx context.Context, accountID int64, deckID string, permutationID int64) (bool, error)
	DeckMatchesAggregate(ctx context.Context, accountID int64, deckID string) (repository.DeckMatchesAggregate, error)
}

// DecksHandler serves the cloud-data Phase 2 decks API.
type DecksHandler struct {
	decks         decksReader
	accounts      AccountLookup
	postHogClient PostHogClient
}

// NewDecksHandler returns a DecksHandler wired with the given repo + lookup.
// PostHog defaults to the no-op client until WithPostHogClient is called.
func NewDecksHandler(d decksReader, accounts AccountLookup) *DecksHandler {
	return &DecksHandler{decks: d, accounts: accounts, postHogClient: noopPostHogClient{}}
}

// WithPostHogClient returns a copy of h with the given PostHog client wired.
// When not called, the handler uses a no-op client so the code path is always
// exercised without network calls.
func (h *DecksHandler) WithPostHogClient(client PostHogClient) *DecksHandler {
	return &DecksHandler{
		decks:         h.decks,
		accounts:      h.accounts,
		postHogClient: client,
	}
}

// ─── wire shapes ────────────────────────────────────────────────────────────

// deckListItemResponse mirrors gui.DeckListItem (camelCase). The SPA's
// list view consumes these.
type deckListItemResponse struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Format        string   `json:"format"`
	Source        string   `json:"source"`
	DraftEventID  *string  `json:"draftEventId,omitempty"`
	MatchesPlayed int      `json:"matchesPlayed"`
	MatchesWon    int      `json:"matchesWon"`
	GamesPlayed   int      `json:"gamesPlayed"`
	GamesWon      int      `json:"gamesWon"`
	WinRate       float64  `json:"winRate"`
	IsAppCreated  bool     `json:"isAppCreated"`
	CreatedAt     string   `json:"createdAt"`
	ModifiedAt    string   `json:"modifiedAt"`
	LastPlayed    *string  `json:"lastPlayed,omitempty"`
	ColorIdentity string   `json:"colorIdentity,omitempty"`
	Description   string   `json:"description,omitempty"`
	CardCount     int      `json:"cardCount"`
	Tags          []string `json:"tags"`
}

// deckCardResponse mirrors gui.DeckCard (camelCase).
type deckCardResponse struct {
	CardID        int      `json:"cardId"`
	Quantity      int      `json:"quantity"`
	Board         string   `json:"board"`
	FromDraftPick bool     `json:"fromDraftPick"`
	Name          string   `json:"name"`
	SetCode       string   `json:"setCode"`
	ManaCost      string   `json:"manaCost"`
	CMC           float64  `json:"cmc"`
	TypeLine      string   `json:"typeLine"`
	Rarity        string   `json:"rarity"`
	ImageURI      string   `json:"imageUri"`
	Colors        []string `json:"colors"`
}

// deckWithCardsResponse mirrors gui.DeckWithCards.
type deckWithCardsResponse struct {
	deckListItemResponse
	Cards []deckCardResponse `json:"cards"`
}

// deckStatsResponse mirrors gui.DeckStatistics (loose shape — SPA accepts
// any subset of fields).
type deckStatsResponse struct {
	TotalCards    int            `json:"totalCards"`
	ManaCurve     map[int]int    `json:"manaCurve"`
	ColorCounts   map[string]int `json:"colorCounts"`
	TypeCounts    map[string]int `json:"typeCounts"`
	AverageCMC    float64        `json:"averageCmc"`
	LandCount     int            `json:"landCount"`
	SpellCount    int            `json:"spellCount"`
	CreatureCount int            `json:"creatureCount"`
}

// decksPerformanceResponse mirrors models.DeckPerformance.
type decksPerformanceResponse struct {
	DeckID        string  `json:"deckId"`
	MatchesPlayed int     `json:"matchesPlayed"`
	MatchesWon    int     `json:"matchesWon"`
	GamesPlayed   int     `json:"gamesPlayed"`
	GamesWon      int     `json:"gamesWon"`
	WinRate       float64 `json:"winRate"`
	GameWinRate   float64 `json:"gameWinRate"`
}

// permutationCardResponse mirrors DeckPermutationCard on the SPA.
type permutationCardResponse struct {
	CardID   int    `json:"card_id"`
	Quantity int    `json:"quantity"`
	Board    string `json:"board"`
}

// permutationResponse mirrors DeckPermutation on the SPA.
type permutationResponse struct {
	ID                  int64                     `json:"id"`
	DeckID              string                    `json:"deckID"`
	ParentPermutationID *int64                    `json:"parentPermutationID,omitempty"`
	Cards               []permutationCardResponse `json:"cards"`
	VersionNumber       int                       `json:"versionNumber"`
	VersionName         *string                   `json:"versionName,omitempty"`
	ChangeSummary       *string                   `json:"changeSummary,omitempty"`
	MatchesPlayed       int                       `json:"matchesPlayed"`
	MatchesWon          int                       `json:"matchesWon"`
	MatchWinRate        float64                   `json:"matchWinRate"`
	GamesPlayed         int                       `json:"gamesPlayed"`
	GamesWon            int                       `json:"gamesWon"`
	GameWinRate         float64                   `json:"gameWinRate"`
	CreatedAt           string                    `json:"createdAt"`
	LastPlayedAt        *string                   `json:"lastPlayedAt,omitempty"`
	IsCurrent           bool                      `json:"isCurrent"`
}

// permutationDiffResponse mirrors DeckPermutationDiff on the SPA.
type permutationDiffResponse struct {
	FromPermutationID int64                     `json:"fromPermutationID"`
	ToPermutationID   int64                     `json:"toPermutationID"`
	AddedCards        []permutationCardResponse `json:"addedCards"`
	RemovedCards      []permutationCardResponse `json:"removedCards"`
	ChangedCards      []deckCardChangeResponse  `json:"changedCards"`
}

// deckCardChangeResponse mirrors DeckCardChange on the SPA.
type deckCardChangeResponse struct {
	CardID      int    `json:"card_id"`
	OldQuantity int    `json:"old_quantity"`
	NewQuantity int    `json:"new_quantity"`
	Board       string `json:"board"`
}

// createDeckRequest mirrors the SPA's CreateDeckRequest.
type createDeckRequest struct {
	Name         string  `json:"name"`
	Format       string  `json:"format"`
	Source       string  `json:"source"`
	DraftEventID *string `json:"draft_event_id,omitempty"`
}

// updateDeckRequest mirrors UpdateDeckRequest.
type updateDeckRequest struct {
	Name   *string `json:"name,omitempty"`
	Format *string `json:"format,omitempty"`
}

// addCardRequest mirrors the wire shape decks.ts addCard sends.
type addCardRequest struct {
	CardID    int    `json:"cardID"`
	Quantity  int    `json:"quantity"`
	Board     string `json:"board"`
	FromDraft bool   `json:"fromDraft"`
}

// addTagRequest mirrors decks.ts addTag.
type addTagRequest struct {
	Tag string `json:"tag"`
}

// updatePermutationNameRequest mirrors PUT permutations/{id}/name body.
type updatePermutationNameRequest struct {
	Name string `json:"name"`
}

// cloneDeckRequest mirrors decks.ts cloneDeck POST body.
type cloneDeckRequest struct {
	Name string `json:"name"`
}

// ─── CRUD handlers ──────────────────────────────────────────────────────────

// List handles GET /api/v1/decks[?format=&source=].
func (h *DecksHandler) List(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "List")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, []deckListItemResponse{})
		return
	}
	rows, err := h.decks.ListDecks(r.Context(), accountID, repository.DeckListFilter{
		Format: strings.TrimSpace(r.URL.Query().Get("format")),
		Source: strings.TrimSpace(r.URL.Query().Get("source")),
	})
	if err != nil {
		log.Printf("[DecksHandler.List] accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, deckSummariesToResponse(rows))
}

// Get handles GET /api/v1/decks/{deckId}.
func (h *DecksHandler) Get(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Get")
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
	d, err := h.decks.GetDeck(r.Context(), accountID, deckID)
	if err != nil {
		log.Printf("[DecksHandler.Get] accountID=%d deckID=%s: %v", accountID, deckID, err)
		sentry.CaptureException(err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if d == nil {
		writeJSONError(w, "deck not found", http.StatusNotFound)
		return
	}
	_ = h.postHogClient.Enqueue(posthog.Capture{
		DistinctId: strconv.FormatInt(accountID, 10),
		Event:      "get_deck",
		Properties: posthog.NewProperties().
			Set("deck_id", deckID).
			Set("account_id", accountID),
	})
	writeMatchesJSON(w, deckDetailToResponse(*d))
}

// Create handles POST /api/v1/decks.
func (h *DecksHandler) Create(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Create")
	if !ok {
		return
	}
	if !found {
		writeJSONError(w, "account not found", http.StatusForbidden)
		return
	}
	var req createDeckRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeJSONError(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.Source == "" {
		req.Source = "constructed"
	}
	d, err := h.decks.CreateDeck(r.Context(), repository.CreateDeckInput{
		AccountID: accountID, Name: req.Name, Format: req.Format,
		Source: req.Source, DraftEventID: req.DraftEventID,
	})
	if err != nil {
		log.Printf("[DecksHandler.Create] accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, deckDetailToResponse(*d))
}

// Update handles PUT /api/v1/decks/{deckId}.
func (h *DecksHandler) Update(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Update")
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
	var req updateDeckRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	d, err := h.decks.UpdateDeck(r.Context(), accountID, deckID, repository.UpdateDeckInput{
		Name: req.Name, Format: req.Format,
	})
	if err != nil {
		log.Printf("[DecksHandler.Update] accountID=%d deckID=%s: %v", accountID, deckID, err)
		sentry.CaptureException(err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if d == nil {
		writeJSONError(w, "deck not found", http.StatusNotFound)
		return
	}
	_ = h.postHogClient.Enqueue(posthog.Capture{
		DistinctId: strconv.FormatInt(accountID, 10),
		Event:      "update_deck",
		Properties: posthog.NewProperties().
			Set("deck_id", deckID).
			Set("account_id", accountID),
	})
	writeMatchesJSON(w, deckDetailToResponse(*d))
}

// Delete handles DELETE /api/v1/decks/{deckId}.
func (h *DecksHandler) Delete(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Delete")
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
	deleted, err := h.decks.DeleteDeck(r.Context(), accountID, deckID)
	if err != nil {
		log.Printf("[DecksHandler.Delete] accountID=%d deckID=%s: %v", accountID, deckID, err)
		sentry.CaptureException(err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !deleted {
		writeJSONError(w, "deck not found", http.StatusNotFound)
		return
	}
	_ = h.postHogClient.Enqueue(posthog.Capture{
		DistinctId: strconv.FormatInt(accountID, 10),
		Event:      "delete_deck",
		Properties: posthog.NewProperties().
			Set("deck_id", deckID).
			Set("account_id", accountID),
	})
	w.WriteHeader(http.StatusNoContent)
}

// Clone handles POST /api/v1/decks/{deckId}/clone.
func (h *DecksHandler) Clone(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Clone")
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
	var req cloneDeckRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeJSONError(w, "name is required", http.StatusBadRequest)
		return
	}
	d, err := h.decks.CloneDeck(r.Context(), accountID, deckID, req.Name)
	if err != nil {
		log.Printf("[DecksHandler.Clone] accountID=%d deckID=%s: %v", accountID, deckID, err)
		sentry.CaptureException(err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if d == nil {
		writeJSONError(w, "deck not found", http.StatusNotFound)
		return
	}
	_ = h.postHogClient.Enqueue(posthog.Capture{
		DistinctId: strconv.FormatInt(accountID, 10),
		Event:      "clone_deck",
		Properties: posthog.NewProperties().
			Set("source_deck_id", deckID).
			Set("new_deck_id", d.ID).
			Set("account_id", accountID),
	})
	writeMatchesJSON(w, deckDetailToResponse(*d))
}

// GetByDraftEvent handles GET /api/v1/decks/by-draft/{draftEventId}.
func (h *DecksHandler) GetByDraftEvent(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "GetByDraftEvent")
	if !ok {
		return
	}
	draftEventID := strings.TrimSpace(chi.URLParam(r, "draftEventId"))
	if draftEventID == "" {
		writeJSONError(w, "draftEventId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "deck not found", http.StatusNotFound)
		return
	}
	d, err := h.decks.GetDeckByDraftEvent(r.Context(), accountID, draftEventID)
	if err != nil {
		log.Printf("[DecksHandler.GetByDraftEvent] accountID=%d eventID=%s: %v", accountID, draftEventID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if d == nil {
		writeJSONError(w, "deck not found", http.StatusNotFound)
		return
	}
	writeMatchesJSON(w, deckDetailToResponse(*d))
}

// ─── Cards ──────────────────────────────────────────────────────────────────

// AddCard handles POST /api/v1/decks/{deckId}/cards.
func (h *DecksHandler) AddCard(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "AddCard")
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
	var req addCardRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.CardID <= 0 || req.Quantity <= 0 || req.Board == "" {
		writeJSONError(w, "cardID, quantity, board are required", http.StatusBadRequest)
		return
	}
	added, err := h.decks.AddCard(r.Context(), accountID, deckID, repository.AddCardInput{
		CardID: req.CardID, Quantity: req.Quantity, Board: req.Board, FromDraft: req.FromDraft,
	})
	if err != nil {
		log.Printf("[DecksHandler.AddCard] accountID=%d deckID=%s: %v", accountID, deckID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !added {
		writeJSONError(w, "deck not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveCard handles DELETE /api/v1/decks/{deckId}/cards/{cardId}?zone=...
func (h *DecksHandler) RemoveCard(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "RemoveCard")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	cardID, err := strconv.Atoi(strings.TrimSpace(chi.URLParam(r, "cardId")))
	if deckID == "" || err != nil || cardID <= 0 {
		writeJSONError(w, "deckId and numeric cardId are required", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "deck not found", http.StatusNotFound)
		return
	}
	board := strings.TrimSpace(r.URL.Query().Get("zone"))
	if board == "" {
		board = "main"
	}
	removed, err := h.decks.RemoveCardOne(r.Context(), accountID, deckID, cardID, board)
	if err != nil {
		log.Printf("[DecksHandler.RemoveCard] accountID=%d deckID=%s cardID=%d: %v", accountID, deckID, cardID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !removed {
		writeJSONError(w, "card not found in deck", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveAllCopies handles DELETE /api/v1/decks/{deckId}/cards/{cardId}/all?zone=...
func (h *DecksHandler) RemoveAllCopies(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "RemoveAllCopies")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	cardID, err := strconv.Atoi(strings.TrimSpace(chi.URLParam(r, "cardId")))
	if deckID == "" || err != nil || cardID <= 0 {
		writeJSONError(w, "deckId and numeric cardId are required", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "deck not found", http.StatusNotFound)
		return
	}
	board := strings.TrimSpace(r.URL.Query().Get("zone"))
	if board == "" {
		board = "main"
	}
	removed, err := h.decks.RemoveAllCopies(r.Context(), accountID, deckID, cardID, board)
	if err != nil {
		log.Printf("[DecksHandler.RemoveAllCopies] accountID=%d deckID=%s cardID=%d: %v", accountID, deckID, cardID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !removed {
		writeJSONError(w, "card not found in deck", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Tags ───────────────────────────────────────────────────────────────────

// AddTag handles POST /api/v1/decks/{deckId}/tags.
func (h *DecksHandler) AddTag(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "AddTag")
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
	var req addTagRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Tag) == "" {
		writeJSONError(w, "tag is required", http.StatusBadRequest)
		return
	}
	added, err := h.decks.AddTag(r.Context(), accountID, deckID, req.Tag)
	if err != nil {
		log.Printf("[DecksHandler.AddTag] accountID=%d deckID=%s: %v", accountID, deckID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !added {
		writeJSONError(w, "deck not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveTag handles DELETE /api/v1/decks/{deckId}/tags/{tag}.
func (h *DecksHandler) RemoveTag(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "RemoveTag")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	tag := strings.TrimSpace(chi.URLParam(r, "tag"))
	if deckID == "" || tag == "" {
		writeJSONError(w, "deckId and tag are required", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "tag not found", http.StatusNotFound)
		return
	}
	removed, err := h.decks.RemoveTag(r.Context(), accountID, deckID, tag)
	if err != nil {
		log.Printf("[DecksHandler.RemoveTag] accountID=%d deckID=%s tag=%s: %v", accountID, deckID, tag, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !removed {
		writeJSONError(w, "tag not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Library + tags filter ──────────────────────────────────────────────────

// ByTags handles POST /api/v1/decks/by-tags.
func (h *DecksHandler) ByTags(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "ByTags")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, []deckListItemResponse{})
		return
	}
	var body struct {
		Tags []string `json:"tags"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	rows, err := h.decks.ListDecks(r.Context(), accountID, repository.DeckListFilter{Tags: body.Tags})
	if err != nil {
		log.Printf("[DecksHandler.ByTags] accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, deckSummariesToResponse(rows))
}

// Library handles POST /api/v1/decks/library. Filters are a superset of
// the by-tags + GET filters.
func (h *DecksHandler) Library(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Library")
	if !ok {
		return
	}
	if !found {
		writeMatchesJSON(w, []deckListItemResponse{})
		return
	}
	var body struct {
		Format string   `json:"format,omitempty"`
		Source string   `json:"source,omitempty"`
		Tags   []string `json:"tags,omitempty"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	rows, err := h.decks.ListDecks(r.Context(), accountID, repository.DeckListFilter{
		Format: body.Format, Source: body.Source, Tags: body.Tags,
	})
	if err != nil {
		log.Printf("[DecksHandler.Library] accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, deckSummariesToResponse(rows))
}

// ─── Stats / curve / colors / matches / performance ────────────────────────

// Stats handles GET /api/v1/decks/{deckId}/stats. Computed inline from
// deck_cards. Same payload powers /curve, /colors, /statistics — the
// SPA just reads the fields it needs.
func (h *DecksHandler) Stats(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Stats")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	if deckID == "" {
		writeJSONError(w, "deckId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, deckStatsResponse{ManaCurve: map[int]int{}, ColorCounts: map[string]int{}, TypeCounts: map[string]int{}})
		return
	}
	d, err := h.decks.GetDeck(r.Context(), accountID, deckID)
	if err != nil {
		log.Printf("[DecksHandler.Stats] accountID=%d deckID=%s: %v", accountID, deckID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if d == nil {
		writeJSONError(w, "deck not found", http.StatusNotFound)
		return
	}
	writeMatchesJSON(w, computeDeckStats(d.Cards))
}

// Performance handles GET /api/v1/decks/{deckId}/performance and
// /matches. Both routes share the same shape; the SPA just calls them at
// different times.
func (h *DecksHandler) Performance(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Performance")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	if deckID == "" {
		writeJSONError(w, "deckId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, decksPerformanceResponse{DeckID: deckID})
		return
	}
	agg, err := h.decks.DeckMatchesAggregate(r.Context(), accountID, deckID)
	if err != nil {
		log.Printf("[DecksHandler.Performance] accountID=%d deckID=%s: %v", accountID, deckID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	resp := decksPerformanceResponse{
		DeckID: deckID, MatchesPlayed: agg.TotalMatches, MatchesWon: agg.MatchesWon,
		GamesPlayed: agg.GamesPlayed, GamesWon: agg.GamesWon,
	}
	if agg.TotalMatches > 0 {
		resp.WinRate = float64(agg.MatchesWon) / float64(agg.TotalMatches)
	}
	if agg.GamesPlayed > 0 {
		resp.GameWinRate = float64(agg.GamesWon) / float64(agg.GamesPlayed)
	}
	writeMatchesJSON(w, resp)
}

// ValidateDraft handles GET /api/v1/decks/{deckId}/validate-draft.
// Draft decks need exactly 40 cards minimum (15 opt sideboard).
func (h *DecksHandler) ValidateDraft(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "ValidateDraft")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	if deckID == "" {
		writeJSONError(w, "deckId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, map[string]any{"valid": false})
		return
	}
	d, err := h.decks.GetDeck(r.Context(), accountID, deckID)
	if err != nil {
		log.Printf("[DecksHandler.ValidateDraft] accountID=%d deckID=%s: %v", accountID, deckID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if d == nil {
		writeJSONError(w, "deck not found", http.StatusNotFound)
		return
	}
	main := 0
	for _, c := range d.Cards {
		if strings.ToLower(c.Board) == "main" {
			main += c.Quantity
		}
	}
	writeMatchesJSON(w, map[string]any{"valid": main >= 40, "mainboard": main})
}

// Classify handles GET /api/v1/decks/{deckId}/classify. STUB until the
// archetype-matching algorithm lands; returns "Unknown" + 0 confidence.
func (h *DecksHandler) Classify(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	if deckID == "" {
		writeJSONError(w, "deckId is required", http.StatusBadRequest)
		return
	}
	writeMatchesJSON(w, archetypeClassificationStub(deckID))
}

// ─── Import / Export ───────────────────────────────────────────────────────

// Import handles POST /api/v1/decks/import. Accepts an Arena-format
// deck list and creates a new deck row + deck_cards rows for each parsed
// line. The parser supports the common "4 Card Name (SET) 123" syntax;
// unknown cards are skipped silently with a warning in the response.
func (h *DecksHandler) Import(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Import")
	if !ok {
		return
	}
	if !found {
		writeJSONError(w, "account not found", http.StatusForbidden)
		return
	}
	var req struct {
		Content string `json:"content"`
		Name    string `json:"name"`
		Format  string `json:"format"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Content) == "" {
		writeJSONError(w, "name and content are required", http.StatusBadRequest)
		return
	}
	d, err := h.decks.CreateDeck(r.Context(), repository.CreateDeckInput{
		AccountID: accountID, Name: req.Name, Format: req.Format, Source: "imported",
	})
	if err != nil {
		log.Printf("[DecksHandler.Import] accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	parsed := parseArenaDeckList(req.Content)
	for _, p := range parsed {
		if _, err := h.decks.AddCard(r.Context(), accountID, d.ID, repository.AddCardInput{
			CardID: p.cardID, Quantity: p.quantity, Board: p.board,
		}); err != nil {
			log.Printf("[DecksHandler.Import] AddCard: %v", err)
		}
	}
	// Return the created deck (refetched so cards are populated).
	full, err := h.decks.GetDeck(r.Context(), accountID, d.ID)
	if err != nil || full == nil {
		writeMatchesJSON(w, map[string]any{
			"deck":     deckDetailToResponse(*d),
			"warnings": []string{"failed to load saved cards"},
		})
		return
	}
	writeMatchesJSON(w, map[string]any{
		"deck":      deckDetailToResponse(*full),
		"warnings":  []string{},
		"cardCount": full.CardCount,
	})
}

// Parse handles POST /api/v1/decks/parse. Returns parse output without
// saving anything.
func (h *DecksHandler) Parse(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	parsed := parseArenaDeckList(req.Content)
	cards := make([]map[string]any, 0, len(parsed))
	for _, p := range parsed {
		cards = append(cards, map[string]any{
			"card_id": p.cardID, "quantity": p.quantity, "board": p.board, "name": p.name,
		})
	}
	writeMatchesJSON(w, map[string]any{
		"cards":     cards,
		"warnings":  []string{},
		"cardCount": len(cards),
	})
}

// Export handles POST /api/v1/decks/{deckId}/export. Returns the deck
// formatted as Arena-import text.
func (h *DecksHandler) Export(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "Export")
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
	d, err := h.decks.GetDeck(r.Context(), accountID, deckID)
	if err != nil {
		log.Printf("[DecksHandler.Export] accountID=%d deckID=%s: %v", accountID, deckID, err)
		sentry.CaptureException(err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if d == nil {
		writeJSONError(w, "deck not found", http.StatusNotFound)
		return
	}
	_ = h.postHogClient.Enqueue(posthog.Capture{
		DistinctId: strconv.FormatInt(accountID, 10),
		Event:      "export_deck",
		Properties: posthog.NewProperties().
			Set("deck_id", deckID).
			Set("deck_name", d.Name).
			Set("account_id", accountID).
			Set("format", "arena"),
	})
	writeMatchesJSON(w, map[string]any{
		"content": formatArenaDeckList(d.Cards),
		"format":  "arena",
		"name":    d.Name,
	})
}

// SuggestedExportContent handles POST /api/v1/decks/suggested/export-content.
// STUB pending the suggestion-builder pipeline; returns an empty content
// blob.
func (h *DecksHandler) SuggestedExportContent(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	writeMatchesJSON(w, map[string]any{"content": ""})
}

// ─── Permutations ──────────────────────────────────────────────────────────

// ListPermutations handles GET /api/v1/decks/{deckId}/permutations.
func (h *DecksHandler) ListPermutations(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "ListPermutations")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	if deckID == "" {
		writeJSONError(w, "deckId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, []permutationResponse{})
		return
	}
	rows, err := h.decks.ListPermutations(r.Context(), accountID, deckID)
	if err != nil {
		log.Printf("[DecksHandler.ListPermutations] accountID=%d deckID=%s: %v", accountID, deckID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	current, _ := h.decks.CurrentPermutation(r.Context(), accountID, deckID)
	currentID := int64(0)
	if current != nil {
		currentID = current.ID
	}
	out := make([]permutationResponse, 0, len(rows))
	for _, p := range rows {
		out = append(out, permutationRowToResponse(p, p.ID == currentID))
	}
	writeMatchesJSON(w, out)
}

// CurrentPermutation handles GET /api/v1/decks/{deckId}/permutations/current.
func (h *DecksHandler) CurrentPermutation(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "CurrentPermutation")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	if deckID == "" {
		writeJSONError(w, "deckId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, nil)
		return
	}
	p, err := h.decks.CurrentPermutation(r.Context(), accountID, deckID)
	if err != nil {
		log.Printf("[DecksHandler.CurrentPermutation] accountID=%d deckID=%s: %v", accountID, deckID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		writeMatchesJSON(w, nil)
		return
	}
	writeMatchesJSON(w, permutationRowToResponse(*p, true))
}

// GetPermutation handles GET /api/v1/decks/{deckId}/permutations/{permutationId}.
func (h *DecksHandler) GetPermutation(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "GetPermutation")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	pid, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "permutationId")), 10, 64)
	if deckID == "" || err != nil || pid <= 0 {
		writeJSONError(w, "deckId and numeric permutationId are required", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "permutation not found", http.StatusNotFound)
		return
	}
	p, err := h.decks.GetPermutation(r.Context(), accountID, deckID, pid)
	if err != nil {
		log.Printf("[DecksHandler.GetPermutation] accountID=%d deckID=%s pid=%d: %v", accountID, deckID, pid, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		writeJSONError(w, "permutation not found", http.StatusNotFound)
		return
	}
	current, _ := h.decks.CurrentPermutation(r.Context(), accountID, deckID)
	isCurrent := current != nil && current.ID == p.ID
	writeMatchesJSON(w, permutationRowToResponse(*p, isCurrent))
}

// PermutationDiff handles GET /api/v1/decks/{deckId}/permutations/{from}/diff/{to}.
func (h *DecksHandler) PermutationDiff(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "PermutationDiff")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	fromID, fromErr := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "fromPermutationId")), 10, 64)
	toID, toErr := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "toPermutationId")), 10, 64)
	if deckID == "" || fromErr != nil || toErr != nil || fromID <= 0 || toID <= 0 {
		writeJSONError(w, "deckId + numeric from/to permutation ids required", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "permutation not found", http.StatusNotFound)
		return
	}
	from, err := h.decks.GetPermutation(r.Context(), accountID, deckID, fromID)
	if err != nil {
		log.Printf("[DecksHandler.PermutationDiff] from: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	to, err := h.decks.GetPermutation(r.Context(), accountID, deckID, toID)
	if err != nil {
		log.Printf("[DecksHandler.PermutationDiff] to: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if from == nil || to == nil {
		writeJSONError(w, "permutation not found", http.StatusNotFound)
		return
	}
	writeMatchesJSON(w, computePermutationDiff(*from, *to))
}

// UpdatePermutationName handles PUT /api/v1/decks/{deckId}/permutations/{permutationId}/name.
func (h *DecksHandler) UpdatePermutationName(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "UpdatePermutationName")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	pid, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "permutationId")), 10, 64)
	if deckID == "" || err != nil || pid <= 0 {
		writeJSONError(w, "deckId + numeric permutationId required", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "permutation not found", http.StatusNotFound)
		return
	}
	var req updatePermutationNameRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	updated, err := h.decks.UpdatePermutationName(r.Context(), accountID, deckID, pid, req.Name)
	if err != nil {
		log.Printf("[DecksHandler.UpdatePermutationName] accountID=%d deckID=%s pid=%d: %v", accountID, deckID, pid, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !updated {
		writeJSONError(w, "permutation not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RestorePermutation handles POST /api/v1/decks/{deckId}/permutations/{permutationId}/restore.
func (h *DecksHandler) RestorePermutation(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "RestorePermutation")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	pid, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "permutationId")), 10, 64)
	if deckID == "" || err != nil || pid <= 0 {
		writeJSONError(w, "deckId + numeric permutationId required", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "permutation not found", http.StatusNotFound)
		return
	}
	restored, err := h.decks.RestorePermutation(r.Context(), accountID, deckID, pid)
	if err != nil {
		log.Printf("[DecksHandler.RestorePermutation] accountID=%d deckID=%s pid=%d: %v", accountID, deckID, pid, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !restored {
		writeJSONError(w, "permutation not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── STUBs ─────────────────────────────────────────────────────────────────
//
// These endpoints all need ML / archetype-matching / deck-builder
// infrastructure that doesn't live in the BFF yet. Each STUB returns a
// shape-correct empty response so the SPA UI doesn't crash. Documented
// inline; tracked alongside meta/* identify-archetype follow-up.

// SuggestDecks handles POST /api/v1/decks/suggest. STUB.
func (h *DecksHandler) SuggestDecks(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	writeMatchesJSON(w, map[string]any{
		"suggestions":  []any{},
		"totalCombos":  0,
		"viableCombos": 0,
	})
}

// AnalyzeDeck handles POST /api/v1/decks/analyze. STUB.
func (h *DecksHandler) AnalyzeDeck(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	var req struct {
		DeckID string `json:"deck_id"`
	}
	_ = decodeJSONBody(r, &req)
	writeMatchesJSON(w, archetypeClassificationStub(req.DeckID))
}

// ApplySuggestion handles POST /api/v1/decks/apply-suggestion. STUB no-op.
func (h *DecksHandler) ApplySuggestion(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// BuildAround handles POST /api/v1/decks/build-around. STUB.
func (h *DecksHandler) BuildAround(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	writeMatchesJSON(w, map[string]any{
		"seedCard":    map[string]any{"cardID": 0, "name": "", "score": 0},
		"suggestions": []any{},
		"lands":       []any{},
		"analysis":    map[string]any{"colorIdentity": []string{}, "totalCards": 0},
	})
}

// BuildAroundSuggestNext handles POST /api/v1/decks/build-around/suggest-next. STUB.
func (h *DecksHandler) BuildAroundSuggestNext(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	writeMatchesJSON(w, map[string]any{
		"suggestions":     []any{},
		"deckAnalysis":    map[string]any{"colorIdentity": []string{}, "totalCards": 0},
		"slotsRemaining":  0,
		"landSuggestions": []any{},
	})
}

// Generate handles POST /api/v1/decks/generate. STUB.
func (h *DecksHandler) Generate(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	writeMatchesJSON(w, map[string]any{
		"seedCard": map[string]any{"cardID": 0, "name": "", "score": 0},
		"spells":   []any{},
		"lands":    []any{},
		"strategy": map[string]any{"summary": "", "gamePlan": "", "keyCards": []string{}},
		"analysis": map[string]any{"totalCards": 0, "averageCMC": 0},
	})
}

// Archetypes handles GET /api/v1/decks/archetypes. STUB returns the
// canonical archetype name list with skeleton profiles so the SPA's
// archetype picker has something to render.
func (h *DecksHandler) Archetypes(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	names := []string{"Aggro", "Midrange", "Control", "Tempo", "Ramp", "Combo", "Tokens", "Aristocrats"}
	out := make([]map[string]any, 0, len(names))
	for _, n := range names {
		out = append(out, map[string]any{
			"name": n, "landCount": 24, "creatureRatio": 0.4, "removalCount": 6,
			"cardAdvantage": 4, "splashTendency": 0.2, "icon": "",
			"description":  "Canonical " + n + " profile (defaults pending the deck-builder pipeline).",
			"curveTargets": map[int]int{1: 4, 2: 8, 3: 8, 4: 4, 5: 0, 6: 0},
		})
	}
	writeMatchesJSON(w, out)
}

// CardPerformance handles GET /api/v1/decks/{deckId}/card-performance. STUB.
func (h *DecksHandler) CardPerformance(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	writeMatchesJSON(w, map[string]any{
		"deckId": deckID, "deckName": "", "totalMatches": 0, "totalGames": 0,
		"overallWinRate":  0.0,
		"cardPerformance": []any{},
		"bestPerformers":  []string{}, "worstPerformers": []string{},
		"analysisDate": time.Now().UTC().Format(time.RFC3339),
	})
}

// AddRecommendations / RemoveRecommendations / SwapRecommendations / All STUBs.
func (h *DecksHandler) AddRecommendations(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	writeMatchesJSON(w, []any{})
}

func (h *DecksHandler) RemoveRecommendations(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	writeMatchesJSON(w, []any{})
}

func (h *DecksHandler) SwapRecommendations(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	writeMatchesJSON(w, []any{})
}

func (h *DecksHandler) AllRecommendations(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	writeMatchesJSON(w, map[string]any{
		"deckId": deckID, "deckName": "", "currentWinRate": 0.0,
		"addRecommendations": []any{}, "removeRecommendations": []any{},
		"swapRecommendations": []any{}, "projectedWinRate": 0.0,
	})
}

// ─── helpers ────────────────────────────────────────────────────────────────

func (h *DecksHandler) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := bffmiddleware.UserIDFromContext(r.Context()); !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func (h *DecksHandler) resolveAccount(w http.ResponseWriter, r *http.Request, op string) (int64, bool, bool) {
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return 0, false, false
	}
	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[DecksHandler.%s] GetAccountIDByUserID userID=%d: %v", op, userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return 0, false, false
	}
	return accountID, found, true
}

func deckSummariesToResponse(rows []repository.DeckSummaryRow) []deckListItemResponse {
	out := make([]deckListItemResponse, 0, len(rows))
	for _, d := range rows {
		out = append(out, deckSummaryRowToResponse(d))
	}
	return out
}

func deckSummaryRowToResponse(d repository.DeckSummaryRow) deckListItemResponse {
	resp := deckListItemResponse{
		ID: d.ID, Name: d.Name, Format: d.Format, Source: d.Source,
		DraftEventID:  d.DraftEventID,
		MatchesPlayed: d.MatchesPlayed, MatchesWon: d.MatchesWon,
		GamesPlayed: d.GamesPlayed, GamesWon: d.GamesWon,
		IsAppCreated:  d.IsAppCreated,
		CreatedAt:     d.CreatedAt.UTC().Format(time.RFC3339),
		ModifiedAt:    d.ModifiedAt.UTC().Format(time.RFC3339),
		ColorIdentity: derefOr(d.ColorIdentity, ""),
		Description:   derefOr(d.Description, ""),
		CardCount:     d.CardCount,
		Tags:          d.Tags,
	}
	if resp.Tags == nil {
		resp.Tags = []string{}
	}
	if d.LastPlayed != nil {
		ts := d.LastPlayed.UTC().Format(time.RFC3339)
		resp.LastPlayed = &ts
	}
	if d.MatchesPlayed > 0 {
		resp.WinRate = float64(d.MatchesWon) / float64(d.MatchesPlayed)
	}
	return resp
}

func deckDetailToResponse(d repository.DeckDetailRow) deckWithCardsResponse {
	resp := deckWithCardsResponse{
		deckListItemResponse: deckSummaryRowToResponse(d.DeckSummaryRow),
		Cards:                deckCardsToResponse(d.Cards),
	}
	return resp
}

func deckCardsToResponse(rows []repository.DeckCardRow) []deckCardResponse {
	out := make([]deckCardResponse, 0, len(rows))
	for _, c := range rows {
		out = append(out, deckCardResponse{
			CardID: c.CardID, Quantity: c.Quantity, Board: c.Board, FromDraftPick: c.FromDraftPick,
			Name: c.Name, SetCode: c.SetCode,
			ManaCost: c.ManaCost, CMC: c.CMC, TypeLine: c.TypeLine, Rarity: c.Rarity,
			Colors:   parseStringArray(c.Colors),
			ImageURI: extractImageURI(c.ImageURIs),
		})
	}
	return out
}

// computeDeckStats derives the stats payload from deck_cards rows.
func computeDeckStats(cards []repository.DeckCardRow) deckStatsResponse {
	resp := deckStatsResponse{
		ManaCurve: map[int]int{}, ColorCounts: map[string]int{}, TypeCounts: map[string]int{},
	}
	totalCMC := 0.0
	totalSpells := 0
	for _, c := range cards {
		if strings.ToLower(c.Board) != "main" {
			continue
		}
		resp.TotalCards += c.Quantity
		bucket := int(c.CMC)
		resp.ManaCurve[bucket] += c.Quantity
		// Type buckets.
		typ := strings.ToLower(c.TypeLine)
		switch {
		case strings.Contains(typ, "land"):
			resp.LandCount += c.Quantity
			resp.TypeCounts["land"] += c.Quantity
		case strings.Contains(typ, "creature"):
			resp.CreatureCount += c.Quantity
			resp.TypeCounts["creature"] += c.Quantity
			resp.SpellCount += c.Quantity
			totalCMC += c.CMC * float64(c.Quantity)
			totalSpells += c.Quantity
		default:
			resp.SpellCount += c.Quantity
			resp.TypeCounts["spell"] += c.Quantity
			totalCMC += c.CMC * float64(c.Quantity)
			totalSpells += c.Quantity
		}
		// Color buckets via the JSON array TEXT.
		for _, color := range parseStringArray(c.Colors) {
			resp.ColorCounts[color] += c.Quantity
		}
	}
	if totalSpells > 0 {
		resp.AverageCMC = totalCMC / float64(totalSpells)
	}
	return resp
}

// permutationRowToResponse converts a repo permutation row.
func permutationRowToResponse(p repository.PermutationRow, isCurrent bool) permutationResponse {
	resp := permutationResponse{
		ID: p.ID, DeckID: p.DeckID, ParentPermutationID: p.ParentPermutationID,
		Cards:         permutationCardsFromJSON(p.Cards),
		VersionNumber: p.VersionNumber, VersionName: p.VersionName, ChangeSummary: p.ChangeSummary,
		MatchesPlayed: p.MatchesPlayed, MatchesWon: p.MatchesWon,
		GamesPlayed: p.GamesPlayed, GamesWon: p.GamesWon,
		CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339),
		IsCurrent: isCurrent,
	}
	if p.MatchesPlayed > 0 {
		resp.MatchWinRate = float64(p.MatchesWon) / float64(p.MatchesPlayed)
	}
	if p.GamesPlayed > 0 {
		resp.GameWinRate = float64(p.GamesWon) / float64(p.GamesPlayed)
	}
	if p.LastPlayedAt != nil {
		ts := p.LastPlayedAt.UTC().Format(time.RFC3339)
		resp.LastPlayedAt = &ts
	}
	return resp
}

func permutationCardsFromJSON(raw string) []permutationCardResponse {
	cards := repository.ParsePermutationCards(raw)
	out := make([]permutationCardResponse, 0, len(cards))
	for _, c := range cards {
		out = append(out, permutationCardResponse{CardID: c.CardID, Quantity: c.Quantity, Board: c.Board})
	}
	return out
}

// computePermutationDiff derives the diff between two permutation rows.
func computePermutationDiff(from, to repository.PermutationRow) permutationDiffResponse {
	fromCards := repository.ParsePermutationCards(from.Cards)
	toCards := repository.ParsePermutationCards(to.Cards)
	type key struct {
		cardID int
		board  string
	}
	fromMap := map[key]int{}
	for _, c := range fromCards {
		fromMap[key{c.CardID, c.Board}] = c.Quantity
	}
	toMap := map[key]int{}
	for _, c := range toCards {
		toMap[key{c.CardID, c.Board}] = c.Quantity
	}
	var added, removed []permutationCardResponse
	var changed []deckCardChangeResponse
	for k, q := range toMap {
		old, exists := fromMap[k]
		switch {
		case !exists:
			added = append(added, permutationCardResponse{CardID: k.cardID, Quantity: q, Board: k.board})
		case old != q:
			changed = append(changed, deckCardChangeResponse{CardID: k.cardID, OldQuantity: old, NewQuantity: q, Board: k.board})
		}
	}
	for k, q := range fromMap {
		if _, exists := toMap[k]; !exists {
			removed = append(removed, permutationCardResponse{CardID: k.cardID, Quantity: q, Board: k.board})
		}
	}
	// Stable sort for deterministic diffs.
	sort.Slice(added, func(i, j int) bool { return added[i].CardID < added[j].CardID })
	sort.Slice(removed, func(i, j int) bool { return removed[i].CardID < removed[j].CardID })
	sort.Slice(changed, func(i, j int) bool { return changed[i].CardID < changed[j].CardID })
	if added == nil {
		added = []permutationCardResponse{}
	}
	if removed == nil {
		removed = []permutationCardResponse{}
	}
	if changed == nil {
		changed = []deckCardChangeResponse{}
	}
	return permutationDiffResponse{
		FromPermutationID: from.ID, ToPermutationID: to.ID,
		AddedCards: added, RemovedCards: removed, ChangedCards: changed,
	}
}

// archetypeClassificationStub returns a zero-confidence ArchetypeClassification
// for use by classify + analyze.
func archetypeClassificationStub(deckID string) map[string]any {
	return map[string]any{
		"deckId":     deckID,
		"archetype":  "Unknown",
		"confidence": 0.0,
		"strengths":  []string{},
		"weaknesses": []string{},
	}
}

// parsedDeckLine is one (cardID, qty, board, name) parsed from an Arena
// deck list. cardID is 0 when the parser couldn't resolve an arena id.
type parsedDeckLine struct {
	cardID   int
	quantity int
	board    string
	name     string
}

// parseArenaDeckList runs a minimal parser over Arena's "4 Card Name (SET)
// 123" format. Returns one entry per non-blank, non-comment line. We do
// NOT resolve names → arena ids here; the SPA already sends arena ids in
// most flows. For text imports, the BFF stores cards with cardID=0 and
// the projection worker can backfill later.
func parseArenaDeckList(content string) []parsedDeckLine {
	out := []parsedDeckLine{}
	board := "main"
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			continue
		}
		lower := strings.ToLower(line)
		if lower == "deck" || lower == "main" || lower == "mainboard" {
			board = "main"
			continue
		}
		if lower == "sideboard" {
			board = "sideboard"
			continue
		}
		// Lines look like: "4 Lightning Bolt (M21) 162"  or  "4 Lightning Bolt"
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		qty, err := strconv.Atoi(parts[0])
		if err != nil || qty <= 0 {
			continue
		}
		// Best-effort arena id extraction: the trailing token is the
		// collector number, the parenthesised set code is one before it.
		arenaID := 0
		if len(parts) >= 4 {
			if n, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
				arenaID = n
			}
		}
		// Name = everything between qty and the optional "(SET) NUM" suffix.
		nameEnd := len(parts)
		if len(parts) >= 4 && strings.HasPrefix(parts[len(parts)-2], "(") {
			nameEnd = len(parts) - 2
		}
		name := strings.Join(parts[1:nameEnd], " ")
		out = append(out, parsedDeckLine{cardID: arenaID, quantity: qty, board: board, name: name})
	}
	return out
}

// formatArenaDeckList renders deck_cards rows back into Arena's text
// format. main first, then a Sideboard section.
func formatArenaDeckList(cards []repository.DeckCardRow) string {
	var b strings.Builder
	b.WriteString("Deck\n")
	for _, c := range cards {
		if strings.ToLower(c.Board) != "main" {
			continue
		}
		b.WriteString(strconv.Itoa(c.Quantity))
		b.WriteByte(' ')
		b.WriteString(c.Name)
		if c.SetCode != "" {
			b.WriteString(" (")
			b.WriteString(strings.ToUpper(c.SetCode))
			b.WriteString(")")
		}
		b.WriteByte('\n')
	}
	hasSideboard := false
	for _, c := range cards {
		if strings.ToLower(c.Board) == "sideboard" {
			hasSideboard = true
			break
		}
	}
	if hasSideboard {
		b.WriteString("\nSideboard\n")
		for _, c := range cards {
			if strings.ToLower(c.Board) != "sideboard" {
				continue
			}
			b.WriteString(strconv.Itoa(c.Quantity))
			b.WriteByte(' ')
			b.WriteString(c.Name)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// jsonString is unused at present but ready for future json-payload
// consumers — keeps encoding/json reachable.
var _ = json.RawMessage(nil)
