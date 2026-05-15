package localapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-daemon/internal/localapi"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// postJSON sends a POST request to srv at path with the given JSON body and
// returns the *http.Response.  The caller must close the body.
func postJSON(t *testing.T, srv *localapi.Server, path string, body any) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp, err := http.Post("http://"+srv.Addr()+path, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestReplay_NoTrigger verifies that POST /api/v1/replay returns 503 when no
// ReplayFunc has been registered, indicating the daemon is not fully
// initialised.
func TestReplay_NoTrigger(t *testing.T) {
	srv := startTestServer(t, nil)
	// Do NOT call srv.SetReplayTrigger — leave it nil.

	resp := postJSON(t, srv, "/api/v1/replay", map[string]any{"clearDataFirst": false})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503", resp.StatusCode)
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error == "" {
		t.Errorf("expected non-empty error field in 503 response")
	}
}

// TestReplay_HappyPath verifies that POST /api/v1/replay returns 202 Accepted
// when a ReplayFunc is registered and fires the trigger asynchronously.  The
// test uses an atomic bool to confirm the trigger was actually called.
func TestReplay_HappyPath(t *testing.T) {
	var triggered atomic.Bool

	srv := startTestServer(t, nil)
	srv.SetReplayTrigger(func(_ context.Context, clearDataFirst bool) {
		triggered.Store(true)
	})

	resp := postJSON(t, srv, "/api/v1/replay", map[string]any{"clearDataFirst": false})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status: got %d, want 202", resp.StatusCode)
	}

	var body struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "accepted" {
		t.Errorf("status field: got %q, want %q", body.Status, "accepted")
	}
	if body.Message == "" {
		t.Errorf("expected non-empty message field")
	}

	// Wait briefly for the goroutine to be scheduled before asserting.
	deadline := time.Now().Add(100 * time.Millisecond)
	for !triggered.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !triggered.Load() {
		t.Error("replay trigger was not called within 100ms")
	}
}

// TestReplay_ClearDataFirstForwarded verifies that the clearDataFirst boolean
// from the request body is correctly passed to the ReplayFunc.
func TestReplay_ClearDataFirstForwarded(t *testing.T) {
	var gotClearDataFirst atomic.Bool
	gotClearDataFirst.Store(false)
	var called atomic.Bool

	srv := startTestServer(t, nil)
	srv.SetReplayTrigger(func(_ context.Context, clearDataFirst bool) {
		gotClearDataFirst.Store(clearDataFirst)
		called.Store(true)
	})

	resp := postJSON(t, srv, "/api/v1/replay", map[string]any{"clearDataFirst": true})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status: got %d, want 202", resp.StatusCode)
	}

	// Wait for the goroutine.
	deadline := time.Now().Add(100 * time.Millisecond)
	for !called.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !gotClearDataFirst.Load() {
		t.Error("clearDataFirst=true was not forwarded to the ReplayFunc")
	}
}

// TestReplay_MethodNotAllowed verifies that GET /api/v1/replay returns 405 —
// the endpoint is POST-only.
func TestReplay_MethodNotAllowed(t *testing.T) {
	srv := startTestServer(t, nil)

	resp, err := http.Get("http://" + srv.Addr() + "/api/v1/replay")
	if err != nil {
		t.Fatalf("GET /api/v1/replay: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", resp.StatusCode)
	}
}

// TestReplay_EmptyBody verifies that POST /api/v1/replay with no body (Content-
// Length == 0) is accepted — clearDataFirst defaults to false.
func TestReplay_EmptyBody(t *testing.T) {
	var triggered atomic.Bool
	srv := startTestServer(t, nil)
	srv.SetReplayTrigger(func(_ context.Context, _ bool) {
		triggered.Store(true)
	})

	resp, err := http.Post("http://"+srv.Addr()+"/api/v1/replay", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /api/v1/replay (empty body): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("status: got %d, want 202", resp.StatusCode)
	}
}

// TestReplay_InvalidJSON verifies that POST /api/v1/replay with a non-empty but
// invalid JSON body returns 400 Bad Request.
func TestReplay_InvalidJSON(t *testing.T) {
	srv := startTestServer(t, nil)
	srv.SetReplayTrigger(func(_ context.Context, _ bool) {})

	resp, err := http.Post(
		"http://"+srv.Addr()+"/api/v1/replay",
		"application/json",
		bytes.NewBufferString("not-valid-json"),
	)
	if err != nil {
		t.Fatalf("POST /api/v1/replay (invalid json): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

// TestReplay_CORSPreflight verifies that OPTIONS /api/v1/replay gets 204 with
// the correct CORS headers — matching the pattern used by other local API
// endpoints that the SPA calls cross-origin from the browser.
func TestReplay_CORSPreflight(t *testing.T) {
	srv := startTestServer(t, nil)

	req, err := http.NewRequest(http.MethodOptions, "http://"+srv.Addr()+"/api/v1/replay", nil)
	if err != nil {
		t.Fatalf("build OPTIONS request: %v", err)
	}
	req.Header.Set("Origin", "https://app.vaultmtg.app")
	req.Header.Set("Access-Control-Request-Method", "POST")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS /api/v1/replay: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("preflight status: got %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://app.vaultmtg.app" {
		t.Errorf("Allow-Origin: got %q, want %q", got, "https://app.vaultmtg.app")
	}
}
