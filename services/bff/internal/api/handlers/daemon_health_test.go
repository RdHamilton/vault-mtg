package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/api/handlers"
	"github.com/ramonehamilton/mtga-bff/internal/api/middleware"
)

// stubDaemonHealthChecker is a test double for DaemonHealthChecker.
type stubDaemonHealthChecker struct {
	connected bool
	err       error
}

func (s *stubDaemonHealthChecker) HasRecentEventByUserID(_ context.Context, _ int64, _ time.Duration) (bool, error) {
	return s.connected, s.err
}

// authedHealthHandler injects userID into context and delegates to GetDaemonHealth.
func authedHealthHandler(h *handlers.DaemonHealthHandler, userID int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.WithUserID(r.Context(), userID)
		r = r.WithContext(ctx)
		h.GetDaemonHealth(w, r)
	})
}

func TestGetDaemonHealth_Connected(t *testing.T) {
	checker := &stubDaemonHealthChecker{connected: true}
	h := handlers.NewDaemonHealthHandler(checker)
	handler := authedHealthHandler(h, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/daemon", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp["status"] != "connected" {
		t.Errorf("expected status=connected, got %q", resp["status"])
	}
}

func TestGetDaemonHealth_Disconnected(t *testing.T) {
	checker := &stubDaemonHealthChecker{connected: false}
	h := handlers.NewDaemonHealthHandler(checker)
	handler := authedHealthHandler(h, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/daemon", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp["status"] != "disconnected" {
		t.Errorf("expected status=disconnected, got %q", resp["status"])
	}
}

func TestGetDaemonHealth_DBError_ReturnsDisconnected(t *testing.T) {
	// When the DB errors out we still return 200 with "disconnected" — the
	// frontend should degrade gracefully and not show a hard error.
	checker := &stubDaemonHealthChecker{err: errors.New("db unavailable")}
	h := handlers.NewDaemonHealthHandler(checker)
	handler := authedHealthHandler(h, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/daemon", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp["status"] != "disconnected" {
		t.Errorf("expected status=disconnected on DB error, got %q", resp["status"])
	}
}

func TestGetDaemonHealth_Unauthorized(t *testing.T) {
	checker := &stubDaemonHealthChecker{connected: true}
	h := handlers.NewDaemonHealthHandler(checker)
	// No user ID injected — simulate missing auth.

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/daemon", nil)
	rr := httptest.NewRecorder()
	h.GetDaemonHealth(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestGetDaemonHealth_ContentType(t *testing.T) {
	checker := &stubDaemonHealthChecker{connected: true}
	h := handlers.NewDaemonHealthHandler(checker)
	handler := authedHealthHandler(h, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/daemon", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type=application/json, got %q", ct)
	}
}
