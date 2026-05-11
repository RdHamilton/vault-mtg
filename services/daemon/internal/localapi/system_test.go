package localapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-daemon/internal/localapi"
)

// startTestServer spins up a Server on an ephemeral port with a baseline
// State suitable for happy-path assertions. Callers can override fields via
// the optional mutator.
func startTestServer(t *testing.T, mut func(*localapi.State)) *localapi.Server {
	t.Helper()
	started := time.Date(2026, 5, 11, 21, 0, 0, 0, time.UTC)
	state := localapi.State{
		Version:      "0.3.1-rc18",
		SessionID:    "live-test-session",
		StartedAt:    started,
		AccountID:    "user_abc",
		CloudAPIURL:  "https://staging-api.vaultmtg.app/api/v1",
		BFFReachable: true,
	}
	if mut != nil {
		mut(&state)
	}
	srv := localapi.New(0, state)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop() })
	return srv
}

func getJSON(t *testing.T, srv *localapi.Server, path string, out any) *http.Response {
	t.Helper()
	resp, err := http.Get("http://" + srv.Addr() + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status: got %d, want 200", path, resp.StatusCode)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
	}
	return resp
}

func TestSystemStatusConnected(t *testing.T) {
	srv := startTestServer(t, nil)
	var body struct {
		Status    string `json:"status"`
		Connected bool   `json:"connected"`
		Mode      string `json:"mode"`
		URL       string `json:"url"`
		Port      int    `json:"port"`
	}
	getJSON(t, srv, "/api/v1/system/status", &body)
	if body.Status != "connected" || !body.Connected {
		t.Errorf("expected connected, got %+v", body)
	}
	if body.URL != "https://staging-api.vaultmtg.app/api/v1" {
		t.Errorf("url: got %q", body.URL)
	}
	if body.Port != 9001 {
		t.Errorf("port: got %d", body.Port)
	}
}

func TestSystemStatusDegradedWhenBFFUnreachable(t *testing.T) {
	srv := startTestServer(t, func(s *localapi.State) { s.BFFReachable = false })
	var body struct {
		Status    string `json:"status"`
		Connected bool   `json:"connected"`
	}
	getJSON(t, srv, "/api/v1/system/status", &body)
	if body.Status != "degraded" {
		t.Errorf("status: got %q, want degraded", body.Status)
	}
}

func TestSystemDaemonStatus(t *testing.T) {
	srv := startTestServer(t, nil)
	var body struct {
		Status    string `json:"status"`
		Connected bool   `json:"connected"`
	}
	getJSON(t, srv, "/api/v1/system/daemon/status", &body)
	if !body.Connected || body.Status != "connected" {
		t.Errorf("daemon status: %+v", body)
	}
}

func TestSystemVersion(t *testing.T) {
	srv := startTestServer(t, nil)
	var body struct {
		Version string `json:"version"`
		Service string `json:"service"`
	}
	getJSON(t, srv, "/api/v1/system/version", &body)
	if body.Version != "0.3.1-rc18" {
		t.Errorf("version: %q", body.Version)
	}
	if body.Service != "vaultmtg-daemon" {
		t.Errorf("service: %q", body.Service)
	}
}

func TestSystemHealthIncludesLastDispatch(t *testing.T) {
	dispatch := time.Date(2026, 5, 11, 21, 5, 0, 0, time.UTC)
	srv := startTestServer(t, func(s *localapi.State) { s.LastDispatchAt = &dispatch })

	var body struct {
		Status     string `json:"status"`
		Version    string `json:"version"`
		Uptime     int64  `json:"uptime"`
		LogMonitor struct {
			Status   string `json:"status"`
			LastRead string `json:"lastRead"`
		} `json:"logMonitor"`
	}
	getJSON(t, srv, "/api/v1/system/health", &body)
	if body.Status != "ok" {
		t.Errorf("status: %q", body.Status)
	}
	if body.LogMonitor.LastRead != "2026-05-11T21:05:00Z" {
		t.Errorf("lastRead: %q", body.LogMonitor.LastRead)
	}
}

func TestSystemAccountStubShape(t *testing.T) {
	srv := startTestServer(t, nil)
	var body struct {
		ID        int    `json:"ID"`
		Name      string `json:"Name"`
		IsDefault bool   `json:"IsDefault"`
	}
	getJSON(t, srv, "/api/v1/system/account", &body)
	if !body.IsDefault {
		t.Errorf("expected IsDefault=true on stub, got %+v", body)
	}
}

func TestSystemDatabasePathEmpty(t *testing.T) {
	srv := startTestServer(t, nil)
	var body struct {
		Path string `json:"path"`
	}
	getJSON(t, srv, "/api/v1/system/database/path", &body)
	if body.Path != "" {
		t.Errorf("expected empty path, got %q", body.Path)
	}
}

func TestSystemDaemonConnectAndDisconnect(t *testing.T) {
	srv := startTestServer(t, nil)
	for _, path := range []string{"/api/v1/system/daemon/connect", "/api/v1/system/daemon/disconnect"} {
		resp, err := http.Post("http://"+srv.Addr()+path, "application/json", nil)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("POST %s status: %d", path, resp.StatusCode)
		}
		var body struct {
			Status string `json:"status"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		_ = resp.Body.Close()
		if body.Status != "ok" {
			t.Errorf("POST %s body: %+v", path, body)
		}
	}
}

func TestSetStateUpdatesEndpoints(t *testing.T) {
	srv := startTestServer(t, nil)

	// Mutate published state and re-fetch /version.
	srv.SetState(localapi.State{
		Version:     "0.3.1-rc19",
		SessionID:   "live-new-session",
		StartedAt:   time.Now().UTC(),
		CloudAPIURL: "https://api.vaultmtg.app/api/v1",
	})

	var body struct {
		Version string `json:"version"`
	}
	getJSON(t, srv, "/api/v1/system/version", &body)
	if body.Version != "0.3.1-rc19" {
		t.Errorf("version after SetState: %q", body.Version)
	}
}
