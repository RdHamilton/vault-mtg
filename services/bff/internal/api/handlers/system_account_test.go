package handlers_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/handlers"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/middleware"
	"github.com/RdHamilton/vault-mtg/services/bff/internal/storage/repository"
)

// stubAccountReader is a test double for systemAccountReader.
type stubAccountReader struct {
	row *repository.AccountRow
	err error
}

func (s *stubAccountReader) GetByUserID(_ context.Context, _ int64) (*repository.AccountRow, bool, error) {
	if s.err != nil {
		return nil, false, s.err
	}
	if s.row == nil {
		return nil, false, nil
	}
	return s.row, true, nil
}

// authedSystemAccountHandler injects userID into the request context and
// delegates to GetSystemAccount.
func authedSystemAccountHandler(h *handlers.SystemAccountHandler, userID int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.WithUserID(r.Context(), userID)
		r = r.WithContext(ctx)
		h.GetSystemAccount(w, r)
	})
}

func TestGetSystemAccount_OK(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	stub := &stubAccountReader{row: &repository.AccountRow{
		ID:           7,
		Name:         "Ramone",
		ScreenName:   sql.NullString{String: "ArenaHandle", Valid: true},
		ClientID:     sql.NullString{String: "MTGA_client_1", Valid: true},
		IsDefault:    1,
		DailyWins:    3,
		WeeklyWins:   7,
		MasteryLevel: 4,
		MasteryPass:  sql.NullString{String: "Standard", Valid: true},
		MasteryMax:   80,
		CreatedAt:    now,
		UpdatedAt:    now,
	}}

	h := handlers.NewSystemAccountHandler(stub)
	handler := authedSystemAccountHandler(h, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/account", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", ct)
	}

	// Decode the {"data": {...}} envelope.
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	data := envelope.Data

	// Verify PascalCase keys and values — these mirror models.Account exactly.
	checkField := func(key string, want any) {
		t.Helper()
		got, ok := data[key]
		if !ok {
			t.Errorf("missing key %q in response", key)
			return
		}
		// JSON numbers decode as float64.
		switch w := want.(type) {
		case int:
			if gf, ok := got.(float64); !ok || int(gf) != w {
				t.Errorf("key %q: want %v, got %v", key, w, got)
			}
		case bool:
			if gv, ok := got.(bool); !ok || gv != w {
				t.Errorf("key %q: want %v, got %v", key, w, got)
			}
		case string:
			if gv, ok := got.(string); !ok || gv != w {
				t.Errorf("key %q: want %q, got %v", key, w, got)
			}
		}
	}

	checkField("ID", 7)
	checkField("Name", "Ramone")
	checkField("ScreenName", "ArenaHandle")
	checkField("ClientID", "MTGA_client_1")
	checkField("IsDefault", true)
	checkField("DailyWins", 3)
	checkField("WeeklyWins", 7)
	checkField("MasteryLevel", 4)
	checkField("MasteryPass", "Standard")
	checkField("MasteryMax", 80)

	// Verify snake_case keys are NOT present — they would silently zero SPA fields.
	for _, bad := range []string{
		"id", "name", "screen_name", "client_id", "is_default",
		"daily_wins", "weekly_wins", "mastery_level", "mastery_pass", "mastery_max",
		"created_at", "updated_at",
	} {
		if _, found := data[bad]; found {
			t.Errorf("unexpected snake_case key %q in response — would silently zero SPA fields", bad)
		}
	}
}

func TestGetSystemAccount_NotFound(t *testing.T) {
	// No account row — first-run state.
	stub := &stubAccountReader{row: nil}

	h := handlers.NewSystemAccountHandler(stub)
	handler := authedSystemAccountHandler(h, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/account", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["error"] == "" {
		t.Error("expected non-empty error field in 404 response")
	}
}

func TestGetSystemAccount_Unauthorized_MissingContext(t *testing.T) {
	// No user ID in context — !ok branch.
	stub := &stubAccountReader{}
	h := handlers.NewSystemAccountHandler(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/account", nil)
	rr := httptest.NewRecorder()
	h.GetSystemAccount(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 (!ok branch), got %d", rr.Code)
	}
}

func TestGetSystemAccount_Unauthorized_ZeroUserID(t *testing.T) {
	// User ID is 0 (auth not set) — !ok || userID == 0 branch.
	stub := &stubAccountReader{}
	h := handlers.NewSystemAccountHandler(stub)
	handler := authedSystemAccountHandler(h, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/account", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 (userID==0 branch), got %d", rr.Code)
	}
}

func TestGetSystemAccount_InternalError(t *testing.T) {
	stub := &stubAccountReader{err: sql.ErrConnDone}
	h := handlers.NewSystemAccountHandler(stub)
	handler := authedSystemAccountHandler(h, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/account", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}
