package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	"github.com/ramonehamilton/mtga-bff/internal/storage"
)

func TestHealthzHandler_Returns200WithCorrectEnv(t *testing.T) {
	checker := func(_ string) string { return storage.MigrationStatusUpToDate }
	h := handlers.NewHealthzHandler("staging", "postgres://localhost/test", checker)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body struct {
		Status     string `json:"status"`
		Env        string `json:"env"`
		Migrations string `json:"migrations"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", body.Status)
	}

	if body.Env != "staging" {
		t.Errorf("expected env 'staging', got %q", body.Env)
	}

	if body.Migrations != storage.MigrationStatusUpToDate {
		t.Errorf("expected migrations %q, got %q", storage.MigrationStatusUpToDate, body.Migrations)
	}
}

func TestHealthzHandler_Returns200WithUnknownMigrationsWhenDBUnreachable(t *testing.T) {
	// Simulates a DB that is down: checker returns "unknown".
	checker := func(_ string) string { return storage.MigrationStatusUnknown }
	h := handlers.NewHealthzHandler("staging", "postgres://unreachable/db", checker)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 even when DB unreachable, got %d", w.Code)
	}

	var body struct {
		Migrations string `json:"migrations"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Migrations != storage.MigrationStatusUnknown {
		t.Errorf("expected migrations %q, got %q", storage.MigrationStatusUnknown, body.Migrations)
	}
}

// TestHealthzHandler_NoAuthHeaderRequired verifies the handler works without
// any Authorization header — it must be mounted outside the auth middleware
// group.
func TestHealthzHandler_NoAuthHeaderRequired(t *testing.T) {
	checker := func(_ string) string { return storage.MigrationStatusUpToDate }
	h := handlers.NewHealthzHandler("production", "", checker)

	// No Authorization header set — should still return 200.
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 without auth header, got %d", w.Code)
	}
}

// TestHealthzHandler_ProductionEnvInResponse verifies the env field reflects
// what was passed to NewHealthzHandler.
func TestHealthzHandler_ProductionEnvInResponse(t *testing.T) {
	checker := func(_ string) string { return storage.MigrationStatusUpToDate }
	h := handlers.NewHealthzHandler("production", "", checker)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	var body struct {
		Env string `json:"env"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Env != "production" {
		t.Errorf("expected env 'production', got %q", body.Env)
	}
}
