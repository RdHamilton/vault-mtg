package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/RdHamilton/vault-mtg/services/bff/internal/api/handlers"
)

// pingFake is a minimal Pinger fake used by healthz tests.
// pingErr controls whether PingContext returns an error.
// pingCalls counts how many times PingContext was called.
type pingFake struct {
	pingErr   error
	pingCalls atomic.Int64
}

func (f *pingFake) PingContext(_ context.Context) error {
	f.pingCalls.Add(1)
	return f.pingErr
}

// --- 3 new tests (written first, TDD) ---

// TestHealthzHandler_PingerCalledExactlyOnce verifies that each ServeHTTP
// invocation calls PingContext exactly once.
func TestHealthzHandler_PingerCalledExactlyOnce(t *testing.T) {
	pinger := &pingFake{}
	h := handlers.NewHealthzHandler("staging", pinger, "42")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if got := pinger.pingCalls.Load(); got != 1 {
		t.Errorf("PingContext call count: want 1, got %d", got)
	}
}

// TestHealthzHandler_NilDB_ReturnsUnknown verifies that when no *sql.DB is
// provided (development mode), migration_version is "unknown" and the
// handler still returns 200.
func TestHealthzHandler_NilDB_ReturnsUnknown(t *testing.T) {
	h := handlers.NewHealthzHandler("development", nil, "42")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body struct {
		MigrationVersion string `json:"migration_version"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.MigrationVersion != "unknown" {
		t.Errorf("migration_version: want %q, got %q", "unknown", body.MigrationVersion)
	}
}

// TestHealthzHandler_PingFailure_ReturnsUnknown verifies that a ping error
// degrades migration_version to "unknown" while the status code stays 200.
func TestHealthzHandler_PingFailure_ReturnsUnknown(t *testing.T) {
	pinger := &pingFake{pingErr: errors.New("connection refused")}
	h := handlers.NewHealthzHandler("production", pinger, "42")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 even on ping failure, got %d", w.Code)
	}

	var body struct {
		MigrationVersion string `json:"migration_version"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.MigrationVersion != "unknown" {
		t.Errorf("migration_version: want %q, got %q", "unknown", body.MigrationVersion)
	}
}

// --- 6 existing tests updated to use Pinger fake ---

func TestHealthzHandler_Returns200WithCorrectEnv(t *testing.T) {
	pinger := &pingFake{}
	h := handlers.NewHealthzHandler("staging", pinger, "42")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body struct {
		Status           string `json:"status"`
		Env              string `json:"env"`
		MigrationVersion string `json:"migration_version"`
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

	if body.MigrationVersion != "42" {
		t.Errorf("expected migration_version %q, got %q", "42", body.MigrationVersion)
	}
}

func TestHealthzHandler_Returns200WithUnknownMigrationsWhenDBUnreachable(t *testing.T) {
	// Simulates a DB that is down: ping returns error.
	pinger := &pingFake{pingErr: errors.New("unreachable")}
	h := handlers.NewHealthzHandler("staging", pinger, "42")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 even when DB unreachable, got %d", w.Code)
	}

	var body struct {
		MigrationVersion string `json:"migration_version"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.MigrationVersion != "unknown" {
		t.Errorf("expected migration_version %q, got %q", "unknown", body.MigrationVersion)
	}
}

// TestHealthzHandler_NoAuthHeaderRequired verifies the handler works without
// any Authorization header — it must be mounted outside the auth middleware
// group.
func TestHealthzHandler_NoAuthHeaderRequired(t *testing.T) {
	pinger := &pingFake{}
	h := handlers.NewHealthzHandler("production", pinger, "42")

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
	pinger := &pingFake{}
	h := handlers.NewHealthzHandler("production", pinger, "42")

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

// TestHealthzHandler_ResponseContainsMigrationVersionField is an integration-
// style test that exercises the full handler and asserts the response JSON
// contains a "migration_version" key (not the old "migrations" key).
func TestHealthzHandler_ResponseContainsMigrationVersionField(t *testing.T) {
	const wantVersion = "42"

	pinger := &pingFake{}
	h := handlers.NewHealthzHandler("staging", pinger, wantVersion)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Decode into a generic map so we can assert key presence explicitly.
	var raw map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	// "migration_version" must be present.
	val, ok := raw["migration_version"]
	if !ok {
		t.Fatalf("response JSON missing 'migration_version' key; got keys: %v", keysOf(raw))
	}
	if val != wantVersion {
		t.Errorf("migration_version: want %q, got %q", wantVersion, val)
	}

	// "migrations" must NOT be present (old field name removed).
	if _, found := raw["migrations"]; found {
		t.Errorf("response JSON still contains old 'migrations' key — must be removed")
	}

	// Verify remaining shape.
	if raw["status"] != "ok" {
		t.Errorf("status: want 'ok', got %q", raw["status"])
	}
	if raw["env"] != "staging" {
		t.Errorf("env: want 'staging', got %q", raw["env"])
	}
}

// TestHealthzHandler_ContentTypeIsJSON asserts the handler sets the correct
// Content-Type header.
func TestHealthzHandler_ContentTypeIsJSON(t *testing.T) {
	pinger := &pingFake{}
	h := handlers.NewHealthzHandler("staging", pinger, "42")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: want 'application/json', got %q", ct)
	}
}

// keysOf returns the keys of a map as a slice, for use in error messages.
func keysOf(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
