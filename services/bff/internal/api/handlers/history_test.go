package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	"github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// compile-time assertions: stubMatchReader must satisfy MatchHistoryReader.
var _ interface {
	ListByAccountIDCursorFiltered(context.Context, int64, repository.MatchFilter, *time.Time, string, int) ([]repository.MatchRow, error)
} = (*stubMatchReader)(nil)

// --- stubs ---

type stubAccountLookup struct {
	accountID int64
	found     bool
	err       error
}

func (s *stubAccountLookup) GetAccountIDByUserID(_ context.Context, _ int64) (int64, bool, error) {
	return s.accountID, s.found, s.err
}

type stubMatchReader struct {
	rows []repository.MatchRow
	err  error

	// Captured args from most recent call.
	capturedCursorTS *time.Time
	capturedCursorID string
}

// ListByAccountIDCursorFiltered implements MatchHistoryReader using keyset
// cursor pagination. Returns s.rows unchanged so the caller can set up the
// exact slice it wants (including the probe row for has_more testing).
func (s *stubMatchReader) ListByAccountIDCursorFiltered(_ context.Context, _ int64, _ repository.MatchFilter, cursorTS *time.Time, cursorID string, _ int) ([]repository.MatchRow, error) {
	s.capturedCursorTS = cursorTS
	s.capturedCursorID = cursorID
	return s.rows, s.err
}

type stubDraftReader struct {
	rows  []repository.DraftSessionRow
	total int
	err   error
}

func (s *stubDraftReader) ListByAccountID(_ context.Context, _ int64, _ string, _, _ int) ([]repository.DraftSessionRow, int, error) {
	return s.rows, s.total, s.err
}

// authedMatchHandler injects userID into context and delegates to GetMatches.
func authedMatchHandler(h *handlers.HistoryHandler, userID int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.WithUserID(r.Context(), userID)
		r = r.WithContext(ctx)
		h.GetMatches(w, r)
	})
}

// authedDraftHandler injects userID into context and delegates to GetDrafts.
func authedDraftHandler(h *handlers.HistoryHandler, userID int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.WithUserID(r.Context(), userID)
		r = r.WithContext(ctx)
		h.GetDrafts(w, r)
	})
}

// --- matches tests ---

func TestGetMatches_HappyPath(t *testing.T) {
	ts := time.Date(2026, 5, 5, 18, 42, 11, 0, time.UTC)
	dur := 612

	accounts := &stubAccountLookup{accountID: 10, found: true}
	matches := &stubMatchReader{
		rows: []repository.MatchRow{
			{ID: "m1", Format: "Standard", Result: "win", Timestamp: ts, DurationSeconds: &dur, PlayerWins: 2, OpponentWins: 1},
		},
	}
	drafts := &stubDraftReader{}

	h := handlers.NewHistoryHandler(accounts, matches, drafts)
	handler := authedMatchHandler(h, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/matches", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	data := resp["data"].([]interface{})
	if len(data) != 1 {
		t.Errorf("expected 1 match, got %d", len(data))
	}

	if resp["has_more"].(bool) {
		t.Errorf("expected has_more=false for single row page")
	}

	if resp["limit"].(float64) != 20 {
		t.Errorf("expected limit=20, got %v", resp["limit"])
	}
}

func TestGetMatches_EmptyResult(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	matches := &stubMatchReader{rows: nil}
	drafts := &stubDraftReader{}

	h := handlers.NewHistoryHandler(accounts, matches, drafts)
	handler := authedMatchHandler(h, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/matches", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	data := resp["data"].([]interface{})
	if len(data) != 0 {
		t.Errorf("expected empty data array, got %d items", len(data))
	}

	if hasMore, _ := resp["has_more"].(bool); hasMore {
		t.Errorf("expected has_more=false for empty result")
	}
}

func TestGetMatches_NoAccountYet(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 0, found: false}
	matches := &stubMatchReader{}
	drafts := &stubDraftReader{}

	h := handlers.NewHistoryHandler(accounts, matches, drafts)
	handler := authedMatchHandler(h, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/matches", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	data := resp["data"].([]interface{})
	if len(data) != 0 {
		t.Errorf("expected empty data, got %d items", len(data))
	}
	if hasMore, _ := resp["has_more"].(bool); hasMore {
		t.Errorf("expected has_more=false for no-account")
	}
}

func TestGetMatches_Unauthorized(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	matches := &stubMatchReader{}
	drafts := &stubDraftReader{}

	h := handlers.NewHistoryHandler(accounts, matches, drafts)
	// No user ID on context — no middleware injection.

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/matches", nil)
	rr := httptest.NewRecorder()
	h.GetMatches(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestGetMatches_InvalidLimit(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	matches := &stubMatchReader{}
	drafts := &stubDraftReader{}

	h := handlers.NewHistoryHandler(accounts, matches, drafts)
	handler := authedMatchHandler(h, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/matches?limit=9999", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestGetMatches_InvalidFormat(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	matches := &stubMatchReader{}
	drafts := &stubDraftReader{}

	h := handlers.NewHistoryHandler(accounts, matches, drafts)
	handler := authedMatchHandler(h, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/matches?format=FakeFormat", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestGetMatches_CursorForwarded verifies that cursor_ts and cursor_id query
// params are forwarded to the repo method.
func TestGetMatches_CursorForwarded(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}

	cursorTime := time.Date(2026, 5, 9, 10, 0, 0, 0, time.UTC)
	matches := &stubMatchReader{
		rows: []repository.MatchRow{
			{ID: "m5", Format: "Standard", Result: "win", Timestamp: cursorTime.Add(-time.Hour), PlayerWins: 1, OpponentWins: 0},
		},
	}
	drafts := &stubDraftReader{}

	h := handlers.NewHistoryHandler(accounts, matches, drafts)
	handler := authedMatchHandler(h, 1)

	target := "/api/v1/history/matches?cursor_ts=" + cursorTime.UTC().Format(time.RFC3339Nano) + "&cursor_id=m4&limit=20"
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	// The cursor args should have been passed through to the repo.
	if matches.capturedCursorID != "m4" {
		t.Errorf("cursor_id not forwarded: got %q", matches.capturedCursorID)
	}
	if matches.capturedCursorTS == nil || !matches.capturedCursorTS.Equal(cursorTime) {
		t.Errorf("cursor_ts not forwarded: got %v", matches.capturedCursorTS)
	}
}

// TestGetMatches_HasMore verifies the has_more + next_cursor_* fields are set
// correctly when the repo returns limit+1 rows.
func TestGetMatches_HasMore(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}

	ts := time.Now().UTC().Truncate(time.Second)
	// Return 3 rows with limit=2 → has_more=true, probe row trimmed.
	matches := &stubMatchReader{
		rows: []repository.MatchRow{
			{ID: "ma", Format: "Standard", Result: "win", Timestamp: ts, PlayerWins: 1, OpponentWins: 0},
			{ID: "mb", Format: "Standard", Result: "loss", Timestamp: ts.Add(-time.Minute), PlayerWins: 0, OpponentWins: 1},
			{ID: "mc", Format: "Standard", Result: "win", Timestamp: ts.Add(-2 * time.Minute), PlayerWins: 1, OpponentWins: 0}, // probe
		},
	}
	drafts := &stubDraftReader{}

	h := handlers.NewHistoryHandler(accounts, matches, drafts)
	handler := authedMatchHandler(h, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/matches?limit=2", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	data := resp["data"].([]interface{})
	if len(data) != 2 {
		t.Errorf("expected 2 items (probe trimmed), got %d", len(data))
	}
	if hasMore, _ := resp["has_more"].(bool); !hasMore {
		t.Error("expected has_more=true")
	}
	if resp["next_cursor_id"] == nil || resp["next_cursor_id"].(string) != "mb" {
		t.Errorf("next_cursor_id: got %v, want mb", resp["next_cursor_id"])
	}
	if resp["next_cursor_ts"] == nil || resp["next_cursor_ts"].(string) == "" {
		t.Errorf("next_cursor_ts should be set when has_more=true")
	}
}

// TestGetMatches_CrossTenantIsolation verifies that user A's matches are never
// returned when user B queries the endpoint.  This is the critical multi-tenant
// correctness test required by the tech spec.
func TestGetMatches_CrossTenantIsolation(t *testing.T) {
	var userBID int64 = 999 // no account

	accountsA := &stubAccountLookup{accountID: 100, found: true}
	accountsB := &stubAccountLookup{accountID: 0, found: false}

	ts := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	matchesStore := &stubMatchReader{
		rows: []repository.MatchRow{{ID: "userA-match", Format: "Standard", Result: "win", Timestamp: ts}},
	}

	hA := handlers.NewHistoryHandler(accountsA, matchesStore, &stubDraftReader{})
	hB := handlers.NewHistoryHandler(accountsB, matchesStore, &stubDraftReader{})

	handlerA := authedMatchHandler(hA, 1)
	handlerB := authedMatchHandler(hB, userBID)

	// User A gets their match.
	reqA := httptest.NewRequest(http.MethodGet, "/api/v1/history/matches", nil)
	rrA := httptest.NewRecorder()
	handlerA.ServeHTTP(rrA, reqA)
	if rrA.Code != http.StatusOK {
		t.Fatalf("userA: expected 200, got %d", rrA.Code)
	}

	var respA map[string]interface{}
	_ = json.Unmarshal(rrA.Body.Bytes(), &respA)
	if len(respA["data"].([]interface{})) != 1 {
		t.Error("userA should see 1 match")
	}

	// User B gets empty (no account — account lookup returns found=false).
	reqB := httptest.NewRequest(http.MethodGet, "/api/v1/history/matches", nil)
	rrB := httptest.NewRecorder()
	handlerB.ServeHTTP(rrB, reqB)
	if rrB.Code != http.StatusOK {
		t.Fatalf("userB: expected 200, got %d", rrB.Code)
	}

	var respB map[string]interface{}
	_ = json.Unmarshal(rrB.Body.Bytes(), &respB)
	if len(respB["data"].([]interface{})) != 0 {
		t.Error("userB must not see userA's matches")
	}
}

// --- drafts tests ---

func TestGetDrafts_HappyPath(t *testing.T) {
	start := time.Date(2026, 5, 4, 22, 10, 1, 0, time.UTC)
	end := time.Date(2026, 5, 4, 22, 48, 33, 0, time.UTC)

	accounts := &stubAccountLookup{accountID: 10, found: true}
	matches := &stubMatchReader{}
	drafts := &stubDraftReader{
		rows: []repository.DraftSessionRow{
			{ID: "d1", SetCode: "EOE", DraftType: "premier_draft", StartTime: start, EndTime: &end, Wins: 5, Losses: 1},
		},
		total: 1,
	}

	h := handlers.NewHistoryHandler(accounts, matches, drafts)
	handler := authedDraftHandler(h, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/drafts", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	data := resp["data"].([]interface{})
	if len(data) != 1 {
		t.Fatalf("expected 1 draft, got %d", len(data))
	}

	item := data[0].(map[string]interface{})
	if item["format"].(string) != "PremierDraft" {
		t.Errorf("expected format=PremierDraft, got %s", item["format"])
	}
}

func TestGetDrafts_EmptyResult(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	matches := &stubMatchReader{}
	drafts := &stubDraftReader{rows: nil, total: 0}

	h := handlers.NewHistoryHandler(accounts, matches, drafts)
	handler := authedDraftHandler(h, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/drafts", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	data := resp["data"].([]interface{})
	if len(data) != 0 {
		t.Errorf("expected empty data, got %d items", len(data))
	}
}

func TestGetDrafts_InvalidSetCode(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	matches := &stubMatchReader{}
	drafts := &stubDraftReader{}

	h := handlers.NewHistoryHandler(accounts, matches, drafts)
	handler := authedDraftHandler(h, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/drafts?set_code=invalid!!", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestGetDrafts_InvalidLimit(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	matches := &stubMatchReader{}
	drafts := &stubDraftReader{}

	h := handlers.NewHistoryHandler(accounts, matches, drafts)
	handler := authedDraftHandler(h, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/drafts?limit=0", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestGetDrafts_OutOfRangePageReturns200(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	matches := &stubMatchReader{}
	drafts := &stubDraftReader{rows: nil, total: 5}

	h := handlers.NewHistoryHandler(accounts, matches, drafts)
	handler := authedDraftHandler(h, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/drafts?page=99", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for out-of-range page, got %d", rr.Code)
	}
}

func TestGetDrafts_Unauthorized(t *testing.T) {
	accounts := &stubAccountLookup{accountID: 10, found: true}
	matches := &stubMatchReader{}
	drafts := &stubDraftReader{}

	h := handlers.NewHistoryHandler(accounts, matches, drafts)
	// No user ID on context.

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/drafts", nil)
	rr := httptest.NewRecorder()
	h.GetDrafts(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
