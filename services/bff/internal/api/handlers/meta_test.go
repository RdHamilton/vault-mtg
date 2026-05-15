package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	bffmiddleware "github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// ─── stubs ──────────────────────────────────────────────────────────────────

type stubMetaReader struct {
	archetypes     []repository.ArchetypeRow
	archetypesErr  error
	archetypesFmt  string
	archetypesTier int

	latest    time.Time
	latestOK  bool
	latestErr error

	byName    *repository.ArchetypeRow
	byNameErr error

	cards    []repository.ArchetypeCardRow
	cardsErr error
}

func (s *stubMetaReader) ListArchetypesByFormat(_ context.Context, format string, tier int) ([]repository.ArchetypeRow, error) {
	s.archetypesFmt = format
	s.archetypesTier = tier
	return s.archetypes, s.archetypesErr
}

func (s *stubMetaReader) LatestArchetypeUpdate(_ context.Context, _ string) (time.Time, bool, error) {
	return s.latest, s.latestOK, s.latestErr
}

func (s *stubMetaReader) ArchetypeByName(_ context.Context, _ string, _ string) (*repository.ArchetypeRow, error) {
	return s.byName, s.byNameErr
}

func (s *stubMetaReader) ArchetypeCardsByID(_ context.Context, _ int64) ([]repository.ArchetypeCardRow, error) {
	return s.cards, s.cardsErr
}

// ─── helpers ────────────────────────────────────────────────────────────────

func authedMetaRequest(t *testing.T, method, target string, body []byte, userID int64) *http.Request {
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

func decodeMetaEnvelope(t *testing.T, body []byte, into any) {
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

// ─── Archetypes ────────────────────────────────────────────────────────────

func TestMetaArchetypes_HappyPath(t *testing.T) {
	tier := "1"
	reader := &stubMetaReader{archetypes: []repository.ArchetypeRow{
		{ID: 1, Name: "Mono Red", Format: "standard", Tier: &tier, LastUpdated: time.Now().UTC()},
	}}
	h := handlers.NewMetaHandler(reader)
	req := authedMetaRequest(t, http.MethodGet, "/api/v1/meta/archetypes?format=standard", nil, 168)
	rr := httptest.NewRecorder()
	h.Archetypes(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var arr []map[string]any
	decodeMetaEnvelope(t, rr.Body.Bytes(), &arr)
	if len(arr) != 1 || arr[0]["name"] != "Mono Red" || arr[0]["tier"].(float64) != 1 {
		t.Errorf("archetypes: %v", arr)
	}
	if reader.archetypesFmt != "standard" {
		t.Errorf("format not forwarded: %v", reader.archetypesFmt)
	}
}

func TestMetaArchetypes_MissingFormat(t *testing.T) {
	h := handlers.NewMetaHandler(&stubMetaReader{})
	req := authedMetaRequest(t, http.MethodGet, "/api/v1/meta/archetypes", nil, 168)
	rr := httptest.NewRecorder()
	h.Archetypes(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestMetaArchetypes_Unauthorized(t *testing.T) {
	h := handlers.NewMetaHandler(&stubMetaReader{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/meta/archetypes?format=standard", nil)
	rr := httptest.NewRecorder()
	h.Archetypes(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── Tier ──────────────────────────────────────────────────────────────────

func TestMetaTier_HappyPath(t *testing.T) {
	reader := &stubMetaReader{archetypes: []repository.ArchetypeRow{
		{ID: 1, Name: "Esper Control", Format: "standard"},
	}}
	h := handlers.NewMetaHandler(reader)
	req := authedMetaRequest(t, http.MethodGet, "/api/v1/meta/tier?format=standard&tier=2", nil, 168)
	rr := httptest.NewRecorder()
	h.Tier(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	if reader.archetypesTier != 2 {
		t.Errorf("tier not forwarded: %v", reader.archetypesTier)
	}
}

func TestMetaTier_BadTier(t *testing.T) {
	h := handlers.NewMetaHandler(&stubMetaReader{})
	req := authedMetaRequest(t, http.MethodGet, "/api/v1/meta/tier?format=standard&tier=notanumber", nil, 168)
	rr := httptest.NewRecorder()
	h.Tier(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}

// ─── ArchetypeCards ────────────────────────────────────────────────────────

func TestMetaArchetypeCards_HappyPath(t *testing.T) {
	reader := &stubMetaReader{
		byName: &repository.ArchetypeRow{ID: 42, Name: "Mono Red"},
		cards: []repository.ArchetypeCardRow{
			{CardName: "Goblin Guide", Role: "Creature", Copies: 4},
			{CardName: "Lightning Bolt", Role: "Removal", Copies: 4},
			{CardName: "Mountain", Role: "Common", Copies: 20},
			{CardName: "Plan B Card", Role: "", Copies: 1},
		},
	}
	h := handlers.NewMetaHandler(reader)
	req := authedMetaRequest(t, http.MethodGet, "/api/v1/meta/archetypes/cards?format=standard&archetype=Mono+Red", nil, 168)
	rr := httptest.NewRecorder()
	h.ArchetypeCards(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeMetaEnvelope(t, rr.Body.Bytes(), &resp)
	creatures, _ := resp["top_creatures"].([]any)
	removal, _ := resp["top_removal"].([]any)
	commons, _ := resp["top_commons"].([]any)
	tops, _ := resp["top_cards"].([]any)
	if len(creatures) != 1 || len(removal) != 1 || len(commons) != 1 || len(tops) != 1 {
		t.Errorf("buckets: %v", resp)
	}
}

func TestMetaArchetypeCards_NotFound(t *testing.T) {
	reader := &stubMetaReader{byName: nil}
	h := handlers.NewMetaHandler(reader)
	req := authedMetaRequest(t, http.MethodGet, "/api/v1/meta/archetypes/cards?format=standard&archetype=Nope", nil, 168)
	rr := httptest.NewRecorder()
	h.ArchetypeCards(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var resp map[string]any
	decodeMetaEnvelope(t, rr.Body.Bytes(), &resp)
	tops, _ := resp["top_cards"].([]any)
	if len(tops) != 0 {
		t.Errorf("expected empty buckets: %v", resp)
	}
}

// ─── Stub endpoints ────────────────────────────────────────────────────────

func TestMetaDeckAnalysis_StubReturnsZeroConfidence(t *testing.T) {
	h := handlers.NewMetaHandler(&stubMetaReader{})
	req := authedMetaRequest(t, http.MethodGet, "/api/v1/meta/deck-analysis?deckId=d1", nil, 168)
	rr := httptest.NewRecorder()
	h.DeckAnalysis(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var resp map[string]any
	decodeMetaEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["archetype"] != "Unknown" || resp["confidence"].(float64) != 0 {
		t.Errorf("stub: %v", resp)
	}
}

func TestMetaIdentifyArchetype_HappyPath(t *testing.T) {
	h := handlers.NewMetaHandler(&stubMetaReader{})
	body, _ := json.Marshal(map[string]any{"cardIds": []int{1, 2, 3}, "format": "standard"})
	req := authedMetaRequest(t, http.MethodPost, "/api/v1/meta/identify-archetype", body, 168)
	rr := httptest.NewRecorder()
	h.IdentifyArchetype(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeMetaEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["archetype"] != "Unknown" {
		t.Errorf("identify: %v", resp)
	}
}

func TestMetaIdentifyArchetype_MissingFormat(t *testing.T) {
	h := handlers.NewMetaHandler(&stubMetaReader{})
	body, _ := json.Marshal(map[string]any{"cardIds": []int{1, 2, 3}})
	req := authedMetaRequest(t, http.MethodPost, "/api/v1/meta/identify-archetype", body, 168)
	rr := httptest.NewRecorder()
	h.IdentifyArchetype(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestMetaFormatInsights_StubHasShape(t *testing.T) {
	h := handlers.NewMetaHandler(&stubMetaReader{})
	req := authedMetaRequest(t, http.MethodGet, "/api/v1/meta/insights?format=standard&setCode=DSK", nil, 168)
	rr := httptest.NewRecorder()
	h.FormatInsights(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeMetaEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["set_code"] != "DSK" || resp["draft_format"] != "standard" {
		t.Errorf("insights stub: %v", resp)
	}
	// Shape sanity: top_bombs etc. should be present (empty array).
	if _, ok := resp["top_bombs"]; !ok {
		t.Errorf("missing top_bombs key: %v", resp)
	}
}

func TestMetaRefresh_HappyPath(t *testing.T) {
	reader := &stubMetaReader{
		archetypes: []repository.ArchetypeRow{{ID: 1, Name: "Mono Red"}},
		latest:     time.Now().UTC(),
		latestOK:   true,
	}
	h := handlers.NewMetaHandler(reader)
	req := authedMetaRequest(t, http.MethodPost, "/api/v1/meta/refresh?format=standard", nil, 168)
	rr := httptest.NewRecorder()
	h.Refresh(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeMetaEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["format"] != "standard" || resp["totalArchetypes"].(float64) != 1 {
		t.Errorf("refresh: %v", resp)
	}
}

// TestMetaRefresh_MissingFormat verifies that Refresh returns 400 when the
// required format query param is absent.
func TestMetaRefresh_MissingFormat(t *testing.T) {
	h := handlers.NewMetaHandler(&stubMetaReader{})
	req := authedMetaRequest(t, http.MethodPost, "/api/v1/meta/refresh", nil, 168)
	rr := httptest.NewRecorder()
	h.Refresh(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

// TestMetaRefresh_Unauthorized verifies that Refresh returns 401 when the
// request context carries no user ID (unauthenticated caller).
func TestMetaRefresh_Unauthorized(t *testing.T) {
	h := handlers.NewMetaHandler(&stubMetaReader{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/meta/refresh?format=standard", nil)
	rr := httptest.NewRecorder()
	h.Refresh(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rr.Code)
	}
}

// TestMetaRefresh_EmptyArchetypes verifies that Refresh returns 200 with a
// valid dashboard response shape (empty archetypes slice, not null) when the
// mtgzone_archetypes table has no rows for the format. This is the empty-state
// case that triggers the "no meta data available" UI on the frontend.
func TestMetaRefresh_EmptyArchetypes(t *testing.T) {
	reader := &stubMetaReader{
		archetypes: nil, // empty table — no rows
		latestOK:   false,
	}
	h := handlers.NewMetaHandler(reader)
	req := authedMetaRequest(t, http.MethodPost, "/api/v1/meta/refresh?format=standard", nil, 168)
	rr := httptest.NewRecorder()
	h.Refresh(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeMetaEnvelope(t, rr.Body.Bytes(), &resp)
	// format must be echoed back
	if resp["format"] != "standard" {
		t.Errorf("format: got %v, want %q", resp["format"], "standard")
	}
	// totalArchetypes must be 0, not missing
	if n, ok := resp["totalArchetypes"].(float64); !ok || n != 0 {
		t.Errorf("totalArchetypes: got %v, want 0", resp["totalArchetypes"])
	}
	// archetypes must be a non-null empty array, not null — frontend depends on this
	archetypes, ok := resp["archetypes"].([]any)
	if !ok {
		t.Fatalf("archetypes: expected []any, got %T: %v", resp["archetypes"], resp["archetypes"])
	}
	if len(archetypes) != 0 {
		t.Errorf("archetypes: expected empty array, got %v", archetypes)
	}
	// sources must be present
	if _, ok := resp["sources"]; !ok {
		t.Errorf("sources field missing from response: %v", resp)
	}
}

// TestMetaRefresh_LatestUpdateFallback verifies that when LatestArchetypeUpdate
// returns ok=false (no rows in the DB for the format), the handler falls back to
// time.Now() for lastUpdated and still returns 200 with a valid response body.
// This guards against a nil-pointer panic in the Refresh handler when the DB
// has no archetypes.
func TestMetaRefresh_LatestUpdateFallback(t *testing.T) {
	reader := &stubMetaReader{
		archetypes: []repository.ArchetypeRow{{ID: 1, Name: "Some Deck"}},
		latestOK:   false, // LatestArchetypeUpdate returns ok=false
	}
	h := handlers.NewMetaHandler(reader)
	req := authedMetaRequest(t, http.MethodPost, "/api/v1/meta/refresh?format=historic", nil, 168)
	rr := httptest.NewRecorder()
	h.Refresh(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeMetaEnvelope(t, rr.Body.Bytes(), &resp)
	if resp["format"] != "historic" {
		t.Errorf("format: got %v, want %q", resp["format"], "historic")
	}
	// lastUpdated must be a non-empty string (ISO-8601 from time.Now() fallback)
	lastUpdated, ok := resp["lastUpdated"].(string)
	if !ok || lastUpdated == "" {
		t.Errorf("lastUpdated: expected non-empty string, got %v", resp["lastUpdated"])
	}
}
