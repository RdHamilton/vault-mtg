package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	"github.com/ramonehamilton/mtga-bff/internal/api/listing"
	"github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// ─── Stub implementations ────────────────────────────────────────────────────

type stubMatchCursorReader struct {
	rows []repository.MatchRow
	err  error
}

func (s *stubMatchCursorReader) ListByAccountIDCursor(
	_ context.Context, _ int64, _ string, _ *time.Time, _ string, _ int,
) ([]repository.MatchRow, error) {
	return s.rows, s.err
}

type stubDraftCursorReader struct {
	rows []repository.DraftSessionRow
	err  error
}

func (s *stubDraftCursorReader) ListByAccountIDCursorP(
	_ context.Context, _ int64, _ string, _ *time.Time, _ string, _ int,
) ([]repository.DraftSessionRow, error) {
	return s.rows, s.err
}

type stubDeckCursorReader struct {
	rows []repository.DeckRow
	err  error
}

func (s *stubDeckCursorReader) ListByAccountIDCursor(
	_ context.Context, _ int64, _ string, _ *time.Time, _ string, _ int,
) ([]repository.DeckRow, error) {
	return s.rows, s.err
}

type stubCollectionCursorReader struct {
	rows []repository.CardInventoryRow
	err  error
}

func (s *stubCollectionCursorReader) ListByAccountIDCursor(
	_ context.Context, _ int64, _ int, _ int,
) ([]repository.CardInventoryRow, error) {
	return s.rows, s.err
}

// ─── Test helpers ────────────────────────────────────────────────────────────

// newListV2Handler builds a ListV2Handler with all provided stubs.
func newListV2Handler(
	accounts *stubAccountLookup,
	matches *stubMatchCursorReader,
	drafts *stubDraftCursorReader,
	decks *stubDeckCursorReader,
	collection *stubCollectionCursorReader,
) *handlers.ListV2Handler {
	return handlers.NewListV2Handler(accounts, matches, drafts, decks, collection)
}

// authedV2Handler wraps a handler func with a user ID injected into context.
func authedV2Handler(fn http.HandlerFunc, userID int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.WithUserID(r.Context(), userID)
		fn(w, r.WithContext(ctx))
	})
}

// decodeV2MatchEnvelope decodes a ListEnvelope[map] from a response body.
func decodeEnvelope(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	return resp
}

// ─── GET /api/v2/history/matches ─────────────────────────────────────────────

func TestListV2_GetMatches_HappyPath(t *testing.T) {
	ts := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	dur := 300

	accounts := &stubAccountLookup{accountID: 10, found: true}
	matches := &stubMatchCursorReader{
		rows: []repository.MatchRow{
			{ID: "m1", Format: "Standard", Result: "win", Timestamp: ts, DurationSeconds: &dur, PlayerWins: 2, OpponentWins: 0},
		},
	}
	h := newListV2Handler(accounts, matches, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetMatches, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/history/matches", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	resp := decodeEnvelope(t, rr.Body.Bytes())

	data := resp["data"].([]interface{})
	if len(data) != 1 {
		t.Fatalf("expected 1 match, got %d", len(data))
	}

	item := data[0].(map[string]interface{})
	if item["id"] != "m1" {
		t.Errorf("id: want m1, got %v", item["id"])
	}

	if item["occurred_at"] == nil {
		t.Error("occurred_at should not be nil")
	}

	page := resp["page"].(map[string]interface{})
	if page["has_more"].(bool) {
		t.Error("has_more should be false for single-row result within limit")
	}

	if page["next_cursor"] != nil {
		t.Error("next_cursor should be nil when has_more=false")
	}
}

func TestListV2_GetMatches_Unauthorized(t *testing.T) {
	h := newListV2Handler(&stubAccountLookup{}, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	req := httptest.NewRequest(http.MethodGet, "/api/v2/history/matches", nil)
	rr := httptest.NewRecorder()
	h.GetMatches(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestListV2_GetMatches_NoAccount(t *testing.T) {
	accounts := &stubAccountLookup{found: false}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetMatches, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/history/matches", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	resp := decodeEnvelope(t, rr.Body.Bytes())
	data := resp["data"].([]interface{})

	if len(data) != 0 {
		t.Errorf("expected empty data for no-account, got %d rows", len(data))
	}
}

func TestListV2_GetMatches_InvalidFormat(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetMatches, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/history/matches?format=InvalidFormat", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown format, got %d", rr.Code)
	}
}

func TestListV2_GetMatches_InvalidLimit(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetMatches, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/history/matches?limit=0", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for limit=0, got %d", rr.Code)
	}
}

func TestListV2_GetMatches_LimitClamped(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	matches := &stubMatchCursorReader{}
	h := newListV2Handler(accounts, matches, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetMatches, 1)

	// limit=999 should be clamped to 200, not return an error.
	req := httptest.NewRequest(http.MethodGet, "/api/v2/history/matches?limit=999", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for clamped limit, got %d", rr.Code)
	}

	resp := decodeEnvelope(t, rr.Body.Bytes())
	page := resp["page"].(map[string]interface{})

	if int(page["limit"].(float64)) != listing.MaxLimit {
		t.Errorf("limit in response: want %d, got %v", listing.MaxLimit, page["limit"])
	}
}

func TestListV2_GetMatches_UnknownSortField(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetMatches, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/history/matches?sort=unknown_field", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown sort, got %d", rr.Code)
	}
}

func TestListV2_GetMatches_HasMore_NextCursor(t *testing.T) {
	// Build limit+1 rows so has_more=true.
	const limit = 5
	var rows []repository.MatchRow
	for i := 0; i < limit+1; i++ {
		ts := time.Date(2026, 5, 1, 0, i, 0, 0, time.UTC)
		rows = append(rows, repository.MatchRow{
			ID:        "m" + string(rune('a'+i)),
			Format:    "Standard",
			Result:    "win",
			Timestamp: ts,
		})
	}

	accounts := &stubAccountLookup{accountID: 10, found: true}
	matches := &stubMatchCursorReader{rows: rows}
	h := newListV2Handler(accounts, matches, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetMatches, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/history/matches?limit=5", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	resp := decodeEnvelope(t, rr.Body.Bytes())
	data := resp["data"].([]interface{})
	page := resp["page"].(map[string]interface{})

	if len(data) != limit {
		t.Errorf("data length: want %d, got %d", limit, len(data))
	}

	if !page["has_more"].(bool) {
		t.Error("has_more should be true")
	}

	if page["next_cursor"] == nil {
		t.Error("next_cursor should not be nil when has_more=true")
	}
}

func TestListV2_GetMatches_ValidCursor(t *testing.T) {
	ts := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	c := listing.Cursor{OccurredAt: &ts, ID: "m-cursor"}
	encoded := c.Encode()

	accounts := &stubAccountLookup{accountID: 10, found: true}
	matches := &stubMatchCursorReader{}
	h := newListV2Handler(accounts, matches, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetMatches, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/history/matches?cursor="+encoded, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid cursor, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestListV2_GetMatches_MalformedCursor(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetMatches, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/history/matches?cursor=not-valid-base64!!!", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed cursor, got %d", rr.Code)
	}
}

// TestListV2_GetMatches_CrossTenantIsolation verifies that user B cannot see
// user A's matches when their account lookup returns found=false.
func TestListV2_GetMatches_CrossTenantIsolation(t *testing.T) {
	ts := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	matchesStore := &stubMatchCursorReader{
		rows: []repository.MatchRow{{ID: "userA-m1", Format: "Standard", Result: "win", Timestamp: ts}},
	}

	// User A sees their match.
	hA := newListV2Handler(&stubAccountLookup{accountID: 100, found: true}, matchesStore, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	reqA := httptest.NewRequest(http.MethodGet, "/api/v2/history/matches", nil)
	rrA := httptest.NewRecorder()
	authedV2Handler(hA.GetMatches, 1).ServeHTTP(rrA, reqA)

	if rrA.Code != http.StatusOK {
		t.Fatalf("user A: expected 200, got %d", rrA.Code)
	}

	respA := decodeEnvelope(t, rrA.Body.Bytes())
	if len(respA["data"].([]interface{})) != 1 {
		t.Error("user A should see 1 match")
	}

	// User B (no account) gets an empty result.
	hB := newListV2Handler(&stubAccountLookup{found: false}, matchesStore, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	reqB := httptest.NewRequest(http.MethodGet, "/api/v2/history/matches", nil)
	rrB := httptest.NewRecorder()
	authedV2Handler(hB.GetMatches, 999).ServeHTTP(rrB, reqB)

	if rrB.Code != http.StatusOK {
		t.Fatalf("user B: expected 200, got %d", rrB.Code)
	}

	respB := decodeEnvelope(t, rrB.Body.Bytes())
	if len(respB["data"].([]interface{})) != 0 {
		t.Error("user B must not see user A's matches")
	}
}

// ─── GET /api/v2/history/drafts ──────────────────────────────────────────────

func TestListV2_GetDrafts_HappyPath(t *testing.T) {
	start := time.Date(2026, 4, 20, 14, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 20, 15, 30, 0, 0, time.UTC)

	accounts := &stubAccountLookup{accountID: 10, found: true}
	drafts := &stubDraftCursorReader{
		rows: []repository.DraftSessionRow{
			{ID: "d1", SetCode: "EOE", DraftType: "premier_draft", StartTime: start, EndTime: &end, Wins: 6, Losses: 1},
		},
	}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, drafts, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetDrafts, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/history/drafts", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	resp := decodeEnvelope(t, rr.Body.Bytes())
	data := resp["data"].([]interface{})

	if len(data) != 1 {
		t.Fatalf("expected 1 draft, got %d", len(data))
	}

	item := data[0].(map[string]interface{})
	if item["format"].(string) != "PremierDraft" {
		t.Errorf("format: want PremierDraft, got %v", item["format"])
	}

	if item["started_at"] == nil {
		t.Error("started_at should not be nil")
	}
}

func TestListV2_GetDrafts_Unauthorized(t *testing.T) {
	h := newListV2Handler(&stubAccountLookup{}, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	req := httptest.NewRequest(http.MethodGet, "/api/v2/history/drafts", nil)
	rr := httptest.NewRecorder()
	h.GetDrafts(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestListV2_GetDrafts_InvalidSetCode(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetDrafts, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/history/drafts?set_code=invalid!!", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid set_code, got %d", rr.Code)
	}
}

func TestListV2_GetDrafts_UnknownSortField(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetDrafts, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/history/drafts?sort=bad_field", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown sort field, got %d", rr.Code)
	}
}

func TestListV2_GetDrafts_HasMoreNextCursor(t *testing.T) {
	const limit = 3
	var rows []repository.DraftSessionRow
	for i := 0; i < limit+1; i++ {
		ts := time.Date(2026, 4, i+1, 0, 0, 0, 0, time.UTC)
		rows = append(rows, repository.DraftSessionRow{
			ID:        "d" + string(rune('a'+i)),
			SetCode:   "EOE",
			DraftType: "quick_draft",
			StartTime: ts,
		})
	}

	accounts := &stubAccountLookup{accountID: 10, found: true}
	drafts := &stubDraftCursorReader{rows: rows}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, drafts, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetDrafts, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/history/drafts?limit=3", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	resp := decodeEnvelope(t, rr.Body.Bytes())
	page := resp["page"].(map[string]interface{})

	if !page["has_more"].(bool) {
		t.Error("has_more should be true")
	}

	if page["next_cursor"] == nil {
		t.Error("next_cursor should be set when has_more=true")
	}
}

// ─── GET /api/v2/decks ───────────────────────────────────────────────────────

func TestListV2_GetDecks_HappyPath(t *testing.T) {
	modAt := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)

	accounts := &stubAccountLookup{accountID: 10, found: true}
	decks := &stubDeckCursorReader{
		rows: []repository.DeckRow{
			{ID: "deck-1", Name: "Red Deck Wins", Format: "Standard", Source: "arena", ModifiedAt: modAt},
		},
	}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, &stubDraftCursorReader{}, decks, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetDecks, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/decks", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	resp := decodeEnvelope(t, rr.Body.Bytes())
	data := resp["data"].([]interface{})

	if len(data) != 1 {
		t.Fatalf("expected 1 deck, got %d", len(data))
	}

	item := data[0].(map[string]interface{})
	if item["name"] != "Red Deck Wins" {
		t.Errorf("name: want 'Red Deck Wins', got %v", item["name"])
	}
}

func TestListV2_GetDecks_Unauthorized(t *testing.T) {
	h := newListV2Handler(&stubAccountLookup{}, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	req := httptest.NewRequest(http.MethodGet, "/api/v2/decks", nil)
	rr := httptest.NewRecorder()
	h.GetDecks(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestListV2_GetDecks_InvalidFormat(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetDecks, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/decks?format=BadFormat", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown format, got %d", rr.Code)
	}
}

func TestListV2_GetDecks_UnknownSortField(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetDecks, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/decks?sort=bad", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown sort, got %d", rr.Code)
	}
}

func TestListV2_GetDecks_NoAccount(t *testing.T) {
	accounts := &stubAccountLookup{found: false}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetDecks, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/decks", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	resp := decodeEnvelope(t, rr.Body.Bytes())
	if len(resp["data"].([]interface{})) != 0 {
		t.Error("expected empty data for no-account")
	}
}

func TestListV2_GetDecks_HasMoreNextCursor(t *testing.T) {
	const limit = 2
	var rows []repository.DeckRow
	for i := 0; i < limit+1; i++ {
		ts := time.Date(2026, 5, i+1, 0, 0, 0, 0, time.UTC)
		rows = append(rows, repository.DeckRow{
			ID:         "deck-" + string(rune('a'+i)),
			Name:       "Deck " + string(rune('A'+i)),
			Format:     "Standard",
			Source:     "arena",
			ModifiedAt: ts,
		})
	}

	accounts := &stubAccountLookup{accountID: 10, found: true}
	decks := &stubDeckCursorReader{rows: rows}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, &stubDraftCursorReader{}, decks, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetDecks, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/decks?limit=2", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	resp := decodeEnvelope(t, rr.Body.Bytes())
	page := resp["page"].(map[string]interface{})

	if !page["has_more"].(bool) {
		t.Error("has_more should be true")
	}

	if page["next_cursor"] == nil {
		t.Error("next_cursor should be set")
	}
}

// ─── GET /api/v2/collection ──────────────────────────────────────────────────

func TestListV2_GetCollection_HappyPath(t *testing.T) {
	ts := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	accounts := &stubAccountLookup{accountID: 10, found: true}
	collection := &stubCollectionCursorReader{
		rows: []repository.CardInventoryRow{
			{CardID: 100001, Count: 4, UpdatedAt: ts},
			{CardID: 100002, Count: 2, UpdatedAt: ts},
		},
	}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, collection)
	handler := authedV2Handler(h.GetCollection, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/collection", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	resp := decodeEnvelope(t, rr.Body.Bytes())
	data := resp["data"].([]interface{})

	if len(data) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(data))
	}

	first := data[0].(map[string]interface{})
	if int(first["card_id"].(float64)) != 100001 {
		t.Errorf("card_id: want 100001, got %v", first["card_id"])
	}
}

func TestListV2_GetCollection_Unauthorized(t *testing.T) {
	h := newListV2Handler(&stubAccountLookup{}, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	req := httptest.NewRequest(http.MethodGet, "/api/v2/collection", nil)
	rr := httptest.NewRecorder()
	h.GetCollection(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestListV2_GetCollection_NoAccount(t *testing.T) {
	accounts := &stubAccountLookup{found: false}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetCollection, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/collection", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	resp := decodeEnvelope(t, rr.Body.Bytes())
	if len(resp["data"].([]interface{})) != 0 {
		t.Error("expected empty data for no-account")
	}
}

func TestListV2_GetCollection_InvalidLimit(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetCollection, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/collection?limit=-5", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for negative limit, got %d", rr.Code)
	}
}

func TestListV2_GetCollection_HasMoreNextCursor(t *testing.T) {
	const limit = 3
	ts := time.Now().UTC()
	var rows []repository.CardInventoryRow

	for i := 0; i < limit+1; i++ {
		rows = append(rows, repository.CardInventoryRow{
			CardID:    100001 + i,
			Count:     1,
			UpdatedAt: ts,
		})
	}

	accounts := &stubAccountLookup{accountID: 10, found: true}
	collection := &stubCollectionCursorReader{rows: rows}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, collection)
	handler := authedV2Handler(h.GetCollection, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/collection?limit=3", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	resp := decodeEnvelope(t, rr.Body.Bytes())
	page := resp["page"].(map[string]interface{})

	if !page["has_more"].(bool) {
		t.Error("has_more should be true")
	}

	if page["next_cursor"] == nil {
		t.Error("next_cursor should be set when has_more=true")
	}

	// The cursor must decode successfully.
	cursor, err := listing.DecodeCursor(page["next_cursor"].(string))
	if err != nil {
		t.Fatalf("next_cursor decode: %v", err)
	}

	// For collection the cursor ID is the card_id of the last returned row.
	if cursor.ID == "" {
		t.Error("cursor ID should not be empty")
	}
}

func TestListV2_GetCollection_MalformedCursor(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, &stubCollectionCursorReader{})
	handler := authedV2Handler(h.GetCollection, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/collection?cursor=!!bad!!", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed cursor, got %d", rr.Code)
	}
}

func TestListV2_GetCollection_EmptyResult(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	collection := &stubCollectionCursorReader{rows: nil}
	h := newListV2Handler(accounts, &stubMatchCursorReader{}, &stubDraftCursorReader{}, &stubDeckCursorReader{}, collection)
	handler := authedV2Handler(h.GetCollection, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/collection", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	resp := decodeEnvelope(t, rr.Body.Bytes())
	data := resp["data"].([]interface{})
	page := resp["page"].(map[string]interface{})

	if len(data) != 0 {
		t.Errorf("expected empty data, got %d", len(data))
	}

	if page["has_more"].(bool) {
		t.Error("has_more should be false for empty result")
	}

	if page["next_cursor"] != nil {
		t.Error("next_cursor should be nil for empty result")
	}
}
