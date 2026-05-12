package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// ─── stubs ──────────────────────────────────────────────────────────────────

type notesAccountLookup struct {
	accountID int64
	found     bool
	err       error
}

func (n *notesAccountLookup) GetAccountIDByUserID(_ context.Context, _ int64) (int64, bool, error) {
	return n.accountID, n.found, n.err
}

type stubNotesReader struct {
	deckNotes    []repository.DeckNoteRow
	deckNotesErr error

	deckNote    *repository.DeckNoteRow
	deckNoteErr error

	createdNote    *repository.DeckNoteRow
	createdNoteErr error

	updatedNote    *repository.DeckNoteRow
	updatedNoteErr error

	deleted    bool
	deletedErr error

	matchNotes    *repository.MatchNotesRow
	matchNotesErr error

	updatedMatchNotes    *repository.MatchNotesRow
	updatedMatchNotesErr error
	updatedNotes         string
	updatedRating        int

	suggestions    []repository.SuggestionRow
	suggestionsErr error

	dismissed    bool
	dismissedErr error
}

func (s *stubNotesReader) ListDeckNotes(_ context.Context, _ int64, _, _ string) ([]repository.DeckNoteRow, error) {
	return s.deckNotes, s.deckNotesErr
}

func (s *stubNotesReader) GetDeckNote(_ context.Context, _ int64, _ string, _ int64) (*repository.DeckNoteRow, error) {
	return s.deckNote, s.deckNoteErr
}

func (s *stubNotesReader) CreateDeckNote(_ context.Context, _ int64, _, _, _ string) (*repository.DeckNoteRow, error) {
	return s.createdNote, s.createdNoteErr
}

func (s *stubNotesReader) UpdateDeckNote(_ context.Context, _ int64, _ string, _ int64, _, _ string) (*repository.DeckNoteRow, error) {
	return s.updatedNote, s.updatedNoteErr
}

func (s *stubNotesReader) DeleteDeckNote(_ context.Context, _ int64, _ string, _ int64) (bool, error) {
	return s.deleted, s.deletedErr
}

func (s *stubNotesReader) GetMatchNotes(_ context.Context, _ int64, _ string) (*repository.MatchNotesRow, error) {
	return s.matchNotes, s.matchNotesErr
}

func (s *stubNotesReader) UpdateMatchNotes(_ context.Context, _ int64, _, notes string, rating int) (*repository.MatchNotesRow, error) {
	s.updatedNotes = notes
	s.updatedRating = rating
	return s.updatedMatchNotes, s.updatedMatchNotesErr
}

func (s *stubNotesReader) ListSuggestions(_ context.Context, _ int64, _ string, _ bool) ([]repository.SuggestionRow, error) {
	return s.suggestions, s.suggestionsErr
}

func (s *stubNotesReader) DismissSuggestion(_ context.Context, _ int64, _ int64) (bool, error) {
	return s.dismissed, s.dismissedErr
}

// ─── helpers ────────────────────────────────────────────────────────────────

func authedNotesRequest(t *testing.T, method, target string, body []byte, userID int64) *http.Request {
	t.Helper()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, target, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	return req.WithContext(bffmiddleware.WithUserID(req.Context(), userID))
}

func decodeNotesEnvelope(t *testing.T, body []byte, into any) {
	t.Helper()
	wrapper := struct {
		Data json.RawMessage `json:"data"`
	}{}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		t.Fatalf("envelope decode: %v body=%s", err, string(body))
	}
	if err := json.Unmarshal(wrapper.Data, into); err != nil {
		t.Fatalf("payload decode: %v data=%s", err, string(wrapper.Data))
	}
}

func chiNotesContext(req *http.Request, kvs ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i+1 < len(kvs); i += 2 {
		rctx.URLParams.Add(kvs[i], kvs[i+1])
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// ─── Deck Notes ────────────────────────────────────────────────────────────

func TestNotesListDeck_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubNotesReader{deckNotes: []repository.DeckNoteRow{
		{ID: 1, DeckID: "d1", Content: "Sideboard plan vs aggro", Category: "matchup", CreatedAt: now, UpdatedAt: now},
	}}
	h := handlers.NewNotesHandler(reader, &notesAccountLookup{accountID: 7, found: true})
	req := authedNotesRequest(t, http.MethodGet, "/api/v1/decks/d1/notes", nil, 168)
	req = chiNotesContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.ListDeckNotes(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeNotesEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["category"] != "matchup" {
		t.Errorf("notes: %v", arr)
	}
}

func TestNotesCreateDeck_RequiresContent(t *testing.T) {
	h := handlers.NewNotesHandler(&stubNotesReader{}, &notesAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"content": ""})
	req := authedNotesRequest(t, http.MethodPost, "/api/v1/decks/d1/notes", body, 168)
	req = chiNotesContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.CreateDeckNote(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestNotesCreateDeck_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	reader := &stubNotesReader{createdNote: &repository.DeckNoteRow{
		ID: 1, DeckID: "d1", Content: "Sideboard", Category: "general", CreatedAt: now, UpdatedAt: now,
	}}
	h := handlers.NewNotesHandler(reader, &notesAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"content": "Sideboard"})
	req := authedNotesRequest(t, http.MethodPost, "/api/v1/decks/d1/notes", body, 168)
	req = chiNotesContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.CreateDeckNote(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeNotesEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["content"] != "Sideboard" {
		t.Errorf("created: %v", resp)
	}
}

func TestNotesUpdateDeck_NotFound(t *testing.T) {
	h := handlers.NewNotesHandler(&stubNotesReader{updatedNote: nil}, &notesAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"content": "x"})
	req := authedNotesRequest(t, http.MethodPut, "/api/v1/decks/d1/notes/99", body, 168)
	req = chiNotesContext(req, "deckId", "d1", "noteId", "99")
	rr := httptest.NewRecorder()
	h.UpdateDeckNote(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestNotesDeleteDeck_HappyPath(t *testing.T) {
	reader := &stubNotesReader{deleted: true}
	h := handlers.NewNotesHandler(reader, &notesAccountLookup{accountID: 7, found: true})
	req := authedNotesRequest(t, http.MethodDelete, "/api/v1/decks/d1/notes/1", nil, 168)
	req = chiNotesContext(req, "deckId", "d1", "noteId", "1")
	rr := httptest.NewRecorder()
	h.DeleteDeckNote(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── Match Notes ───────────────────────────────────────────────────────────

func TestNotesGetMatch_HappyPath(t *testing.T) {
	reader := &stubNotesReader{matchNotes: &repository.MatchNotesRow{
		MatchID: "m1", Notes: "Misplayed turn 5", Rating: 3,
	}}
	h := handlers.NewNotesHandler(reader, &notesAccountLookup{accountID: 7, found: true})
	req := authedNotesRequest(t, http.MethodGet, "/api/v1/matches/m1/notes", nil, 168)
	req = chiNotesContext(req, "matchId", "m1")
	rr := httptest.NewRecorder()
	h.GetMatchNotes(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeNotesEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["matchId"] != "m1" || resp["rating"].(float64) != 3 {
		t.Errorf("match notes: %v", resp)
	}
}

func TestNotesUpdateMatch_RejectsBadRating(t *testing.T) {
	h := handlers.NewNotesHandler(&stubNotesReader{}, &notesAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"notes": "x", "rating": 7})
	req := authedNotesRequest(t, http.MethodPut, "/api/v1/matches/m1/notes", body, 168)
	req = chiNotesContext(req, "matchId", "m1")
	rr := httptest.NewRecorder()
	h.UpdateMatchNotes(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestNotesUpdateMatch_HappyPath(t *testing.T) {
	reader := &stubNotesReader{updatedMatchNotes: &repository.MatchNotesRow{
		MatchID: "m1", Notes: "Better next time", Rating: 4,
	}}
	h := handlers.NewNotesHandler(reader, &notesAccountLookup{accountID: 7, found: true})
	body, _ := json.Marshal(map[string]any{"notes": "Better next time", "rating": 4})
	req := authedNotesRequest(t, http.MethodPut, "/api/v1/matches/m1/notes", body, 168)
	req = chiNotesContext(req, "matchId", "m1")
	rr := httptest.NewRecorder()
	h.UpdateMatchNotes(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	if reader.updatedRating != 4 || reader.updatedNotes != "Better next time" {
		t.Errorf("not forwarded: notes=%q rating=%d", reader.updatedNotes, reader.updatedRating)
	}
}

// ─── Suggestions ───────────────────────────────────────────────────────────

func TestNotesListSuggestions_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	cardID := 100
	reader := &stubNotesReader{suggestions: []repository.SuggestionRow{
		{
			ID: 1, DeckID: "d1", SuggestionType: "curve", Confidence: 0.85, Title: "Add 1-drops",
			CardID: &cardID, CreatedAt: now,
		},
	}}
	h := handlers.NewNotesHandler(reader, &notesAccountLookup{accountID: 7, found: true})
	req := authedNotesRequest(t, http.MethodGet, "/api/v1/decks/d1/suggestions", nil, 168)
	req = chiNotesContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.ListSuggestions(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeNotesEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["priority"] != "high" {
		t.Errorf("suggestion shape: %v", arr)
	}
	if arr[0]["cardReferences"] == nil || arr[0]["cardReferences"] == "" {
		t.Errorf("expected cardReferences JSON: %v", arr[0])
	}
}

func TestNotesGenerate_StubReturnsExisting(t *testing.T) {
	reader := &stubNotesReader{suggestions: []repository.SuggestionRow{
		{ID: 1, DeckID: "d1", SuggestionType: "removal", Confidence: 0.5, Title: "Cut Lightning Strike"},
	}}
	h := handlers.NewNotesHandler(reader, &notesAccountLookup{accountID: 7, found: true})
	req := authedNotesRequest(t, http.MethodPost, "/api/v1/decks/d1/suggestions/generate", nil, 168)
	req = chiNotesContext(req, "deckId", "d1")
	rr := httptest.NewRecorder()
	h.GenerateSuggestions(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var arr []map[string]any
	decodeNotesEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["priority"] != "medium" {
		t.Errorf("priority mapping: %v", arr)
	}
}

func TestNotesDismiss_HappyPath(t *testing.T) {
	reader := &stubNotesReader{dismissed: true}
	h := handlers.NewNotesHandler(reader, &notesAccountLookup{accountID: 7, found: true})
	req := authedNotesRequest(t, http.MethodPut, "/api/v1/suggestions/1/dismiss", nil, 168)
	req = chiNotesContext(req, "suggestionId", "1")
	rr := httptest.NewRecorder()
	h.DismissSuggestion(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestNotesDismiss_BadID(t *testing.T) {
	h := handlers.NewNotesHandler(&stubNotesReader{}, &notesAccountLookup{accountID: 7, found: true})
	req := authedNotesRequest(t, http.MethodPut, "/api/v1/suggestions/abc/dismiss", nil, 168)
	req = chiNotesContext(req, "suggestionId", "abc")
	rr := httptest.NewRecorder()
	h.DismissSuggestion(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}
