package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/api/listing"
	mw "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// ─── Interfaces ──────────────────────────────────────────────────────────────

// MatchCursorReader fetches matches using keyset pagination.
type MatchCursorReader interface {
	ListByAccountIDCursor(ctx context.Context, accountID int64, format string, cursorTS *time.Time, cursorID string, limit int) ([]repository.MatchRow, error)
}

// DraftCursorReader fetches draft sessions using keyset pagination.
type DraftCursorReader interface {
	ListByAccountIDCursorP(ctx context.Context, accountID int64, setCode string, cursorTS *time.Time, cursorID string, limit int) ([]repository.DraftSessionRow, error)
}

// DeckCursorReader fetches decks using keyset pagination.
type DeckCursorReader interface {
	ListByAccountIDCursor(ctx context.Context, accountID int64, format string, cursorModifiedAt *time.Time, cursorID string, limit int) ([]repository.DeckRow, error)
}

// CollectionCursorReader fetches card_inventory rows using keyset pagination.
type CollectionCursorReader interface {
	ListByAccountIDCursor(ctx context.Context, accountID int64, afterCardID int, limit int) ([]repository.CardInventoryRow, error)
}

// ─── Sort allowlists ─────────────────────────────────────────────────────────

var matchSortAllowlist = map[string]struct{}{
	"occurred_at": {},
}

var draftSortAllowlist = map[string]struct{}{
	"started_at": {},
}

var deckSortAllowlist = map[string]struct{}{
	"updated_at": {},
}

var collectionSortAllowlist = map[string]struct{}{
	"card_id": {},
}

// ─── ListV2Handler ───────────────────────────────────────────────────────────

// ListV2Handler handles the v2 cursor-paginated list endpoints for
// matches, drafts, decks, and collection.
type ListV2Handler struct {
	accounts   AccountLookup
	matches    MatchCursorReader
	drafts     DraftCursorReader
	decks      DeckCursorReader
	collection CollectionCursorReader
}

// NewListV2Handler returns a ListV2Handler wired with the provided repos.
func NewListV2Handler(
	accounts AccountLookup,
	matches MatchCursorReader,
	drafts DraftCursorReader,
	decks DeckCursorReader,
	collection CollectionCursorReader,
) *ListV2Handler {
	return &ListV2Handler{
		accounts:   accounts,
		matches:    matches,
		drafts:     drafts,
		decks:      decks,
		collection: collection,
	}
}

// ─── Response shapes ─────────────────────────────────────────────────────────

// matchV2Response is the JSON shape for a single match in the v2 list.
type matchV2Response struct {
	ID              string    `json:"id"`
	Format          string    `json:"format"`
	Result          string    `json:"result"`
	OccurredAt      time.Time `json:"occurred_at"`
	DurationSeconds *int      `json:"duration_seconds"`
	DeckID          *string   `json:"deck_id"`
	RankBefore      *string   `json:"rank_before"`
	RankAfter       *string   `json:"rank_after"`
	PlayerWins      int       `json:"player_wins"`
	OpponentWins    int       `json:"opponent_wins"`
}

// draftV2Response is the JSON shape for a single draft in the v2 list.
type draftV2Response struct {
	ID          string     `json:"id"`
	SetCode     string     `json:"set_code"`
	Format      string     `json:"format"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Wins        int        `json:"wins"`
	Losses      int        `json:"losses"`
}

// deckV2Response is the JSON shape for a single deck in the v2 list.
type deckV2Response struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Format     string    `json:"format"`
	Source     string    `json:"source"`
	ModifiedAt time.Time `json:"modified_at"`
}

// collectionItemV2Response is the JSON shape for one card inventory row in the v2 list.
type collectionItemV2Response struct {
	CardID    int       `json:"card_id"`
	Count     int       `json:"count"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ─── GET /api/v2/history/matches ─────────────────────────────────────────────

// GetMatches handles GET /api/v2/history/matches.
// Query params: cursor, limit (default 50, max 200), sort (occurred_at), order (asc|desc), format.
func (h *ListV2Handler) GetMatches(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	params, err := listing.ParseListParams(r, matchSortAllowlist, "occurred_at")
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	format := r.URL.Query().Get("format")
	if format != "" {
		if _, ok := knownFormats[format]; !ok {
			writeJSONError(w, "unknown format: "+format, http.StatusBadRequest)
			return
		}
	}

	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[ListV2Handler.GetMatches] GetAccountIDByUserID userID=%d: %v", userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if !found {
		writeListV2JSON(w, listing.ListEnvelope[matchV2Response]{
			Data: []matchV2Response{},
			Page: listing.Page{HasMore: false, Limit: params.Limit},
		})
		return
	}

	var cursorTS *time.Time
	var cursorID string

	if params.Cursor != nil {
		cursorTS = params.Cursor.OccurredAt
		cursorID = params.Cursor.ID
	}

	rows, err := h.matches.ListByAccountIDCursor(r.Context(), accountID, format, cursorTS, cursorID, params.Limit)
	if err != nil {
		log.Printf("[ListV2Handler.GetMatches] ListByAccountIDCursor accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	env := listing.BuildEnvelope(rows, params.Limit, func(m repository.MatchRow) listing.Cursor {
		return listing.Cursor{OccurredAt: &m.Timestamp, ID: m.ID}
	})

	data := make([]matchV2Response, 0, len(env.Data))
	for _, m := range env.Data {
		data = append(data, matchV2Response{
			ID:              m.ID,
			Format:          m.Format,
			Result:          m.Result,
			OccurredAt:      m.Timestamp,
			DurationSeconds: m.DurationSeconds,
			DeckID:          m.DeckID,
			RankBefore:      m.RankBefore,
			RankAfter:       m.RankAfter,
			PlayerWins:      m.PlayerWins,
			OpponentWins:    m.OpponentWins,
		})
	}

	writeListV2JSON(w, listing.ListEnvelope[matchV2Response]{Data: data, Page: env.Page})
}

// ─── GET /api/v2/history/drafts ──────────────────────────────────────────────

// GetDrafts handles GET /api/v2/history/drafts.
// Query params: cursor, limit, sort (started_at), order, set_code.
func (h *ListV2Handler) GetDrafts(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	params, err := listing.ParseListParams(r, draftSortAllowlist, "started_at")
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	setCode := r.URL.Query().Get("set_code")
	if setCode != "" && !isValidSetCode(setCode) {
		writeJSONError(w, "invalid set_code: must be 3-5 uppercase letters", http.StatusBadRequest)
		return
	}

	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[ListV2Handler.GetDrafts] GetAccountIDByUserID userID=%d: %v", userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if !found {
		writeListV2JSON(w, listing.ListEnvelope[draftV2Response]{
			Data: []draftV2Response{},
			Page: listing.Page{HasMore: false, Limit: params.Limit},
		})
		return
	}

	var cursorTS *time.Time
	var cursorID string

	if params.Cursor != nil {
		cursorTS = params.Cursor.OccurredAt
		cursorID = params.Cursor.ID
	}

	rows, err := h.drafts.ListByAccountIDCursorP(r.Context(), accountID, setCode, cursorTS, cursorID, params.Limit)
	if err != nil {
		log.Printf("[ListV2Handler.GetDrafts] ListByAccountIDCursorP accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	env := listing.BuildEnvelope(rows, params.Limit, func(d repository.DraftSessionRow) listing.Cursor {
		return listing.Cursor{OccurredAt: &d.StartTime, ID: d.ID}
	})

	data := make([]draftV2Response, 0, len(env.Data))
	for _, d := range env.Data {
		data = append(data, draftV2Response{
			ID:          d.ID,
			SetCode:     d.SetCode,
			Format:      draftTypeToFormat(d.DraftType),
			StartedAt:   d.StartTime,
			CompletedAt: d.EndTime,
			Wins:        d.Wins,
			Losses:      d.Losses,
		})
	}

	writeListV2JSON(w, listing.ListEnvelope[draftV2Response]{Data: data, Page: env.Page})
}

// ─── GET /api/v2/decks ───────────────────────────────────────────────────────

// GetDecks handles GET /api/v2/decks.
// Query params: cursor, limit, sort (updated_at), order, format.
func (h *ListV2Handler) GetDecks(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	params, err := listing.ParseListParams(r, deckSortAllowlist, "updated_at")
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	format := r.URL.Query().Get("format")
	if format != "" {
		if _, ok := knownFormats[format]; !ok {
			writeJSONError(w, "unknown format: "+format, http.StatusBadRequest)
			return
		}
	}

	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[ListV2Handler.GetDecks] GetAccountIDByUserID userID=%d: %v", userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if !found {
		writeListV2JSON(w, listing.ListEnvelope[deckV2Response]{
			Data: []deckV2Response{},
			Page: listing.Page{HasMore: false, Limit: params.Limit},
		})
		return
	}

	var cursorModifiedAt *time.Time
	var cursorID string

	if params.Cursor != nil {
		cursorModifiedAt = params.Cursor.OccurredAt
		cursorID = params.Cursor.ID
	}

	rows, err := h.decks.ListByAccountIDCursor(r.Context(), accountID, format, cursorModifiedAt, cursorID, params.Limit)
	if err != nil {
		log.Printf("[ListV2Handler.GetDecks] ListByAccountIDCursor accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	env := listing.BuildEnvelope(rows, params.Limit, func(d repository.DeckRow) listing.Cursor {
		return listing.Cursor{OccurredAt: &d.ModifiedAt, ID: d.ID}
	})

	data := make([]deckV2Response, 0, len(env.Data))
	for _, d := range env.Data {
		data = append(data, deckV2Response{
			ID:         d.ID,
			Name:       d.Name,
			Format:     d.Format,
			Source:     d.Source,
			ModifiedAt: d.ModifiedAt,
		})
	}

	writeListV2JSON(w, listing.ListEnvelope[deckV2Response]{Data: data, Page: env.Page})
}

// ─── GET /api/v2/collection ──────────────────────────────────────────────────

// GetCollection handles GET /api/v2/collection.
// Query params: cursor, limit. Sorting is always card_id ASC.
// The cursor encodes the last card_id seen as a decimal string in the ID field.
func (h *ListV2Handler) GetCollection(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	params, err := listing.ParseListParams(r, collectionSortAllowlist, "card_id")
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Decode afterCardID from the cursor.  afterCardID=0 means first page.
	afterCardID := 0
	if params.Cursor != nil {
		n, parseErr := strconv.Atoi(params.Cursor.ID)
		if parseErr != nil || n < 0 {
			writeJSONError(w, "invalid cursor", http.StatusBadRequest)
			return
		}

		afterCardID = n
	}

	accountID, found, err := h.accounts.GetAccountIDByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[ListV2Handler.GetCollection] GetAccountIDByUserID userID=%d: %v", userID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if !found {
		writeListV2JSON(w, listing.ListEnvelope[collectionItemV2Response]{
			Data: []collectionItemV2Response{},
			Page: listing.Page{HasMore: false, Limit: params.Limit},
		})
		return
	}

	rows, err := h.collection.ListByAccountIDCursor(r.Context(), accountID, afterCardID, params.Limit)
	if err != nil {
		log.Printf("[ListV2Handler.GetCollection] ListByAccountIDCursor accountID=%d: %v", accountID, err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	env := listing.BuildEnvelope(rows, params.Limit, func(c repository.CardInventoryRow) listing.Cursor {
		// OccurredAt is nil for collection — cursor is card_id only.
		return listing.Cursor{ID: strconv.Itoa(c.CardID)}
	})

	data := make([]collectionItemV2Response, 0, len(env.Data))
	for _, c := range env.Data {
		data = append(data, collectionItemV2Response{
			CardID:    c.CardID,
			Count:     c.Count,
			UpdatedAt: c.UpdatedAt,
		})
	}

	writeListV2JSON(w, listing.ListEnvelope[collectionItemV2Response]{Data: data, Page: env.Page})
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// writeListV2JSON writes the envelope as JSON with a 200 status.
func writeListV2JSON[T any](w http.ResponseWriter, env listing.ListEnvelope[T]) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(env); err != nil {
		log.Printf("[writeListV2JSON] encode: %v", err)
	}
}
