package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ramonehamilton/MTGA-Companion/internal/api/response"
	"github.com/ramonehamilton/MTGA-Companion/internal/events"
	"github.com/ramonehamilton/MTGA-Companion/internal/storage"
	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
)

// CardFetcher defines the interface for fetching and caching card data.
type CardFetcher interface {
	FetchAndCacheSet(ctx context.Context, mtgaSetCode string) (int, error)
}

// StandardHandler handles Standard format validation and set management endpoints.
type StandardHandler struct {
	storage    *storage.Service
	setFetcher CardFetcher
	dispatcher *events.EventDispatcher
}

// NewStandardHandler creates a new StandardHandler.
func NewStandardHandler(storage *storage.Service) *StandardHandler {
	return &StandardHandler{storage: storage}
}

// SetCardFetcher sets the card fetcher for syncing set cards.
func (h *StandardHandler) SetCardFetcher(fetcher CardFetcher) {
	h.setFetcher = fetcher
}

// SetEventDispatcher sets the event dispatcher for progress events.
func (h *StandardHandler) SetEventDispatcher(dispatcher *events.EventDispatcher) {
	h.dispatcher = dispatcher
}

// GetStandardSets returns all Standard-legal sets.
// GET /api/v1/standard/sets
func (h *StandardHandler) GetStandardSets(w http.ResponseWriter, r *http.Request) {
	sets, err := h.storage.StandardRepo().GetStandardSets(r.Context())
	if err != nil {
		response.InternalError(w, err)
		return
	}

	// Ensure we return an empty array instead of null
	if sets == nil {
		sets = []*models.StandardSet{}
	}

	response.Success(w, sets)
}

// GetUpcomingRotation returns information about the upcoming Standard rotation.
// GET /api/v1/standard/rotation
func (h *StandardHandler) GetUpcomingRotation(w http.ResponseWriter, r *http.Request) {
	rotation, err := h.storage.StandardRepo().GetUpcomingRotation(r.Context())
	if err != nil {
		response.InternalError(w, err)
		return
	}

	response.Success(w, rotation)
}

// GetRotationAffectedDecks returns all Standard decks affected by the upcoming rotation.
// GET /api/v1/standard/rotation/affected-decks
func (h *StandardHandler) GetRotationAffectedDecks(w http.ResponseWriter, r *http.Request) {
	decks, err := h.storage.StandardRepo().GetRotationAffectedDecks(r.Context())
	if err != nil {
		response.InternalError(w, err)
		return
	}

	// Ensure we return an empty array instead of null
	if decks == nil {
		decks = []*models.RotationAffectedDeck{}
	}

	response.Success(w, decks)
}

// GetStandardConfig returns the Standard rotation configuration.
// GET /api/v1/standard/config
func (h *StandardHandler) GetStandardConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.storage.StandardRepo().GetConfig(r.Context())
	if err != nil {
		response.InternalError(w, err)
		return
	}

	response.Success(w, config)
}

// ValidateDeckStandard validates a deck for Standard legality.
// POST /api/v1/standard/validate/{deckID}
func (h *StandardHandler) ValidateDeckStandard(w http.ResponseWriter, r *http.Request) {
	deckID := chi.URLParam(r, "deckID")
	if deckID == "" {
		response.BadRequest(w, errors.New("deck ID is required"))
		return
	}

	// Get deck
	deck, err := h.storage.DeckRepo().GetByID(r.Context(), deckID)
	if err != nil {
		response.InternalError(w, err)
		return
	}
	if deck == nil {
		response.NotFound(w, errors.New("deck not found"))
		return
	}

	// Get deck cards
	cards, err := h.storage.DeckRepo().GetCards(r.Context(), deckID)
	if err != nil {
		response.InternalError(w, err)
		return
	}

	// Get legality for all cards
	arenaIDs := make([]string, len(cards))
	for i, card := range cards {
		arenaIDs[i] = fmt.Sprintf("%d", card.CardID)
	}

	legalities, err := h.storage.StandardRepo().GetCardsLegality(r.Context(), arenaIDs)
	if err != nil {
		response.InternalError(w, err)
		return
	}

	// Get card names for error messages
	cardNames, err := h.storage.GetCardNames(r.Context(), arenaIDs)
	if err != nil {
		// Non-fatal - continue without names
		cardNames = make(map[string]string)
	}

	// Build validation result
	result := validateDeckForStandard(cards, legalities, cardNames)

	response.Success(w, result)
}

// validateDeckForStandard validates a deck's cards against Standard legality.
func validateDeckForStandard(cards []*models.DeckCard, legalities map[string]*models.CardLegality, cardNames map[string]string) *models.DeckValidationResult {
	result := &models.DeckValidationResult{
		IsLegal:      true,
		Errors:       []models.ValidationError{},
		Warnings:     []models.ValidationWarning{},
		SetBreakdown: []models.DeckSetInfo{},
	}

	// Track card counts for 4-copy rule (keyed by arenaID for name lookup)
	cardCounts := make(map[string]int)

	// Basic lands that are exempt from the 4-copy rule
	basicLands := map[string]bool{
		"Plains": true, "Island": true, "Swamp": true,
		"Mountain": true, "Forest": true, "Wastes": true,
	}

	for _, card := range cards {
		arenaID := fmt.Sprintf("%d", card.CardID)
		cardCounts[arenaID] += card.Quantity
		cardName := cardNames[arenaID]

		legality, found := legalities[arenaID]
		if !found {
			// Card not in legality database - warn but don't fail
			result.Warnings = append(result.Warnings, models.ValidationWarning{
				CardID:   card.CardID,
				CardName: cardName,
				Type:     "unknown_legality",
				Details:  "Card legality information not available",
			})
			continue
		}

		// Check Standard legality
		switch legality.Standard {
		case "banned":
			result.IsLegal = false
			result.Errors = append(result.Errors, models.ValidationError{
				CardID:   card.CardID,
				CardName: cardName,
				Reason:   "banned",
				Details:  "Card is banned in Standard",
			})
		case "not_legal":
			result.IsLegal = false
			result.Errors = append(result.Errors, models.ValidationError{
				CardID:   card.CardID,
				CardName: cardName,
				Reason:   "not_legal",
				Details:  "Card is not legal in Standard",
			})
		}
	}

	// Check 4-copy rule (excluding basic lands)
	for arenaID, count := range cardCounts {
		if count > 4 {
			cardName := cardNames[arenaID]
			// Basic lands are exempt from the 4-copy rule
			if basicLands[cardName] {
				continue
			}
			// If card name is unavailable, we can't determine if it's a basic land
			// Skip validation to avoid false positives
			if cardName == "" {
				var cardID int
				_, _ = fmt.Sscanf(arenaID, "%d", &cardID)
				result.Warnings = append(result.Warnings, models.ValidationWarning{
					CardID:  cardID,
					Type:    "unknown_card",
					Details: fmt.Sprintf("Cannot validate 4-copy rule - card name unavailable (has %d copies)", count),
				})
				continue
			}
			result.IsLegal = false
			var cardID int
			_, _ = fmt.Sscanf(arenaID, "%d", &cardID) // Ignore error - cardID defaults to 0
			result.Errors = append(result.Errors, models.ValidationError{
				CardID:   cardID,
				CardName: cardName,
				Reason:   "too_many_copies",
				Details:  fmt.Sprintf("Deck contains %d copies (maximum 4 allowed)", count),
			})
		}
	}

	// Check minimum deck size
	totalCards := 0
	for _, card := range cards {
		if card.Board == "main" {
			totalCards += card.Quantity
		}
	}

	if totalCards < 60 {
		result.IsLegal = false
		result.Errors = append(result.Errors, models.ValidationError{
			Reason:  "deck_size",
			Details: fmt.Sprintf("Deck has %d cards (minimum 60 required)", totalCards),
		})
	}

	return result
}

// GetCardLegality returns the legality of a single card.
// GET /api/v1/standard/cards/{arenaID}/legality
func (h *StandardHandler) GetCardLegality(w http.ResponseWriter, r *http.Request) {
	arenaID := chi.URLParam(r, "arenaID")
	if arenaID == "" {
		response.BadRequest(w, errors.New("arena ID is required"))
		return
	}

	legality, err := h.storage.StandardRepo().GetCardLegality(r.Context(), arenaID)
	if err != nil {
		response.InternalError(w, err)
		return
	}

	if legality == nil {
		response.NotFound(w, errors.New("card legality not found"))
		return
	}

	response.Success(w, legality)
}

// SyncStandardSetCards synchronizes card data for all Standard-legal sets.
// POST /api/v1/standard/sync
func (h *StandardHandler) SyncStandardSetCards(w http.ResponseWriter, r *http.Request) {
	if h.setFetcher == nil {
		response.InternalError(w, errors.New("card fetcher not configured"))
		return
	}

	// Get Standard-legal sets
	sets, err := h.storage.StandardRepo().GetStandardSets(r.Context())
	if err != nil {
		response.InternalError(w, err)
		return
	}

	if len(sets) == 0 {
		response.Success(w, map[string]interface{}{
			"message":     "No Standard-legal sets found",
			"sets_synced": 0,
			"total_cards": 0,
		})
		return
	}

	log.Printf("[StandardHandler] Starting sync for %d Standard sets", len(sets))

	// Use a longer timeout context for sync operation
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	// Generate unique task ID for progress tracking
	taskID := fmt.Sprintf("sync-standard-%d", time.Now().UnixNano())

	// Emit initial progress event (using task:progress for frontend compatibility)
	if h.dispatcher != nil {
		h.dispatcher.Dispatch(events.NewTypedEvent("task:progress", map[string]interface{}{
			"id":       taskID,
			"title":    "Syncing Standard Sets",
			"category": "sync",
			"progress": 0,
			"detail":   "Starting sync...",
		}, ctx))
	}

	type SetSyncResult struct {
		SetCode   string `json:"set_code"`
		SetName   string `json:"set_name"`
		CardCount int    `json:"card_count"`
		Error     string `json:"error,omitempty"`
	}

	results := make([]SetSyncResult, 0, len(sets))
	totalCards := 0
	successCount := 0

	for i, set := range sets {
		log.Printf("[StandardHandler] Syncing set %d/%d: %s (%s)", i+1, len(sets), set.Name, set.Code)

		// Emit progress event for current set (using task:progress for frontend compatibility)
		if h.dispatcher != nil {
			h.dispatcher.Dispatch(events.NewTypedEvent("task:progress", map[string]interface{}{
				"id":       taskID,
				"title":    "Syncing Standard Sets",
				"category": "sync",
				"progress": float64(i) / float64(len(sets)) * 100,
				"detail":   fmt.Sprintf("Syncing %s... (%d cards so far)", set.Name, totalCards),
			}, ctx))
		}

		result := SetSyncResult{
			SetCode: set.Code,
			SetName: set.Name,
		}

		cardCount, err := h.setFetcher.FetchAndCacheSet(ctx, set.Code)
		if err != nil {
			log.Printf("[StandardHandler] Failed to sync %s: %v", set.Code, err)
			result.Error = err.Error()

			// Emit error event for failed set (using task:error for frontend compatibility)
			if h.dispatcher != nil {
				h.dispatcher.Dispatch(events.NewTypedEvent("task:error", map[string]interface{}{
					"id":    taskID,
					"error": fmt.Sprintf("Failed to sync %s: %v", set.Code, err),
				}, ctx))
			}
		} else {
			result.CardCount = cardCount
			totalCards += cardCount
			successCount++
			log.Printf("[StandardHandler] Synced %d cards for %s", cardCount, set.Code)
		}

		results = append(results, result)

		// Rate limiting between API calls
		if i < len(sets)-1 {
			time.Sleep(200 * time.Millisecond)
		}
	}

	log.Printf("[StandardHandler] Sync complete: %d/%d sets successful, %d total cards", successCount, len(sets), totalCards)

	// Emit completion event (using task:complete for frontend compatibility)
	if h.dispatcher != nil {
		h.dispatcher.Dispatch(events.NewTypedEvent("task:complete", map[string]interface{}{
			"id": taskID,
		}, ctx))
	}

	response.Success(w, map[string]interface{}{
		"message":     fmt.Sprintf("Synced %d/%d Standard sets", successCount, len(sets)),
		"sets_synced": successCount,
		"sets_failed": len(sets) - successCount,
		"total_cards": totalCards,
		"set_results": results,
	})
}
