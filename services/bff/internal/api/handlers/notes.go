// Phase 2 PR #7 — notes + suggestions handlers.
//
// Replaces the SPA's daemonClient surface for notes.ts. Routes mount under
// three URL prefixes per the SPA contract:
//   - GET/POST/PUT/DELETE /api/v1/decks/{deckId}/notes[/{noteId}]
//   - GET/PUT             /api/v1/matches/{matchId}/notes
//   - GET/POST            /api/v1/decks/{deckId}/suggestions[/generate]
//   - PUT                 /api/v1/suggestions/{suggestionId}/dismiss
//
// All routes are guarded by DaemonAPIKeyAuth + the standard envelope.
// Account ownership is enforced in the repository (every query joins
// decks.account_id or matches.account_id).
//
// `generateSuggestions` is a STUB until the ML pipeline lands: it returns
// the existing ml_suggestions list for the deck (same as the GET) so the
// SPA's "regenerate" button does not crash. The real pipeline is tracked
// alongside the meta/* identify-archetype follow-up (PR #5b notes).

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

// notesReader is the minimal repo surface the handler needs.
type notesReader interface {
	ListDeckNotes(ctx context.Context, accountID int64, deckID, category string) ([]repository.DeckNoteRow, error)
	GetDeckNote(ctx context.Context, accountID int64, deckID string, noteID int64) (*repository.DeckNoteRow, error)
	CreateDeckNote(ctx context.Context, accountID int64, deckID, content, category string) (*repository.DeckNoteRow, error)
	UpdateDeckNote(ctx context.Context, accountID int64, deckID string, noteID int64, content, category string) (*repository.DeckNoteRow, error)
	DeleteDeckNote(ctx context.Context, accountID int64, deckID string, noteID int64) (bool, error)
	GetMatchNotes(ctx context.Context, accountID int64, matchID string) (*repository.MatchNotesRow, error)
	UpdateMatchNotes(ctx context.Context, accountID int64, matchID, notes string, rating int) (*repository.MatchNotesRow, error)
	ListSuggestions(ctx context.Context, accountID int64, deckID string, activeOnly bool) ([]repository.SuggestionRow, error)
	DismissSuggestion(ctx context.Context, accountID, suggestionID int64) (bool, error)
}

// NotesHandler serves the cloud-data Phase 2 notes + suggestions API.
type NotesHandler struct {
	notes    notesReader
	accounts AccountLookup
}

// NewNotesHandler returns a NotesHandler wired with the given repo + lookup.
func NewNotesHandler(n notesReader, accounts AccountLookup) *NotesHandler {
	return &NotesHandler{notes: n, accounts: accounts}
}

// ─── wire shapes ────────────────────────────────────────────────────────────

// deckNoteResponse mirrors the SPA's DeckNote (camelCase).
type deckNoteResponse struct {
	ID        int64  `json:"id"`
	DeckID    string `json:"deckId"`
	Content   string `json:"content"`
	Category  string `json:"category"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// matchNotesResponse mirrors the SPA's MatchNotes (camelCase).
type matchNotesResponse struct {
	MatchID string `json:"matchId"`
	Notes   string `json:"notes"`
	Rating  int    `json:"rating"`
}

// suggestionResponse mirrors the SPA's ImprovementSuggestion (camelCase).
// priority is derived from confidence; cardReferences is a JSON-encoded
// {card_id, card_name, swap_for_card_id, swap_for_card_name} blob so the
// SPA's parseEvidence helper can pull it apart.
type suggestionResponse struct {
	ID             int64  `json:"id"`
	DeckID         string `json:"deckId"`
	SuggestionType string `json:"suggestionType"`
	Priority       string `json:"priority"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Evidence       string `json:"evidence,omitempty"`
	CardReferences string `json:"cardReferences,omitempty"`
	IsDismissed    bool   `json:"isDismissed"`
	CreatedAt      string `json:"createdAt"`
}

// createDeckNoteRequest mirrors the SPA's CreateDeckNoteRequest.
type createDeckNoteRequest struct {
	Content  string `json:"content"`
	Category string `json:"category,omitempty"`
}

// updateDeckNoteRequest mirrors the SPA's UpdateDeckNoteRequest.
type updateDeckNoteRequest struct {
	Content  string `json:"content"`
	Category string `json:"category,omitempty"`
}

// updateMatchNotesRequest mirrors the SPA's UpdateMatchNotesRequest.
type updateMatchNotesRequest struct {
	Notes  string `json:"notes"`
	Rating int    `json:"rating"`
}

// ─── deck notes handlers ────────────────────────────────────────────────────

// ListDeckNotes handles GET /api/v1/decks/{deckId}/notes[?category=X].
func (h *NotesHandler) ListDeckNotes(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "ListDeckNotes")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	if deckID == "" {
		writeJSONError(w, "deckId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, []deckNoteResponse{})
		return
	}
	rows, err := h.notes.ListDeckNotes(r.Context(), accountID, deckID, strings.TrimSpace(r.URL.Query().Get("category")))
	if err != nil {
		log.Printf("[NotesHandler.ListDeckNotes] accountID=%d deckID=%s: %v", accountID, deckID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, deckNoteRowsToResponse(rows))
}

// GetDeckNote handles GET /api/v1/decks/{deckId}/notes/{noteId}.
func (h *NotesHandler) GetDeckNote(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "GetDeckNote")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	noteID, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "noteId")), 10, 64)
	if deckID == "" || err != nil || noteID <= 0 {
		writeJSONError(w, "deckId and numeric noteId are required", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "note not found", http.StatusNotFound)
		return
	}
	row, err := h.notes.GetDeckNote(r.Context(), accountID, deckID, noteID)
	if err != nil {
		log.Printf("[NotesHandler.GetDeckNote] accountID=%d deckID=%s noteID=%d: %v", accountID, deckID, noteID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if row == nil {
		writeJSONError(w, "note not found", http.StatusNotFound)
		return
	}
	writeMatchesJSON(w, deckNoteRowToResponse(*row))
}

// CreateDeckNote handles POST /api/v1/decks/{deckId}/notes.
func (h *NotesHandler) CreateDeckNote(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "CreateDeckNote")
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
	var req createDeckNoteRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeJSONError(w, "content is required", http.StatusBadRequest)
		return
	}
	row, err := h.notes.CreateDeckNote(r.Context(), accountID, deckID, req.Content, req.Category)
	if err != nil {
		log.Printf("[NotesHandler.CreateDeckNote] accountID=%d deckID=%s: %v", accountID, deckID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if row == nil {
		writeJSONError(w, "deck not found", http.StatusNotFound)
		return
	}
	writeMatchesJSON(w, deckNoteRowToResponse(*row))
}

// UpdateDeckNote handles PUT /api/v1/decks/{deckId}/notes/{noteId}.
func (h *NotesHandler) UpdateDeckNote(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "UpdateDeckNote")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	noteID, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "noteId")), 10, 64)
	if deckID == "" || err != nil || noteID <= 0 {
		writeJSONError(w, "deckId and numeric noteId are required", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "note not found", http.StatusNotFound)
		return
	}
	var req updateDeckNoteRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeJSONError(w, "content is required", http.StatusBadRequest)
		return
	}
	row, err := h.notes.UpdateDeckNote(r.Context(), accountID, deckID, noteID, req.Content, req.Category)
	if err != nil {
		log.Printf("[NotesHandler.UpdateDeckNote] accountID=%d deckID=%s noteID=%d: %v", accountID, deckID, noteID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if row == nil {
		writeJSONError(w, "note not found", http.StatusNotFound)
		return
	}
	writeMatchesJSON(w, deckNoteRowToResponse(*row))
}

// DeleteDeckNote handles DELETE /api/v1/decks/{deckId}/notes/{noteId}.
func (h *NotesHandler) DeleteDeckNote(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "DeleteDeckNote")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	noteID, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "noteId")), 10, 64)
	if deckID == "" || err != nil || noteID <= 0 {
		writeJSONError(w, "deckId and numeric noteId are required", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "note not found", http.StatusNotFound)
		return
	}
	deleted, err := h.notes.DeleteDeckNote(r.Context(), accountID, deckID, noteID)
	if err != nil {
		log.Printf("[NotesHandler.DeleteDeckNote] accountID=%d deckID=%s noteID=%d: %v", accountID, deckID, noteID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !deleted {
		writeJSONError(w, "note not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── match notes handlers ───────────────────────────────────────────────────

// GetMatchNotes handles GET /api/v1/matches/{matchId}/notes.
func (h *NotesHandler) GetMatchNotes(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "GetMatchNotes")
	if !ok {
		return
	}
	matchID := strings.TrimSpace(chi.URLParam(r, "matchId"))
	if matchID == "" {
		writeJSONError(w, "matchId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "match not found", http.StatusNotFound)
		return
	}
	row, err := h.notes.GetMatchNotes(r.Context(), accountID, matchID)
	if err != nil {
		log.Printf("[NotesHandler.GetMatchNotes] accountID=%d matchID=%s: %v", accountID, matchID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if row == nil {
		writeJSONError(w, "match not found", http.StatusNotFound)
		return
	}
	writeMatchesJSON(w, matchNotesRowToResponse(*row, matchID))
}

// UpdateMatchNotes handles PUT /api/v1/matches/{matchId}/notes.
func (h *NotesHandler) UpdateMatchNotes(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "UpdateMatchNotes")
	if !ok {
		return
	}
	matchID := strings.TrimSpace(chi.URLParam(r, "matchId"))
	if matchID == "" {
		writeJSONError(w, "matchId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeJSONError(w, "match not found", http.StatusNotFound)
		return
	}
	var req updateMatchNotesRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Rating < 0 || req.Rating > 5 {
		writeJSONError(w, "rating must be in [0, 5]", http.StatusBadRequest)
		return
	}
	row, err := h.notes.UpdateMatchNotes(r.Context(), accountID, matchID, req.Notes, req.Rating)
	if err != nil {
		log.Printf("[NotesHandler.UpdateMatchNotes] accountID=%d matchID=%s: %v", accountID, matchID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if row == nil {
		writeJSONError(w, "match not found", http.StatusNotFound)
		return
	}
	writeMatchesJSON(w, matchNotesRowToResponse(*row, matchID))
}

// ─── suggestion handlers ────────────────────────────────────────────────────

// ListSuggestions handles GET /api/v1/decks/{deckId}/suggestions[?active=false].
func (h *NotesHandler) ListSuggestions(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "ListSuggestions")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	if deckID == "" {
		writeJSONError(w, "deckId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, []suggestionResponse{})
		return
	}
	activeOnly := true
	if v := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("active"))); v == "false" {
		activeOnly = false
	}
	rows, err := h.notes.ListSuggestions(r.Context(), accountID, deckID, activeOnly)
	if err != nil {
		log.Printf("[NotesHandler.ListSuggestions] accountID=%d deckID=%s: %v", accountID, deckID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, suggestionRowsToResponse(rows))
}

// GenerateSuggestions handles POST /api/v1/decks/{deckId}/suggestions/generate.
//
// STUB: returns the existing ml_suggestions list for the deck. The real
// generation pipeline (ML model invocation) lives outside this PR.
func (h *NotesHandler) GenerateSuggestions(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "GenerateSuggestions")
	if !ok {
		return
	}
	deckID := strings.TrimSpace(chi.URLParam(r, "deckId"))
	if deckID == "" {
		writeJSONError(w, "deckId is required", http.StatusBadRequest)
		return
	}
	if !found {
		writeMatchesJSON(w, []suggestionResponse{})
		return
	}
	rows, err := h.notes.ListSuggestions(r.Context(), accountID, deckID, true)
	if err != nil {
		log.Printf("[NotesHandler.GenerateSuggestions] accountID=%d deckID=%s: %v", accountID, deckID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeMatchesJSON(w, suggestionRowsToResponse(rows))
}

// DismissSuggestion handles PUT /api/v1/suggestions/{suggestionId}/dismiss.
func (h *NotesHandler) DismissSuggestion(w http.ResponseWriter, r *http.Request) {
	accountID, found, ok := h.resolveAccount(w, r, "DismissSuggestion")
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
	dismissed, err := h.notes.DismissSuggestion(r.Context(), accountID, suggestionID)
	if err != nil {
		log.Printf("[NotesHandler.DismissSuggestion] accountID=%d suggestionID=%d: %v", accountID, suggestionID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !dismissed {
		writeJSONError(w, "suggestion not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── helpers ────────────────────────────────────────────────────────────────

func (h *NotesHandler) resolveAccount(w http.ResponseWriter, r *http.Request, op string) (int64, bool, bool) {
	userID, ok := bffmiddleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return 0, false, false
	}
	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[NotesHandler.%s] GetAccountIDByUserID userID=%d: %v", op, userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return 0, false, false
	}
	return accountID, found, true
}

func deckNoteRowsToResponse(rows []repository.DeckNoteRow) []deckNoteResponse {
	out := make([]deckNoteResponse, 0, len(rows))
	for _, n := range rows {
		out = append(out, deckNoteRowToResponse(n))
	}
	return out
}

func deckNoteRowToResponse(n repository.DeckNoteRow) deckNoteResponse {
	return deckNoteResponse{
		ID: n.ID, DeckID: n.DeckID, Content: n.Content, Category: n.Category,
		CreatedAt: n.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: n.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func matchNotesRowToResponse(m repository.MatchNotesRow, fallbackMatchID string) matchNotesResponse {
	matchID := m.MatchID
	if matchID == "" {
		matchID = fallbackMatchID
	}
	return matchNotesResponse{MatchID: matchID, Notes: m.Notes, Rating: m.Rating}
}

func suggestionRowsToResponse(rows []repository.SuggestionRow) []suggestionResponse {
	out := make([]suggestionResponse, 0, len(rows))
	for _, s := range rows {
		out = append(out, suggestionResponse{
			ID: s.ID, DeckID: s.DeckID, SuggestionType: s.SuggestionType,
			Priority:       priorityFromConfidence(s.Confidence),
			Title:          s.Title,
			Description:    derefOr(s.Description, ""),
			Evidence:       derefOr(s.Evidence, ""),
			CardReferences: cardReferencesJSON(s),
			IsDismissed:    s.IsDismissed,
			CreatedAt:      s.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return out
}

// priorityFromConfidence maps a 0..1 confidence score onto the SPA's
// low/medium/high priority bucket.
func priorityFromConfidence(c float64) string {
	switch {
	case c >= 0.7:
		return "high"
	case c >= 0.4:
		return "medium"
	default:
		return "low"
	}
}

// cardReferencesJSON encodes the swap card pair as a JSON object string so
// the SPA's parseEvidence helper can pull the fields out. Empty when no
// card references exist.
func cardReferencesJSON(s repository.SuggestionRow) string {
	if s.CardID == nil && s.CardName == nil && s.SwapForCardID == nil && s.SwapForCardName == nil {
		return ""
	}
	payload := map[string]any{}
	if s.CardID != nil {
		payload["card_id"] = *s.CardID
	}
	if s.CardName != nil {
		payload["card_name"] = *s.CardName
	}
	if s.SwapForCardID != nil {
		payload["swap_for_card_id"] = *s.SwapForCardID
	}
	if s.SwapForCardName != nil {
		payload["swap_for_card_name"] = *s.SwapForCardName
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(b)
}
